//go:build linux && (amd64 || arm64)

package vulkan

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/tyler-builds/jamb/internal/render"
)

// The Vulkan API, loaded from libvulkan.so.1 with purego: only the commands
// and structures this renderer uses. Structures are declared with the field
// types of the C headers, which Go lays out the same way on 64-bit targets.
// Dispatchable handles (instance, physical device, device, queue, command
// buffer) are pointers, uintptr here; the other handles are uint64.

type vkResult int32

const (
	vkSuccess                vkResult = 0
	vkTimeout                vkResult = 2
	vkIncomplete             vkResult = 5
	vkErrorDeviceLost        vkResult = -4
	vkErrorOutOfPoolMemory   vkResult = -1000069000
	vkErrorFragmentedPool    vkResult = -12
	vkErrorOutOfHostMemory   vkResult = -1
	vkErrorOutOfDeviceMemory vkResult = -2
)

func (r vkResult) String() string {
	switch r {
	case vkErrorOutOfHostMemory:
		return "VK_ERROR_OUT_OF_HOST_MEMORY"
	case vkErrorOutOfDeviceMemory:
		return "VK_ERROR_OUT_OF_DEVICE_MEMORY"
	case -3:
		return "VK_ERROR_INITIALIZATION_FAILED"
	case vkErrorDeviceLost:
		return "VK_ERROR_DEVICE_LOST"
	case -6:
		return "VK_ERROR_LAYER_NOT_PRESENT"
	case -7:
		return "VK_ERROR_EXTENSION_NOT_PRESENT"
	case -8:
		return "VK_ERROR_FEATURE_NOT_PRESENT"
	case -9:
		return "VK_ERROR_INCOMPATIBLE_DRIVER"
	case -11:
		return "VK_ERROR_FORMAT_NOT_SUPPORTED"
	case vkTimeout:
		return "VK_TIMEOUT"
	}
	return fmt.Sprintf("VkResult(%d)", int32(r))
}

// check turns a failed result into an error; a lost device wraps
// render.ErrDeviceLost.
func check(what string, r vkResult) error {
	if r >= 0 {
		return nil
	}
	if r == vkErrorDeviceLost {
		return fmt.Errorf("%w: vulkan: %s: %v", render.ErrDeviceLost, what, r)
	}
	return fmt.Errorf("vulkan: %s: %v", what, r)
}

const (
	stApplicationInfo                    = 0
	stInstanceCreateInfo                 = 1
	stDeviceQueueCreateInfo              = 2
	stDeviceCreateInfo                   = 3
	stSubmitInfo                         = 4
	stMemoryAllocateInfo                 = 5
	stFenceCreateInfo                    = 8
	stBufferCreateInfo                   = 12
	stImageCreateInfo                    = 14
	stImageViewCreateInfo                = 15
	stShaderModuleCreateInfo             = 16
	stPipelineShaderStageCreateInfo      = 18
	stPipelineVertexInputStateCreateInfo = 19
	stPipelineInputAssemblyCreateInfo    = 20
	stPipelineViewportStateCreateInfo    = 22
	stPipelineRasterizationCreateInfo    = 23
	stPipelineMultisampleCreateInfo      = 24
	stPipelineColorBlendStateCreateInfo  = 26
	stPipelineDynamicStateCreateInfo     = 27
	stGraphicsPipelineCreateInfo         = 28
	stPipelineLayoutCreateInfo           = 30
	stSamplerCreateInfo                  = 31
	stDescriptorSetLayoutCreateInfo      = 32
	stDescriptorPoolCreateInfo           = 33
	stDescriptorSetAllocateInfo          = 34
	stWriteDescriptorSet                 = 35
	stFramebufferCreateInfo              = 37
	stRenderPassCreateInfo               = 38
	stCommandPoolCreateInfo              = 39
	stCommandBufferAllocateInfo          = 40
	stCommandBufferBeginInfo             = 42
	stRenderPassBeginInfo                = 43
	stImageMemoryBarrier                 = 45

	stPhysicalDeviceProperties2                = 1000059001
	stFormatProperties2                        = 1000059002
	stImageFormatProperties2                   = 1000059003
	stPhysicalDeviceImageFormatInfo2           = 1000059004
	stPhysicalDeviceExternalImageFormatInfo    = 1000071000
	stExternalImageFormatProperties            = 1000071001
	stExternalMemoryImageCreateInfo            = 1000072001
	stExportMemoryAllocateInfo                 = 1000072002
	stMemoryGetFdInfo                          = 1000074002
	stMemoryDedicatedAllocateInfo              = 1000127001
	stDrmFormatModifierPropertiesList          = 1000158000
	stPhysicalDeviceImageDrmFormatModifierInfo = 1000158002
	stImageDrmFormatModifierListCreateInfo     = 1000158003
	stImageDrmFormatModifierProperties         = 1000158005
	stPhysicalDeviceDrmProperties              = 1000353000
)

const (
	formatR8Unorm          = 9
	formatR8G8B8A8Unorm    = 37
	formatB8G8R8A8Unorm    = 44
	formatR32G32B32A32Sflt = 109

	formatFeatureColorAttachment      = 0x80
	formatFeatureColorAttachmentBlend = 0x100
	formatFeatureTransferDst          = 0x8000

	imageUsageTransferSrc      = 0x1
	imageUsageTransferDst      = 0x2
	imageUsageSampled          = 0x4
	imageUsageColorAttachment  = 0x10
	bufferUsageTransferSrc     = 0x1
	bufferUsageTransferDst     = 0x2
	bufferUsageVertexBuffer    = 0x80
	memoryPropertyDeviceLocal  = 0x1
	memoryPropertyHostVisible  = 0x2
	memoryPropertyHostCoherent = 0x4

	imageTilingOptimal           = 0
	imageTilingDrmFormatModifier = 1000158000
	imageType2D                  = 1
	imageViewType2D              = 1
	sharingExclusive             = 0
	sampleCount1                 = 1
	aspectColor                  = 0x1
	aspectMemoryPlane0           = 0x80 // planes 1-3 follow at 0x100, 0x200, 0x400

	layoutUndefined              = 0
	layoutGeneral                = 1
	layoutColorAttachmentOptimal = 2
	layoutShaderReadOnlyOptimal  = 5
	layoutTransferSrcOptimal     = 6
	layoutTransferDstOptimal     = 7

	stageTopOfPipe             = 0x1
	stageFragmentShader        = 0x80
	stageColorAttachmentOutput = 0x400
	stageTransfer              = 0x1000
	stageBottomOfPipe          = 0x2000
	stageHost                  = 0x4000

	accessShaderRead           = 0x20
	accessColorAttachmentWrite = 0x100
	accessTransferRead         = 0x800
	accessTransferWrite        = 0x1000
	accessHostRead             = 0x2000

	queueGraphics            = 0x1
	queueFamilyIgnored       = ^uint32(0)
	queueFamilyForeign       = ^uint32(2) // VK_QUEUE_FAMILY_FOREIGN_EXT
	externalMemoryDmaBuf     = 0x200
	externalMemoryExportable = 0x2

	physicalDeviceTypeIntegrated = 1
	physicalDeviceTypeDiscrete   = 2
	physicalDeviceTypeVirtual    = 3
	physicalDeviceTypeCPU        = 4

	descriptorTypeSampler      = 0
	descriptorTypeSampledImage = 2
	shaderStageVertex          = 0x1
	shaderStageFragment        = 0x10

	blendZero              = 0
	blendOne               = 1
	blendOneMinusSrcAlpha  = 7
	blendOneMinusSrc1Color = 16
	blendOneMinusSrc1Alpha = 18

	topologyTriangleStrip   = 4
	vertexInputRateInstance = 1
	dynamicStateViewport    = 0
	dynamicStateScissor     = 1
	loadOpClear             = 1
	storeOpStore            = 0
	loadOpDontCare          = 2
	storeOpDontCare         = 1
	bindPointGraphics       = 0
	subpassInline           = 0
	subpassExternal         = ^uint32(0)

	commandPoolResetBuffer = 0x2
	commandBufferOneTime   = 0x1
	descriptorPoolFreeSet  = 0x1
	filterNearest          = 0
	filterLinear           = 1
	addressClampToEdge     = 2
)

func vkMakeVersion(major, minor uint32) uint32 { return major<<22 | minor<<12 }

type applicationInfo struct {
	sType              uint32
	pNext              unsafe.Pointer
	pApplicationName   *byte
	applicationVersion uint32
	pEngineName        *byte
	engineVersion      uint32
	apiVersion         uint32
}

type instanceCreateInfo struct {
	sType                   uint32
	pNext                   unsafe.Pointer
	flags                   uint32
	pApplicationInfo        *applicationInfo
	enabledLayerCount       uint32
	ppEnabledLayerNames     **byte
	enabledExtensionCount   uint32
	ppEnabledExtensionNames **byte
}

type extensionProperties struct {
	extensionName [256]byte
	specVersion   uint32
}

type physicalDeviceProperties struct {
	apiVersion    uint32
	driverVersion uint32
	vendorID      uint32
	deviceID      uint32
	deviceType    uint32
	deviceName    [256]byte
	uuid          [16]byte
	_             [536]byte // limits and sparse properties, unused
}

type physicalDeviceProperties2 struct {
	sType      uint32
	pNext      unsafe.Pointer
	properties physicalDeviceProperties
}

type physicalDeviceDrmProperties struct {
	sType        uint32
	pNext        unsafe.Pointer
	hasPrimary   uint32
	hasRender    uint32
	primaryMajor int64
	primaryMinor int64
	renderMajor  int64
	renderMinor  int64
}

type queueFamilyProperties struct {
	queueFlags         uint32
	queueCount         uint32
	timestampValidBits uint32
	granularity        [3]uint32
}

type physicalDeviceMemoryProperties struct {
	memoryTypeCount uint32
	memoryTypes     [32]struct{ propertyFlags, heapIndex uint32 }
	memoryHeapCount uint32
	memoryHeaps     [16]struct {
		size  uint64
		flags uint32
	}
}

// physicalDeviceFeatures is VkPhysicalDeviceFeatures: 55 VkBool32s.
type physicalDeviceFeatures [55]uint32

const featureDualSrcBlend = 7

type deviceQueueCreateInfo struct {
	sType            uint32
	pNext            unsafe.Pointer
	flags            uint32
	queueFamilyIndex uint32
	queueCount       uint32
	pQueuePriorities *float32
}

type deviceCreateInfo struct {
	sType                   uint32
	pNext                   unsafe.Pointer
	flags                   uint32
	queueCreateInfoCount    uint32
	pQueueCreateInfos       *deviceQueueCreateInfo
	enabledLayerCount       uint32
	ppEnabledLayerNames     **byte
	enabledExtensionCount   uint32
	ppEnabledExtensionNames **byte
	pEnabledFeatures        *physicalDeviceFeatures
}

type formatProperties2 struct {
	sType         uint32
	pNext         unsafe.Pointer
	linear        uint32
	optimal       uint32
	bufferFeature uint32
}

type drmFormatModifierPropertiesList struct {
	sType                        uint32
	pNext                        unsafe.Pointer
	drmFormatModifierCount       uint32
	pDrmFormatModifierProperties *drmFormatModifierProperties
}

type drmFormatModifierProperties struct {
	drmFormatModifier               uint64
	drmFormatModifierPlaneCount     uint32
	drmFormatModifierTilingFeatures uint32
}

type physicalDeviceImageFormatInfo2 struct {
	sType  uint32
	pNext  unsafe.Pointer
	format uint32
	typ    uint32
	tiling uint32
	usage  uint32
	flags  uint32
}

type physicalDeviceExternalImageFormatInfo struct {
	sType      uint32
	pNext      unsafe.Pointer
	handleType uint32
}

type physicalDeviceImageDrmFormatModifierInfo struct {
	sType                 uint32
	pNext                 unsafe.Pointer
	drmFormatModifier     uint64
	sharingMode           uint32
	queueFamilyIndexCount uint32
	pQueueFamilyIndices   *uint32
}

type imageFormatProperties2 struct {
	sType           uint32
	pNext           unsafe.Pointer
	maxExtent       [3]uint32
	maxMipLevels    uint32
	maxArrayLayers  uint32
	sampleCounts    uint32
	maxResourceSize uint64
}

type externalImageFormatProperties struct {
	sType                         uint32
	pNext                         unsafe.Pointer
	externalMemoryFeatures        uint32
	exportFromImportedHandleTypes uint32
	compatibleHandleTypes         uint32
}

type memoryAllocateInfo struct {
	sType           uint32
	pNext           unsafe.Pointer
	allocationSize  uint64
	memoryTypeIndex uint32
}

type exportMemoryAllocateInfo struct {
	sType       uint32
	pNext       unsafe.Pointer
	handleTypes uint32
}

type memoryDedicatedAllocateInfo struct {
	sType  uint32
	pNext  unsafe.Pointer
	image  uint64
	buffer uint64
}

type memoryGetFdInfo struct {
	sType      uint32
	pNext      unsafe.Pointer
	memory     uint64
	handleType uint32
}

type memoryRequirements struct {
	size           uint64
	alignment      uint64
	memoryTypeBits uint32
}

type bufferCreateInfo struct {
	sType                 uint32
	pNext                 unsafe.Pointer
	flags                 uint32
	size                  uint64
	usage                 uint32
	sharingMode           uint32
	queueFamilyIndexCount uint32
	pQueueFamilyIndices   *uint32
}

type extent3D struct{ width, height, depth uint32 }

type imageCreateInfo struct {
	sType                 uint32
	pNext                 unsafe.Pointer
	flags                 uint32
	imageType             uint32
	format                uint32
	extent                extent3D
	mipLevels             uint32
	arrayLayers           uint32
	samples               uint32
	tiling                uint32
	usage                 uint32
	sharingMode           uint32
	queueFamilyIndexCount uint32
	pQueueFamilyIndices   *uint32
	initialLayout         uint32
}

type externalMemoryImageCreateInfo struct {
	sType       uint32
	pNext       unsafe.Pointer
	handleTypes uint32
}

type imageDrmFormatModifierListCreateInfo struct {
	sType                  uint32
	pNext                  unsafe.Pointer
	drmFormatModifierCount uint32
	pDrmFormatModifiers    *uint64
}

type imageDrmFormatModifierProperties struct {
	sType             uint32
	pNext             unsafe.Pointer
	drmFormatModifier uint64
}

type imageSubresource struct{ aspectMask, mipLevel, arrayLayer uint32 }

type subresourceLayout struct{ offset, size, rowPitch, arrayPitch, depthPitch uint64 }

type imageSubresourceRange struct {
	aspectMask, baseMipLevel, levelCount, baseArrayLayer, layerCount uint32
}

var colorRange = imageSubresourceRange{aspectMask: aspectColor, levelCount: 1, layerCount: 1}

type imageViewCreateInfo struct {
	sType            uint32
	pNext            unsafe.Pointer
	flags            uint32
	image            uint64
	viewType         uint32
	format           uint32
	components       [4]uint32
	subresourceRange imageSubresourceRange
}

type shaderModuleCreateInfo struct {
	sType    uint32
	pNext    unsafe.Pointer
	flags    uint32
	codeSize uintptr
	pCode    *uint32
}

type pipelineShaderStageCreateInfo struct {
	sType               uint32
	pNext               unsafe.Pointer
	flags               uint32
	stage               uint32
	module              uint64
	pName               *byte
	pSpecializationInfo unsafe.Pointer
}

type vertexInputBindingDescription struct{ binding, stride, inputRate uint32 }

type vertexInputAttributeDescription struct{ location, binding, format, offset uint32 }

type pipelineVertexInputStateCreateInfo struct {
	sType                           uint32
	pNext                           unsafe.Pointer
	flags                           uint32
	vertexBindingDescriptionCount   uint32
	pVertexBindingDescriptions      *vertexInputBindingDescription
	vertexAttributeDescriptionCount uint32
	pVertexAttributeDescriptions    *vertexInputAttributeDescription
}

type pipelineInputAssemblyStateCreateInfo struct {
	sType                  uint32
	pNext                  unsafe.Pointer
	flags                  uint32
	topology               uint32
	primitiveRestartEnable uint32
}

type pipelineViewportStateCreateInfo struct {
	sType         uint32
	pNext         unsafe.Pointer
	flags         uint32
	viewportCount uint32
	pViewports    unsafe.Pointer
	scissorCount  uint32
	pScissors     unsafe.Pointer
}

type pipelineRasterizationStateCreateInfo struct {
	sType                   uint32
	pNext                   unsafe.Pointer
	flags                   uint32
	depthClampEnable        uint32
	rasterizerDiscardEnable uint32
	polygonMode             uint32
	cullMode                uint32
	frontFace               uint32
	depthBiasEnable         uint32
	depthBiasConstantFactor float32
	depthBiasClamp          float32
	depthBiasSlopeFactor    float32
	lineWidth               float32
}

type pipelineMultisampleStateCreateInfo struct {
	sType                 uint32
	pNext                 unsafe.Pointer
	flags                 uint32
	rasterizationSamples  uint32
	sampleShadingEnable   uint32
	minSampleShading      float32
	pSampleMask           *uint32
	alphaToCoverageEnable uint32
	alphaToOneEnable      uint32
}

type pipelineColorBlendAttachmentState struct {
	blendEnable                              uint32
	srcColorBlendFactor, dstColorBlendFactor uint32
	colorBlendOp                             uint32
	srcAlphaBlendFactor, dstAlphaBlendFactor uint32
	alphaBlendOp                             uint32
	colorWriteMask                           uint32
}

type pipelineColorBlendStateCreateInfo struct {
	sType           uint32
	pNext           unsafe.Pointer
	flags           uint32
	logicOpEnable   uint32
	logicOp         uint32
	attachmentCount uint32
	pAttachments    *pipelineColorBlendAttachmentState
	blendConstants  [4]float32
}

type pipelineDynamicStateCreateInfo struct {
	sType             uint32
	pNext             unsafe.Pointer
	flags             uint32
	dynamicStateCount uint32
	pDynamicStates    *uint32
}

type graphicsPipelineCreateInfo struct {
	sType               uint32
	pNext               unsafe.Pointer
	flags               uint32
	stageCount          uint32
	pStages             *pipelineShaderStageCreateInfo
	pVertexInputState   *pipelineVertexInputStateCreateInfo
	pInputAssemblyState *pipelineInputAssemblyStateCreateInfo
	pTessellationState  unsafe.Pointer
	pViewportState      *pipelineViewportStateCreateInfo
	pRasterizationState *pipelineRasterizationStateCreateInfo
	pMultisampleState   *pipelineMultisampleStateCreateInfo
	pDepthStencilState  unsafe.Pointer
	pColorBlendState    *pipelineColorBlendStateCreateInfo
	pDynamicState       *pipelineDynamicStateCreateInfo
	layout              uint64
	renderPass          uint64
	subpass             uint32
	basePipelineHandle  uint64
	basePipelineIndex   int32
}

type pushConstantRange struct{ stageFlags, offset, size uint32 }

type pipelineLayoutCreateInfo struct {
	sType                  uint32
	pNext                  unsafe.Pointer
	flags                  uint32
	setLayoutCount         uint32
	pSetLayouts            *uint64
	pushConstantRangeCount uint32
	pPushConstantRanges    *pushConstantRange
}

type descriptorSetLayoutBinding struct {
	binding            uint32
	descriptorType     uint32
	descriptorCount    uint32
	stageFlags         uint32
	pImmutableSamplers *uint64
}

type descriptorSetLayoutCreateInfo struct {
	sType        uint32
	pNext        unsafe.Pointer
	flags        uint32
	bindingCount uint32
	pBindings    *descriptorSetLayoutBinding
}

type samplerCreateInfo struct {
	sType                   uint32
	pNext                   unsafe.Pointer
	flags                   uint32
	magFilter, minFilter    uint32
	mipmapMode              uint32
	addressModeU            uint32
	addressModeV            uint32
	addressModeW            uint32
	mipLodBias              float32
	anisotropyEnable        uint32
	maxAnisotropy           float32
	compareEnable           uint32
	compareOp               uint32
	minLod, maxLod          float32
	borderColor             uint32
	unnormalizedCoordinates uint32
}

type descriptorPoolSize struct{ typ, descriptorCount uint32 }

type descriptorPoolCreateInfo struct {
	sType         uint32
	pNext         unsafe.Pointer
	flags         uint32
	maxSets       uint32
	poolSizeCount uint32
	pPoolSizes    *descriptorPoolSize
}

type descriptorSetAllocateInfo struct {
	sType              uint32
	pNext              unsafe.Pointer
	descriptorPool     uint64
	descriptorSetCount uint32
	pSetLayouts        *uint64
}

type descriptorImageInfo struct {
	sampler     uint64
	imageView   uint64
	imageLayout uint32
}

type writeDescriptorSet struct {
	sType            uint32
	pNext            unsafe.Pointer
	dstSet           uint64
	dstBinding       uint32
	dstArrayElement  uint32
	descriptorCount  uint32
	descriptorType   uint32
	pImageInfo       *descriptorImageInfo
	pBufferInfo      unsafe.Pointer
	pTexelBufferView unsafe.Pointer
}

type attachmentDescription struct {
	flags, format, samples        uint32
	loadOp, storeOp               uint32
	stencilLoadOp, stencilStoreOp uint32
	initialLayout, finalLayout    uint32
}

type attachmentReference struct{ attachment, layout uint32 }

type subpassDescription struct {
	flags                   uint32
	pipelineBindPoint       uint32
	inputAttachmentCount    uint32
	pInputAttachments       *attachmentReference
	colorAttachmentCount    uint32
	pColorAttachments       *attachmentReference
	pResolveAttachments     *attachmentReference
	pDepthStencilAttachment *attachmentReference
	preserveAttachmentCount uint32
	pPreserveAttachments    *uint32
}

type subpassDependency struct {
	srcSubpass, dstSubpass       uint32
	srcStageMask, dstStageMask   uint32
	srcAccessMask, dstAccessMask uint32
	dependencyFlags              uint32
}

type renderPassCreateInfo struct {
	sType           uint32
	pNext           unsafe.Pointer
	flags           uint32
	attachmentCount uint32
	pAttachments    *attachmentDescription
	subpassCount    uint32
	pSubpasses      *subpassDescription
	dependencyCount uint32
	pDependencies   *subpassDependency
}

type framebufferCreateInfo struct {
	sType           uint32
	pNext           unsafe.Pointer
	flags           uint32
	renderPass      uint64
	attachmentCount uint32
	pAttachments    *uint64
	width, height   uint32
	layers          uint32
}

type rect2D struct {
	x, y          int32
	width, height uint32
}

type renderPassBeginInfo struct {
	sType           uint32
	pNext           unsafe.Pointer
	renderPass      uint64
	framebuffer     uint64
	renderArea      rect2D
	clearValueCount uint32
	pClearValues    *[4]float32
}

type viewport struct{ x, y, width, height, minDepth, maxDepth float32 }

type commandPoolCreateInfo struct {
	sType            uint32
	pNext            unsafe.Pointer
	flags            uint32
	queueFamilyIndex uint32
}

type commandBufferAllocateInfo struct {
	sType              uint32
	pNext              unsafe.Pointer
	commandPool        uint64
	level              uint32
	commandBufferCount uint32
}

type commandBufferBeginInfo struct {
	sType            uint32
	pNext            unsafe.Pointer
	flags            uint32
	pInheritanceInfo unsafe.Pointer
}

type submitInfo struct {
	sType                uint32
	pNext                unsafe.Pointer
	waitSemaphoreCount   uint32
	pWaitSemaphores      *uint64
	pWaitDstStageMask    *uint32
	commandBufferCount   uint32
	pCommandBuffers      *uintptr
	signalSemaphoreCount uint32
	pSignalSemaphores    *uint64
}

type fenceCreateInfo struct {
	sType uint32
	pNext unsafe.Pointer
	flags uint32
}

type imageMemoryBarrier struct {
	sType               uint32
	pNext               unsafe.Pointer
	srcAccessMask       uint32
	dstAccessMask       uint32
	oldLayout           uint32
	newLayout           uint32
	srcQueueFamilyIndex uint32
	dstQueueFamilyIndex uint32
	image               uint64
	subresourceRange    imageSubresourceRange
}

type imageSubresourceLayers struct{ aspectMask, mipLevel, baseArrayLayer, layerCount uint32 }

type imageCopy struct {
	srcSubresource imageSubresourceLayers
	srcOffset      [3]int32
	dstSubresource imageSubresourceLayers
	dstOffset      [3]int32
	extent         extent3D
}

type memoryBarrier struct {
	sType         uint32
	pNext         unsafe.Pointer
	srcAccessMask uint32
	dstAccessMask uint32
}

const stMemoryBarrier = 46

type bufferImageCopy struct {
	bufferOffset      uint64
	bufferRowLength   uint32
	bufferImageHeight uint32
	aspectMask        uint32
	mipLevel          uint32
	baseArrayLayer    uint32
	layerCount        uint32
	imageOffset       [3]int32
	imageExtent       extent3D
}

// Commands of the loader and the instance, looked up with
// vkGetInstanceProcAddr. A field named X holds the address of vkX.
type instanceFuncs struct {
	DestroyInstance                         uintptr
	EnumeratePhysicalDevices                uintptr
	GetPhysicalDeviceProperties2            uintptr
	GetPhysicalDeviceFeatures               uintptr
	GetPhysicalDeviceQueueFamilyProperties  uintptr
	GetPhysicalDeviceMemoryProperties       uintptr
	GetPhysicalDeviceFormatProperties2      uintptr
	GetPhysicalDeviceImageFormatProperties2 uintptr
	EnumerateDeviceExtensionProperties      uintptr
	CreateDevice                            uintptr
	GetDeviceProcAddr                       uintptr
}

// Commands of a device, looked up with vkGetDeviceProcAddr.
type deviceFuncs struct {
	DestroyDevice                          uintptr
	GetDeviceQueue                         uintptr
	DeviceWaitIdle                         uintptr
	AllocateMemory                         uintptr
	FreeMemory                             uintptr
	MapMemory                              uintptr
	CreateBuffer                           uintptr
	DestroyBuffer                          uintptr
	GetBufferMemoryRequirements            uintptr
	BindBufferMemory                       uintptr
	CreateImage                            uintptr
	DestroyImage                           uintptr
	GetImageMemoryRequirements             uintptr
	BindImageMemory                        uintptr
	GetImageSubresourceLayout              uintptr
	GetImageDrmFormatModifierPropertiesEXT uintptr
	GetMemoryFdKHR                         uintptr
	CreateImageView                        uintptr
	DestroyImageView                       uintptr
	CreateSampler                          uintptr
	DestroySampler                         uintptr
	CreateShaderModule                     uintptr
	DestroyShaderModule                    uintptr
	CreateRenderPass                       uintptr
	DestroyRenderPass                      uintptr
	CreateFramebuffer                      uintptr
	DestroyFramebuffer                     uintptr
	CreateDescriptorSetLayout              uintptr
	DestroyDescriptorSetLayout             uintptr
	CreatePipelineLayout                   uintptr
	DestroyPipelineLayout                  uintptr
	CreateGraphicsPipelines                uintptr
	DestroyPipeline                        uintptr
	CreateDescriptorPool                   uintptr
	DestroyDescriptorPool                  uintptr
	AllocateDescriptorSets                 uintptr
	FreeDescriptorSets                     uintptr
	UpdateDescriptorSets                   uintptr
	CreateCommandPool                      uintptr
	DestroyCommandPool                     uintptr
	AllocateCommandBuffers                 uintptr
	BeginCommandBuffer                     uintptr
	EndCommandBuffer                       uintptr
	CreateFence                            uintptr
	DestroyFence                           uintptr
	ResetFences                            uintptr
	WaitForFences                          uintptr
	QueueSubmit                            uintptr
	CmdBeginRenderPass                     uintptr
	CmdEndRenderPass                       uintptr
	CmdBindPipeline                        uintptr
	CmdBindVertexBuffers                   uintptr
	CmdBindDescriptorSets                  uintptr
	CmdPushConstants                       uintptr
	CmdSetViewport                         uintptr
	CmdSetScissor                          uintptr
	CmdDraw                                uintptr
	CmdPipelineBarrier                     uintptr
	CmdCopyBufferToImage                   uintptr
	CmdCopyImageToBuffer                   uintptr
	CmdCopyImage                           uintptr
}

// loader is libvulkan, loaded once.
var loader struct {
	once sync.Once
	err  error
	// getInstanceProcAddr is vkGetInstanceProcAddr.
	getInstanceProcAddr uintptr
}

func loadLibrary() error {
	loader.once.Do(func() {
		lib, err := purego.Dlopen("libvulkan.so.1", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			loader.err = fmt.Errorf("vulkan: libvulkan.so.1 is not available: %w", err)
			return
		}
		sym, err := purego.Dlsym(lib, "vkGetInstanceProcAddr")
		if err != nil {
			loader.err = fmt.Errorf("vulkan: libvulkan.so.1 lacks vkGetInstanceProcAddr: %w", err)
			return
		}
		loader.getInstanceProcAddr = sym
	})
	return loader.err
}

// call calls a Vulkan command and returns its result. Pointer arguments must
// be converted with uintptr(unsafe.Pointer(...)) in the call expression:
// go:uintptrescapes then moves what they point to to the heap and keeps it
// alive for the call, as it does for purego.SyscallN, which call wraps.
//
//go:uintptrescapes
func call(fn uintptr, args ...uintptr) vkResult {
	r, _, _ := purego.SyscallN(fn, args...)
	return vkResult(int32(r))
}

// callVoid calls a Vulkan command that returns nothing; see call.
//
//go:uintptrescapes
func callVoid(fn uintptr, args ...uintptr) { purego.SyscallN(fn, args...) }

func instanceProc(instance uintptr, name string) uintptr {
	return procAddr(loader.getInstanceProcAddr, instance, name)
}

// procAddr looks up a command with vkGetInstanceProcAddr or
// vkGetDeviceProcAddr (getProc) for the handle.
func procAddr(getProc, handle uintptr, name string) uintptr {
	n := cstr(name)
	r, _, _ := purego.SyscallN(getProc, handle, uintptr(unsafe.Pointer(n)))
	return r
}

// loadFuncs fills the uintptr fields of the struct *dst with lookup("vk" +
// field name). A command the driver lacks is an error.
func loadFuncs(dst any, lookup func(name string) uintptr) error {
	v := reflect.ValueOf(dst).Elem()
	t := v.Type()
	for i := range t.NumField() {
		name := "vk" + t.Field(i).Name
		fn := lookup(name)
		if fn == 0 {
			return fmt.Errorf("vulkan: the driver lacks %s", name)
		}
		v.Field(i).SetUint(uint64(fn))
	}
	return nil
}

// cstr returns s as a NUL-terminated C string.
func cstr(s string) *byte {
	b := make([]byte, len(s)+1)
	copy(b, s)
	return &b[0]
}

// cstrs returns the strings as an array of C strings, or nil when empty.
func cstrs(ss []string) **byte {
	if len(ss) == 0 {
		return nil
	}
	ptrs := make([]*byte, len(ss))
	for i, s := range ss {
		ptrs[i] = cstr(s)
	}
	return &ptrs[0]
}

// gostr reads a NUL-terminated string from a fixed-size C array.
func gostr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// ptr converts a pointer Vulkan returned (mapped memory) to unsafe.Pointer.
func ptr(p uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&p)) }

var errNoDevice = errors.New("vulkan: no GPU with the features jamb needs (dma-buf export with DRM format modifiers)")
