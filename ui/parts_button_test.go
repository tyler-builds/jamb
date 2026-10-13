package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// partButton is a primary button with an icon and a label, with variant v, the
// instance style st and disabled or not.
func partButton(v ButtonVariant, st ButtonStyle, disabled bool) Element {
	return Div().Column().Child(Button("Save").Icon(icons.Check).Key("save").Disabled(disabled).
		Variant(v).Styles(st))
}

func testButtonParts(t *testing.T, applied map[string]bool) {
	type buttonCase struct {
		part     string
		set      func(*ButtonStyle, Style)
		disabled bool
		text     bool // the part is colored by its text color
		act      func(*testing.T, *accessRig)
	}
	cases := []buttonCase{
		{part: "Button.Root", set: func(s *ButtonStyle, m Style) { s.Root = m }},
		{part: "Button.RootHover", set: func(s *ButtonStyle, m Style) { s.RootHover = m },
			act: func(t *testing.T, r *accessRig) { hoverNode(r, partNode(t, r.rt, "Button.Root", "")) }},
		{part: "Button.RootPressed", set: func(s *ButtonStyle, m Style) { s.RootPressed = m },
			act: func(t *testing.T, r *accessRig) { pressNode(r, partNode(t, r.rt, "Button.Root", "")) }},
		{part: "Button.RootDisabled", set: func(s *ButtonStyle, m Style) { s.RootDisabled = m }, disabled: true},
		{part: "Button.RootFocusVisible", set: func(s *ButtonStyle, m Style) { s.RootFocusVisible = m },
			act: func(t *testing.T, r *accessRig) { focusNode(r, partNode(t, r.rt, "Button.Root", "")) }},
		{part: "Button.Label", set: func(s *ButtonStyle, m Style) { s.Label = m }, text: true},
		{part: "Button.Icon", set: func(s *ButtonStyle, m Style) { s.Icon = m }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st ButtonStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element { return partButton(ButtonPrimary, st, c.disabled) })
			if c.act != nil {
				c.act(t, r)
			}
			n := partNode(t, r.rt, drawnBy(c.part), "")
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
}

// TestButtonVariantStyles checks that the theme's Button applies to every
// variant, and the theme's Buttons to its own variant only, with the instance
// over both.
func TestButtonVariantStyles(t *testing.T) {
	themeAll, themeDanger, inst := Hex("#111213"), Hex("#121314"), Hex("#141516")
	bg := func(th *Theme, v ButtonVariant, st ButtonStyle) Color {
		r := newThemedRig(th, func(ctx *Context) Element { return partButton(v, st, false) })
		return partNode(t, r.rt, "Button.Root", "").style.Background
	}
	th := *DarkTheme()
	th.Button.Root.Background = themeAll
	th.Buttons.Danger.Root.Background = themeDanger
	if got := bg(&th, ButtonPrimary, ButtonStyle{}); got != themeAll {
		t.Errorf("primary: background %v, want the theme's Button %v", got, themeAll)
	}
	if got := bg(&th, ButtonDanger, ButtonStyle{}); got != themeDanger {
		t.Errorf("danger: background %v, want the variant's %v", got, themeDanger)
	}
	if got := bg(&th, ButtonDanger, ButtonStyle{Root: Style{Background: inst}}); got != inst {
		t.Errorf("danger with an instance: background %v, want %v", got, inst)
	}
	if got := bg(&th, ButtonPrimary, ButtonStyle{}); got == themeDanger {
		t.Errorf("primary took the danger variant's style")
	}
}

// TestButtonPartPrecedence checks instance over theme over defaults for the
// root background of a button.
func TestButtonPartPrecedence(t *testing.T) {
	themeBG, instBG := Hex("#111213"), Hex("#141516")
	bgOf := func(th *Theme, st ButtonStyle) Color {
		r := newThemedRig(th, func(ctx *Context) Element { return partButton(ButtonPrimary, st, false) })
		return partNode(t, r.rt, "Button.Root", "").style.Background
	}
	plain := *DarkTheme()
	withTheme := *DarkTheme()
	withTheme.Button.Root.Background = themeBG
	if got := bgOf(&plain, ButtonStyle{}); got != DarkTheme().Accent {
		t.Errorf("no override: background %v, want the default %v", got, DarkTheme().Accent)
	}
	if got := bgOf(&withTheme, ButtonStyle{}); got != themeBG {
		t.Errorf("theme override: background %v, want %v", got, themeBG)
	}
	if got := bgOf(&withTheme, ButtonStyle{Root: Style{Background: instBG}}); got != instBG {
		t.Errorf("instance over theme: background %v, want %v", got, instBG)
	}
}

// TestButtonPartsUnderHighContrast checks the guardrails for a button: an app
// color does not reach the button, its spacing does, and the focus ring stays.
func TestButtonPartsUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Button = ButtonStyle{Root: Style{Background: markBG, TextColor: markFG, Padding: XY(9, 0)}}
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return partButton(ButtonPrimary, ButtonStyle{Root: Style{Background: markBG, Padding: XY(12, 0)}}, false)
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	n := partNode(t, r.rt, "Button.Root", "")
	if n.style.Background == markBG {
		t.Errorf("high contrast: an app color reached the button")
	}
	if n.style.Background != th.Accent {
		t.Errorf("high contrast: the primary button is %v, want Highlight %v", n.style.Background, th.Accent)
	}
	if n.style.Padding != XY(12, 0) {
		t.Errorf("high contrast: the instance padding %v did not apply", XY(12, 0))
	}
	focusNode(r, n)
	if len(n.style.Shadows) == 0 {
		t.Errorf("high contrast: the focused button has no focus ring")
	}
}

// TestButtonClearRemovesDefaults checks the clear through the Root part: a
// ghost-like primary with no fill and no shadow.
func TestButtonClearRemovesDefaults(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return partButton(ButtonPrimary, ButtonStyle{Root: Style{Clear: ClearBackground | ClearShadows, Background: Transparent}}, false)
	})
	n := partNode(t, r.rt, "Button.Root", "")
	if n.style.Background.A != 0 {
		t.Errorf("the button kept its fill %v", n.style.Background)
	}
	if len(n.style.Shadows) != 0 {
		t.Errorf("the button kept its shadow")
	}
	accessNode(t, r, access.RoleButton, "Save")
}
