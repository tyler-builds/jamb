package ui

import (
	"math"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// TestAvatarInitials checks the initials of a name of two words, of one word, of
// more words, and of no letters.
func TestAvatarInitials(t *testing.T) {
	cases := []struct{ name, want string }{
		{"Dana Lee", "DL"},
		{"cher", "C"},
		{"Mary Ann Smith", "MS"},
		{"  dana   lee ", "DL"},
		{"élan vital", "ÉV"},
		{"", ""},
	}
	for _, c := range cases {
		if got := avatarInitials(c.name); got != c.want {
			t.Errorf("avatarInitials(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// relativeLuminance is the WCAG relative luminance of c, its components taken as
// sRGB.
func relativeLuminance(c Color) float64 {
	lin := func(v float32) float64 {
		if v <= 0.03928 {
			return float64(v) / 12.92
		}
		return math.Pow((float64(v)+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// TestAvatarPaletteContrast checks that the initials read on every fill of the
// palette at 4.5:1 or more (WCAG 1.4.3).
func TestAvatarPaletteContrast(t *testing.T) {
	ink := relativeLuminance(avatarInk)
	for _, c := range avatarPalette {
		l := relativeLuminance(c)
		hi, lo := math.Max(l, ink), math.Min(l, ink)
		if ratio := (hi + 0.05) / (lo + 0.05); ratio < 4.5 {
			t.Errorf("the initials on %v read at %.2f:1, want 4.5:1 or more", c, ratio)
		}
	}
}

// TestAvatarColorFromName checks that the fill is a palette color, and that a
// name keeps its fill from one render to the next.
func TestAvatarColorFromName(t *testing.T) {
	fill := func(name string) Color {
		r := newAccessRig(func(ctx *Context) Element { return Avatar(name) })
		return partNode(t, r.rt, "Avatar.Avatar", "").style.Background
	}
	got := fill("Dana Lee")
	if again := fill("Dana Lee"); again != got {
		t.Errorf("the same name has fills %v and %v", got, again)
	}
	inPalette := false
	for _, c := range avatarPalette {
		inPalette = inPalette || c == got
	}
	if !inPalette {
		t.Errorf("the fill %v is not a palette color", got)
	}
}

// TestAvatarImageAndAccess checks that an image replaces the initials, that the
// avatar is named by its name and is an image, and that Decorative hides it.
func TestAvatarImageAndAccess(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element { return Avatar("Dana Lee") })
	if n := len(partNodes(r.rt, "Avatar.Initials")); n != 1 {
		t.Errorf("the initials are drawn %d times, want 1", n)
	}
	accesstest.Find(t, r.tree(), access.RoleImage, "Dana Lee")

	withImage := newAccessRig(func(ctx *Context) Element { return Avatar("Dana Lee").Image(testImage()) })
	if n := len(partNodes(withImage.rt, "Avatar.Initials")); n != 0 {
		t.Errorf("an avatar with an image draws %d initials, want none", n)
	}
	accesstest.Find(t, withImage.tree(), access.RoleImage, "Dana Lee")

	decorative := newAccessRig(func(ctx *Context) Element { return Avatar("Dana Lee").Decorative() })
	accesstest.Absent(t, decorative.tree(), access.RoleImage, "Dana Lee")

	unnamed := newAccessRig(func(ctx *Context) Element { return Avatar("") })
	accesstest.Find(t, unnamed.tree(), access.RoleImage, "Avatar")
}

// TestAvatarSize checks that the diameter is the Size at the default Text size, and
// grows with the Text size.
func TestAvatarSize(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element { return Avatar("Dana Lee").Size(40) })
	if w := partNode(t, r.rt, "Avatar.Avatar", "").style.Width; w != Px(40) {
		t.Errorf("the diameter is %v at 100 percent, want 40", w)
	}
	big := newAccessRig(func(ctx *Context) Element { return Avatar("Dana Lee").Size(40) })
	big.rt.SetSystemSettings(SystemSettings{TextScale: 150})
	big.frame()
	if w := partNode(t, big.rt, "Avatar.Avatar", "").style.Width; w != Px(60) {
		t.Errorf("the diameter is %v at 150 percent, want 60", w)
	}
}

// TestAvatarUnderHighContrast checks that an app color does not reach the avatar
// under high contrast: it is the window color with an outline, and the initials
// are the text color.
func TestAvatarUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Avatar = AvatarStyle{Avatar: Style{Background: markBG}, Initials: Style{TextColor: markFG}}
	r := newThemedRig(&theme, func(ctx *Context) Element { return Avatar("Dana Lee") })
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	n := partNode(t, r.rt, "Avatar.Avatar", "")
	if n.style.Background != th.Surface {
		t.Errorf("high contrast: the avatar is %v, want the window color %v", n.style.Background, th.Surface)
	}
	if n.style.BorderWidth != 1 || n.style.BorderColor != th.Text {
		t.Errorf("high contrast: the border is %v wide in %v, want 1 in the text color", n.style.BorderWidth, n.style.BorderColor)
	}
	wantTextColor(t, partNode(t, r.rt, "Avatar.Initials", ""), "Avatar.Initials", th.Text)
}

func testAvatarParts(t *testing.T, applied map[string]bool) {
	applied["Avatar.Avatar"] = true
	t.Run("Avatar.Avatar", func(t *testing.T) {
		var st AvatarStyle
		st.Avatar = Style{Background: markBG}
		r := newAccessRig(func(ctx *Context) Element { return Avatar("Dana Lee").Styles(st) })
		wantBackground(t, partNode(t, r.rt, "Avatar.Avatar", ""), "Avatar.Avatar", markBG)
	})
	applied["Avatar.Initials"] = true
	t.Run("Avatar.Initials", func(t *testing.T) {
		var st AvatarStyle
		st.Initials = Style{TextColor: markFG}
		r := newAccessRig(func(ctx *Context) Element { return Avatar("Dana Lee").Styles(st) })
		wantTextColor(t, partNode(t, r.rt, "Avatar.Initials", ""), "Avatar.Initials", markFG)
	})
}
