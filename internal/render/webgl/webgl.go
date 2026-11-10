//go:build js

// Package webgl draws scenes with WebGL 2 into a canvas of a web page.
//
// It draws what the Vulkan and Direct3D 11 renderers draw, with their shaders
// ported to GLSL ES 3.00 (see shaders.go). Each call into WebGL is a call from
// WebAssembly into JavaScript, so a frame makes few: the instances of the
// whole frame are written into one float texture with a single upload, and
// each batch of instances is then drawn with a uniform for where its
// instances start and one instanced draw. Vertex shaders read their instance
// with texelFetch, and there are no vertex buffers.
package webgl

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sync/atomic"
	"syscall/js"
	"time"
	"unsafe"

	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/scene"
)

// The instances, in texels of four floats; the layouts are the Vulkan
// renderer's, and must match the shaders' inst reads.
type quadInstance struct {
	Rect, Clip, Color, Border, Radii, Params, Color2, Grad, Sides, CShape, CRadii [4]float32
}

type maskInstance struct {
	Rect, UV, Color, Clip, CShape, CRadii [4]float32
}

type imageInstance struct {
	Rect, UV, Clip, Radii, Params, CShape, CRadii [4]float32
}

type pipelineKind uint8

const (
	pipeQuad pipelineKind = iota
	pipeMask
	pipeImage
	pipeCount
)

// stride is the texels of an instance of each pipeline.
var stride = [pipeCount]int{
	pipeQuad:  int(unsafe.Sizeof(quadInstance{}) / 16),
	pipeMask:  int(unsafe.Sizeof(maskInstance{}) / 16),
	pipeImage: int(unsafe.Sizeof(imageInstance{}) / 16),
}

// command is a batch of instances drawn with one pipeline and texture. start
// is its first texel in the frame's data.
type command struct {
	pipe         pipelineKind
	start, count int
	tex          *scene.Texture
}

// clipState is one level of the clip stack, as in the other renderers.
type clipState struct {
	rect  scene.Rect
	shape scene.Rect
	radii scene.Corners
}

// glTexture is the GPU copy of a scene.Texture.
type glTexture struct {
	tex           js.Value
	width, height int
	version       uint64
	used          time.Time // when a frame last drew it
}

// Textures no frame has drawn for textureIdle are released, as in the other
// renderers.
const (
	textureIdle  = time.Minute
	textureSweep = 10 * time.Second
)

type program struct {
	prog           js.Value
	viewport, base js.Value // uniform locations
	inst, tex      js.Value
	viewW, viewH   int // the viewport last set, to set it once per size
	hasTex         bool
}

// Renderer implements render.Renderer with WebGL 2 for one canvas.
type Renderer struct {
	canvas, gl js.Value
	lost       atomic.Bool // the context was lost, and not restored yet
	listeners  []js.Func

	progs    [pipeCount]program
	dataTex  js.Value // the instance texture (see data)
	dataRows int      // its height

	// data holds the frame's instances, in paint order, as texels of four
	// floats; dataJS is the JavaScript copy it is uploaded from, as floats
	// and as the bytes that js.CopyBytesToJS writes.
	data          []float32
	dataJS        js.Value // Float32Array
	dataBytes     js.Value // Uint8Array over the same buffer
	staging       []byte   // texture uploads
	stagingJS     js.Value // Uint8Array
	textures      map[uint64]*glTexture
	lastSweep     time.Time
	width, height int
	clear         scene.Color
	clips         []clipState
	commands      []command

	capture func(*image.RGBA, error)
}

var _ render.Renderer = (*Renderer)(nil)

// ErrNoWebGL2 is returned by New when the page cannot draw with WebGL 2.
var ErrNoWebGL2 = errors.New("webgl: the browser has no WebGL 2")

// New makes a renderer that draws into canvas. The canvas must have no other
// context.
func New(canvas js.Value) (*Renderer, error) {
	gl := canvas.Call("getContext", "webgl2", map[string]any{
		"alpha": false, "antialias": false, "depth": false, "stencil": false,
		"premultipliedAlpha": true, "preserveDrawingBuffer": false,
	})
	if !gl.Truthy() {
		return nil, ErrNoWebGL2
	}
	r := &Renderer{canvas: canvas, gl: gl, textures: map[uint64]*glTexture{}}
	// A lost context (the GPU was reset, or the browser took it back) is
	// restored by the browser only if the loss is prevented.
	lost := js.FuncOf(func(_ js.Value, args []js.Value) any {
		args[0].Call("preventDefault")
		r.lost.Store(true)
		return nil
	})
	r.listeners = append(r.listeners, lost)
	canvas.Call("addEventListener", "webglcontextlost", lost)
	if err := r.setup(); err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
}

// Describe names what the renderer draws with.
func (r *Renderer) Describe() string {
	if ext := r.gl.Call("getExtension", "WEBGL_debug_renderer_info"); ext.Truthy() {
		if s := r.gl.Call("getParameter", ext.Get("UNMASKED_RENDERER_WEBGL")); s.Type() == js.TypeString {
			return "WebGL 2 (" + s.String() + ")"
		}
	}
	return "WebGL 2"
}

// setup makes the programs and the instance texture.
func (r *Renderer) setup() error {
	gl := r.gl
	srcs := [pipeCount][2]string{pipeQuad: {quadVert, quadFrag}, pipeMask: {maskVert, maskFrag}, pipeImage: {imageVert, imageFrag}}
	for i, s := range srcs {
		p, err := r.link(s[0], s[1])
		if err != nil {
			return err
		}
		pr := program{prog: p, hasTex: pipelineKind(i) != pipeQuad}
		pr.viewport = gl.Call("getUniformLocation", p, "u_viewport")
		pr.base = gl.Call("getUniformLocation", p, "u_base")
		gl.Call("useProgram", p)
		gl.Call("uniform1i", gl.Call("getUniformLocation", p, "u_inst"), 0)
		if pr.hasTex {
			gl.Call("uniform1i", gl.Call("getUniformLocation", p, "u_tex"), 1)
		}
		r.progs[i] = pr
	}
	r.dataTex = gl.Call("createTexture")
	r.dataRows = 0
	gl.Call("pixelStorei", gl.Get("UNPACK_ALIGNMENT"), 1)
	gl.Call("enable", gl.Get("BLEND"))
	gl.Call("blendFunc", gl.Get("ONE"), gl.Get("ONE_MINUS_SRC_ALPHA"))
	gl.Call("disable", gl.Get("DEPTH_TEST"))
	return nil
}

func (r *Renderer) link(vs, fs string) (js.Value, error) {
	gl := r.gl
	p := gl.Call("createProgram")
	for _, s := range []struct {
		kind string
		src  string
	}{{"VERTEX_SHADER", vs}, {"FRAGMENT_SHADER", fs}} {
		sh := gl.Call("createShader", gl.Get(s.kind))
		gl.Call("shaderSource", sh, s.src)
		gl.Call("compileShader", sh)
		if !gl.Call("getShaderParameter", sh, gl.Get("COMPILE_STATUS")).Bool() && !gl.Call("isContextLost").Bool() {
			return js.Value{}, fmt.Errorf("webgl: compiling a shader: %s", gl.Call("getShaderInfoLog", sh).String())
		}
		gl.Call("attachShader", p, sh)
		gl.Call("deleteShader", sh)
	}
	gl.Call("linkProgram", p)
	if !gl.Call("getProgramParameter", p, gl.Get("LINK_STATUS")).Bool() && !gl.Call("isContextLost").Bool() {
		return js.Value{}, fmt.Errorf("webgl: linking a program: %s", gl.Call("getProgramInfoLog", p).String())
	}
	return p, nil
}

var errLost = fmt.Errorf("%w: webgl: the WebGL context was lost", render.ErrDeviceLost)

// Recreate makes the programs and textures again after the context was lost,
// once the browser has restored it.
func (r *Renderer) Recreate() error {
	if r.gl.Call("isContextLost").Bool() {
		return errors.New("webgl: the WebGL context is not restored yet")
	}
	r.textures = map[uint64]*glTexture{}
	r.progs = [pipeCount]program{}
	if err := r.setup(); err != nil {
		return err
	}
	r.lost.Store(false)
	return nil
}

// Close releases the renderer's listeners. The context goes with the canvas.
func (r *Renderer) Close() {
	for _, f := range r.listeners {
		r.canvas.Call("removeEventListener", "webglcontextlost", f)
		f.Release()
	}
	r.listeners = nil
}

func (r *Renderer) BeginFrame(width, height int, clear scene.Color) error {
	if width <= 0 || height <= 0 {
		return errors.New("webgl: BeginFrame called with an empty size")
	}
	if r.lost.Load() {
		return errLost
	}
	r.width, r.height, r.clear = width, height, clear
	r.clips = append(r.clips[:0], clipState{rect: scene.Rect{W: float32(width), H: float32(height)}})
	r.data = r.data[:0]
	r.commands = r.commands[:0]
	return nil
}

func (r *Renderer) clipInstance() (clip, shape, radii [4]float32) {
	c := r.clips[len(r.clips)-1]
	return [4]float32{c.rect.X, c.rect.Y, c.rect.Right(), c.rect.Bottom()}, rect4(c.shape), corners4(c.radii)
}

func (r *Renderer) PushClip(c scene.Rect) {
	top := r.clips[len(r.clips)-1]
	r.clips = append(r.clips, clipState{rect: top.rect.Intersect(c), shape: top.shape, radii: top.radii})
}

func (r *Renderer) PushRoundedClip(c scene.Rect, radii scene.Corners) {
	top := r.clips[len(r.clips)-1]
	r.clips = append(r.clips, clipState{rect: top.rect.Intersect(c), shape: c, radii: radii})
}

func (r *Renderer) PopClip() {
	if len(r.clips) > 1 {
		r.clips = r.clips[:len(r.clips)-1]
	}
}

// add appends an instance to the frame's data and to the batch it draws in.
func add[T quadInstance | maskInstance | imageInstance](r *Renderer, p pipelineKind, tex *scene.Texture, in *T) {
	start := len(r.data) / 4
	r.data = append(r.data, unsafe.Slice((*float32)(unsafe.Pointer(in)), unsafe.Sizeof(*in)/4)...)
	if n := len(r.commands); n > 0 {
		last := &r.commands[n-1]
		if last.pipe == p && last.tex == tex && last.start+last.count*stride[p] == start {
			last.count++
			return
		}
	}
	r.commands = append(r.commands, command{pipe: p, start: start, count: 1, tex: tex})
}

func rect4(b scene.Rect) [4]float32       { return [4]float32{b.X, b.Y, b.W, b.H} }
func color4(c scene.Color) [4]float32     { return [4]float32{c.R, c.G, c.B, c.A} }
func corners4(c scene.Corners) [4]float32 { return [4]float32{c.TL, c.TR, c.BR, c.BL} }

func (r *Renderer) DrawQuads(qs []scene.Quad) {
	clip, cshape, cradii := r.clipInstance()
	for _, q := range qs {
		in := quadInstance{
			Rect: rect4(q.Bounds), Clip: clip, Color: color4(q.Background), Border: color4(q.BorderColor),
			Radii: corners4(q.Radii), Params: quadParams(q), Color2: color4(q.Color2), Grad: gradient4(q),
			Sides: quadSides(q), CShape: cshape, CRadii: cradii,
		}
		add(r, pipeQuad, nil, &in)
	}
}

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

func quadSides(q scene.Quad) [4]float32 {
	if s := q.BorderSides; s != (scene.Sides{}) {
		return [4]float32{s.Top, s.Right, s.Bottom, s.Left}
	}
	w := q.BorderWidth
	return [4]float32{w, w, w, w}
}

func gradient4(q scene.Quad) [4]float32 {
	if !q.Gradient {
		return [4]float32{}
	}
	rad := float64(q.Angle) * math.Pi / 180
	return [4]float32{float32(math.Sin(rad)), float32(-math.Cos(rad)), q.Stop0, q.Stop1}
}

// DrawGlyphs draws a run's glyphs. A page has grayscale text only (see
// text.Atlas): a subpixel atlas is drawn as grayscale from its first channel.
func (r *Renderer) DrawGlyphs(run scene.GlyphRun) {
	if len(run.Glyphs) == 0 || run.Texture == nil {
		return
	}
	clip, cshape, cradii := r.clipInstance()
	tw, th := float32(run.Texture.Width), float32(run.Texture.Height)
	for _, g := range run.Glyphs {
		in := maskInstance{
			Rect:  rect4(g.Bounds),
			UV:    [4]float32{g.Src.X / tw, g.Src.Y / th, (g.Src.X + g.Src.W) / tw, (g.Src.Y + g.Src.H) / th},
			Color: color4(g.Color), Clip: clip, CShape: cshape, CRadii: cradii,
		}
		add(r, pipeMask, run.Texture, &in)
	}
}

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
		in := imageInstance{
			Rect:   rect4(im.Bounds),
			UV:     [4]float32{src.X / tw, src.Y / th, (src.X + src.W) / tw, (src.Y + src.H) / th},
			Clip:   clip,
			Radii:  corners4(im.Radii),
			Params: [4]float32{im.Opacity},
			CShape: cshape,
			CRadii: cradii,
		}
		add(r, pipeImage, im.Texture, &in)
	}
}

func (r *Renderer) CaptureNextFrame(fn func(*image.RGBA, error)) { r.capture = fn }

// EndFrame draws the frame. The browser shows it when the page's script
// returns to the event loop.
func (r *Renderer) EndFrame() error {
	if r.lost.Load() {
		r.fail(errLost)
		return errLost
	}
	gl := r.gl
	if w, h := r.canvas.Get("width").Int(), r.canvas.Get("height").Int(); w != r.width || h != r.height {
		r.canvas.Set("width", r.width)
		r.canvas.Set("height", r.height)
	}
	now := time.Now()
	r.unifyTextures()
	r.uploadTextures(now)
	r.uploadData()

	gl.Call("viewport", 0, 0, r.width, r.height)
	c := r.clear
	gl.Call("clearColor", c.R*c.A, c.G*c.A, c.B*c.A, c.A)
	gl.Call("clear", gl.Get("COLOR_BUFFER_BIT"))
	gl.Call("activeTexture", gl.Get("TEXTURE0"))
	gl.Call("bindTexture", gl.Get("TEXTURE_2D"), r.dataTex)
	gl.Call("activeTexture", gl.Get("TEXTURE1"))
	strip := gl.Get("TRIANGLE_STRIP")
	current := pipeCount
	var bound *scene.Texture
	for _, cmd := range r.commands {
		p := &r.progs[cmd.pipe]
		if cmd.pipe != current {
			current = cmd.pipe
			gl.Call("useProgram", p.prog)
			if p.viewW != r.width || p.viewH != r.height {
				p.viewW, p.viewH = r.width, r.height
				gl.Call("uniform2f", p.viewport, r.width, r.height)
			}
		}
		if cmd.tex != nil && cmd.tex != bound {
			bound = cmd.tex
			gl.Call("bindTexture", gl.Get("TEXTURE_2D"), r.textures[cmd.tex.ID].tex)
		}
		gl.Call("uniform1i", p.base, cmd.start)
		gl.Call("drawArraysInstanced", strip, 0, 4, cmd.count)
	}
	if now.Sub(r.lastSweep) >= textureSweep {
		r.lastSweep = now
		r.sweepTextures(now)
	}
	if r.capture != nil {
		fn := r.capture
		r.capture = nil
		fn(r.readBack(), nil)
	}
	if gl.Call("isContextLost").Bool() {
		r.lost.Store(true)
		return errLost
	}
	return nil
}

func (r *Renderer) fail(err error) {
	if r.capture != nil {
		r.capture(nil, err)
		r.capture = nil
	}
}

// uploadData copies the frame's instances into the instance texture, which
// grows to hold them.
func (r *Renderer) uploadData() {
	if len(r.data) == 0 {
		return
	}
	gl := r.gl
	rows := (len(r.data)/4 + dataWidth - 1) / dataWidth
	n := rows * dataWidth * 4 // floats uploaded: whole rows
	if r.dataJS.IsUndefined() || r.dataJS.Get("length").Int() < n {
		buf := js.Global().Get("ArrayBuffer").New(n * 4 * 2) // room to grow
		r.dataJS = js.Global().Get("Float32Array").New(buf)
		r.dataBytes = js.Global().Get("Uint8Array").New(buf)
	}
	js.CopyBytesToJS(r.dataBytes, unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(r.data))), len(r.data)*4))
	tex2d := gl.Get("TEXTURE_2D")
	gl.Call("activeTexture", gl.Get("TEXTURE0"))
	gl.Call("bindTexture", tex2d, r.dataTex)
	if rows > r.dataRows {
		r.dataRows = max(rows, r.dataRows*2, 4)
		gl.Call("texImage2D", tex2d, 0, gl.Get("RGBA32F"), dataWidth, r.dataRows, 0, gl.Get("RGBA"), gl.Get("FLOAT"), nil)
		nearest := gl.Get("NEAREST")
		gl.Call("texParameteri", tex2d, gl.Get("TEXTURE_MIN_FILTER"), nearest)
		gl.Call("texParameteri", tex2d, gl.Get("TEXTURE_MAG_FILTER"), nearest)
	}
	gl.Call("texSubImage2D", tex2d, 0, 0, 0, dataWidth, rows, gl.Get("RGBA"), gl.Get("FLOAT"), r.dataJS)
}

// unifyTextures makes each command draw the newest texture of its ID, as the
// Vulkan renderer does: a glyph atlas that grew during the frame is a bigger
// texture with the same ID, which holds the old one's pixels in its top-left
// corner, and the older one's UVs are scaled to it.
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
		return
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
		// The UV is the second texel of a mask and of an image instance.
		for j := range c.count {
			uv := r.data[(c.start+j*stride[c.pipe]+1)*4:][:4]
			uv[0], uv[1], uv[2], uv[3] = uv[0]*sx, uv[1]*sy, uv[2]*sx, uv[3]*sy
		}
		c.tex = n
	}
}

// uploadTextures makes the GPU textures of the frame's new textures, and
// uploads what changed of the others.
func (r *Renderer) uploadTextures(now time.Time) {
	gl := r.gl
	tex2d := gl.Get("TEXTURE_2D")
	gl.Call("activeTexture", gl.Get("TEXTURE1"))
	for _, c := range r.commands {
		t := c.tex
		if t == nil {
			continue
		}
		g := r.textures[t.ID]
		if g != nil && g.version == t.Version && g.width == t.Width && g.height == t.Height {
			g.used = now
			continue
		}
		x0, y0, x1, y1 := 0, 0, t.Width, t.Height
		fresh := g == nil || g.width != t.Width || g.height != t.Height
		if fresh {
			if g != nil {
				gl.Call("deleteTexture", g.tex)
			}
			g = &glTexture{tex: gl.Call("createTexture"), width: t.Width, height: t.Height}
			r.textures[t.ID] = g
			gl.Call("bindTexture", tex2d, g.tex)
			filter := gl.Get("NEAREST") // glyph masks are drawn texel for pixel
			if t.Format == scene.FormatRGBA8 {
				filter = gl.Get("LINEAR")
			}
			gl.Call("texParameteri", tex2d, gl.Get("TEXTURE_MIN_FILTER"), filter)
			gl.Call("texParameteri", tex2d, gl.Get("TEXTURE_MAG_FILTER"), filter)
			gl.Call("texParameteri", tex2d, gl.Get("TEXTURE_WRAP_S"), gl.Get("CLAMP_TO_EDGE"))
			gl.Call("texParameteri", tex2d, gl.Get("TEXTURE_WRAP_T"), gl.Get("CLAMP_TO_EDGE"))
			internal, format := r.formats(t)
			gl.Call("texImage2D", tex2d, 0, internal, t.Width, t.Height, 0, format, gl.Get("UNSIGNED_BYTE"), nil)
		} else if t.DirtyX1 > t.DirtyX0 && t.DirtyY1 > t.DirtyY0 {
			x0, y0, x1, y1 = t.DirtyX0, t.DirtyY0, min(t.DirtyX1, t.Width), min(t.DirtyY1, t.Height)
			gl.Call("bindTexture", tex2d, g.tex)
		} else {
			g.version, g.used = t.Version, now
			continue
		}
		g.version, g.used = t.Version, now
		r.uploadRegion(t, x0, y0, x1, y1)
		t.ClearDirty()
	}
}

// formats returns a texture's internal format and the format of its pixels.
// A subpixel mask, which a page does not make, is uploaded whole and drawn
// from its first channel.
func (r *Renderer) formats(t *scene.Texture) (js.Value, js.Value) {
	gl := r.gl
	if t.BytesPerPixel() == 4 {
		return gl.Get("RGBA8"), gl.Get("RGBA")
	}
	return gl.Get("R8"), gl.Get("RED")
}

// uploadRegion copies a region of t's pixels into its bound texture.
func (r *Renderer) uploadRegion(t *scene.Texture, x0, y0, x1, y1 int) {
	gl := r.gl
	bpp := t.BytesPerPixel()
	row := (x1 - x0) * bpp
	n := row * (y1 - y0)
	if cap(r.staging) < n {
		r.staging = make([]byte, n)
		r.stagingJS = js.Global().Get("Uint8Array").New(n)
	}
	buf := r.staging[:n]
	for y := y0; y < y1; y++ {
		copy(buf[(y-y0)*row:][:row], t.Pix[(y*t.Width+x0)*bpp:])
	}
	js.CopyBytesToJS(r.stagingJS, buf)
	_, format := r.formats(t)
	// The region is the start of the staging array (the last argument is its
	// offset there); a view of just the region would be one more JavaScript
	// value for each upload, which TinyGo's collector counts (see
	// platform/web's host.go).
	gl.Call("texSubImage2D", gl.Get("TEXTURE_2D"), 0, x0, y0, x1-x0, y1-y0, format, gl.Get("UNSIGNED_BYTE"),
		r.stagingJS, 0)
}

// sweepTextures releases the textures no frame has drawn for textureIdle.
func (r *Renderer) sweepTextures(now time.Time) {
	for id, g := range r.textures {
		if now.Sub(g.used) > textureIdle {
			r.gl.Call("deleteTexture", g.tex)
			delete(r.textures, id)
		}
	}
}

// readBack returns the frame drawn, from the bottom row up as WebGL reads it.
func (r *Renderer) readBack() *image.RGBA {
	w, h := r.width, r.height
	px := js.Global().Get("Uint8Array").New(w * h * 4)
	r.gl.Call("readPixels", 0, 0, w, h, r.gl.Get("RGBA"), r.gl.Get("UNSIGNED_BYTE"), px)
	raw := make([]byte, w*h*4)
	js.CopyBytesToGo(raw, px)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		copy(img.Pix[y*img.Stride:][:w*4], raw[(h-1-y)*w*4:][:w*4])
	}
	return img
}
