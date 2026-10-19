package ui

import (
	"fmt"
	"strings"

	"github.com/tyler-builds/jamb/access"
)

// KbdStyle is the look of each part of a keyboard shortcut. A part's style merges
// over the built-in defaults, then over the theme's Kbd, then over the Styles of
// the instance; see docs/styling.md. Kbd is the row that holds the key caps, and
// its Gap is the space between the strokes of a sequence such as "Ctrl+K Ctrl+T".
// Key is one key cap.
type KbdStyle struct {
	Kbd Style // the row of key caps; its Gap is the space between strokes
	Key Style // one key cap: its shape, fill, border and text
}

// merge returns s with the set fields of o applied on top, part by part.
func (s KbdStyle) merge(o KbdStyle) KbdStyle {
	return KbdStyle{Kbd: s.Kbd.Merge(o.Kbd), Key: s.Key.Merge(o.Key)}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s KbdStyle) plain() KbdStyle {
	return KbdStyle{Kbd: plainStyle(s.Kbd), Key: plainStyle(s.Key)}
}

// kbdDefaults returns the built-in look of a keyboard shortcut: small caps in the
// surface color, with a border that is heavier at the bottom, and muted text.
// Under high contrast the caps are outlined in the text color.
func kbdDefaults(t *Theme) KbdStyle {
	d := KbdStyle{
		Kbd: Style{Direction: Row, Align: Center, Gap: 6, NoShrink: true, fitWidth: true},
		Key: Style{Direction: Row, Align: Center, Justify: Center, NoShrink: true, MinWidth: Px(20),
			Padding: XY(5, 1), Radius: 4, Background: t.Surface, BorderColor: t.Border, FontSize: 12,
			TextColor: t.TextMuted, BorderEdges: Edges{Top: 1, Right: 1, Bottom: 2, Left: 1}, fitWidth: true},
	}
	if t.HighContrast {
		d.Key.BorderColor, d.Key.TextColor = t.Text, t.Text
	}
	return d
}

// resolveKbd returns the look of a keyboard shortcut: the defaults, merged with
// the theme's Kbd and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveKbd(ctx *Context, inst KbdStyle) KbdStyle {
	t := ctx.Theme()
	return layered(ctx, kbdDefaults(t), KbdStyle.merge, KbdStyle.plain, themeLayer(t.Kbd), instanceLayer(inst))
}

// KbdElement shows a keyboard shortcut as key caps, such as Ctrl, Shift and P for
// "ctrl+shift+p". Each key is a cap of its own, and the strokes of a sequence are
// spaced apart. The key names are the ones a menu shows for the same keys.
type KbdElement struct {
	strokes [][]string // the key names of each stroke, such as {"Ctrl", "P"}
	name    string     // the shortcut as text, such as "Ctrl+Shift+P"
	key     string
	styles  KbdStyle
	size    sizing // the shortcut's sizes, from the sizing methods (see Sizing in docs/styling.md)
	tooltip string // see Tooltip
}

// Kbd creates a keyboard shortcut from keys, written as for Bind: strokes
// separated by spaces, each stroke modifiers and one key joined by "+", such as
// "ctrl+shift+p" or "ctrl+k ctrl+t". Kbd panics on a malformed keys string, as
// Bind does.
func Kbd(keys string) *KbdElement {
	strokes, err := parseKeys(keys)
	if err != nil {
		panic(fmt.Sprintf("ui.Kbd(%q): %s", keys, err))
	}
	k := &KbdElement{}
	names := make([]string, len(strokes))
	for i, s := range strokes {
		// The caps and the name are made as a menu makes its shortcut text, so the
		// two match (see Runtime.KeysFor).
		names[i] = strokeLabel(s)
		k.strokes = append(k.strokes, strokeKeys(s))
	}
	k.name = strings.Join(names, " ")
	return k
}

func (*KbdElement) isElement()           {}
func (k *KbdElement) elementKey() string { return k.key }

// Key identifies the shortcut among its siblings (see DivElement.Key).
func (k *KbdElement) Key(key string) *KbdElement { k.key = key; return k }

// Tooltip shows s below the shortcut after the pointer rests on it, such as the
// command it runs. Assistive technology reads it as the shortcut's description.
func (k *KbdElement) Tooltip(s string) *KbdElement { k.tooltip = s; return k }

// The sizing methods give the shortcut's outer box a size. See Sizing in docs/styling.md.
func (k *KbdElement) Width(l Length) *KbdElement { k.size.setWidth(l); return k }

// MinWidth sets the smallest width of the shortcut.
func (k *KbdElement) MinWidth(l Length) *KbdElement { k.size.setMinWidth(l); return k }

// MaxWidth sets the largest width of the shortcut.
func (k *KbdElement) MaxWidth(l Length) *KbdElement { k.size.setMaxWidth(l); return k }

// Grow makes the shortcut take a share of the free space on its parent's main axis.
func (k *KbdElement) Grow(f float32) *KbdElement { k.size.setGrow(f); return k }

// GridSpan sets how many columns and rows the shortcut covers in its parent's grid.
func (k *KbdElement) GridSpan(cols, rows int) *KbdElement { k.size.setSpan(cols, rows); return k }

// Styles sets the look of the parts of the shortcut, for this instance only. Set
// fields win over the theme's Kbd and the defaults; see KbdStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (k *KbdElement) Styles(st KbdStyle) *KbdElement {
	k.styles = k.styles.merge(st)
	return k
}

func (k *KbdElement) expand(ctx *Context) Element {
	st := resolveKbd(ctx, k.styles)
	// The shortcut is one static text, such as "Ctrl+Shift+P", so it is read as
	// one thing; the caps under it are hidden.
	row := Div().asPart("Kbd.Kbd").Style(st.Kbd).Role(access.RoleStaticText).Label(k.name).Tooltip(k.tooltip)
	for _, keys := range k.strokes {
		// The caps of one stroke sit close together; the row's Gap spaces the strokes.
		stroke := Div().Row().Align(Center).Gap(3).AccessHidden()
		for _, name := range keys {
			stroke.Child(Div().asPart("Kbd.Key").Style(st.Key).Child(Text(name).NoWrap()))
		}
		row.Child(stroke)
	}
	return k.size.apply(row)
}
