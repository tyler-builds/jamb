package main

import (
	"image"
	"image/color"
)

// trayIcon draws the notification area icon: a tile whose bar rises with the
// processor load, from 0 to 100, green under 60, amber under 85, and red above.
// Windows scales it to the small icon size.
func trayIcon(load float64) image.Image {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	tile := color.RGBA{0x1f, 0x29, 0x37, 0xff}
	bar := color.RGBA{0x34, 0xd3, 0x99, 0xff}
	switch {
	case load >= 85:
		bar = color.RGBA{0xf8, 0x71, 0x71, 0xff}
	case load >= 60:
		bar = color.RGBA{0xfb, 0xbf, 0x24, 0xff}
	}
	top := size - 3 - int(float64(size-6)*max(0, min(load, 100))/100)
	for y := range size {
		for x := range size {
			if !inRounded(x, y, size, 6) {
				continue
			}
			c := tile
			if x >= 7 && x < size-7 && y >= top && y < size-3 {
				c = bar
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// badgeIcon draws the taskbar badge of an active alert: a red dot with a white
// ring, so it stands out on any icon.
func badgeIcon() image.Image {
	const size = 16
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			dx, dy := float64(x)-7.5, float64(y)-7.5
			switch d := dx*dx + dy*dy; {
			case d <= 5.5*5.5:
				img.SetRGBA(x, y, color.RGBA{0xdc, 0x26, 0x26, 0xff})
			case d <= 7.5*7.5:
				img.SetRGBA(x, y, color.RGBA{0xff, 0xff, 0xff, 0xff})
			}
		}
	}
	return img
}

// inRounded reports whether pixel (x, y) is inside a square of side size with
// corners of radius r.
func inRounded(x, y, size, r int) bool {
	cx := min(max(x, r), size-1-r)
	cy := min(max(y, r), size-1-r)
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}
