package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// motionCase is a widget and an action on it. root builds the rig's root once and
// returns the render function, so the widget's signals outlive the renders. act
// runs after the first frame and leaves the rig after its own frame. animates says
// whether the action starts a motion when reduced motion is off: a transition, an
// enter or exit, or a repeating animation. Under reduced motion no case may ask for
// a frame.
type motionCase struct {
	name     string
	animates bool
	root     func(open *Signal[bool]) func(*Context) Element
	act      func(t *testing.T, r *accessRig, open *Signal[bool])
}

// hoverPart moves the pointer to the center of the part's node, and frames.
func hoverPart(t *testing.T, r *accessRig, part string) {
	t.Helper()
	rc := r.rt.nodeRect(partNode(t, r.rt, part, ""))
	r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
	r.frame()
}

// clickPart presses the center of the part's node, and frames.
func clickPart(t *testing.T, r *accessRig, part string) {
	t.Helper()
	rc := r.rt.nodeRect(partNode(t, r.rt, part, ""))
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
}

// TestWidgetsRequestNoFramesUnderReducedMotion runs each widget's action with
// reduced motion on and off. Under reduced motion no widget asks for a frame, and
// the widgets that animate ask for frames without it, so the check is real.
func TestWidgetsRequestNoFramesUnderReducedMotion(t *testing.T) {
	cases := []motionCase{
		{name: "TextInput", animates: true,
			root: func(open *Signal[bool]) func(*Context) Element {
				name := NewSignal("")
				return func(ctx *Context) Element {
					return Div().Column().Child(TextInput(name).Label("Name"))
				}
			},
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) { hoverPart(t, r, "TextInput.Frame") }},
		{name: "TextArea", animates: true,
			root: func(open *Signal[bool]) func(*Context) Element {
				notes := NewSignal("")
				return func(ctx *Context) Element {
					return Div().Column().Child(TextArea(notes).Label("Notes"))
				}
			},
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) { hoverPart(t, r, "TextArea.Frame") }},
		{name: "NumberField", animates: true,
			root: func(open *Signal[bool]) func(*Context) Element {
				count := NewSignal(3)
				return func(ctx *Context) Element {
					return Div().Column().Child(NumberField(count, 0, 10, 1).Label("Count"))
				}
			},
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) { hoverPart(t, r, "NumberField.Frame") }},
		{name: "Checkbox",
			root: func(open *Signal[bool]) func(*Context) Element {
				include := NewSignal(false)
				return func(ctx *Context) Element {
					return Div().Column().Child(Checkbox("Include", include).Key("inc"))
				}
			},
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) { r.clickKey(t, "inc") }},
		{name: "RadioGroup",
			root: func(open *Signal[bool]) func(*Context) Element {
				freq := NewSignal("a")
				return func(ctx *Context) Element {
					return Div().Column().Child(RadioGroup(freq,
						Option[string]{Value: "a", Label: "A"}, Option[string]{Value: "b", Label: "B"}).Label("Pick").Key("grp"))
				}
			},
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) { r.clickKey(t, "grp/b") }},
		{name: "Split", animates: false,
			root: func(open *Signal[bool]) func(*Context) Element {
				pos := NewSignal(0.4)
				collapsed := NewSignal(false)
				return func(ctx *Context) Element {
					return Split(Div(), Div()).Position(pos).Collapsible(FirstPane).Collapsed(collapsed)
				}
			},
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) {
				// The split learns its length one frame after its first render, so the
				// window settles before the action.
				r.frame()
				r.frame()
				focusNode(r, partNode(t, r.rt, "Split.Handle", ""))
				r.press(input.KeyEnter, 0)
			}},
		{name: "Collapsible",
			root: func(open *Signal[bool]) func(*Context) Element {
				adv := NewSignal(false)
				return func(ctx *Context) Element {
					return Div().Column().Child(Collapsible("Advanced", adv, Text("body")).Key("adv"))
				}
			},
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) { r.clickKey(t, "adv/header") }},
		{name: "TreeView",
			root: func(open *Signal[bool]) func(*Context) Element {
				sel := NewSignal("src")
				nodes := []TreeNode{{Key: "src", Label: "Source", Children: func() []TreeNode {
					return []TreeNode{{Key: "a", Label: "A"}}
				}}}
				return func(ctx *Context) Element {
					return Div().Column().Child(TreeView(nodes, sel).Label("Files").Key("files"))
				}
			},
			// The keyboard opens a node without a reveal, which is what a tree has no
			// motion for; a click can need a frame to reveal the row (not an animation).
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) {
				r.focusKey(t, "files")
				r.press(input.KeyRight, 0)
			}},
		{name: "Menu", animates: true,
			root: func(open *Signal[bool]) func(*Context) Element {
				return func(ctx *Context) Element {
					return Div().Padding(20).Child(DropdownMenu(Div().Key("trig").Size(Px(200), Px(30)).Focusable(),
						func() []MenuItem { return []MenuItem{{Label: "Open"}} }))
				}
			},
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) { r.clickKey(t, "trig") }},
		{name: "MenuBar", animates: true,
			root: func(open *Signal[bool]) func(*Context) Element {
				return func(ctx *Context) Element {
					return Div().Column().Child(MenuBar(TopMenu{Label: "File", Items: func() []MenuItem {
						return []MenuItem{{Label: "Open"}}
					}}))
				}
			},
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) { clickPart(t, r, "MenuBar.BarItem") }},
		{name: "Modal", animates: true,
			root: func(open *Signal[bool]) func(*Context) Element {
				return func(ctx *Context) Element {
					if !open.Get() {
						return Div().Column().Child(Text("page"))
					}
					return Div().Column().Child(Modal("Settings", Text("Body")))
				}
			},
			act: func(t *testing.T, r *accessRig, open *Signal[bool]) { open.Set(true); r.frame() }},
		{name: "ProgressBar", animates: true,
			root: func(open *Signal[bool]) func(*Context) Element {
				return func(ctx *Context) Element {
					return Div().Column().Child(IndeterminateProgressBar().Label("Working"))
				}
			},
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) {}},
		{name: "ProgressRing", animates: true,
			root: func(open *Signal[bool]) func(*Context) Element {
				return func(ctx *Context) Element {
					return Div().Column().Child(ProgressRing().Label("Sending"))
				}
			},
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) {}},
		{name: "Badge",
			root: func(open *Signal[bool]) func(*Context) Element {
				return func(ctx *Context) Element { return Div().Column().Child(Badge(5)) }
			},
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) {}},
		{name: "Toast", animates: true,
			root: func(open *Signal[bool]) func(*Context) Element {
				return func(ctx *Context) Element { return Div().Column().Child(Toaster()) }
			},
			// A sticky toast has no timer, so only its enter motion asks for frames.
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) {
				r.rt.toast(ToastInfo, "Saved").Sticky()
				r.frame()
			}},
		{name: "Tooltip", animates: true,
			root: func(open *Signal[bool]) func(*Context) Element {
				return func(ctx *Context) Element {
					return Div().Column().Child(Div().Key("t").Size(Px(100), Px(30)).Tooltip("Hello"))
				}
			},
			act: func(t *testing.T, r *accessRig, _ *Signal[bool]) {
				rc := r.rt.nodeRect(keyNode(t, r.rt, "t"))
				r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
				r.now = r.now.Add(600 * time.Millisecond)
				r.frame()
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, reduced := range []bool{true, false} {
				open := NewSignal(false)
				r := newAccessRig(c.root(open))
				r.rt.SetSystemSettings(SystemSettings{ReducedMotion: reduced})
				r.frame()
				c.act(t, r, open)
				asks := r.rt.NeedsFrame()
				if reduced && asks {
					t.Errorf("reduced motion %v: the action asks for frames", reduced)
				}
				if !reduced && c.animates && !asks {
					t.Errorf("reduced motion off: the action asks for no frames; it should animate")
				}
			}
		})
	}
}
