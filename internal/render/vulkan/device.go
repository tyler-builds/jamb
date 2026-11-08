//go:build linux && (amd64 || arm64)

// Package vulkan is the Vulkan backend for render.Renderer, for Linux.
//
// It is pure Go: libvulkan is loaded at run time with purego, and the shaders
// ship as SPIR-V compiled from the GLSL in shaders/ (see shaders/compile.sh).
// The platform adapter speaks Wayland itself, without libwayland, so Vulkan's
// window-system integration (VK_KHR_wayland_surface) cannot be used: frames
// are drawn into images exported as dma-bufs, which the adapter hands to the
// compositor (see package dmabuf).
package vulkan

import (
	"embed"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"time"
	"unsafe"

	"github.com/tyler-builds/jamb/internal/dmabuf"
)

// The SPIR-V is committed, so building needs no shader compiler. The shaders
// workflow (.github/workflows/shaders.yml) compiles it for every pull request
// that changes a shader; go generate does the same where glslc is installed.
//
//go:generate sh shaders/compile.sh
//go:embed shaders/spv/*.spv
var spirv embed.FS

type pipelineKind uint8

const (
	pipeQuad pipelineKind = iota
	pipeMask
	pipeImage
	pipeSubpixel // ClearType glyphs: mask instances, dual-source blending
	pipeCount
)

// modifier is a DRM format modifier the device can render B8G8R8A8 into and
// export as a dma-buf.
type modifier struct {
	mod    uint64
	planes uint32
	// copy is set when the image can also be copied from, which
	// CaptureNextFrame needs.
	copy bool
}

// hostBuffer is a buffer in host-visible memory, mapped for its whole life.
type hostBuffer struct {
	buf, mem uint64
	data     unsafe.Pointer
	size     int
}

// Device is the Vulkan device and the GPU state every window shares:
// pipelines, samplers, the instance and staging buffers, and textures. Create
// one per process with NewDevice; all windows render from the UI thread, one
// at a time.
type Device struct {
	instance uintptr
	vki      instanceFuncs
	// prefer is the device to choose when several qualify: the compositor's.
	prefer    dmabuf.Device
	hasPrefer bool

	phys       uintptr
	name       string
	drm        []dmabuf.Device // the device's primary and render nodes, when known
	family     uint32
	foreign    bool // VK_EXT_queue_family_foreign is enabled
	formatList bool // VK_KHR_image_format_list is enabled
	// linearCopy is set when the device can copy frames into linear images it
	// exports, for a compositor on another GPU.
	linearCopy bool
	dualSource bool
	memProps   physicalDeviceMemoryProperties
	modifiers  []modifier

	dev   uintptr
	vk    deviceFuncs
	queue uintptr

	renderPass  uint64
	samplerSets uint64 // set layout 0: the two samplers
	textureSets uint64 // set layout 1: one texture
	pipeLayout  uint64
	samplers    [2]uint64 // point, linear
	samplerPool uint64
	samplerSet  uint64
	pipes       [pipeCount]uint64
	texPools    []uint64

	cmdPool uint64
	cmd     uintptr
	fence   uint64

	instances hostBuffer
	staging   hostBuffer

	// textures are uploaded once per device, so a glyph atlas or image is
	// shared by every window that draws it. After Recover the map is empty and
	// each texture is uploaded again from its scene.Texture.Pix.
	textures  map[uint64]*gpuTexture
	lastSweep time.Time

	renderers []*Renderer
}

// NewDevice loads Vulkan and creates the device and the pipelines every
// window shares. Call it once; NewRenderer adds a window to it.
func NewDevice() (*Device, error) {
	if err := loadLibrary(); err != nil {
		return nil, err
	}
	d := &Device{textures: map[uint64]*gpuTexture{}}
	if err := d.createInstance(); err != nil {
		return nil, err
	}
	if err := d.create(); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// Adapter names the GPU the device runs on, for diagnostics.
func (d *Device) Adapter() string { return d.name }

// OnDevice reports whether the device runs on dev, the compositor's device.
func (d *Device) OnDevice(dev dmabuf.Device) bool { return slices.Contains(d.drm, dev) }

// Prefer makes dev the device that Recover chooses when it qualifies: the GPU
// the compositor composites with, so frames are not copied between GPUs.
func (d *Device) Prefer(dev dmabuf.Device) { d.prefer, d.hasPrefer = dev, true }

func (d *Device) createInstance() error {
	app := applicationInfo{sType: stApplicationInfo, pApplicationName: cstr("jamb"), pEngineName: cstr("jamb"),
		apiVersion: vkMakeVersion(1, 1)}
	info := instanceCreateInfo{sType: stInstanceCreateInfo, pApplicationInfo: &app}
	fn := instanceProc(0, "vkCreateInstance")
	if fn == 0 {
		return errors.New("vulkan: the loader lacks vkCreateInstance")
	}
	if err := check("vkCreateInstance", call(fn, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&d.instance)))); err != nil {
		return err
	}
	return loadFuncs(&d.vki, func(name string) uintptr { return instanceProc(d.instance, name) })
}

// create chooses a physical device and creates the device and every object
// it owns.
func (d *Device) create() error {
	if err := d.choosePhysical(); err != nil {
		return err
	}
	if err := d.createDevice(); err != nil {
		return err
	}
	if err := d.createPipelines(); err != nil {
		return err
	}
	return d.createFrameState()
}

// Recover recreates the device and every object it owns after a loss (or after
// ReleaseDevice). Textures are uploaded again the next time a frame draws
// them; each window then calls Renderer.Recreate.
func (d *Device) Recover() error {
	d.release()
	return d.create()
}

// ReleaseDevice releases the device and every object it owns, as a loss
// leaves them. It is the simulation hook for the devicelost script step.
func (d *Device) ReleaseDevice() { d.release() }

// candidate is a physical device that can run the renderer.
type candidate struct {
	phys       uintptr
	name       string
	typ        uint32
	drm        []dmabuf.Device
	family     uint32
	foreign    bool
	formatList bool // VK_KHR_image_format_list is offered
	dualSource bool
	modifiers  []modifier
	linearCopy bool
}

func (d *Device) choosePhysical() error {
	var n uint32
	if err := check("vkEnumeratePhysicalDevices", call(d.vki.EnumeratePhysicalDevices, d.instance, uintptr(unsafe.Pointer(&n)), 0)); err != nil {
		return err
	}
	if n == 0 {
		return errNoDevice
	}
	devs := make([]uintptr, n)
	if err := check("vkEnumeratePhysicalDevices", call(d.vki.EnumeratePhysicalDevices, d.instance, uintptr(unsafe.Pointer(&n)), uintptr(unsafe.Pointer(&devs[0])))); err != nil {
		return err
	}
	var best *candidate
	bestScore := -1
	var reasons []string
	for _, p := range devs[:n] {
		c, err := d.inspect(p)
		if err != nil {
			reasons = append(reasons, err.Error())
			continue
		}
		score := map[uint32]int{physicalDeviceTypeDiscrete: 3, physicalDeviceTypeIntegrated: 2, physicalDeviceTypeVirtual: 1}[c.typ]
		if d.hasPrefer && slices.Contains(c.drm, d.prefer) {
			score += 10
		}
		if score > bestScore {
			best, bestScore = c, score
		}
	}
	if best == nil {
		if len(reasons) > 0 {
			return fmt.Errorf("%w: %s", errNoDevice, reasons[0])
		}
		return errNoDevice
	}
	d.phys, d.name, d.drm, d.family = best.phys, best.name, best.drm, best.family
	d.foreign, d.dualSource, d.modifiers = best.foreign, best.dualSource, best.modifiers
	callVoid(d.vki.GetPhysicalDeviceMemoryProperties, d.phys, uintptr(unsafe.Pointer(&d.memProps)))
	d.formatList, d.linearCopy = best.formatList, best.linearCopy
	return nil
}

var requiredExtensions = []string{
	"VK_KHR_external_memory_fd",
	"VK_EXT_external_memory_dma_buf",
	"VK_EXT_image_drm_format_modifier",
}

// inspect reports whether a physical device can run the renderer, and how.
func (d *Device) inspect(p uintptr) (*candidate, error) {
	exts, err := d.deviceExtensions(p)
	if err != nil {
		return nil, err
	}
	c := &candidate{phys: p}
	drm := physicalDeviceDrmProperties{sType: stPhysicalDeviceDrmProperties}
	props := physicalDeviceProperties2{sType: stPhysicalDeviceProperties2}
	hasDrm := exts["VK_EXT_physical_device_drm"]
	if hasDrm {
		props.pNext = unsafe.Pointer(&drm)
	}
	callVoid(d.vki.GetPhysicalDeviceProperties2, p, uintptr(unsafe.Pointer(&props)))
	c.name = gostr(props.properties.deviceName[:])
	c.typ = props.properties.deviceType
	if c.typ == physicalDeviceTypeCPU {
		// A software rasterizer is no faster than the CPU renderer.
		return nil, fmt.Errorf("%s is a software rasterizer", c.name)
	}
	if props.properties.apiVersion < vkMakeVersion(1, 1) {
		return nil, fmt.Errorf("%s supports only Vulkan 1.0", c.name)
	}
	for _, e := range requiredExtensions {
		if !exts[e] {
			return nil, fmt.Errorf("%s lacks %s", c.name, e)
		}
	}
	c.foreign = exts["VK_EXT_queue_family_foreign"]
	c.formatList = exts["VK_KHR_image_format_list"]
	if hasDrm {
		if drm.hasPrimary != 0 {
			c.drm = append(c.drm, dmabuf.Device{Major: uint32(drm.primaryMajor), Minor: uint32(drm.primaryMinor)})
		}
		if drm.hasRender != 0 {
			c.drm = append(c.drm, dmabuf.Device{Major: uint32(drm.renderMajor), Minor: uint32(drm.renderMinor)})
		}
	}

	var nq uint32
	callVoid(d.vki.GetPhysicalDeviceQueueFamilyProperties, p, uintptr(unsafe.Pointer(&nq)), 0)
	qs := make([]queueFamilyProperties, nq)
	if nq > 0 {
		callVoid(d.vki.GetPhysicalDeviceQueueFamilyProperties, p, uintptr(unsafe.Pointer(&nq)), uintptr(unsafe.Pointer(&qs[0])))
	}
	c.family = ^uint32(0)
	for i, q := range qs[:nq] {
		if q.queueFlags&queueGraphics != 0 && q.queueCount > 0 {
			c.family = uint32(i)
			break
		}
	}
	if c.family == ^uint32(0) {
		return nil, fmt.Errorf("%s has no graphics queue", c.name)
	}

	var features physicalDeviceFeatures
	callVoid(d.vki.GetPhysicalDeviceFeatures, p, uintptr(unsafe.Pointer(&features)))
	c.dualSource = features[featureDualSrcBlend] != 0

	c.modifiers = d.renderModifiers(p)
	c.linearCopy = d.linearCopyTarget(p)
	if len(c.modifiers) == 0 {
		return nil, fmt.Errorf("%s cannot export a B8G8R8A8 render target as a dma-buf", c.name)
	}
	return c, nil
}

func (d *Device) deviceExtensions(p uintptr) (map[string]bool, error) {
	var n uint32
	if err := check("vkEnumerateDeviceExtensionProperties", call(d.vki.EnumerateDeviceExtensionProperties, p, 0, uintptr(unsafe.Pointer(&n)), 0)); err != nil {
		return nil, err
	}
	props := make([]extensionProperties, n)
	if n > 0 {
		if err := check("vkEnumerateDeviceExtensionProperties", call(d.vki.EnumerateDeviceExtensionProperties, p, 0, uintptr(unsafe.Pointer(&n)), uintptr(unsafe.Pointer(&props[0])))); err != nil {
			return nil, err
		}
	}
	exts := map[string]bool{}
	for _, e := range props[:n] {
		exts[gostr(e.extensionName[:])] = true
	}
	return exts, nil
}

// renderModifiers lists the modifiers in which the device can render
// B8G8R8A8 with blending and export the image as a dma-buf.
func (d *Device) renderModifiers(p uintptr) []modifier {
	list := drmFormatModifierPropertiesList{sType: stDrmFormatModifierPropertiesList}
	fp := formatProperties2{sType: stFormatProperties2, pNext: unsafe.Pointer(&list)}
	callVoid(d.vki.GetPhysicalDeviceFormatProperties2, p, formatB8G8R8A8Unorm, uintptr(unsafe.Pointer(&fp)))
	if list.drmFormatModifierCount == 0 {
		return nil
	}
	props := make([]drmFormatModifierProperties, list.drmFormatModifierCount)
	list.pDrmFormatModifierProperties = &props[0]
	callVoid(d.vki.GetPhysicalDeviceFormatProperties2, p, formatB8G8R8A8Unorm, uintptr(unsafe.Pointer(&fp)))
	const need = formatFeatureColorAttachment | formatFeatureColorAttachmentBlend
	var mods []modifier
	for _, m := range props[:list.drmFormatModifierCount] {
		if m.drmFormatModifierTilingFeatures&need != need || m.drmFormatModifierPlaneCount == 0 || m.drmFormatModifierPlaneCount > 4 {
			continue
		}
		switch {
		case d.exportable(p, m.drmFormatModifier, imageUsageColorAttachment|imageUsageTransferSrc):
			mods = append(mods, modifier{mod: m.drmFormatModifier, planes: m.drmFormatModifierPlaneCount, copy: true})
		case d.exportable(p, m.drmFormatModifier, imageUsageColorAttachment):
			mods = append(mods, modifier{mod: m.drmFormatModifier, planes: m.drmFormatModifierPlaneCount})
		}
	}
	return mods
}

// linearCopyTarget reports whether the device can copy into a linear B8G8R8A8
// image and export it as a dma-buf, which is how frames reach a compositor on
// another GPU (see placement).
func (d *Device) linearCopyTarget(p uintptr) bool {
	list := drmFormatModifierPropertiesList{sType: stDrmFormatModifierPropertiesList}
	fp := formatProperties2{sType: stFormatProperties2, pNext: unsafe.Pointer(&list)}
	callVoid(d.vki.GetPhysicalDeviceFormatProperties2, p, formatB8G8R8A8Unorm, uintptr(unsafe.Pointer(&fp)))
	if list.drmFormatModifierCount == 0 {
		return false
	}
	props := make([]drmFormatModifierProperties, list.drmFormatModifierCount)
	list.pDrmFormatModifierProperties = &props[0]
	callVoid(d.vki.GetPhysicalDeviceFormatProperties2, p, formatB8G8R8A8Unorm, uintptr(unsafe.Pointer(&fp)))
	for _, m := range props[:list.drmFormatModifierCount] {
		if m.drmFormatModifier == dmabuf.ModLinear && m.drmFormatModifierTilingFeatures&formatFeatureTransferDst != 0 {
			return d.exportable(p, dmabuf.ModLinear, imageUsageTransferDst)
		}
	}
	return false
}

// exportable reports whether a B8G8R8A8 image with modifier mod and the usage
// can be created and exported as a dma-buf.
func (d *Device) exportable(p uintptr, mod uint64, usage uint32) bool {
	drmInfo := physicalDeviceImageDrmFormatModifierInfo{sType: stPhysicalDeviceImageDrmFormatModifierInfo, drmFormatModifier: mod, sharingMode: sharingExclusive}
	ext := physicalDeviceExternalImageFormatInfo{sType: stPhysicalDeviceExternalImageFormatInfo, pNext: unsafe.Pointer(&drmInfo), handleType: externalMemoryDmaBuf}
	info := physicalDeviceImageFormatInfo2{sType: stPhysicalDeviceImageFormatInfo2, pNext: unsafe.Pointer(&ext),
		format: formatB8G8R8A8Unorm, typ: imageType2D, tiling: imageTilingDrmFormatModifier, usage: usage}
	extProps := externalImageFormatProperties{sType: stExternalImageFormatProperties}
	props := imageFormatProperties2{sType: stImageFormatProperties2, pNext: unsafe.Pointer(&extProps)}
	r := call(d.vki.GetPhysicalDeviceImageFormatProperties2, p, uintptr(unsafe.Pointer(&info)), uintptr(unsafe.Pointer(&props)))
	return r == vkSuccess && extProps.externalMemoryFeatures&externalMemoryExportable != 0
}

func (d *Device) createDevice() error {
	priority := float32(1)
	q := deviceQueueCreateInfo{sType: stDeviceQueueCreateInfo, queueFamilyIndex: d.family, queueCount: 1, pQueuePriorities: &priority}
	exts := slices.Clone(requiredExtensions)
	if d.foreign {
		exts = append(exts, "VK_EXT_queue_family_foreign")
	}
	if d.formatList {
		// Required by VK_EXT_image_drm_format_modifier before Vulkan 1.2.
		exts = append(exts, "VK_KHR_image_format_list")
	}
	var features physicalDeviceFeatures
	if d.dualSource {
		features[featureDualSrcBlend] = 1
	}
	info := deviceCreateInfo{sType: stDeviceCreateInfo, queueCreateInfoCount: 1, pQueueCreateInfos: &q,
		enabledExtensionCount: uint32(len(exts)), ppEnabledExtensionNames: cstrs(exts), pEnabledFeatures: &features}
	if err := check("vkCreateDevice", call(d.vki.CreateDevice, d.phys, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&d.dev)))); err != nil {
		return err
	}
	err := loadFuncs(&d.vk, func(name string) uintptr {
		return procAddr(d.vki.GetDeviceProcAddr, d.dev, name)
	})
	if err != nil {
		return err
	}
	callVoid(d.vk.GetDeviceQueue, d.dev, uintptr(d.family), 0, uintptr(unsafe.Pointer(&d.queue)))
	return nil
}

// shaderModule creates a shader module from an embedded SPIR-V file.
func (d *Device) shaderModule(name string) (uint64, error) {
	b, err := spirv.ReadFile("shaders/spv/" + name + ".spv")
	if err != nil {
		return 0, fmt.Errorf("vulkan: shader %s: %w", name, err)
	}
	code := make([]uint32, len(b)/4)
	for i := range code {
		code[i] = binary.LittleEndian.Uint32(b[i*4:])
	}
	info := shaderModuleCreateInfo{sType: stShaderModuleCreateInfo, codeSize: uintptr(len(code) * 4), pCode: &code[0]}
	var m uint64
	if err := check("vkCreateShaderModule("+name+")", call(d.vk.CreateShaderModule, d.dev, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&m)))); err != nil {
		return 0, err
	}
	return m, nil
}

func (d *Device) createPipelines() error {
	// Render pass: one B8G8R8A8 color attachment, cleared, left for the
	// barriers of EndFrame.
	att := attachmentDescription{format: formatB8G8R8A8Unorm, samples: sampleCount1, loadOp: loadOpClear, storeOp: storeOpStore,
		stencilLoadOp: loadOpDontCare, stencilStoreOp: storeOpDontCare, initialLayout: layoutUndefined, finalLayout: layoutColorAttachmentOptimal}
	ref := attachmentReference{attachment: 0, layout: layoutColorAttachmentOptimal}
	sub := subpassDescription{pipelineBindPoint: bindPointGraphics, colorAttachmentCount: 1, pColorAttachments: &ref}
	dep := subpassDependency{srcSubpass: subpassExternal, dstSubpass: 0,
		srcStageMask: stageColorAttachmentOutput, dstStageMask: stageColorAttachmentOutput, dstAccessMask: accessColorAttachmentWrite}
	rp := renderPassCreateInfo{sType: stRenderPassCreateInfo, attachmentCount: 1, pAttachments: &att, subpassCount: 1, pSubpasses: &sub,
		dependencyCount: 1, pDependencies: &dep}
	if err := check("vkCreateRenderPass", call(d.vk.CreateRenderPass, d.dev, uintptr(unsafe.Pointer(&rp)), 0, uintptr(unsafe.Pointer(&d.renderPass)))); err != nil {
		return err
	}

	// Samplers: point for masks, linear for images. They are immutable in set 0.
	for i, filter := range []uint32{filterNearest, filterLinear} {
		si := samplerCreateInfo{sType: stSamplerCreateInfo, magFilter: filter, minFilter: filter,
			addressModeU: addressClampToEdge, addressModeV: addressClampToEdge, addressModeW: addressClampToEdge, maxLod: 0.25}
		if err := check("vkCreateSampler", call(d.vk.CreateSampler, d.dev, uintptr(unsafe.Pointer(&si)), 0, uintptr(unsafe.Pointer(&d.samplers[i])))); err != nil {
			return err
		}
	}
	samplerBindings := []descriptorSetLayoutBinding{
		{binding: 0, descriptorType: descriptorTypeSampler, descriptorCount: 1, stageFlags: shaderStageFragment, pImmutableSamplers: &d.samplers[0]},
		{binding: 1, descriptorType: descriptorTypeSampler, descriptorCount: 1, stageFlags: shaderStageFragment, pImmutableSamplers: &d.samplers[1]},
	}
	sl := descriptorSetLayoutCreateInfo{sType: stDescriptorSetLayoutCreateInfo, bindingCount: 2, pBindings: &samplerBindings[0]}
	if err := check("vkCreateDescriptorSetLayout", call(d.vk.CreateDescriptorSetLayout, d.dev, uintptr(unsafe.Pointer(&sl)), 0, uintptr(unsafe.Pointer(&d.samplerSets)))); err != nil {
		return err
	}
	texBinding := descriptorSetLayoutBinding{binding: 0, descriptorType: descriptorTypeSampledImage, descriptorCount: 1, stageFlags: shaderStageFragment}
	tl := descriptorSetLayoutCreateInfo{sType: stDescriptorSetLayoutCreateInfo, bindingCount: 1, pBindings: &texBinding}
	if err := check("vkCreateDescriptorSetLayout", call(d.vk.CreateDescriptorSetLayout, d.dev, uintptr(unsafe.Pointer(&tl)), 0, uintptr(unsafe.Pointer(&d.textureSets)))); err != nil {
		return err
	}
	layouts := []uint64{d.samplerSets, d.textureSets}
	push := pushConstantRange{stageFlags: shaderStageVertex | shaderStageFragment, size: pushSize}
	pl := pipelineLayoutCreateInfo{sType: stPipelineLayoutCreateInfo, setLayoutCount: 2, pSetLayouts: &layouts[0], pushConstantRangeCount: 1, pPushConstantRanges: &push}
	if err := check("vkCreatePipelineLayout", call(d.vk.CreatePipelineLayout, d.dev, uintptr(unsafe.Pointer(&pl)), 0, uintptr(unsafe.Pointer(&d.pipeLayout)))); err != nil {
		return err
	}

	// The sampler set, allocated once.
	size := descriptorPoolSize{typ: descriptorTypeSampler, descriptorCount: 2}
	pool := descriptorPoolCreateInfo{sType: stDescriptorPoolCreateInfo, maxSets: 1, poolSizeCount: 1, pPoolSizes: &size}
	if err := check("vkCreateDescriptorPool", call(d.vk.CreateDescriptorPool, d.dev, uintptr(unsafe.Pointer(&pool)), 0, uintptr(unsafe.Pointer(&d.samplerPool)))); err != nil {
		return err
	}
	alloc := descriptorSetAllocateInfo{sType: stDescriptorSetAllocateInfo, descriptorPool: d.samplerPool, descriptorSetCount: 1, pSetLayouts: &d.samplerSets}
	if err := check("vkAllocateDescriptorSets", call(d.vk.AllocateDescriptorSets, d.dev, uintptr(unsafe.Pointer(&alloc)), uintptr(unsafe.Pointer(&d.samplerSet)))); err != nil {
		return err
	}

	type spec struct {
		kind       pipelineKind
		vs, fs     string
		attributes int // vec4 per instance
		dual       bool
	}
	subFS, dual := "subpixel_gray.frag", false
	if d.dualSource {
		subFS, dual = "subpixel.frag", true
	}
	specs := []spec{
		{pipeQuad, "quad.vert", "quad.frag", int(unsafe.Sizeof(quadInstance{}) / 16), false},
		{pipeMask, "mask.vert", "mask.frag", int(unsafe.Sizeof(maskInstance{}) / 16), false},
		{pipeImage, "image.vert", "image.frag", int(unsafe.Sizeof(imageInstance{}) / 16), false},
		{pipeSubpixel, "mask.vert", subFS, int(unsafe.Sizeof(maskInstance{}) / 16), dual},
	}
	for _, s := range specs {
		p, err := d.createPipeline(s.vs, s.fs, s.attributes, s.dual)
		if err != nil {
			return err
		}
		d.pipes[s.kind] = p
	}
	return nil
}

// pushSize is the size of the push constants: the viewport and the subpixel
// text parameters, two vec4s.
const pushSize = 32

func (d *Device) createPipeline(vsName, fsName string, attributes int, dual bool) (uint64, error) {
	vs, err := d.shaderModule(vsName)
	if err != nil {
		return 0, err
	}
	defer callVoid(d.vk.DestroyShaderModule, d.dev, uintptr(vs), 0)
	fs, err := d.shaderModule(fsName)
	if err != nil {
		return 0, err
	}
	defer callVoid(d.vk.DestroyShaderModule, d.dev, uintptr(fs), 0)
	main := cstr("main")
	stages := []pipelineShaderStageCreateInfo{
		{sType: stPipelineShaderStageCreateInfo, stage: shaderStageVertex, module: vs, pName: main},
		{sType: stPipelineShaderStageCreateInfo, stage: shaderStageFragment, module: fs, pName: main},
	}
	binding := vertexInputBindingDescription{binding: 0, stride: uint32(attributes * 16), inputRate: vertexInputRateInstance}
	attrs := make([]vertexInputAttributeDescription, attributes)
	for i := range attrs {
		attrs[i] = vertexInputAttributeDescription{location: uint32(i), format: formatR32G32B32A32Sflt, offset: uint32(i * 16)}
	}
	vi := pipelineVertexInputStateCreateInfo{sType: stPipelineVertexInputStateCreateInfo, vertexBindingDescriptionCount: 1, pVertexBindingDescriptions: &binding,
		vertexAttributeDescriptionCount: uint32(attributes), pVertexAttributeDescriptions: &attrs[0]}
	ia := pipelineInputAssemblyStateCreateInfo{sType: stPipelineInputAssemblyCreateInfo, topology: topologyTriangleStrip}
	vp := pipelineViewportStateCreateInfo{sType: stPipelineViewportStateCreateInfo, viewportCount: 1, scissorCount: 1}
	rs := pipelineRasterizationStateCreateInfo{sType: stPipelineRasterizationCreateInfo, lineWidth: 1}
	ms := pipelineMultisampleStateCreateInfo{sType: stPipelineMultisampleCreateInfo, rasterizationSamples: sampleCount1}
	// Premultiplied alpha: dst = src + dst * (1 - src.a). The dual-source state
	// of subpixel glyphs takes each channel's coverage from the second output:
	// dst = src0 + dst * (1 - src1).
	blend := pipelineColorBlendAttachmentState{blendEnable: 1,
		srcColorBlendFactor: blendOne, dstColorBlendFactor: blendOneMinusSrcAlpha,
		srcAlphaBlendFactor: blendOne, dstAlphaBlendFactor: blendOneMinusSrcAlpha, colorWriteMask: 0xf}
	if dual {
		blend.dstColorBlendFactor, blend.dstAlphaBlendFactor = blendOneMinusSrc1Color, blendOneMinusSrc1Alpha
	}
	cb := pipelineColorBlendStateCreateInfo{sType: stPipelineColorBlendStateCreateInfo, attachmentCount: 1, pAttachments: &blend}
	dyn := []uint32{dynamicStateViewport, dynamicStateScissor}
	ds := pipelineDynamicStateCreateInfo{sType: stPipelineDynamicStateCreateInfo, dynamicStateCount: 2, pDynamicStates: &dyn[0]}
	info := graphicsPipelineCreateInfo{sType: stGraphicsPipelineCreateInfo, stageCount: 2, pStages: &stages[0],
		pVertexInputState: &vi, pInputAssemblyState: &ia, pViewportState: &vp, pRasterizationState: &rs,
		pMultisampleState: &ms, pColorBlendState: &cb, pDynamicState: &ds, layout: d.pipeLayout, renderPass: d.renderPass,
		basePipelineIndex: -1}
	var p uint64
	if err := check("vkCreateGraphicsPipelines("+fsName+")", call(d.vk.CreateGraphicsPipelines, d.dev, 0, 1, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&p)))); err != nil {
		return 0, err
	}
	return p, nil
}

// createFrameState creates the command buffer and fence that every frame
// reuses. A frame waits for its fence before it returns, so one of each is
// enough, and the instance and staging buffers can be rewritten each frame.
func (d *Device) createFrameState() error {
	pi := commandPoolCreateInfo{sType: stCommandPoolCreateInfo, flags: commandPoolResetBuffer, queueFamilyIndex: d.family}
	if err := check("vkCreateCommandPool", call(d.vk.CreateCommandPool, d.dev, uintptr(unsafe.Pointer(&pi)), 0, uintptr(unsafe.Pointer(&d.cmdPool)))); err != nil {
		return err
	}
	ai := commandBufferAllocateInfo{sType: stCommandBufferAllocateInfo, commandPool: d.cmdPool, commandBufferCount: 1}
	if err := check("vkAllocateCommandBuffers", call(d.vk.AllocateCommandBuffers, d.dev, uintptr(unsafe.Pointer(&ai)), uintptr(unsafe.Pointer(&d.cmd)))); err != nil {
		return err
	}
	fi := fenceCreateInfo{sType: stFenceCreateInfo}
	return check("vkCreateFence", call(d.vk.CreateFence, d.dev, uintptr(unsafe.Pointer(&fi)), 0, uintptr(unsafe.Pointer(&d.fence))))
}

// memoryType returns a memory type allowed by bits with the properties, or
// with as many of them as possible when want is not all available.
func (d *Device) memoryType(bits, want uint32) (uint32, bool) {
	for i := range d.memProps.memoryTypeCount {
		if bits&(1<<i) != 0 && d.memProps.memoryTypes[i].propertyFlags&want == want {
			return i, true
		}
	}
	return 0, false
}

// ensureHost makes b a host-visible buffer of at least size bytes. A frame's
// earlier contents are lost when it grows; the previous frame has finished
// with it, since each frame waits for its fence.
func (d *Device) ensureHost(b *hostBuffer, size int, usage uint32) error {
	if b.size >= size {
		return nil
	}
	d.freeHost(b)
	capacity := max(size, 64*1024)
	for capacity < size*3/2 {
		capacity *= 2
	}
	bi := bufferCreateInfo{sType: stBufferCreateInfo, size: uint64(capacity), usage: usage, sharingMode: sharingExclusive}
	if err := check("vkCreateBuffer", call(d.vk.CreateBuffer, d.dev, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&b.buf)))); err != nil {
		return err
	}
	var req memoryRequirements
	callVoid(d.vk.GetBufferMemoryRequirements, d.dev, uintptr(b.buf), uintptr(unsafe.Pointer(&req)))
	t, ok := d.memoryType(req.memoryTypeBits, memoryPropertyHostVisible|memoryPropertyHostCoherent)
	if !ok {
		d.freeHost(b)
		return errors.New("vulkan: no host-visible coherent memory")
	}
	mi := memoryAllocateInfo{sType: stMemoryAllocateInfo, allocationSize: req.size, memoryTypeIndex: t}
	if err := check("vkAllocateMemory(host)", call(d.vk.AllocateMemory, d.dev, uintptr(unsafe.Pointer(&mi)), 0, uintptr(unsafe.Pointer(&b.mem)))); err != nil {
		d.freeHost(b)
		return err
	}
	if err := check("vkBindBufferMemory", call(d.vk.BindBufferMemory, d.dev, uintptr(b.buf), uintptr(b.mem), 0)); err != nil {
		d.freeHost(b)
		return err
	}
	var p uintptr
	if err := check("vkMapMemory", call(d.vk.MapMemory, d.dev, uintptr(b.mem), 0, uintptr(capacity), 0, uintptr(unsafe.Pointer(&p)))); err != nil {
		d.freeHost(b)
		return err
	}
	b.data, b.size = ptr(p), capacity
	return nil
}

func (d *Device) freeHost(b *hostBuffer) {
	if b.buf != 0 {
		callVoid(d.vk.DestroyBuffer, d.dev, uintptr(b.buf), 0)
	}
	if b.mem != 0 {
		callVoid(d.vk.FreeMemory, d.dev, uintptr(b.mem), 0) // unmaps it
	}
	*b = hostBuffer{}
}

// Close releases the device and the instance. The windows rendering with it
// must be closed first.
func (d *Device) Close() {
	d.release()
	if d.instance != 0 {
		callVoid(d.vki.DestroyInstance, d.instance, 0)
	}
	*d = Device{}
}

// release releases the device and every object it owns, including the
// windows' render targets, and leaves the Device with an empty texture map.
// It accepts a Device whose creation failed part way or that was already
// released.
func (d *Device) release() {
	if d.dev == 0 {
		d.textures = map[uint64]*gpuTexture{}
		return
	}
	callVoid(d.vk.DeviceWaitIdle, d.dev) // an error is a loss; release anyway
	for _, r := range d.renderers {
		r.dropTargets()
	}
	for _, t := range d.textures {
		d.freeTexture(t)
	}
	for _, p := range d.texPools {
		callVoid(d.vk.DestroyDescriptorPool, d.dev, uintptr(p), 0)
	}
	d.freeHost(&d.instances)
	d.freeHost(&d.staging)
	for _, p := range d.pipes {
		if p != 0 {
			callVoid(d.vk.DestroyPipeline, d.dev, uintptr(p), 0)
		}
	}
	destroy := []struct {
		fn uintptr
		h  uint64
	}{
		{d.vk.DestroyFence, d.fence},
		{d.vk.DestroyCommandPool, d.cmdPool},
		{d.vk.DestroyDescriptorPool, d.samplerPool},
		{d.vk.DestroyPipelineLayout, d.pipeLayout},
		{d.vk.DestroyDescriptorSetLayout, d.textureSets},
		{d.vk.DestroyDescriptorSetLayout, d.samplerSets},
		{d.vk.DestroySampler, d.samplers[0]},
		{d.vk.DestroySampler, d.samplers[1]},
		{d.vk.DestroyRenderPass, d.renderPass},
	}
	for _, o := range destroy {
		if o.h != 0 {
			callVoid(o.fn, d.dev, uintptr(o.h), 0)
		}
	}
	callVoid(d.vk.DestroyDevice, d.dev, 0)

	keep := Device{instance: d.instance, vki: d.vki, prefer: d.prefer, hasPrefer: d.hasPrefer, renderers: d.renderers}
	*d = keep
	d.textures = map[uint64]*gpuTexture{}
}
