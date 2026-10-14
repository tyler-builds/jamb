package ui

import (
	"hash/fnv"
	"strings"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/access"
)

// AvatarStyle is the look of each part of an avatar. A part's style merges over
// the built-in defaults, then over the theme's Avatar, then over the Styles of the
// instance; see docs/styling.md. Avatar is the circle: its border, and its fill,
// which the name picks unless the style sets one. Initials is the text in the
// circle, drawn when the avatar has no image.
type AvatarStyle struct {
	Avatar   Style // the circle: its fill and border
	Initials Style // the initials in the circle
}

// merge returns s with the set fields of o applied on top, part by part.
func (s AvatarStyle) merge(o AvatarStyle) AvatarStyle {
	return AvatarStyle{Avatar: s.Avatar.Merge(o.Avatar), Initials: s.Initials.Merge(o.Initials)}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s AvatarStyle) plain() AvatarStyle {
	return AvatarStyle{Avatar: plainStyle(s.Avatar), Initials: plainStyle(s.Initials)}
}

// avatarPalette is the fills an avatar takes from its name. Each one keeps
// avatarInk at 4.5:1 or more, so the initials read in both themes.
var avatarPalette = [...]Color{
	Hex("#4338ca"), Hex("#0f766e"), Hex("#b45309"), Hex("#be185d"),
	Hex("#15803d"), Hex("#6d28d9"), Hex("#1d4ed8"), Hex("#9f1239"),
}

// avatarInk is the color of the initials on a palette fill.
var avatarInk = Hex("#ffffff")

// avatarFill returns the fill of the avatar of name. A hash of the name picks it
// from the palette, so a name keeps its color from one render to the next.
func avatarFill(name string) Color {
	h := fnv.New32a()
	h.Write([]byte(name))
	return avatarPalette[h.Sum32()%uint32(len(avatarPalette))]
}

// avatarInitials returns the initials of name: the first letter of its first word
// and of its last word, uppercased, or one letter for a name of one word. A name
// with no words has no initials.
func avatarInitials(name string) string {
	words := strings.Fields(name)
	switch len(words) {
	case 0:
		return ""
	case 1:
		return firstLetter(words[0])
	}
	return firstLetter(words[0]) + firstLetter(words[len(words)-1])
}

// firstLetter returns the first letter of word, uppercased.
func firstLetter(word string) string {
	r, _ := utf8.DecodeRuneInString(word)
	return strings.ToUpper(string(r))
}

// avatarDefaults returns the built-in look of an avatar of name, whose diameter is
// size at the default Text size. The fill comes from avatarFill. The initials are
// 0.4 of size: the runtime scales the font with the Text size, as expand scales the
// circle. Under high contrast the circle is an outlined empty circle, and the
// initials are the text color.
func avatarDefaults(t *Theme, name string, size float32) AvatarStyle {
	d := AvatarStyle{
		Avatar:   Style{Direction: Row, Align: Center, Justify: Center, NoShrink: true, Background: avatarFill(name)},
		Initials: Style{FontSize: size * 0.4, FontWeight: FontSemibold, TextColor: avatarInk},
	}
	if t.HighContrast {
		d.Avatar.Background, d.Avatar.BorderWidth, d.Avatar.BorderColor = t.Surface, 1, t.Text
		d.Initials.TextColor = t.Text
	}
	return d
}

// resolveAvatar returns the look of an avatar: the defaults, merged with the
// theme's Avatar and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveAvatar(t *Theme, name string, size float32, inst AvatarStyle) AvatarStyle {
	st := avatarDefaults(t, name, size)
	for _, o := range []AvatarStyle{t.Avatar, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// AvatarElement is a circle that stands for a person: the initials of the name in
// a fill that the name picks, or an image cropped to the circle. It is an image
// named by the name, unless Decorative hides it.
type AvatarElement struct {
	name       string
	size       float32
	src        *ImageSource
	decorative bool
	key        string
	styles     AvatarStyle
}

// Avatar creates an avatar of name, a circle of 32 logical pixels with the
// initials of the name (see Size). An empty name is named with the Avatar text of
// the window's Strings for assistive technology.
func Avatar(name string) *AvatarElement { return &AvatarElement{name: name, size: 32} }

func (*AvatarElement) isElement()           {}
func (a *AvatarElement) elementKey() string { return a.key }

// Key identifies the avatar among its siblings (see DivElement.Key).
func (a *AvatarElement) Key(k string) *AvatarElement { a.key = k; return a }

// Image shows src in the circle, cropped to it, in place of the initials.
func (a *AvatarElement) Image(src *ImageSource) *AvatarElement { a.src = src; return a }

// Size sets the diameter, in logical pixels: 32 by default. The diameter and the
// initials grow with the Text size, as the text in a control does.
func (a *AvatarElement) Size(px float32) *AvatarElement { a.size = px; return a }

// Decorative hides the avatar from assistive technology. Use it for an avatar
// that stands beside the name of the person it shows, since the name is read
// already.
func (a *AvatarElement) Decorative() *AvatarElement { a.decorative = true; return a }

// Styles sets the look of the parts of the avatar, for this instance only. Set
// fields win over the theme's Avatar and the defaults; see AvatarStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (a *AvatarElement) Styles(st AvatarStyle) *AvatarElement {
	a.styles = a.styles.merge(st)
	return a
}

func (a *AvatarElement) expand(ctx *Context) Element {
	t := ctx.Theme()
	st := resolveAvatar(t, a.name, a.size, a.styles)
	// The diameter is set after the style, since the circle is always round.
	d := a.size * textScale(ctx)
	av := Div().asPart("Avatar.Avatar").Style(st.Avatar).Size(Px(d), Px(d)).Radius(d / 2)
	if a.src != nil {
		// The image is hidden: the circle is the one image, and it carries the name.
		av.Child(Div().AccessHidden().Child(Image(a.src).Size(Px(d), Px(d)).Radius(d / 2)))
	} else if initials := avatarInitials(a.name); initials != "" {
		av.Child(Div().asPart("Avatar.Initials").AccessHidden().Style(st.Initials).Child(Text(initials)))
	}
	if a.decorative {
		av.AccessHidden()
		return av
	}
	name := a.name
	if name == "" {
		name = ctx.Strings().Avatar
	}
	return av.Role(access.RoleImage).Label(name)
}
