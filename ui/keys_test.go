package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/icons"
)

// Icon, Image, Field, Modal, DropdownMenu and MenuBar take a Key, which names
// the node they build.
func TestMoreElementsTakeAKey(t *testing.T) {
	img := testImage()
	cases := []struct {
		name string
		el   Element
	}{
		{"Icon", Icon(icons.Check).Key("k")},
		{"Image", Image(img).Alt("dot").Key("k")},
		{"Field", Field("Name", TextInput(NewSignal("")).Label("Name")).Key("k")},
		{"Modal", Modal("Title", Text("Body")).Key("k")},
		{"DropdownMenu", DropdownMenu(Button("Open"), func() []MenuItem { return []MenuItem{{Label: "A"}} }).Key("k")},
		{"MenuBar", MenuBar(TopMenu{Label: "File", Items: func() []MenuItem { return []MenuItem{{Label: "New"}} }}).Key("k")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newAccessRig(func(ctx *Context) Element { return Div().Column().Child(Text("before")).Child(c.el) })
			if r.rt.findKey(r.rt.tree, "k") == nil {
				t.Fatalf("%s: no node has the key", c.name)
			}
		})
	}
}
