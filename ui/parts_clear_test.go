package ui

import "testing"

// TestPartClearRemovesBackgroundDefault checks, for one part of each widget that
// has a built-in fill, that the part's Clear removes the fill, and that the
// default fill was there before the clear. It is the clear through the instance,
// the path an app uses for one widget.
func TestPartClearRemovesBackgroundDefault(t *testing.T) {
	clear := Style{Clear: ClearBackground}
	cases := []struct {
		name string
		// build returns the rig with the part's style set to st.
		build func(st Style) *accessRig
		// node returns the node that draws the part.
		node string
		key  string
	}{
		{"Button.Root", func(st Style) *accessRig {
			return newAccessRig(func(ctx *Context) Element {
				return partButton(ButtonPrimary, ButtonStyle{Root: st}, false)
			})
		}, "Button.Root", ""},
		{"Checkbox.Box", func(st Style) *accessRig {
			return newAccessRig(func(ctx *Context) Element {
				return partCheckbox(false, CheckOff, CheckboxStyle{Box: st}, false)
			})
		}, "Checkbox.Box", ""},
		{"Switch.Track", func(st Style) *accessRig {
			return newAccessRig(func(ctx *Context) Element { return partSwitch(false, SwitchStyle{Track: st}, false) })
		}, "Switch.Track", ""},
		{"ProgressBar.Track", func(st Style) *accessRig {
			return newAccessRig(func(ctx *Context) Element {
				return Div().Column().Child(ProgressBar(0.4).Styles(ProgressBarStyle{Track: st}))
			})
		}, "ProgressBar.Track", ""},
		{"Badge.Badge", func(st Style) *accessRig {
			return newAccessRig(func(ctx *Context) Element { return Badge(3).Styles(BadgeStyle{Badge: st}) })
		}, "Badge.Badge", ""},
		{"Modal.Panel", func(st Style) *accessRig {
			return newAccessRig(func(ctx *Context) Element { return partModal(ModalStyle{Panel: st}) })
		}, "Modal.Panel", ""},
		{"TextInput.Frame", func(st Style) *accessRig {
			sig := NewSignal("")
			return newAccessRig(func(ctx *Context) Element {
				return Div().Column().Child(TextInput(sig).Label("Name").Styles(FieldStyle{Frame: st}))
			})
		}, "TextInput.Frame", ""},
		{"Tree.Row", func(st Style) *accessRig {
			sel := NewSignal("src")
			return newAccessRig(func(ctx *Context) Element { return partTree(sel, TreeStyle{RowSelected: st}, nil, nil) })
		}, "Tree.Row", "src"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := c.build(Style{})
			if n := keyedOrPart(t, before, c.node, c.key); n.style.Background.IsZero() {
				t.Fatalf("%s has no default fill to clear", c.name)
			}
			after := c.build(clear)
			if n := keyedOrPart(t, after, c.node, c.key); !n.style.Background.IsZero() {
				t.Errorf("%s kept its fill %v after Clear", c.name, n.style.Background)
			}
		})
	}
}

// keyedOrPart returns the node of part, or of the element with key when key is
// not empty.
func keyedOrPart(t *testing.T, r *accessRig, part, key string) *node {
	t.Helper()
	if key != "" {
		return keyNode(t, r.rt, key)
	}
	return partNode(t, r.rt, part, "")
}
