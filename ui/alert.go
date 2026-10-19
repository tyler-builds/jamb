package ui

import (
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// AlertVariant is the kind of an alert. It sets the icon and the color.
type AlertVariant uint8

const (
	// AlertInfo is an alert for information. It is the default.
	AlertInfo AlertVariant = iota
	// AlertSuccess is an alert for a completed action or a good state.
	AlertSuccess
	// AlertWarning is an alert for a situation that needs attention.
	AlertWarning
	// AlertDanger is an alert for an error or a dangerous state.
	AlertDanger
)

// AlertStyle is the look of each part of an alert. A part's style merges over the
// built-in defaults of its variant, then over the theme's Alert, then over the
// theme's Alerts for that variant, then over the Styles of the instance; see
// docs/styling.md. Alert is the box, Icon the icon of its variant (its TextColor
// is the icon's color), Title and Description the two texts, and Close the box
// around the close button, which an alert has only when it has OnClose. A
// variant's colors are in its defaults, so an override of Alert or Icon sets the
// color for the variants it applies to.
type AlertStyle struct {
	Alert       Style // the box: its shape, fill and border
	Icon        Style // the icon, in the color of its variant
	Title       Style // the title
	Description Style // the text under the title
	Close       Style // the box around the close button
}

// merge returns s with the set fields of o applied on top, part by part.
func (s AlertStyle) merge(o AlertStyle) AlertStyle {
	return AlertStyle{
		Alert:       s.Alert.Merge(o.Alert),
		Icon:        s.Icon.Merge(o.Icon),
		Title:       s.Title.Merge(o.Title),
		Description: s.Description.Merge(o.Description),
		Close:       s.Close.Merge(o.Close),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s AlertStyle) plain() AlertStyle {
	return AlertStyle{
		Alert:       plainStyle(s.Alert),
		Icon:        plainStyle(s.Icon),
		Title:       plainStyle(s.Title),
		Description: plainStyle(s.Description),
		Close:       plainStyle(s.Close),
	}
}

// AlertVariants holds the style of each variant of an alert, for the theme's
// Alerts. It merges over Theme.Alert, which applies to every variant.
type AlertVariants struct {
	// Info, Success, Warning and Danger are the styles of the alert variants, and each merges over Alert.
	Info, Success, Warning, Danger AlertStyle
}

// of returns the style of variant v.
func (av AlertVariants) of(v AlertVariant) AlertStyle {
	switch v {
	case AlertSuccess:
		return av.Success
	case AlertWarning:
		return av.Warning
	case AlertDanger:
		return av.Danger
	}
	return av.Info
}

// alertDefaults returns the built-in look of an alert of variant v, from the
// colors of t: a box tinted with the variant's color, the icon in that color, the
// title in the text color and the description in the muted text color. Under
// high contrast the box is the window color outlined in the text color, and the
// icon is the text color, so the icon's shape tells the variant.
func alertDefaults(t *Theme, v AlertVariant) AlertStyle {
	fill, edge, ink := alertColors(t, v)
	d := AlertStyle{
		Alert: Style{Direction: Row, Align: Start, Gap: 10, Padding: XY(12, 10), Radius: t.Radius,
			Background: fill, BorderWidth: 1, BorderColor: edge},
		Icon:        Style{NoShrink: true, Padding: Edges{Top: 1}, TextColor: ink},
		Title:       Style{FontWeight: FontSemibold, TextColor: t.Text},
		Description: Style{FontSize: 13, TextColor: t.TextMuted},
		Close:       Style{NoShrink: true},
	}
	if t.HighContrast {
		d.Description.TextColor = t.Text
	}
	return d
}

// alertColors returns the fill, the border and the icon color of an alert. The
// fill is the surface with a tenth or so of the variant's color, so the text
// colors keep their contrast on it in both themes; the icon is the variant's
// color moved towards the text color, which keeps it 3:1 or more on the fill.
func alertColors(t *Theme, v AlertVariant) (fill, edge, ink Color) {
	if t.HighContrast {
		return t.Surface, t.Text, t.Text
	}
	tone := t.Accent
	switch v {
	case AlertSuccess:
		tone = t.Success
	case AlertWarning:
		tone = t.Warning
	case AlertDanger:
		tone = t.Danger
	}
	return t.Surface.Mix(tone, 0.12), t.Surface.Mix(tone, 0.45), tone.Mix(t.Text, 0.3)
}

// alertIcon returns the default icon of a variant. Warning and danger share the
// alert icon, since the X that a toast shows for an error is the close button here.
func alertIcon(v AlertVariant) *icons.Icon {
	switch v {
	case AlertSuccess:
		return icons.Check
	case AlertWarning, AlertDanger:
		return icons.Alert
	}
	return icons.Info
}

// resolveAlert returns the look of an alert of variant v: the defaults, merged
// with the theme's Alert, the theme's Alerts for the variant, and the instance's
// Styles. Under high contrast the overrides are plain (see plainStyle).
func resolveAlert(ctx *Context, v AlertVariant, inst AlertStyle) AlertStyle {
	t := ctx.Theme()
	return layered(ctx, alertDefaults(t, v), AlertStyle.merge, AlertStyle.plain,
		themeLayer(t.Alert), themeLayer(t.Alerts.of(v)), instanceLayer(inst))
}

// AlertElement is a message box in the flow of the page, such as a note above a
// form or the result of an action: an icon, a title, an optional description and
// action, and an optional close button. It is a group named by its title, with
// the description as its description.
//
// An alert is not a live region: it is not announced when it appears. An app that
// shows one after an action the user should hear about announces it too, with
// Announce.
type AlertElement struct {
	title   string
	desc    string
	variant AlertVariant
	icon    *icons.Icon
	iconSet bool // Icon was called; a nil icon then hides it
	action  Element
	onClose func()
	key     string
	styles  AlertStyle
	size    sizing // the alert's sizes, from the sizing methods (see Sizing in docs/styling.md)
	tooltip string // see Tooltip
}

// Alert creates an info alert titled title. An alert fills the width of its
// container by default.
func Alert(title string) *AlertElement { return &AlertElement{title: title} }

func (*AlertElement) isElement()           {}
func (a *AlertElement) elementKey() string { return a.key }

// Key identifies the alert among its siblings (see DivElement.Key).
func (a *AlertElement) Key(k string) *AlertElement { a.key = k; return a }

// Description sets the text under the title.
func (a *AlertElement) Description(s string) *AlertElement { a.desc = s; return a }

// Variant sets the kind: AlertInfo, AlertSuccess, AlertWarning or AlertDanger.
func (a *AlertElement) Variant(v AlertVariant) *AlertElement { a.variant = v; return a }

// Info sets the AlertInfo variant, the default.
func (a *AlertElement) Info() *AlertElement { return a.Variant(AlertInfo) }

// Success sets the AlertSuccess variant.
func (a *AlertElement) Success() *AlertElement { return a.Variant(AlertSuccess) }

// Warning sets the AlertWarning variant.
func (a *AlertElement) Warning() *AlertElement { return a.Variant(AlertWarning) }

// Danger sets the AlertDanger variant.
func (a *AlertElement) Danger() *AlertElement { return a.Variant(AlertDanger) }

// Icon replaces the variant's icon. Icon(nil) shows no icon.
func (a *AlertElement) Icon(ic *icons.Icon) *AlertElement { a.icon, a.iconSet = ic, true; return a }

// Action adds el under the description, such as a button that fixes what the
// alert reports.
func (a *AlertElement) Action(el Element) *AlertElement { a.action = el; return a }

// OnClose adds a close button at the end of the alert, which calls fn when it is
// pressed with the mouse, or with Enter or Space. The button is named with the
// Close text of the window's Strings. fn removes the alert, by changing the state
// it is built from.
func (a *AlertElement) OnClose(fn func()) *AlertElement { a.onClose = fn; return a }

// Tooltip shows s below the alert after the pointer rests on it. Assistive
// technology reads it as the alert's description, when it has none.
func (a *AlertElement) Tooltip(s string) *AlertElement { a.tooltip = s; return a }

// The sizing methods give the alert's outer box a size. See Sizing in docs/styling.md.
func (a *AlertElement) Width(l Length) *AlertElement { a.size.setWidth(l); return a }

// MinWidth sets the smallest width of the alert.
func (a *AlertElement) MinWidth(l Length) *AlertElement { a.size.setMinWidth(l); return a }

// MaxWidth sets the largest width of the alert.
func (a *AlertElement) MaxWidth(l Length) *AlertElement { a.size.setMaxWidth(l); return a }

// Grow makes the alert take a share of the free space on its parent's main axis.
func (a *AlertElement) Grow(f float32) *AlertElement { a.size.setGrow(f); return a }

// GridSpan sets how many columns and rows the alert covers in its parent's grid.
func (a *AlertElement) GridSpan(cols, rows int) *AlertElement { a.size.setSpan(cols, rows); return a }

// Styles sets the look of the parts of the alert, for this instance only. Set
// fields win over the theme's Alert and Alerts and the defaults; see AlertStyle
// and docs/styling.md. Calling Styles again merges with the earlier values.
func (a *AlertElement) Styles(st AlertStyle) *AlertElement {
	a.styles = a.styles.merge(st)
	return a
}

func (a *AlertElement) expand(ctx *Context) Element {
	st := resolveAlert(ctx, a.variant, a.styles)
	strs := ctx.Strings()
	// The alert is a group named by its title, and described by its description.
	// An alert with no title is named by its description.
	name, desc := a.title, a.desc
	if name == "" {
		name, desc = desc, ""
	}
	box := Div().asPart("Alert.Alert").Style(st.Alert).Role(access.RoleGroup).Label(name).Tooltip(a.tooltip)
	if desc != "" {
		box.Description(desc)
	}
	ic := alertIcon(a.variant)
	if a.iconSet {
		ic = a.icon
	}
	if ic != nil {
		box.Child(Div().asPart("Alert.Icon").Style(st.Icon).Child(Icon(ic).Size(16)))
	}
	body := Div().Column().Gap(2).Grow(1).MinWidth(Px(0))
	if a.title != "" {
		body.Child(Div().asPart("Alert.Title").Style(st.Title).Child(Text(a.title)))
	}
	if a.desc != "" {
		body.Child(Div().asPart("Alert.Description").Style(st.Description).Child(Text(a.desc)))
	}
	if a.action != nil {
		body.Child(Div().Row().PaddingY(4).Child(a.action))
	}
	box.Child(body)
	if a.onClose != nil {
		box.Child(Div().asPart("Alert.Close").Style(st.Close).Child(
			IconButton(icons.X).Label(strs.Close).Small().Ghost().OnClick(a.onClose)))
	}
	return a.size.apply(box)
}
