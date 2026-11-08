//go:build linux && (amd64 || arm64)

package vulkan

import (
	"errors"
	"fmt"
	"image"
	"math"
	"slices"
	"time"
	"unsafe"

	"github.com/tyler-builds/jamb/internal/dmabuf"
	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/scene"
	"golang.org/x/sys/unix"
)

// Per-instance vertex data; layouts must match the shaders' inputs. CShape and
// CRadii are the innermost rounded clip of the level (see clipState). They are
// the Direct3D 11 renderer's.
type quadInstance struct {
	Rect, Clip, Color, Border, Radii, Params, Color2, Grad, Sides, CShape, CRadii [4]float32
}

type maskInstance struct {
	Rect, UV, Color, Clip, CShape, CRadii [4]float32
}

type imageInstance struct {
	Rect, UV, Clip, Radii, Params, CShape, CRadii [4]float32
}

type command struct {
	pipe         pipelineKind
	start, count uint32
	tex          *scene.Texture
}

// clipState is one level of the clip stack, as in the Direct3D 11 renderer.
// rect is the intersection of every clip so far. shape and radii are the
// innermost rounded clip (zero when there is none), which is not intersected,
// so an outer clip that cuts it does not round the cut.
type clipState struct {
	rect  scene.Rect
	shape scene.Rect
	radii scene.Corners
}

// target is one image a window's frames are drawn into, shared with the
// compositor as a dma-buf.
type target struct {
	// image is the image the compositor shows, exported as a dma-buf. Frames
	// are drawn into it, or for a shared placement into draw, a private image
	// in the GPU's memory, and then copied into it.
	image, mem    uint64
	draw, drawMem uint64
	view, fb      uint64 // of the image frames are drawn into
	shared        bool
	width, height int
	copy          bool // the image can be copied from (CaptureNextFrame)
	buf           dmabuf.Buffer
	// fd is the image's dma-buf, kept to wait for the compositor's reads of it
	// (see waitReads), or -1.
	fd int
	// shown is set once the image was handed to the compositor, so a frame
	// takes it back before drawing into it again.
	shown bool
}

// maxTargets is the most images a window keeps. Frames are paced by the
// compositor, which holds one image on screen and may hold a second until it
// releases it, so two or three are in use; a fourth is made only when the
// compositor holds every one.
const maxTargets = 4

// Renderer implements render.Renderer with Vulkan for one window. Its frames
// are drawn into images that the window's surface shows; the device it draws
// with is shared.
type Renderer struct {
	*Device
	surface dmabuf.Surface
	// place is how the window's images are made, which the compositor was
	// found to import (see setup).
	place   placement
	targets []*target

	width, height int
	clear         scene.Color
	clips         []clipState

	quads    []quadInstance
	masks    []maskInstance
	subs     []maskInstance // subpixel glyphs
	images   []imageInstance
	text     scene.TextParams // of the subpixel atlas seen this frame
	commands []command
	uploads  []upload // the frame's texture uploads (see uploadTextures)

	capture func(*image.RGBA, error)
}

var _ render.Renderer = (*Renderer)(nil)

// errDeviceReleased is returned for a window drawn on a device that was
// released and not recovered yet.
var errDeviceReleased = fmt.Errorf("%w: vulkan: the device was released and has not been recovered", render.ErrDeviceLost)

// NewRenderer creates the renderer of a window whose frames s shows. It fails
// when the compositor imports none of the images the device can make for it;
// the window is then drawn on the CPU.
func (d *Device) NewRenderer(s dmabuf.Surface) (*Renderer, error) {
	r := &Renderer{Device: d, surface: s}
	if err := r.setup(); err != nil {
		return nil, err
	}
	d.renderers = append(d.renderers, r)
	return r, nil
}

// placement is a kind of image the window's frames are shown in: its modifier
// and the memory it lives in.
type placement struct {
	mod modifier
	// shared puts the image in system memory, linear, so the compositor's GPU
	// can read it when it is another GPU. Frames are drawn into a private
	// image in the GPU's own memory and copied into it, as PRIME does: not
	// every GPU can draw into a linear image.
	shared bool
}

// setup chooses how the window's images are made. Each candidate is probed:
// the compositor imports a small image of that kind or rejects it, and the
// first it imports is kept. A compositor on the same GPU takes the modifiers
// the GPU prefers in its own memory. A compositor on another GPU (a laptop
// that composites on the integrated GPU while jamb draws on the discrete one)
// takes linear images in system memory, which both GPUs reach (see
// placement).
func (r *Renderer) setup() error {
	if r.dev == 0 {
		return errDeviceReleased
	}
	theirs := r.surface.Modifiers(dmabuf.FormatARGB8888)
	var local, linear []placement
	for _, m := range r.modifiers {
		if slices.Contains(theirs, m.mod) {
			local = append(local, placement{mod: m})
		}
	}
	if r.linearCopy && slices.Contains(theirs, dmabuf.ModLinear) {
		linear = append(linear, placement{mod: modifier{mod: dmabuf.ModLinear, planes: 1, copy: true}, shared: true})
	}
	// The modifier the driver picks among them comes first, then the others
	// that can be copied from, so CaptureNextFrame works.
	best, _ := r.preferred(local)
	slices.SortStableFunc(local, func(a, b placement) int {
		rank := func(p placement) int {
			switch {
			case p.mod.mod == best:
				return 0
			case p.mod.copy:
				return 1
			}
			return 2
		}
		return rank(a) - rank(b)
	})
	candidates := append(local, linear...)
	if main, ok := r.surface.MainDevice(); ok && !r.OnDevice(main) {
		candidates = append(linear, local...)
	}
	if len(candidates) == 0 {
		return fmt.Errorf("vulkan: the compositor imports none of the dma-buf layouts %s renders", r.name)
	}
	var errs []error
	for _, p := range candidates {
		err := r.probe(p)
		if err == nil {
			r.place = p
			return nil
		}
		errs = append(errs, err)
	}
	return fmt.Errorf("vulkan: the compositor imports no image %s makes: %w", r.name, errors.Join(errs...))
}

// preferred returns the modifier the driver picks among the placements' for a
// render target, which is the one it renders fastest. Copyable modifiers are
// offered when there are any.
func (r *Renderer) preferred(ps []placement) (uint64, bool) {
	var mods []uint64
	for _, p := range ps {
		if p.mod.copy {
			mods = append(mods, p.mod.mod)
		}
	}
	usage := uint32(imageUsageColorAttachment | imageUsageTransferSrc)
	if len(mods) == 0 {
		for _, p := range ps {
			mods = append(mods, p.mod.mod)
		}
		usage = imageUsageColorAttachment
	}
	if len(mods) == 0 {
		return 0, false
	}
	modList := imageDrmFormatModifierListCreateInfo{sType: stImageDrmFormatModifierListCreateInfo,
		drmFormatModifierCount: uint32(len(mods)), pDrmFormatModifiers: &mods[0]}
	ext := externalMemoryImageCreateInfo{sType: stExternalMemoryImageCreateInfo, pNext: unsafe.Pointer(&modList), handleTypes: externalMemoryDmaBuf}
	ii := imageCreateInfo{sType: stImageCreateInfo, pNext: unsafe.Pointer(&ext), imageType: imageType2D, format: formatB8G8R8A8Unorm,
		extent: extent3D{64, 64, 1}, mipLevels: 1, arrayLayers: 1, samples: sampleCount1,
		tiling: imageTilingDrmFormatModifier, usage: usage, sharingMode: sharingExclusive, initialLayout: layoutUndefined}
	var image uint64
	if call(r.vk.CreateImage, r.dev, uintptr(unsafe.Pointer(&ii)), 0, uintptr(unsafe.Pointer(&image))) != vkSuccess {
		return 0, false
	}
	defer callVoid(r.vk.DestroyImage, r.dev, uintptr(image), 0)
	props := imageDrmFormatModifierProperties{sType: stImageDrmFormatModifierProperties}
	if call(r.vk.GetImageDrmFormatModifierPropertiesEXT, r.dev, uintptr(image), uintptr(unsafe.Pointer(&props))) != vkSuccess {
		return 0, false
	}
	return props.drmFormatModifier, true
}

// probe reports whether the compositor imports an image made as p.
func (r *Renderer) probe(p placement) error {
	t, img, err := r.makeTarget(64, 64, p)
	if err != nil {
		return err
	}
	defer r.freeTarget(t)
	return r.surface.Probe(img)
}

// Describe summarizes how the window's frames are shown, for diagnostics.
func (r *Renderer) Describe() string {
	if r.place.shared {
		return "Vulkan on " + r.name + ", dma-buf in system memory for the compositor's GPU"
	}
	return "Vulkan on " + r.name + ", dma-buf"
}

// Recreate prepares the window to draw again on the device after
// Device.Recover. Its images were released with the old device. When the
// device still renders the kind of image the compositor imported, it is kept
// without probing again.
func (r *Renderer) Recreate() error {
	r.dropTargets()
	r.width, r.height = 0, 0
	if r.dev == 0 {
		return errDeviceReleased
	}
	if r.place.shared && r.linearCopy || !r.place.shared && slices.Contains(r.modifiers, r.place.mod) {
		return nil
	}
	return r.setup()
}

// Close implements render.Renderer. It releases the window's images; the
// device stays for the other windows.
func (r *Renderer) Close() {
	r.dropTargets()
	r.renderers = slices.DeleteFunc(r.renderers, func(o *Renderer) bool { return o == r })
}

func (r *Renderer) dropTargets() {
	for _, t := range r.targets {
		r.freeTarget(t)
	}
	r.targets = nil
}

func (r *Renderer) freeTarget(t *target) {
	if t.buf != nil {
		// The compositor keeps its own reference to the memory while it
		// shows the image, so the image can go now.
		t.buf.Destroy()
	}
	if t.fd >= 0 {
		unix.Close(t.fd)
	}
	if r.dev != 0 {
		for _, o := range []struct {
			fn uintptr
			h  uint64
		}{{r.vk.DestroyFramebuffer, t.fb}, {r.vk.DestroyImageView, t.view}, {r.vk.DestroyImage, t.image}, {r.vk.FreeMemory, t.mem},
			{r.vk.DestroyImage, t.draw}, {r.vk.FreeMemory, t.drawMem}} {
			if o.h != 0 {
				callVoid(o.fn, r.dev, uintptr(o.h), 0)
			}
		}
	}
	*t = target{fd: -1}
}

// nextTarget returns an image of width×height that the compositor is not
// reading.
func (r *Renderer) nextTarget(width, height int) (*target, error) {
	for i, t := range r.targets {
		if t.buf.Busy() {
			continue
		}
		if t.width == width && t.height == height {
			waitReads(t)
			return t, nil
		}
		r.freeTarget(t)
		nt, err := r.newTarget(width, height)
		if err != nil {
			r.targets = slices.Delete(r.targets, i, i+1)
			return nil, err
		}
		r.targets[i] = nt
		return nt, nil
	}
	if len(r.targets) >= maxTargets {
		// The compositor holds every image: draw into a new one rather than
		// waiting, and let the others go when it releases them.
		r.dropTargets()
	}
	t, err := r.newTarget(width, height)
	if err != nil {
		return nil, err
	}
	r.targets = append(r.targets, t)
	return t, nil
}

// newTarget creates an image of width×height, exports it as a dma-buf and
// has the compositor import it.
func (r *Renderer) newTarget(width, height int) (*target, error) {
	t, img, err := r.makeTarget(width, height, r.place)
	if err != nil {
		return nil, err
	}
	if t.buf, err = r.surface.Import(img); err != nil {
		r.freeTarget(t)
		return nil, err
	}
	return t, nil
}

// readWait bounds the wait for the compositor's reads of an image it
// released.
const readWait = 100 * time.Millisecond

// waitReads waits until the GPU work that reads the image, the compositor's,
// is done, so drawing into it cannot change a frame being shown. A compositor
// releases a buffer once it has queued its last read, which may not have run
// yet; the kernel tracks that read on the dma-buf (implicit synchronization),
// and the dma-buf polls writable once it is done. A driver that does not track
// it makes the poll return at once.
func waitReads(t *target) {
	if !t.shown || t.fd < 0 {
		return
	}
	fds := []unix.PollFd{{Fd: int32(t.fd), Events: unix.POLLOUT}}
	for {
		_, err := unix.Poll(fds, int(readWait.Milliseconds()))
		if err != unix.EINTR {
			return
		}
	}
}

// makeTarget creates an image of width×height made as p and exports it as a
// dma-buf. It returns the dma-buf's description; the target keeps the
// descriptor.
func (r *Renderer) makeTarget(width, height int, p placement) (t *target, img dmabuf.Image, err error) {
	t = &target{width: width, height: height, copy: p.mod.copy, shared: p.shared, fd: -1}
	defer func() {
		if err != nil {
			r.freeTarget(t)
			t = nil
		}
	}()
	usage := uint32(imageUsageColorAttachment)
	if t.copy {
		usage |= imageUsageTransferSrc
	}
	if p.shared {
		usage = imageUsageTransferDst
	}
	mods := []uint64{p.mod.mod}
	modList := imageDrmFormatModifierListCreateInfo{sType: stImageDrmFormatModifierListCreateInfo,
		drmFormatModifierCount: 1, pDrmFormatModifiers: &mods[0]}
	ext := externalMemoryImageCreateInfo{sType: stExternalMemoryImageCreateInfo, pNext: unsafe.Pointer(&modList), handleTypes: externalMemoryDmaBuf}
	ii := imageCreateInfo{sType: stImageCreateInfo, pNext: unsafe.Pointer(&ext), imageType: imageType2D, format: formatB8G8R8A8Unorm,
		extent: extent3D{uint32(width), uint32(height), 1}, mipLevels: 1, arrayLayers: 1, samples: sampleCount1,
		tiling: imageTilingDrmFormatModifier, usage: usage, sharingMode: sharingExclusive, initialLayout: layoutUndefined}
	if err := check("vkCreateImage(target)", call(r.vk.CreateImage, r.dev, uintptr(unsafe.Pointer(&ii)), 0, uintptr(unsafe.Pointer(&t.image)))); err != nil {
		return nil, img, err
	}
	var req memoryRequirements
	callVoid(r.vk.GetImageMemoryRequirements, r.dev, uintptr(t.image), uintptr(unsafe.Pointer(&req)))
	mt, ok := r.targetMemory(req.memoryTypeBits, p.shared)
	if !ok {
		if p.shared {
			return nil, img, fmt.Errorf("vulkan: %s cannot export an image from system memory", r.name)
		}
		return nil, img, errors.New("vulkan: no memory type for a render target")
	}
	dedicated := memoryDedicatedAllocateInfo{sType: stMemoryDedicatedAllocateInfo, image: t.image}
	export := exportMemoryAllocateInfo{sType: stExportMemoryAllocateInfo, pNext: unsafe.Pointer(&dedicated), handleTypes: externalMemoryDmaBuf}
	mi := memoryAllocateInfo{sType: stMemoryAllocateInfo, pNext: unsafe.Pointer(&export), allocationSize: req.size, memoryTypeIndex: mt}
	if err := check("vkAllocateMemory(target)", call(r.vk.AllocateMemory, r.dev, uintptr(unsafe.Pointer(&mi)), 0, uintptr(unsafe.Pointer(&t.mem)))); err != nil {
		return nil, img, err
	}
	if err := check("vkBindImageMemory(target)", call(r.vk.BindImageMemory, r.dev, uintptr(t.image), uintptr(t.mem), 0)); err != nil {
		return nil, img, err
	}

	img = dmabuf.Image{Width: width, Height: height, Format: dmabuf.FormatARGB8888, Modifier: p.mod.mod}
	for i := range p.mod.planes {
		sub := imageSubresource{aspectMask: aspectMemoryPlane0 << i}
		var layout subresourceLayout
		callVoid(r.vk.GetImageSubresourceLayout, r.dev, uintptr(t.image), uintptr(unsafe.Pointer(&sub)), uintptr(unsafe.Pointer(&layout)))
		img.Planes = append(img.Planes, dmabuf.Plane{Offset: uint32(layout.offset), Stride: uint32(layout.rowPitch)})
	}
	fdInfo := memoryGetFdInfo{sType: stMemoryGetFdInfo, memory: t.mem, handleType: externalMemoryDmaBuf}
	var fd int32 = -1
	if err := check("vkGetMemoryFdKHR", call(r.vk.GetMemoryFdKHR, r.dev, uintptr(unsafe.Pointer(&fdInfo)), uintptr(unsafe.Pointer(&fd)))); err != nil {
		return nil, img, err
	}
	t.fd, img.FD = int(fd), int(fd)

	drawn := t.image
	if p.shared {
		if err := r.makeDrawImage(t); err != nil {
			return nil, img, err
		}
		drawn = t.draw
	}
	vi := imageViewCreateInfo{sType: stImageViewCreateInfo, image: drawn, viewType: imageViewType2D, format: formatB8G8R8A8Unorm, subresourceRange: colorRange}
	if err := check("vkCreateImageView(target)", call(r.vk.CreateImageView, r.dev, uintptr(unsafe.Pointer(&vi)), 0, uintptr(unsafe.Pointer(&t.view)))); err != nil {
		return nil, img, err
	}
	fi := framebufferCreateInfo{sType: stFramebufferCreateInfo, renderPass: r.renderPass, attachmentCount: 1, pAttachments: &t.view,
		width: uint32(width), height: uint32(height), layers: 1}
	if err := check("vkCreateFramebuffer", call(r.vk.CreateFramebuffer, r.dev, uintptr(unsafe.Pointer(&fi)), 0, uintptr(unsafe.Pointer(&t.fb)))); err != nil {
		return nil, img, err
	}
	return t, img, nil
}

// makeDrawImage creates the private image a shared target's frames are drawn
// into, in the GPU's own memory.
func (r *Renderer) makeDrawImage(t *target) error {
	ii := imageCreateInfo{sType: stImageCreateInfo, imageType: imageType2D, format: formatB8G8R8A8Unorm,
		extent: extent3D{uint32(t.width), uint32(t.height), 1}, mipLevels: 1, arrayLayers: 1, samples: sampleCount1,
		tiling: imageTilingOptimal, usage: imageUsageColorAttachment | imageUsageTransferSrc, sharingMode: sharingExclusive, initialLayout: layoutUndefined}
	if err := check("vkCreateImage(draw)", call(r.vk.CreateImage, r.dev, uintptr(unsafe.Pointer(&ii)), 0, uintptr(unsafe.Pointer(&t.draw)))); err != nil {
		return err
	}
	var req memoryRequirements
	callVoid(r.vk.GetImageMemoryRequirements, r.dev, uintptr(t.draw), uintptr(unsafe.Pointer(&req)))
	mt, ok := r.targetMemory(req.memoryTypeBits, false)
	if !ok {
		return errors.New("vulkan: no memory type for a render target")
	}
	mi := memoryAllocateInfo{sType: stMemoryAllocateInfo, allocationSize: req.size, memoryTypeIndex: mt}
	if err := check("vkAllocateMemory(draw)", call(r.vk.AllocateMemory, r.dev, uintptr(unsafe.Pointer(&mi)), 0, uintptr(unsafe.Pointer(&t.drawMem)))); err != nil {
		return err
	}
	return check("vkBindImageMemory(draw)", call(r.vk.BindImageMemory, r.dev, uintptr(t.draw), uintptr(t.drawMem), 0))
}

// targetMemory returns the memory type for a render target: the GPU's own
// memory, or for a shared image, system memory the GPU can write (host
// visible and not device local), which another GPU can read.
func (r *Renderer) targetMemory(bits uint32, shared bool) (uint32, bool) {
	if !shared {
		if mt, ok := r.memoryType(bits, memoryPropertyDeviceLocal); ok {
			return mt, true
		}
		return r.memoryType(bits, 0)
	}
	for i := range r.memProps.memoryTypeCount {
		f := r.memProps.memoryTypes[i].propertyFlags
		if bits&(1<<i) != 0 && f&memoryPropertyHostVisible != 0 && f&memoryPropertyDeviceLocal == 0 {
			return i, true
		}
	}
	return 0, false
}

// BeginFrame implements render.Renderer.
func (r *Renderer) BeginFrame(width, height int, clear scene.Color) error {
	if width <= 0 || height <= 0 {
		return errors.New("vulkan: BeginFrame called with an empty size; skip rendering while minimized")
	}
	if r.dev == 0 {
		return errDeviceReleased
	}
	r.width, r.height = width, height
	r.clear = clear
	r.clips = append(r.clips[:0], clipState{rect: scene.Rect{W: float32(width), H: float32(height)}})
	r.quads = r.quads[:0]
	r.masks = r.masks[:0]
	r.subs = r.subs[:0]
	r.images = r.images[:0]
	r.commands = r.commands[:0]
	return nil
}

// clipInstance returns the clip of the current level for the instances being
// added: the rectangle (x0, y0, x1, y1), and the rounded shape and its radii.
func (r *Renderer) clipInstance() (clip, shape, radii [4]float32) {
	c := r.clips[len(r.clips)-1]
	return [4]float32{c.rect.X, c.rect.Y, c.rect.Right(), c.rect.Bottom()}, rect4(c.shape), corners4(c.radii)
}

// PushClip implements render.Renderer.
func (r *Renderer) PushClip(c scene.Rect) {
	top := r.clips[len(r.clips)-1]
	r.clips = append(r.clips, clipState{rect: top.rect.Intersect(c), shape: top.shape, radii: top.radii})
}

// PushRoundedClip implements render.Renderer.
func (r *Renderer) PushRoundedClip(c scene.Rect, radii scene.Corners) {
	top := r.clips[len(r.clips)-1]
	r.clips = append(r.clips, clipState{rect: top.rect.Intersect(c), shape: c, radii: radii})
}

// PopClip implements render.Renderer.
func (r *Renderer) PopClip() {
	if len(r.clips) > 1 {
		r.clips = r.clips[:len(r.clips)-1]
	}
}

func (r *Renderer) addCommand(p pipelineKind, start, count int, tex *scene.Texture) {
	if n := len(r.commands); n > 0 {
		last := &r.commands[n-1]
		if last.pipe == p && last.tex == tex && int(last.start+last.count) == start {
			last.count += uint32(count)
			return
		}
	}
	r.commands = append(r.commands, command{pipe: p, start: uint32(start), count: uint32(count), tex: tex})
}

func rect4(b scene.Rect) [4]float32       { return [4]float32{b.X, b.Y, b.W, b.H} }
func color4(c scene.Color) [4]float32     { return [4]float32{c.R, c.G, c.B, c.A} }
func corners4(c scene.Corners) [4]float32 { return [4]float32{c.TL, c.TR, c.BR, c.BL} }

// DrawQuads implements render.Renderer.
func (r *Renderer) DrawQuads(qs []scene.Quad) {
	if len(qs) == 0 {
		return
	}
	clip, cshape, cradii := r.clipInstance()
	start := len(r.quads)
	for _, q := range qs {
		r.quads = append(r.quads, quadInstance{
			Rect: rect4(q.Bounds), Clip: clip, Color: color4(q.Background), Border: color4(q.BorderColor),
			Radii: corners4(q.Radii), Params: quadParams(q), Color2: color4(q.Color2), Grad: gradient4(q),
			Sides: quadSides(q), CShape: cshape, CRadii: cradii,
		})
	}
	r.addCommand(pipeQuad, start, len(qs), nil)
}

// quadParams packs the shadow sigma and the gradient flags.
func quadParams(q scene.Quad) [4]float32 {
	p := [4]float32{q.Blur, 0, 0, 0}
	if q.Gradient {
		p[1] = 1
		if q.Oklab {
			p[2] = 1
		}
	}
	return p
}

// quadSides returns the border width of each side (top, right, bottom, left):
// BorderSides when set, else BorderWidth on every side.
func quadSides(q scene.Quad) [4]float32 {
	if s := q.BorderSides; s != (scene.Sides{}) {
		return [4]float32{s.Top, s.Right, s.Bottom, s.Left}
	}
	w := q.BorderWidth
	return [4]float32{w, w, w, w}
}

// gradient4 returns the gradient line's unit direction and stop positions. The
// angle is CSS's: 0 points up, 90 right, 180 down.
func gradient4(q scene.Quad) [4]float32 {
	if !q.Gradient {
		return [4]float32{}
	}
	rad := float64(q.Angle) * math.Pi / 180
	return [4]float32{float32(math.Sin(rad)), float32(-math.Cos(rad)), q.Stop0, q.Stop1}
}

// DrawGlyphs implements render.Renderer.
func (r *Renderer) DrawGlyphs(run scene.GlyphRun) {
	if len(run.Glyphs) == 0 || run.Texture == nil {
		return
	}
	// Subpixel (ClearType) masks have their own pipeline and instances.
	pipe, list := pipeMask, &r.masks
	if run.Texture.Format == scene.FormatSubpixel8 {
		pipe, list = pipeSubpixel, &r.subs
		r.text = run.Texture.Text
	}
	clip, cshape, cradii := r.clipInstance()
	tw, th := float32(run.Texture.Width), float32(run.Texture.Height)
	start := len(*list)
	for _, g := range run.Glyphs {
		*list = append(*list, maskInstance{
			Rect:  rect4(g.Bounds),
			UV:    [4]float32{g.Src.X / tw, g.Src.Y / th, (g.Src.X + g.Src.W) / tw, (g.Src.Y + g.Src.H) / th},
			Color: color4(g.Color), Clip: clip, CShape: cshape, CRadii: cradii,
		})
	}
	r.addCommand(pipe, start, len(run.Glyphs), run.Texture)
}

// DrawImages implements render.Renderer.
func (r *Renderer) DrawImages(ims []scene.ImageDraw) {
	clip, cshape, cradii := r.clipInstance()
	for _, im := range ims {
		if im.Texture == nil {
			continue
		}
		tw, th := float32(im.Texture.Width), float32(im.Texture.Height)
		src := im.Src
		if src.Empty() {
			src = scene.Rect{W: tw, H: th}
		}
		r.images = append(r.images, imageInstance{
			Rect:   rect4(im.Bounds),
			UV:     [4]float32{src.X / tw, src.Y / th, (src.X + src.W) / tw, (src.Y + src.H) / th},
			Clip:   clip,
			Radii:  corners4(im.Radii),
			Params: [4]float32{im.Opacity},
			CShape: cshape,
			CRadii: cradii,
		})
		r.addCommand(pipeImage, len(r.images)-1, 1, im.Texture)
	}
}

// CaptureNextFrame implements render.Renderer.
func (r *Renderer) CaptureNextFrame(fn func(*image.RGBA, error)) { r.capture = fn }

// fenceTimeout is how long a frame waits for the GPU before the device is
// taken as lost.
const fenceTimeout = 5 * time.Second

// EndFrame implements render.Renderer. It records the frame, submits it and
// waits for the GPU to finish, then presents the image: the compositor reads
// it as soon as it is attached, so it must be complete. A frame of a desktop
// UI takes the GPU well under a millisecond.
func (r *Renderer) EndFrame() error {
	if r.dev == 0 {
		return errDeviceReleased
	}
	t, err := r.nextTarget(r.width, r.height)
	if err != nil {
		return err
	}
	now := time.Now()
	r.sweepTextures(now)

	cmd := r.cmd
	begin := commandBufferBeginInfo{sType: stCommandBufferBeginInfo, flags: commandBufferOneTime}
	if err := check("vkBeginCommandBuffer", call(r.vk.BeginCommandBuffer, cmd, uintptr(unsafe.Pointer(&begin)))); err != nil {
		return err
	}
	src, dst := queueFamilyIgnored, queueFamilyIgnored
	if r.foreign {
		src, dst = r.family, queueFamilyForeign
	}
	if t.shown && r.foreign && !t.shared {
		// Take the image back from the compositor. Its contents are not kept:
		// the render pass clears it.
		r.barrier(t.image, 0, accessColorAttachmentWrite, layoutGeneral, layoutColorAttachmentOptimal,
			stageTopOfPipe, stageColorAttachmentOutput, queueFamilyForeign, r.family)
	}
	if err := r.uploadTextures(now); err != nil {
		call(r.vk.EndCommandBuffer, cmd)
		return err
	}
	offsets, err := r.writeInstances()
	if err != nil {
		call(r.vk.EndCommandBuffer, cmd)
		return err
	}

	c := r.clear
	clearValue := [4]float32{c.R * c.A, c.G * c.A, c.B * c.A, c.A}
	rp := renderPassBeginInfo{sType: stRenderPassBeginInfo, renderPass: r.renderPass, framebuffer: t.fb,
		renderArea: rect2D{width: uint32(r.width), height: uint32(r.height)}, clearValueCount: 1, pClearValues: &clearValue}
	callVoid(r.vk.CmdBeginRenderPass, cmd, uintptr(unsafe.Pointer(&rp)), subpassInline)
	vp := viewport{width: float32(r.width), height: float32(r.height), maxDepth: 1}
	callVoid(r.vk.CmdSetViewport, cmd, 0, 1, uintptr(unsafe.Pointer(&vp)))
	scissor := rect2D{width: uint32(r.width), height: uint32(r.height)}
	callVoid(r.vk.CmdSetScissor, cmd, 0, 1, uintptr(unsafe.Pointer(&scissor)))
	push := [8]float32{float32(r.width), float32(r.height), 0, 0, r.text.Gamma, r.text.Contrast, r.text.Level, 0}
	callVoid(r.vk.CmdPushConstants, cmd, uintptr(r.pipeLayout), shaderStageVertex|shaderStageFragment, 0, pushSize, uintptr(unsafe.Pointer(&push[0])))
	callVoid(r.vk.CmdBindDescriptorSets, cmd, bindPointGraphics, uintptr(r.pipeLayout), 0, 1, uintptr(unsafe.Pointer(&r.samplerSet)), 0, 0)

	current := pipelineKind(255)
	var boundTex *scene.Texture
	for _, c := range r.commands {
		if c.pipe != current {
			current = c.pipe
			callVoid(r.vk.CmdBindPipeline, cmd, bindPointGraphics, uintptr(r.pipes[c.pipe]))
			buf, off := r.instances.buf, uint64(offsets[c.pipe])
			callVoid(r.vk.CmdBindVertexBuffers, cmd, 0, 1, uintptr(unsafe.Pointer(&buf)), uintptr(unsafe.Pointer(&off)))
		}
		if c.tex != nil && c.tex != boundTex {
			boundTex = c.tex
			set := r.textures[c.tex.ID].set
			callVoid(r.vk.CmdBindDescriptorSets, cmd, bindPointGraphics, uintptr(r.pipeLayout), 1, 1, uintptr(unsafe.Pointer(&set)), 0, 0)
		}
		callVoid(r.vk.CmdDraw, cmd, 4, uintptr(c.count), 0, uintptr(c.start))
	}
	callVoid(r.vk.CmdEndRenderPass, cmd)

	// drawn is the image the frame was drawn into, in layout.
	drawn, layout := t.image, uint32(layoutColorAttachmentOptimal)
	if t.shared {
		// Copy the frame into the shared image, taking it back from the
		// compositor first (its contents are not kept).
		drawn = t.draw
		r.barrier(drawn, accessColorAttachmentWrite, accessTransferRead, layout, layoutTransferSrcOptimal,
			stageColorAttachmentOutput, stageTransfer, queueFamilyIgnored, queueFamilyIgnored)
		layout = layoutTransferSrcOptimal
		from, to := queueFamilyIgnored, queueFamilyIgnored
		if t.shown && r.foreign {
			from, to = queueFamilyForeign, r.family
		}
		r.barrier(t.image, 0, accessTransferWrite, layoutUndefined, layoutTransferDstOptimal,
			stageTopOfPipe, stageTransfer, from, to)
		color := imageSubresourceLayers{aspectMask: aspectColor, layerCount: 1}
		region := imageCopy{srcSubresource: color, dstSubresource: color, extent: extent3D{uint32(r.width), uint32(r.height), 1}}
		callVoid(r.vk.CmdCopyImage, cmd, uintptr(drawn), layoutTransferSrcOptimal, uintptr(t.image), layoutTransferDstOptimal, 1, uintptr(unsafe.Pointer(&region)))
	}
	var readback hostBuffer
	capture := r.capture
	r.capture = nil
	if capture != nil && !t.copy {
		capture(nil, errors.New("vulkan: the window's images cannot be copied from on this GPU and compositor"))
		capture = nil
	}
	if capture != nil {
		if err := r.ensureHost(&readback, r.width*r.height*4, bufferUsageTransferDst); err != nil {
			call(r.vk.EndCommandBuffer, cmd)
			capture(nil, err)
			return err
		}
		defer r.freeHost(&readback)
		if layout != layoutTransferSrcOptimal {
			r.barrier(drawn, accessColorAttachmentWrite, accessTransferRead, layout, layoutTransferSrcOptimal,
				stageColorAttachmentOutput, stageTransfer, queueFamilyIgnored, queueFamilyIgnored)
			layout = layoutTransferSrcOptimal
		}
		region := bufferImageCopy{aspectMask: aspectColor, layerCount: 1, imageExtent: extent3D{uint32(r.width), uint32(r.height), 1}}
		callVoid(r.vk.CmdCopyImageToBuffer, cmd, uintptr(drawn), layoutTransferSrcOptimal, uintptr(readback.buf), 1, uintptr(unsafe.Pointer(&region)))
		// Make the copy visible to the CPU, which reads it after the fence.
		mb := memoryBarrier{sType: stMemoryBarrier, srcAccessMask: accessTransferWrite, dstAccessMask: accessHostRead}
		callVoid(r.vk.CmdPipelineBarrier, cmd, stageTransfer, stageHost, 0, 1, uintptr(unsafe.Pointer(&mb)), 0, 0, 0, 0)
	}
	// Hand the image to the compositor: VK_QUEUE_FAMILY_FOREIGN_EXT is the
	// family of a consumer outside Vulkan.
	if t.shared {
		r.barrier(t.image, accessTransferWrite, 0, layoutTransferDstOptimal, layoutGeneral,
			stageTransfer, stageBottomOfPipe, src, dst)
	} else {
		r.barrier(t.image, accessColorAttachmentWrite|accessTransferRead, 0, layout, layoutGeneral,
			stageColorAttachmentOutput|stageTransfer, stageBottomOfPipe, src, dst)
	}
	if err := check("vkEndCommandBuffer", call(r.vk.EndCommandBuffer, cmd)); err != nil {
		return err
	}
	if err := r.submit(); err != nil {
		if capture != nil {
			capture(nil, err)
		}
		return err
	}
	t.shown = true
	r.commitUploads()
	if capture != nil {
		capture(r.readBack(readback), nil)
	}
	return r.surface.Present(t.buf)
}

// submit submits the command buffer and waits for it to finish.
func (r *Renderer) submit() error {
	cmd := r.cmd
	si := submitInfo{sType: stSubmitInfo, commandBufferCount: 1, pCommandBuffers: &cmd}
	if err := check("vkQueueSubmit", call(r.vk.QueueSubmit, r.queue, 1, uintptr(unsafe.Pointer(&si)), uintptr(r.fence))); err != nil {
		return err
	}
	fence := r.fence
	res := call(r.vk.WaitForFences, r.dev, 1, uintptr(unsafe.Pointer(&fence)), 1, uintptr(fenceTimeout.Nanoseconds()))
	if res == vkTimeout {
		return fmt.Errorf("%w: vulkan: a frame did not finish within %v", render.ErrDeviceLost, fenceTimeout)
	}
	if err := check("vkWaitForFences", res); err != nil {
		return err
	}
	return check("vkResetFences", call(r.vk.ResetFences, r.dev, 1, uintptr(unsafe.Pointer(&fence))))
}

func (r *Renderer) barrier(img uint64, srcAccess, dstAccess, oldLayout, newLayout, srcStage, dstStage, srcFamily, dstFamily uint32) {
	b := imageMemoryBarrier{sType: stImageMemoryBarrier, srcAccessMask: srcAccess, dstAccessMask: dstAccess,
		oldLayout: oldLayout, newLayout: newLayout, srcQueueFamilyIndex: srcFamily, dstQueueFamilyIndex: dstFamily,
		image: img, subresourceRange: colorRange}
	callVoid(r.vk.CmdPipelineBarrier, r.cmd, uintptr(srcStage), uintptr(dstStage), 0, 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&b)))
}

// writeInstances copies the frame's instances into the instance buffer, one
// section per pipeline, and returns where each section starts.
func (r *Renderer) writeInstances() ([pipeCount]int, error) {
	var offsets [pipeCount]int
	type section struct {
		pipe pipelineKind
		data unsafe.Pointer
		size int
	}
	sections := []section{
		{pipeQuad, unsafe.Pointer(unsafe.SliceData(r.quads)), len(r.quads) * int(unsafe.Sizeof(quadInstance{}))},
		{pipeMask, unsafe.Pointer(unsafe.SliceData(r.masks)), len(r.masks) * int(unsafe.Sizeof(maskInstance{}))},
		{pipeSubpixel, unsafe.Pointer(unsafe.SliceData(r.subs)), len(r.subs) * int(unsafe.Sizeof(maskInstance{}))},
		{pipeImage, unsafe.Pointer(unsafe.SliceData(r.images)), len(r.images) * int(unsafe.Sizeof(imageInstance{}))},
	}
	total := 0
	for _, s := range sections {
		offsets[s.pipe] = total
		total += s.size
	}
	if total == 0 {
		return offsets, nil
	}
	if err := r.ensureHost(&r.instances, total, bufferUsageVertexBuffer); err != nil {
		return offsets, err
	}
	dst := unsafe.Slice((*byte)(r.instances.data), r.instances.size)
	for _, s := range sections {
		if s.size > 0 {
			copy(dst[offsets[s.pipe]:], unsafe.Slice((*byte)(s.data), s.size))
		}
	}
	return offsets, nil
}

// readBack converts a copied frame (B8G8R8A8) to an opaque RGBA image, as the
// Direct3D 11 renderer's capture does.
func (r *Renderer) readBack(b hostBuffer) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, r.width, r.height))
	src := unsafe.Slice((*byte)(b.data), r.width*r.height*4)
	for y := range r.height {
		row := src[y*r.width*4:]
		dst := img.Pix[y*img.Stride:]
		for x := range r.width {
			dst[x*4+0] = row[x*4+2]
			dst[x*4+1] = row[x*4+1]
			dst[x*4+2] = row[x*4+0]
			dst[x*4+3] = 255
		}
	}
	return img
}
