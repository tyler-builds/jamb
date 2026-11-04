package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/icons"
)

// A scroll offset moves everything in the container by the same whole number of
// device pixels. Text and icons each round their position to the pixel grid, so
// a fractional offset used to move an item's icon a pixel more than its label,
// which showed in a list that scrolls by a pixel or two: the icons moved and the
// labels did not.
func TestScrollMovesTextAndIconsTogether(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, offset := range []float32{0.4, 1.3, 2.34, 5.6} {
			rows := Div().Column().Key("list").Height(Px(120)).ScrollY()
			for i := 0; i < 8; i++ {
				// A fractional row height puts the rows at fractional positions, none of
				// them exactly half a device pixel, where float noise decides the rounding.
				rows.Child(Div().Row().Align(Center).Gap(8).MinHeight(Px(17.37)).
					Child(Icon(icons.Star).Size(16).Key(vKey(i) + "/icon")).
					Child(Text("Item").Key(vKey(i) + "/label")))
			}
			rt := newTestRuntime(func(*Context) Element { return Div().Column().Child(rows) }, 300, 300, scale)
			now := time.Now()
			rt.Frame(now)
			list := keyNode(t, rt, "list")
			glyphTop := func(key string) float32 {
				r := rt.nodeRect(keyNode(t, rt, key))
				for _, g := range rt.scene.Glyphs {
					cy := g.Bounds.Y + g.Bounds.H/2
					if cy >= r.Y*scale && cy < (r.Y+r.H)*scale && g.Bounds.X >= r.X*scale-1 && g.Bounds.X < (r.X+r.W)*scale {
						return g.Bounds.Y
					}
				}
				t.Fatalf("no glyph drawn for %s", key)
				return 0
			}
			var before [6][2]float32
			for i := range before {
				before[i] = [2]float32{glyphTop(vKey(i) + "/icon"), glyphTop(vKey(i) + "/label")}
			}
			list.st.targetY, list.st.scrollY = offset, offset
			rt.needPaint = true
			rt.Frame(now.Add(time.Second))
			for i := range before {
				di := before[i][0] - glyphTop(vKey(i)+"/icon")
				dl := before[i][1] - glyphTop(vKey(i)+"/label")
				if di != dl {
					t.Errorf("scale %v, offset %v, row %d: icon moved %v device px, label %v", scale, offset, i, di, dl)
				}
			}
		}
	}
}
