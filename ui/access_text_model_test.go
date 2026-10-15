package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// asciiContent returns the access content of the ASCII text v for a test: each byte is a point,
// and the text is one line, 8 units to the character.
func asciiContent(v string) *access.TextContent {
	c := &access.TextContent{Text: v, Lines: []access.TextLine{{Start: 0, Height: 16, Right: float32(len(v)) * 8}}, Paras: []int{0}}
	c.X = make([]float32, len(v)+1)
	for i := 0; i <= len(v); i++ {
		c.Points = append(c.Points, i)
		c.X[i] = float32(i) * 8
	}
	return c
}

// A canvas that draws its own text gets the Text pattern from TextAccess, and the tree reports
// its text and selection changes as those of a text field. The element renders the signals it
// reports, as an editor renders its view, so a change builds the tree again.
func TestTextPatternIsExposedAndFollowsChanges(t *testing.T) {
	text := NewSignal("abc")
	caret := NewSignal(3)
	r := newAccessRig(func(ctx *Context) Element {
		text.Get()
		caret.Get()
		return Div().Column().Children(
			Canvas(func(p *Painter, w, h float32) {}).Size(Px(200), Px(60)).
				Role(access.RoleTextArea).Label("Code").
				TextAccess(TextAccess{Info: func() *access.TextInfo {
					v := text.Peek()
					return &access.TextInfo{Content: asciiContent(v), Anchor: caret.Peek(), Caret: caret.Peek()}
				}}))
	})
	var updates []access.Update
	r.bridge.Listen(func(u access.Update) { updates = append(updates, u) })
	field := accesstest.Find(t, r.tree(), access.RoleTextArea, "Code")
	if field.Text == nil {
		t.Fatal("the canvas has no Text pattern")
	}
	if got := field.Text.Content.Read(0, field.Text.Content.Len(), -1); got != "abc" {
		t.Errorf("text = %q, want %q", got, "abc")
	}

	updates = nil
	text.Set("abcd")
	r.tree()
	if !updatesHave(updates, field.ID, func(u access.Update) []access.ID { return u.TextChanged }) {
		t.Errorf("a change of the text was not reported as a text change: %+v", updates)
	}

	updates = nil
	caret.Set(1)
	r.tree()
	if !updatesHave(updates, field.ID, func(u access.Update) []access.ID { return u.TextSelection }) {
		t.Errorf("a change of the caret was not reported as a selection change: %+v", updates)
	}
	if updatesHave(updates, field.ID, func(u access.Update) []access.ID { return u.TextChanged }) {
		t.Error("a change of the caret was reported as a text change")
	}
}

// updatesHave reports whether one of the updates lists id, in the lists that pick selects.
func updatesHave(us []access.Update, id access.ID, pick func(access.Update) []access.ID) bool {
	for _, u := range us {
		for _, x := range pick(u) {
			if x == id {
				return true
			}
		}
	}
	return false
}
