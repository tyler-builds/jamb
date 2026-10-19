package ui

import (
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// otpRig is a six-box code input, split 3 and 3, focused, with its changes and
// completions recorded.
func otpRig(t *testing.T, code *Signal[string], changes, done *[]string, opts func(*OTPInputElement)) *accessRig {
	t.Helper()
	r := newAccessRig(func(ctx *Context) Element {
		o := OTPInput("Verification code", code, 6).Key("otp").Groups(3, 3).
			OnChange(func(s string) { *changes = append(*changes, s) }).
			OnComplete(func(s string) { *done = append(*done, s) })
		if opts != nil {
			opts(o)
		}
		return Div().Column().Child(o)
	})
	r.focusKey(t, "otp")
	return r
}

// otpCaret returns the index of the box drawn at the caret, or -1.
func otpCaret(r *accessRig) int {
	for i, n := range orderedParts(r.rt, "OTPInput.Box") {
		if len(n.style.Shadows) > 0 {
			return i
		}
	}
	return -1
}

// Typing fills the box at the caret and moves on; only digits are taken. The last
// box completes the code, and typing past it does nothing.
func TestOTPInputTypingFillsTheBoxes(t *testing.T) {
	code := NewSignal("")
	var changes, done []string
	r := otpRig(t, code, &changes, &done, nil)
	if got := otpCaret(r); got != 0 {
		t.Fatalf("the caret box is %d before typing, want 0", got)
	}
	r.typeText("1a2 3")
	if code.Peek() != "123" || otpCaret(r) != 3 {
		t.Fatalf("after typing 1a2 3: code %q, caret %d; want 123 and 3", code.Peek(), otpCaret(r))
	}
	if !slices.Equal(changes, []string{"1", "12", "123"}) {
		t.Errorf("OnChange %v, want [1 12 123]", changes)
	}
	r.typeText("4567")
	if code.Peek() != "123456" {
		t.Fatalf("code %q, want 123456 (the 7 has no box)", code.Peek())
	}
	if !slices.Equal(done, []string{"123456"}) {
		t.Errorf("OnComplete %v, want [123456]", done)
	}
	if got := otpCaret(r); got != 5 {
		t.Errorf("the caret box of a full code is %d, want the last, 5", got)
	}
	if n := len(partNodes(r.rt, "OTPInput.Group")); n != 2 {
		t.Errorf("%d groups, want 2", n)
	}
}

// Backspace clears the box before the caret and moves back; Delete clears the box at
// the caret, and the later characters move up. The arrows, Home and End move the
// caret, and never past the first empty box.
func TestOTPInputKeysEditAndMove(t *testing.T) {
	code := NewSignal("123456")
	var changes, done []string
	r := otpRig(t, code, &changes, &done, nil)
	r.press(input.KeyEnd, 0)
	r.press(input.KeyBackspace, 0)
	if code.Peek() != "12345" || otpCaret(r) != 5 {
		t.Fatalf("End, Backspace: code %q, caret %d; want 12345 and 5", code.Peek(), otpCaret(r))
	}
	r.press(input.KeyRight, 0) // the caret is at the first empty box already
	if otpCaret(r) != 5 {
		t.Errorf("Right past the code: caret %d, want 5", otpCaret(r))
	}
	r.press(input.KeyHome, 0)
	r.press(input.KeyRight, 0)
	r.press(input.KeyDelete, 0)
	if code.Peek() != "1345" || otpCaret(r) != 1 {
		t.Fatalf("Home, Right, Delete: code %q, caret %d; want 1345 and 1", code.Peek(), otpCaret(r))
	}
	r.press(input.KeyLeft, 0)
	r.press(input.KeyLeft, 0)
	r.press(input.KeyBackspace, 0) // nothing before the first box
	if code.Peek() != "1345" || otpCaret(r) != 0 {
		t.Errorf("Backspace at the first box: code %q, caret %d", code.Peek(), otpCaret(r))
	}
	r.typeText("9") // over a filled box
	if code.Peek() != "9345" || otpCaret(r) != 1 {
		t.Errorf("typing over the first box: code %q, caret %d; want 9345 and 1", code.Peek(), otpCaret(r))
	}
	if len(done) != 0 {
		t.Errorf("OnComplete ran for an incomplete code: %v", done)
	}
}

// Ctrl+V pastes the clipboard's text from the caret, keeping the characters the input
// takes. The clipboard is the runtime's own, not the system's.
func TestOTPInputPaste(t *testing.T) {
	code := NewSignal("")
	var changes, done []string
	r := otpRig(t, code, &changes, &done, nil)
	r.rt.memClipboard = "Code: 482 916!"
	r.press(input.KeyV, input.ModCtrl)
	if code.Peek() != "482916" {
		t.Fatalf("paste: code %q, want 482916", code.Peek())
	}
	if !slices.Equal(changes, []string{"482916"}) || !slices.Equal(done, []string{"482916"}) {
		t.Errorf("one paste: OnChange %v, OnComplete %v; want one call each", changes, done)
	}
	r.press(input.KeyHome, 0)
	r.press(input.KeyRight, 0)
	r.press(input.KeyRight, 0)
	r.rt.memClipboard = "77777777"
	r.press(input.KeyV, input.ModCtrl)
	if code.Peek() != "487777" {
		t.Errorf("paste from the third box: code %q, want 487777", code.Peek())
	}
}

// A press on a box moves the caret to it, or to the first empty box when it is past
// that, and focuses the input.
func TestOTPInputClickMovesTheCaret(t *testing.T) {
	code := NewSignal("12")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(OTPInput("Code", code, 4).Key("otp"))
	})
	boxes := orderedParts(r.rt, "OTPInput.Box")
	rc := r.rt.nodeRect(boxes[0])
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if r.focusedKey() != "otp" || otpCaret(r) != 0 {
		t.Fatalf("a click on the first box: focus %q, caret %d", r.focusedKey(), otpCaret(r))
	}
	rc = r.rt.nodeRect(orderedParts(r.rt, "OTPInput.Box")[3])
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if otpCaret(r) != 2 {
		t.Errorf("a click on the last box of a two-character code: caret %d, want 2", otpCaret(r))
	}
}

// Alphanumeric takes letters too, as typed.
func TestOTPInputAlphanumeric(t *testing.T) {
	code := NewSignal("")
	var changes, done []string
	r := otpRig(t, code, &changes, &done, func(o *OTPInputElement) { o.Alphanumeric() })
	r.typeText("a-B9 é")
	if code.Peek() != "aB9" {
		t.Errorf("code %q, want aB9", code.Peek())
	}
}

// The input is one text field named by its label, whose value is the code. A masked
// input draws dots, reports the password state and a dot for each character, and
// never the code. Assistive technology can set the code.
func TestOTPInputAccess(t *testing.T) {
	code := NewSignal("1234")
	masked := NewSignal(false)
	var changes []string
	r := newAccessRig(func(ctx *Context) Element {
		o := OTPInput("Verification code", code, 6).Key("otp").OnChange(func(s string) { changes = append(changes, s) })
		if masked.Get() {
			o.Masked()
		}
		return Div().Column().Child(o)
	})
	tr := r.tree()
	f := accesstest.Find(t, tr, access.RoleTextField, "Verification code")
	accesstest.Check(t, f, access.RoleTextField, "Verification code", access.StateFocusable)
	if f.Value != "1234" || f.States.Has(access.StatePassword) || !f.Has(access.ActionSetValue) {
		t.Errorf("value %q, states %v, actions %v", f.Value, f.States, f.Actions)
	}
	if len(f.Children) != 0 {
		t.Errorf("the boxes are in the tree: %d children", len(f.Children))
	}
	accesstest.Absent(t, tr, access.RoleStaticText, "1")
	if len(r.warnings) != 0 {
		t.Errorf("accessibility warnings: %v", r.warnings)
	}

	masked.Set(true)
	r.frame()
	f = accesstest.Find(t, r.tree(), access.RoleTextField, "Verification code")
	if f.Value != "••••" || !f.States.Has(access.StatePassword) {
		t.Errorf("masked: value %q, states %v; want four dots and password", f.Value, f.States)
	}
	if n := len(partNodes(r.rt, "OTPInput.Dot")); n != 4 || len(partNodes(r.rt, "OTPInput.Text")) != 0 {
		t.Errorf("masked: %d dots and %d characters drawn, want 4 and 0", n, len(partNodes(r.rt, "OTPInput.Text")))
	}

	r.bridge.Do(f.ID, access.Request{Action: access.ActionSetValue, Value: "98 76x54"})
	r.frame()
	if code.Peek() != "987654" || !slices.Equal(changes, []string{"987654"}) {
		t.Errorf("SetValue: code %q, OnChange %v; want 987654 once", code.Peek(), changes)
	}
}

// A disabled input shows the code, takes no focus and no typing, and is reported
// disabled.
func TestOTPInputDisabled(t *testing.T) {
	code := NewSignal("12")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(OTPInput("Code", code, 4).Key("otp").Disabled(true))
	})
	if keyNode(t, r.rt, "otp").div.focusable {
		t.Fatal("a disabled input is focusable")
	}
	r.typeText("3")
	if code.Peek() != "12" {
		t.Errorf("a disabled input took typing: %q", code.Peek())
	}
	accesstest.Check(t, accesstest.Find(t, r.tree(), access.RoleTextField, "Code"), access.RoleTextField, "Code", access.StateDisabled)
	for _, n := range partNodes(r.rt, "OTPInput.Box") {
		if n.style.Opacity != 0.5 {
			t.Errorf("a disabled box has opacity %v, want 0.5", n.style.Opacity)
		}
	}
}

// In a Form the input is named by its field, takes focus for it, shows its
// validator's error with the invalid boxes, and Enter submits the form.
func TestOTPInputInAForm(t *testing.T) {
	code := NewSignal("")
	submitted := 0
	form := NewForm().OnSubmit(func() error { submitted++; return nil })
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Gap(12).
			Child(form.Field("code", "Code", OTPInput("Unused name", code, 4).Validate(Required[string](), MinLength(4))).Required()).
			Child(Button("Verify").Key("verify").OnClick(form.Submit))
	})
	r.clickKey(t, "verify")
	f := r.stateOf(t, access.RoleTextField, "Code")
	if !f.States.Has(access.StateFocused | access.StateInvalid | access.StateRequired) {
		t.Fatalf("after a failed submit the input's states are %v, want focused, invalid and required", f.States)
	}
	if f.Description == "" {
		t.Error("the input is not described by its error")
	}
	for _, n := range partNodes(r.rt, "OTPInput.Box") {
		if n.style.BorderWidth != 2 {
			t.Errorf("an invalid box has border width %v, want 2", n.style.BorderWidth)
		}
	}
	r.typeText("1234")
	if r.stateOf(t, access.RoleTextField, "Code").States.Has(access.StateInvalid) {
		t.Error("the input is still invalid with a complete code")
	}
	r.press(input.KeyEnter, 0)
	if submitted != 1 {
		t.Errorf("Enter in the input submitted the form %d times, want 1", submitted)
	}
}

// Groups must add up to the length, and the length must be 1 or more.
func TestOTPInputPanics(t *testing.T) {
	for name, fn := range map[string]func(){
		"length": func() { OTPInput("Code", NewSignal(""), 0) },
		"sum":    func() { OTPInput("Code", NewSignal(""), 6).Groups(3, 2) },
		"size":   func() { OTPInput("Code", NewSignal(""), 3).Groups(3, 0) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no panic", name)
				}
			}()
			fn()
		}()
	}
}

// The states of a box change its border and ring, never its font weight.
func TestOTPInputStatesKeepTheFontWeight(t *testing.T) {
	code := NewSignal("12")
	var changes, done []string
	r := otpRig(t, code, &changes, &done, nil)
	var weights []FontWeight
	for _, n := range partNodes(r.rt, "OTPInput.Box") {
		if !slices.Contains(weights, n.style.FontWeight) {
			weights = append(weights, n.style.FontWeight)
		}
	}
	if len(weights) != 1 {
		t.Errorf("font weights %v across the boxes, want one", weights)
	}
}

// Under high contrast the caret's box has the Highlight border, an invalid box the
// text color, and an app's colors are not used.
func TestOTPInputUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.OTPInput = OTPInputStyle{BoxFocused: Style{BorderColor: markBG}, Box: Style{Background: markBG}}
	code := NewSignal("1")
	r := newThemedRig(&theme, func(ctx *Context) Element { return Div().Column().Child(OTPInput("Code", code, 4).Key("otp")) })
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.focusKey(t, "otp")
	th := r.rt.theme
	boxes := orderedParts(r.rt, "OTPInput.Box")
	if boxes[1].style.BorderColor != th.Accent {
		t.Errorf("the caret's box border is %v, want Highlight %v", boxes[1].style.BorderColor, th.Accent)
	}
	if boxes[2].style.Background != th.Surface || boxes[2].style.BorderColor != th.BorderStrong {
		t.Errorf("an empty box: %v on %v, want the window color with its border", boxes[2].style.BorderColor, boxes[2].style.Background)
	}
}

func testOTPInputParts(t *testing.T, applied map[string]bool) {
	type otpCase struct {
		part     string
		set      func(*OTPInputStyle, Style)
		drawn    string
		index    int // the index of the node among the drawn ones, or -1 for every node
		masked   bool
		disabled bool
		invalid  bool
		focus    bool
	}
	cases := []otpCase{
		{part: "OTPInput.Root", set: func(s *OTPInputStyle, m Style) { s.Root = m }, drawn: "OTPInput.Root", index: -1},
		{part: "OTPInput.Group", set: func(s *OTPInputStyle, m Style) { s.Group = m }, drawn: "OTPInput.Group", index: -1},
		{part: "OTPInput.Box", set: func(s *OTPInputStyle, m Style) { s.Box = m }, drawn: "OTPInput.Box", index: 3},
		{part: "OTPInput.BoxFilled", set: func(s *OTPInputStyle, m Style) { s.BoxFilled = m }, drawn: "OTPInput.Box", index: 0},
		{part: "OTPInput.BoxFocused", set: func(s *OTPInputStyle, m Style) { s.BoxFocused = m }, drawn: "OTPInput.Box", index: 2, focus: true},
		{part: "OTPInput.BoxDisabled", set: func(s *OTPInputStyle, m Style) { s.BoxDisabled = m }, drawn: "OTPInput.Box", index: -1, disabled: true},
		{part: "OTPInput.BoxInvalid", set: func(s *OTPInputStyle, m Style) { s.BoxInvalid = m }, drawn: "OTPInput.Box", index: -1, invalid: true},
		{part: "OTPInput.Text", set: func(s *OTPInputStyle, m Style) { s.Text = m }, drawn: "OTPInput.Text", index: -1},
		{part: "OTPInput.Dot", set: func(s *OTPInputStyle, m Style) { s.Dot = m }, drawn: "OTPInput.Dot", index: -1, masked: true},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st OTPInputStyle
			c.set(&st, Style{Background: markBG})
			r := newAccessRig(func(ctx *Context) Element {
				o := OTPInput("Code", NewSignal("12"), 4).Key("otp").Groups(2, 2).Disabled(c.disabled).Styles(st)
				if c.masked {
					o.Masked()
				}
				if c.invalid {
					return Div().Column().Child(Field("Code", o).Error("Enter the code."))
				}
				return Div().Column().Child(o)
			})
			if c.focus {
				r.focusKey(t, "otp")
			}
			found := orderedParts(r.rt, c.drawn)
			if c.index >= 0 {
				if len(found) <= c.index {
					t.Fatalf("%s: %d nodes drawn, want more than %d", c.part, len(found), c.index)
				}
				found = found[c.index : c.index+1]
			}
			if len(found) == 0 {
				t.Fatalf("%s: no node drawn for it", c.part)
			}
			for _, n := range found {
				wantBackground(t, n, c.part, markBG)
			}
		})
	}
}
