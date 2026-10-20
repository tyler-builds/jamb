package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// A loading button does nothing on a click, Enter, Space or an invoke from
// assistive technology, keeps the focus, and reports the Loading status while
// it stays enabled.
func TestButtonLoadingIsInert(t *testing.T) {
	loading := NewSignal(false)
	clicks := 0
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Button("Save").Key("save").Loading(loading.Get()).OnClick(func() { clicks++ }))
	})
	r.focusKey(t, "save")
	loading.Set(true)
	r.frame()
	if got := r.focusedKey(); got != "save" {
		t.Fatalf("the focus moved to %q when the button began loading", got)
	}
	r.clickKey(t, "save")
	r.press(input.KeyEnter, 0)
	r.press(input.KeySpace, 0)
	btn := accesstest.Find(t, r.tree(), access.RoleButton, "Save")
	r.bridge.Perform(btn.ID, access.ActionClick)
	r.frame()
	if clicks != 0 {
		t.Fatalf("the loading button ran its OnClick %d times", clicks)
	}
	btn = r.stateOf(t, access.RoleButton, "Save")
	if btn.ItemStatus != "Loading" || btn.States.Has(access.StateDisabled) {
		t.Fatalf("the loading button has status %q and states [%s]", btn.ItemStatus, btn.States)
	}
	if got := r.focusedKey(); got != "save" {
		t.Fatalf("the focus is on %q, want the loading button", got)
	}
	// The ring is not in the tree: the button's status says it is busy.
	accesstest.Absent(t, r.tree(), access.RoleProgressBar, "Loading…")
	loading.Set(false)
	r.frame()
	r.clickKey(t, "save")
	if clicks != 1 {
		t.Fatalf("the button that stopped loading ran its OnClick %d times, want 1", clicks)
	}
}

// Disabled wins over Loading.
func TestButtonDisabledWinsOverLoading(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Button("Save").Loading(true).Disabled(true))
	})
	btn := r.stateOf(t, access.RoleButton, "Save")
	if !btn.States.Has(access.StateDisabled) || btn.ItemStatus != "" {
		t.Fatalf("a disabled loading button has states [%s] and status %q", btn.States, btn.ItemStatus)
	}
}

// A loading button keeps its width, with an icon, with a label alone and with
// an icon alone.
func TestButtonLoadingKeepsItsWidth(t *testing.T) {
	for _, c := range []struct {
		name  string
		build func() *ButtonElement
	}{
		{"label", func() *ButtonElement { return Button("Save changes") }},
		{"icon and label", func() *ButtonElement { return Button("Save changes").Icon(icons.Check) }},
		{"icon", func() *ButtonElement { return IconButton(icons.Check).Label("Save") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			loading := NewSignal(false)
			r := newAccessRig(func(ctx *Context) Element {
				return Div().Column().Child(c.build().Key("b").Loading(loading.Get()))
			})
			w := r.nodeByKey(t, "b").lay.W
			loading.Set(true)
			r.frame()
			if got := r.nodeByKey(t, "b").lay.W; got != w {
				t.Fatalf("the loading button is %v wide, want %v", got, w)
			}
			if len(partNodes(r.rt, "ProgressRing.Ring")) != 1 {
				t.Fatal("the loading button draws no ring")
			}
		})
	}
}

// Under high contrast the ring's dots are drawn in the button's text color, so
// they show on its fill.
func TestButtonLoadingRingUnderHighContrast(t *testing.T) {
	th := HighContrastTheme(HighContrastBlack(), DarkTheme())
	r := newThemedRig(th, func(ctx *Context) Element {
		return Div().Column().Child(Button("Save").Primary().Loading(true))
	})
	want := buttonDefaults(th, ButtonPrimary, sizeDefault, false).Root.TextColor
	dots := partNodes(r.rt, "ProgressRing.Dot")
	if len(dots) == 0 {
		t.Fatal("the loading button draws no dots")
	}
	for _, d := range dots {
		if d.style.Background != want {
			t.Fatalf("a dot is %v, want the button's text color %v", d.style.Background, want)
		}
	}
}

// A loading split button runs no action and opens no menu.
func TestSplitButtonLoadingIsInert(t *testing.T) {
	var ran []string
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(SplitButton("Save", func() { ran = append(ran, "Save") }, func() []MenuItem {
			return []MenuItem{{Label: "Save as", OnClick: func() { ran = append(ran, "Save as") }}}
		}).Key("sb").Loading(true))
	})
	arrow := accesstest.Find(t, r.tree(), access.RoleButton, "More options")
	r.bridge.Perform(arrow.ID, access.ActionClick)
	r.frame()
	clickPart(t, r, "SplitButton.Arrow")
	r.focusAccess(t, access.RoleButton, "More options")
	r.press(input.KeyDown, 0)
	r.press(input.KeyEnter, 0)
	if _, ok := r.rt.Find("jamb.menu"); ok {
		t.Fatal("the menu of a loading split button opened")
	}
	r.focusAccess(t, access.RoleButton, "Save")
	r.press(input.KeyEnter, 0)
	if len(ran) != 0 {
		t.Fatalf("a loading split button ran %v", ran)
	}
	if s := r.stateOf(t, access.RoleButton, "Save").ItemStatus; s != "Loading" {
		t.Fatalf("the action segment has status %q", s)
	}
}

// A dropdown menu does not open from a loading trigger.
func TestDropdownMenuLoadingTriggerDoesNotOpen(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(DropdownMenu(Button("Actions").Key("trigger").Loading(true), func() []MenuItem {
			return []MenuItem{{Label: "Rename"}}
		}))
	})
	r.clickKey(t, "trigger")
	r.press(input.KeyEnter, 0)
	if _, ok := r.rt.Find("jamb.menu"); ok {
		t.Fatal("the menu opened from a loading trigger")
	}
}
