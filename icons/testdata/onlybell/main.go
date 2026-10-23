// Command onlybell draws one icon and names no other. The icons tests build it
// and check that the other icons are not linked into its binary.
package main

import (
	"image"

	"github.com/tyler-builds/jamb/icons"
)

func main() {
	icons.Bell.Rasterize(image.NewAlpha(image.Rect(0, 0, 24, 24)))
}
