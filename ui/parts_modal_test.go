package ui

import (
	"testing"
)

// partModal is a modal with the title "Settings", a body and two buttons, with
// the instance style st.
func partModal(st ModalStyle) Element {
	cancel := Button("Cancel").Key("cancel")
	ok := Button("OK").Key("ok").Primary()
	return Div().Column().Child(Modal("Settings", Text("Body text")).
		Footer(cancel, ok).Default(ok).Styles(st))
}

func testModalParts(t *testing.T, applied map[string]bool) {
	type modalCase struct {
		part  string
		set   func(*ModalStyle, Style)
		drawn string // the element that draws the part
		text  bool
	}
	cases := []modalCase{
		{part: "Modal.Scrim", set: func(s *ModalStyle, m Style) { s.Scrim = m }, drawn: "Modal.Scrim"},
		{part: "Modal.Panel", set: func(s *ModalStyle, m Style) { s.Panel = m }, drawn: "Modal.Panel"},
		{part: "Modal.Title", set: func(s *ModalStyle, m Style) { s.Title = m }, drawn: "Modal.Title"},
		{part: "Modal.Body", set: func(s *ModalStyle, m Style) { s.Body = m }, drawn: "Modal.Body"},
		{part: "Modal.Footer", set: func(s *ModalStyle, m Style) { s.Footer = m }, drawn: "Modal.Footer"},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st ModalStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element { return partModal(st) })
			n := partNode(t, r.rt, c.drawn, "")
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
}

// TestModalUnderHighContrast checks the guardrails for a modal: an app color does
// not reach the scrim or the panel under high contrast, the panel's padding does,
// and the scrim is the window color there.
func TestModalUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Modal = ModalStyle{Panel: Style{Background: markBG, Padding: All(5)}}
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return partModal(ModalStyle{Scrim: Style{Background: markBG}, Panel: Style{Padding: All(11)}})
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	if n := partNode(t, r.rt, "Modal.Scrim", ""); n.style.Background != th.Scrim {
		t.Errorf("high contrast: the scrim is %v, want the system color %v", n.style.Background, th.Scrim)
	}
	panel := partNode(t, r.rt, "Modal.Panel", "")
	if panel.style.Background == markBG {
		t.Errorf("high contrast: an app color reached the panel")
	}
	if panel.style.Padding != All(11) {
		t.Errorf("high contrast: the instance padding %v did not apply", All(11))
	}
}
