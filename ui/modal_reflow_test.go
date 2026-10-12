package ui

import (
	"fmt"
	"testing"
)

// modalRig opens a modal with a body of many lines and a field at its end, in a
// window of the test size (500 by 400). The footer has an OK button.
func modalRig(body Element, ok *ButtonElement) *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return Modal("Title", body).Footer(ok).Default(ok)
	})
}

// inView reports whether the rectangle of n lies within the window, and within
// the rectangle of the panel when panel is set.
func inView(r *accessRig, n *node, panel *node) bool {
	rc := r.rt.nodeRect(n)
	if rc.Y < 0 || rc.Y+rc.H > 400 {
		return false
	}
	if panel != nil {
		pr := r.rt.nodeRect(panel)
		return rc.Y >= pr.Y && rc.Y+rc.H <= pr.Y+pr.H
	}
	return true
}

func TestModalPanelFitsTheWindowAndItsBodyScrolls(t *testing.T) {
	body := Div().Column().Gap(8)
	for i := 0; i < 60; i++ {
		body.Child(Text(fmt.Sprintf("Line %d", i)))
	}
	ok := Button("OK").Key("ok")
	r := modalRig(body, ok)
	panel := partNode(t, r.rt, "Modal.Panel", "")
	if h := r.rt.nodeRect(panel).H; h > 400-2*modalMargin+0.5 {
		t.Fatalf("the panel is %v px tall in a 400 px window, want at most %v", h, 400-2*modalMargin)
	}
	footer := partNode(t, r.rt, "Modal.Footer", "")
	if !inView(r, footer, nil) {
		t.Fatal("the footer is not on screen while the body does not fit")
	}
	if !inView(r, r.keyed(t, "ok"), nil) {
		t.Fatal("the OK button is not on screen")
	}
}

func TestModalFocusRevealsAFieldInTheScrolledBody(t *testing.T) {
	last := NewFocusHandle()
	body := Div().Column().Gap(8)
	for i := 0; i < 60; i++ {
		body.Child(Text(fmt.Sprintf("Line %d", i)))
	}
	body.Child(TextInput(NewSignal("")).Key("last").FocusHandle(last))
	ok := Button("OK").Key("ok")
	r := modalRig(body, ok)
	panel := partNode(t, r.rt, "Modal.Panel", "")
	last.Focus()
	r.frame()
	r.frame()
	if !inView(r, r.keyed(t, "last"), panel) {
		t.Fatal("the focused field at the end of the body is not scrolled into view")
	}
	if !inView(r, partNode(t, r.rt, "Modal.Footer", ""), nil) {
		t.Fatal("the footer moved off screen when the body scrolled")
	}
}

func TestModalSubmitRevealsTheFirstInvalidField(t *testing.T) {
	name := NewSignal("")
	form := NewForm().OnSubmit(func() error { return nil })
	body := Div().Column().Gap(8)
	for i := 0; i < 60; i++ {
		body.Child(Text(fmt.Sprintf("Line %d", i)))
	}
	body.Child(form.Field("name", "Name", TextInput(name).Key("name").Validate(Required[string]())).Required())
	create := Button("Create").Key("create").OnClick(form.Submit)
	r := modalRig(body, create)
	panel := partNode(t, r.rt, "Modal.Panel", "")
	r.clickKey(t, "create")
	if got := r.focusedKey(); got != "name" {
		t.Fatalf("focus = %q, want the invalid field", got)
	}
	if !inView(r, r.keyed(t, "name"), panel) {
		t.Fatal("the invalid field took focus but is not scrolled into view")
	}
}
