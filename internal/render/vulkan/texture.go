//go:build linux && (amd64 || arm64)

package vulkan

import (
	"slices"
	"time"
	"unsafe"

	"github.com/tyler-builds/jamb/internal/scene"
)

// gpuTexture is the GPU copy of a scene.Texture.
type gpuTexture struct {
	image, mem, view uint64
	set, pool        uint64 // the descriptor set that binds it, and its pool
	width, height    int
	version          uint64
	// ready is set once a frame that uploaded all of it finished, so its
	// layout is SHADER_READ_ONLY_OPTIMAL and a change uploads only the dirty
	// region.
	ready bool
	used  time.Time // when a frame last drew it (see sweepTextures)
}

// Textures no frame has drawn for textureIdle are released, checked every
// textureSweep, as in the Direct3D 11 renderer. A texture drawn again later is
// uploaded again from its scene.Texture.Pix.
const (
	textureIdle  = time.Minute
	textureSweep = 10 * time.Second
)

// texturesPerPool is the number of descriptor sets in each pool of texture
// sets; another pool is made when they are used up.
const texturesPerPool = 256

// upload is a region of a texture to copy from the staging buffer.
type upload struct {
	tex            *scene.Texture
	g              *gpuTexture
	fresh          bool // the image is new: its contents are undefined
	x0, y0, x1, y1 int
	offset         int // in the staging buffer
}

// uploadTextures records the copies of the frame's textures that are new or
// changed, after creating the images of new ones. The copies count once the
// frame has finished: see commitUploads.
func (r *Renderer) uploadTextures(now time.Time) error {
	r.unifyTextures()
	ups := r.uploads[:0]
	staged := 0
	for _, c := range r.commands {
		t := c.tex
		if t == nil || slices.ContainsFunc(ups, func(u upload) bool { return u.tex == t }) {
			continue
		}
		g := r.textures[t.ID]
		if g != nil {
			g.used = now
			if g.ready && g.version == t.Version {
				continue
			}
		}
		u := upload{tex: t, g: g}
		if g != nil && !g.ready && g.width == t.Width && g.height == t.Height {
			// Created by a frame that failed: upload all of it.
			u.fresh = true
			u.x0, u.y0, u.x1, u.y1 = 0, 0, t.Width, t.Height
		} else if g == nil || g.width != t.Width || g.height != t.Height {
			if g != nil {
				r.freeTexture(g)
				delete(r.textures, t.ID)
			}
			ng, err := r.newTexture(t)
			if err != nil {
				return err
			}
			ng.used = now
			r.textures[t.ID] = ng
			u.g, u.fresh = ng, true
			u.x0, u.y0, u.x1, u.y1 = 0, 0, t.Width, t.Height
		} else if t.DirtyX1 > t.DirtyX0 && t.DirtyY1 > t.DirtyY0 {
			u.x0, u.y0, u.x1, u.y1 = t.DirtyX0, t.DirtyY0, min(t.DirtyX1, t.Width), min(t.DirtyY1, t.Height)
		} else {
			g.version = t.Version
			continue
		}
		u.offset = staged
		staged += (u.x1 - u.x0) * (u.y1 - u.y0) * t.BytesPerPixel()
		staged = (staged + 15) &^ 15
		ups = append(ups, u)
	}
	r.uploads = ups
	if len(ups) == 0 {
		return nil
	}
	if err := r.ensureHost(&r.staging, staged, bufferUsageTransferSrc); err != nil {
		return err
	}
	dst := unsafe.Slice((*byte)(r.staging.data), r.staging.size)
	for _, u := range ups {
		t := u.tex
		bpp := t.BytesPerPixel()
		row := (u.x1 - u.x0) * bpp
		for y := u.y0; y < u.y1; y++ {
			src := t.Pix[(y*t.Width+u.x0)*bpp:]
			copy(dst[u.offset+(y-u.y0)*row:], src[:row])
		}
		old := uint32(layoutShaderReadOnlyOptimal)
		if u.fresh {
			old = layoutUndefined
		}
		r.barrier(u.g.image, accessShaderRead, accessTransferWrite, old, layoutTransferDstOptimal,
			stageFragmentShader, stageTransfer, queueFamilyIgnored, queueFamilyIgnored)
		region := bufferImageCopy{bufferOffset: uint64(u.offset), aspectMask: aspectColor, layerCount: 1,
			imageOffset: [3]int32{int32(u.x0), int32(u.y0), 0}, imageExtent: extent3D{uint32(u.x1 - u.x0), uint32(u.y1 - u.y0), 1}}
		callVoid(r.vk.CmdCopyBufferToImage, r.cmd, uintptr(r.staging.buf), uintptr(u.g.image), layoutTransferDstOptimal, 1, uintptr(unsafe.Pointer(&region)))
		r.barrier(u.g.image, accessTransferWrite, accessShaderRead, layoutTransferDstOptimal, layoutShaderReadOnlyOptimal,
			stageTransfer, stageFragmentShader, queueFamilyIgnored, queueFamilyIgnored)
	}
	return nil
}

// unifyTextures makes each command draw the newest texture of its ID. A glyph
// atlas that grows is replaced by a bigger texture with the same ID that holds
// the old pixels in its top-left corner (text.Atlas.grow), so a frame drawn
// while it grew draws both. They share one GPU texture, made at the newest
// size: uploading both would free the image an earlier copy in the frame
// writes to. The older texture's UVs are scaled to the bigger one.
func (r *Renderer) unifyTextures() {
	var newest map[uint64]*scene.Texture
	var first *scene.Texture
	for _, c := range r.commands {
		t := c.tex
		switch {
		case t == nil || t == first:
		case first == nil:
			first = t
		default:
			if newest == nil {
				newest = map[uint64]*scene.Texture{first.ID: first}
			}
			if n := newest[t.ID]; n == nil || t.Version > n.Version {
				newest[t.ID] = t
			}
		}
	}
	if newest == nil {
		return // one texture at most
	}
	for i := range r.commands {
		c := &r.commands[i]
		if c.tex == nil {
			continue
		}
		n := newest[c.tex.ID]
		if n == c.tex {
			continue
		}
		sx, sy := float32(c.tex.Width)/float32(n.Width), float32(c.tex.Height)/float32(n.Height)
		lo, hi := int(c.start), int(c.start+c.count)
		switch c.pipe {
		case pipeMask:
			scaleUVs(r.masks[lo:hi], sx, sy)
		case pipeSubpixel:
			scaleUVs(r.subs[lo:hi], sx, sy)
		case pipeImage:
			for j := lo; j < hi; j++ {
				scaleUV(&r.images[j].UV, sx, sy)
			}
		}
		c.tex = n
	}
}

func scaleUVs(ins []maskInstance, sx, sy float32) {
	for i := range ins {
		scaleUV(&ins[i].UV, sx, sy)
	}
}

func scaleUV(uv *[4]float32, sx, sy float32) {
	uv[0], uv[1], uv[2], uv[3] = uv[0]*sx, uv[1]*sy, uv[2]*sx, uv[3]*sy
}

// commitUploads marks the frame's uploads done, once the frame has finished.
// After a frame that failed they are not, so the next frame uploads them
// again.
func (r *Renderer) commitUploads() {
	for _, u := range r.uploads {
		u.g.version, u.g.ready = u.tex.Version, true
		u.tex.ClearDirty()
	}
	clear(r.uploads)
	r.uploads = r.uploads[:0]
}

// newTexture creates the image, view and descriptor set of a texture. Its
// contents are uploaded by the caller.
func (d *Device) newTexture(t *scene.Texture) (g *gpuTexture, err error) {
	g = &gpuTexture{width: t.Width, height: t.Height}
	defer func() {
		if err != nil {
			d.freeTexture(g)
			g = nil
		}
	}()
	format := uint32(formatR8Unorm)
	if t.Format == scene.FormatRGBA8 || t.Format == scene.FormatSubpixel8 {
		format = formatR8G8B8A8Unorm
	}
	ii := imageCreateInfo{sType: stImageCreateInfo, imageType: imageType2D, format: format,
		extent: extent3D{uint32(t.Width), uint32(t.Height), 1}, mipLevels: 1, arrayLayers: 1, samples: sampleCount1,
		tiling: imageTilingOptimal, usage: imageUsageSampled | imageUsageTransferDst, sharingMode: sharingExclusive, initialLayout: layoutUndefined}
	if err := check("vkCreateImage(texture)", call(d.vk.CreateImage, d.dev, uintptr(unsafe.Pointer(&ii)), 0, uintptr(unsafe.Pointer(&g.image)))); err != nil {
		return nil, err
	}
	var req memoryRequirements
	callVoid(d.vk.GetImageMemoryRequirements, d.dev, uintptr(g.image), uintptr(unsafe.Pointer(&req)))
	mt, ok := d.memoryType(req.memoryTypeBits, memoryPropertyDeviceLocal)
	if !ok {
		mt, _ = d.memoryType(req.memoryTypeBits, 0)
	}
	mi := memoryAllocateInfo{sType: stMemoryAllocateInfo, allocationSize: req.size, memoryTypeIndex: mt}
	if err := check("vkAllocateMemory(texture)", call(d.vk.AllocateMemory, d.dev, uintptr(unsafe.Pointer(&mi)), 0, uintptr(unsafe.Pointer(&g.mem)))); err != nil {
		return nil, err
	}
	if err := check("vkBindImageMemory(texture)", call(d.vk.BindImageMemory, d.dev, uintptr(g.image), uintptr(g.mem), 0)); err != nil {
		return nil, err
	}
	vi := imageViewCreateInfo{sType: stImageViewCreateInfo, image: g.image, viewType: imageViewType2D, format: format, subresourceRange: colorRange}
	if err := check("vkCreateImageView(texture)", call(d.vk.CreateImageView, d.dev, uintptr(unsafe.Pointer(&vi)), 0, uintptr(unsafe.Pointer(&g.view)))); err != nil {
		return nil, err
	}
	if err := d.allocTextureSet(g); err != nil {
		return nil, err
	}
	info := descriptorImageInfo{imageView: g.view, imageLayout: layoutShaderReadOnlyOptimal}
	w := writeDescriptorSet{sType: stWriteDescriptorSet, dstSet: g.set, descriptorCount: 1, descriptorType: descriptorTypeSampledImage, pImageInfo: &info}
	callVoid(d.vk.UpdateDescriptorSets, d.dev, 1, uintptr(unsafe.Pointer(&w)), 0, 0)
	return g, nil
}

// allocTextureSet allocates g's descriptor set from a pool with room, the
// newest first, or from a new pool when they are all full. Freed sets return
// to their pool, so textures that come and go reuse the pools.
func (d *Device) allocTextureSet(g *gpuTexture) error {
	for i := len(d.texPools) - 1; i >= 0; i-- {
		pool := d.texPools[i]
		ai := descriptorSetAllocateInfo{sType: stDescriptorSetAllocateInfo, descriptorPool: pool, descriptorSetCount: 1, pSetLayouts: &d.textureSets}
		switch res := call(d.vk.AllocateDescriptorSets, d.dev, uintptr(unsafe.Pointer(&ai)), uintptr(unsafe.Pointer(&g.set))); res {
		case vkSuccess:
			g.pool = pool
			return nil
		case vkErrorOutOfPoolMemory, vkErrorFragmentedPool:
		default:
			return check("vkAllocateDescriptorSets", res)
		}
	}
	size := descriptorPoolSize{typ: descriptorTypeSampledImage, descriptorCount: texturesPerPool}
	pi := descriptorPoolCreateInfo{sType: stDescriptorPoolCreateInfo, flags: descriptorPoolFreeSet, maxSets: texturesPerPool, poolSizeCount: 1, pPoolSizes: &size}
	var pool uint64
	if err := check("vkCreateDescriptorPool", call(d.vk.CreateDescriptorPool, d.dev, uintptr(unsafe.Pointer(&pi)), 0, uintptr(unsafe.Pointer(&pool)))); err != nil {
		return err
	}
	d.texPools = append(d.texPools, pool)
	ai := descriptorSetAllocateInfo{sType: stDescriptorSetAllocateInfo, descriptorPool: pool, descriptorSetCount: 1, pSetLayouts: &d.textureSets}
	if err := check("vkAllocateDescriptorSets", call(d.vk.AllocateDescriptorSets, d.dev, uintptr(unsafe.Pointer(&ai)), uintptr(unsafe.Pointer(&g.set)))); err != nil {
		return err
	}
	g.pool = pool
	return nil
}

// freeTexture releases a texture's GPU objects. No frame may be using it:
// each frame waits for the GPU before it returns.
func (d *Device) freeTexture(g *gpuTexture) {
	if d.dev == 0 {
		return
	}
	if g.set != 0 {
		callVoid(d.vk.FreeDescriptorSets, d.dev, uintptr(g.pool), 1, uintptr(unsafe.Pointer(&g.set)))
	}
	if g.view != 0 {
		callVoid(d.vk.DestroyImageView, d.dev, uintptr(g.view), 0)
	}
	if g.image != 0 {
		callVoid(d.vk.DestroyImage, d.dev, uintptr(g.image), 0)
	}
	if g.mem != 0 {
		callVoid(d.vk.FreeMemory, d.dev, uintptr(g.mem), 0)
	}
	*g = gpuTexture{}
}

// sweepTextures releases the textures that no window has drawn for
// textureIdle. The device is shared, so a texture that one window still shows
// is kept as long as that window draws.
func (d *Device) sweepTextures(now time.Time) {
	if now.Sub(d.lastSweep) < textureSweep {
		return
	}
	d.lastSweep = now
	for id, g := range d.textures {
		if now.Sub(g.used) > textureIdle {
			d.freeTexture(g)
			delete(d.textures, id)
		}
	}
}
