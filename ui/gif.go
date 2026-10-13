package ui

import (
	"bytes"
	"image"
	"image/draw"
	"image/gif"
	"time"

	"github.com/tyler-builds/jamb/internal/scene"
)

// imageFrame is one frame of an animated image: the whole picture at that
// point, and how long it is shown.
type imageFrame struct {
	tex   *scene.Texture
	delay time.Duration
}

// minFrameDelay is the shortest time a GIF frame is shown. Browsers treat a
// delay under 20 ms as 100 ms, since many GIFs store 0 and mean "fast".
const (
	minFrameDelay  = 20 * time.Millisecond
	slowFrameDelay = 100 * time.Millisecond
)

// decodeAnimatedGIF decodes every frame of a GIF, composited as a viewer shows
// it (each frame drawn over the last, with its disposal applied), or returns
// nil for a GIF with one frame.
func decodeAnimatedGIF(data []byte) (*ImageSource, error) {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if len(g.Image) < 2 {
		return nil, nil
	}
	w, h := g.Config.Width, g.Config.Height
	if w <= 0 || h <= 0 {
		b := g.Image[0].Bounds()
		w, h = b.Max.X, b.Max.Y
	}
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	src := &ImageSource{width: w, height: h, loops: g.LoopCount}
	for i, frame := range g.Image {
		var saved *image.RGBA
		disposal := byte(0)
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}
		if disposal == gif.DisposalPrevious {
			saved = image.NewRGBA(canvas.Bounds())
			copy(saved.Pix, canvas.Pix)
		}
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		tex := scene.NewTexture(w, h, scene.FormatRGBA8)
		copy(tex.Pix, canvas.Pix) // NewRGBA is premultiplied, as the renderer expects
		delay := slowFrameDelay
		if i < len(g.Delay) {
			if d := time.Duration(g.Delay[i]) * 10 * time.Millisecond; d >= minFrameDelay {
				delay = d
			}
		}
		src.frames = append(src.frames, imageFrame{tex: tex, delay: delay})
		switch disposal {
		case gif.DisposalBackground:
			draw.Draw(canvas, frame.Bounds(), image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			canvas = saved
		}
	}
	src.tex = src.frames[0].tex
	return src, nil
}

// frameAt returns the frame of an animated image shown at elapsed time since it
// started, and how long until the next frame (0 once the animation has ended).
// loops follows gif.GIF.LoopCount: 0 repeats forever, -1 plays once, and n plays
// n+1 times.
func (s *ImageSource) frameAt(elapsed time.Duration) (tex *scene.Texture, next time.Duration) {
	var total time.Duration
	for _, f := range s.frames {
		total += f.delay
	}
	if total <= 0 {
		return s.tex, 0
	}
	var plays int64 // 0: forever
	switch {
	case s.loops < 0:
		plays = 1
	case s.loops > 0:
		plays = int64(s.loops) + 1
	}
	if plays > 0 && int64(elapsed/total) >= plays {
		return s.frames[len(s.frames)-1].tex, 0 // ended on the last frame
	}
	t := elapsed % total
	for _, f := range s.frames {
		if t < f.delay {
			return f.tex, f.delay - t
		}
		t -= f.delay
	}
	return s.frames[len(s.frames)-1].tex, 0
}
