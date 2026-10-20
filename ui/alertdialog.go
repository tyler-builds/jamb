package ui

import (
	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// AlertDialogIntent is what an alert dialog asks: it picks the dialog's icon,
// the look of its confirm button and where the focus goes when it opens.
type AlertDialogIntent uint8

const (
	// AlertDialogInfo is a message to acknowledge, with the Info icon and, as a
	// rule, the confirm button alone ("OK"). It is the default.
	AlertDialogInfo AlertDialogIntent = iota
	// AlertDialogConfirm asks the user to confirm an action, with the Info icon in
	// the accent color.
	AlertDialogConfirm
	// AlertDialogWarning asks about an action that needs care, with the Alert icon
	// in the warning colors.
	AlertDialogWarning
	// AlertDialogDestructive asks about an action that deletes or cannot be undone,
	// with the Alert icon in the danger colors and a Danger confirm button. The
	// focus starts on Cancel, and Enter in the dialog confirms only from the
	// confirm button.
	AlertDialogDestructive
)

// AlertDialogStyle is the look of each part of an alert dialog. A part's style
// merges over the built-in defaults of its intent, then over the theme's
// AlertDialog, then over the Styles of the instance; see docs/styling.md. Scrim,
// Panel, Body and Footer are a modal's parts. Header is the row of the icon and
// the title, Icon the icon, Title the title, Message the message, and Body the
// area under the header that holds the message and the Body element.
type AlertDialogStyle struct {
	Scrim   Style // the dimmed backdrop behind the dialog
	Panel   Style // the dialog
	Header  Style // the row of the icon and the title
	Icon    Style // the icon, before the title
	Title   Style // the title
	Message Style // the message, under the title
	Body    Style // the area of the message and the Body element
	Footer  Style // the row of buttons, at the bottom
}

// merge returns s with the set fields of o applied on top, part by part.
func (s AlertDialogStyle) merge(o AlertDialogStyle) AlertDialogStyle {
	return AlertDialogStyle{
		Scrim:   s.Scrim.Merge(o.Scrim),
		Panel:   s.Panel.Merge(o.Panel),
		Header:  s.Header.Merge(o.Header),
		Icon:    s.Icon.Merge(o.Icon),
		Title:   s.Title.Merge(o.Title),
		Message: s.Message.Merge(o.Message),
		Body:    s.Body.Merge(o.Body),
		Footer:  s.Footer.Merge(o.Footer),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s AlertDialogStyle) plain() AlertDialogStyle {
	return AlertDialogStyle{
		Scrim:   plainStyle(s.Scrim),
		Panel:   plainStyle(s.Panel),
		Header:  plainStyle(s.Header),
		Icon:    plainStyle(s.Icon),
		Title:   plainStyle(s.Title),
		Message: plainStyle(s.Message),
		Body:    plainStyle(s.Body),
		Footer:  plainStyle(s.Footer),
	}
}

// alertDialogDefaults returns the built-in look of an alert dialog of intent i,
// from the colors of t: a modal's parts, with the icon in the intent's color,
// moved towards the text color as an Alert's icon is, and the message in the
// muted text color. Under high contrast both are the text color.
func alertDialogDefaults(t *Theme, i AlertDialogIntent) AlertDialogStyle {
	m := modalDefaults(t)
	tone := t.Accent
	switch i {
	case AlertDialogWarning:
		tone = t.Warning
	case AlertDialogDestructive:
		tone = t.Danger
	}
	ink, muted := tone.Mix(t.Text, 0.3), t.TextMuted
	if t.HighContrast {
		ink, muted = t.Text, t.Text
	}
	return AlertDialogStyle{
		Scrim:   m.Scrim,
		Panel:   m.Panel,
		Header:  Style{Direction: Row, Align: Center, Gap: 10},
		Icon:    Style{NoShrink: true, TextColor: ink},
		Title:   m.Title,
		Message: Style{TextColor: muted},
		Body:    m.Body,
		Footer:  m.Footer,
	}
}

// resolveAlertDialog returns the look of an alert dialog of intent i: the
// defaults, merged with the theme's AlertDialog and then the instance's Styles.
// Under high contrast the two overrides are plain (see plainStyle).
func resolveAlertDialog(ctx *Context, i AlertDialogIntent, inst AlertDialogStyle) AlertDialogStyle {
	t := ctx.Theme()
	return layered(ctx, alertDialogDefaults(t, i), AlertDialogStyle.merge, AlertDialogStyle.plain,
		themeLayer(t.AlertDialog), instanceLayer(inst))
}

// AlertDialogElement is a modal dialog that asks the user to respond to a
// message, such as "Delete 3 files?", with a confirm button and, if it has one,
// a cancel button. It is a Modal with a fixed layout: an icon and the title, the
// message and an optional Body, and the buttons at the end of the footer, the
// app's Actions first, then Cancel, then the confirm button. Build it only while
// it is open, last in the window's root element, as a Modal is, and remove it in
// the confirm and cancel functions:
//
//	ui.If(asking.Get(), ui.AlertDialog("Delete 3 files?", "They cannot be restored.").
//		Intent(ui.AlertDialogDestructive).
//		Confirm("Delete", deleteFiles).
//		Cancel("", func() { asking.Set(false) }))
//
// Focus moves into it when it opens, to the confirm button, or to Cancel for
// AlertDialogDestructive, and returns when it closes; Tab stays inside it. Escape
// runs Cancel, or the confirm function when there is no Cancel. A press on the
// backdrop does nothing: the dialog waits for an answer. Enter presses the
// focused button; elsewhere in the dialog it confirms, except for
// AlertDialogDestructive, whose confirm button has to be pressed.
//
// Assistive technology hears an alert dialog (RoleAlertDialog) named by the title
// and described by the message.
type AlertDialogElement struct {
	title, message string
	intent         AlertDialogIntent
	confirmLabel   string
	onConfirm      func()
	cancelLabel    string
	onCancel       func()
	hasCancel      bool
	actions        []*ButtonElement
	body           Element
	icon           *icons.Icon
	iconSet        bool // Icon was called; a nil icon hides it
	loading        bool // see ConfirmLoading
	key            string
	styles         AlertDialogStyle
	size           sizing
}

// AlertDialog creates an alert dialog with a title and a message. Add its buttons
// with Confirm, Cancel and Action. Without Confirm it has an OK button that runs
// nothing.
func AlertDialog(title, message string) *AlertDialogElement {
	return &AlertDialogElement{title: title, message: message}
}

func (*AlertDialogElement) isElement()           {}
func (a *AlertDialogElement) elementKey() string { return a.key }

// Key identifies the dialog among its siblings (see DivElement.Key). It also
// scopes the key of its message.
func (a *AlertDialogElement) Key(k string) *AlertDialogElement { a.key = k; return a }

// Intent sets what the dialog asks (see AlertDialogIntent), AlertDialogInfo by
// default.
func (a *AlertDialogElement) Intent(i AlertDialogIntent) *AlertDialogElement { a.intent = i; return a }

// Confirm sets the label of the confirm button and the function it runs. An
// empty label is Strings.OK.
func (a *AlertDialogElement) Confirm(label string, fn func()) *AlertDialogElement {
	a.confirmLabel, a.onConfirm = label, fn
	return a
}

// Cancel adds a cancel button, Secondary, before the confirm button, with its
// label and the function it runs. An empty label is Strings.Cancel. Escape runs
// it too.
func (a *AlertDialogElement) Cancel(label string, fn func()) *AlertDialogElement {
	a.cancelLabel, a.onCancel, a.hasCancel = label, fn, true
	return a
}

// Action adds a button of the app's, such as "Save as…", before Cancel. Calling
// it again adds another, after the earlier ones.
func (a *AlertDialogElement) Action(b *ButtonElement) *AlertDialogElement {
	a.actions = append(a.actions, b)
	return a
}

// Body adds e under the message, such as a check box "Don't ask again".
func (a *AlertDialogElement) Body(e Element) *AlertDialogElement { a.body = e; return a }

// Icon replaces the intent's icon with ic; nil hides the icon.
func (a *AlertDialogElement) Icon(ic *icons.Icon) *AlertDialogElement {
	a.icon, a.iconSet = ic, true
	return a
}

// ConfirmLoading shows the confirm button as loading (see ButtonElement.Loading)
// while the work it started runs: it does nothing and keeps the focus. Escape
// does not confirm meanwhile.
func (a *AlertDialogElement) ConfirmLoading(on bool) *AlertDialogElement { a.loading = on; return a }

// Styles sets the look of the parts of the dialog, for this instance only. Set
// fields win over the theme's AlertDialog and the defaults; see AlertDialogStyle
// and docs/styling.md. Calling Styles again merges with the earlier values.
func (a *AlertDialogElement) Styles(st AlertDialogStyle) *AlertDialogElement {
	a.styles = a.styles.merge(st)
	return a
}

// The sizing methods give the panel a width, 420 px by default, as a Modal's. A
// percentage is of the window's width. See Sizing in docs/styling.md.
func (a *AlertDialogElement) Width(l Length) *AlertDialogElement { a.size.setWidth(l); return a }

// MinWidth sets the smallest width of the dialog.
func (a *AlertDialogElement) MinWidth(l Length) *AlertDialogElement { a.size.setMinWidth(l); return a }

// MaxWidth sets the largest width of the dialog.
func (a *AlertDialogElement) MaxWidth(l Length) *AlertDialogElement { a.size.setMaxWidth(l); return a }

// alertDialogIcon returns the default icon of an intent.
func alertDialogIcon(i AlertDialogIntent) *icons.Icon {
	if i == AlertDialogWarning || i == AlertDialogDestructive {
		return icons.Alert
	}
	return icons.Info
}

func (a *AlertDialogElement) expand(ctx *Context) Element {
	strs := ctx.Strings()
	st := resolveAlertDialog(ctx, a.intent, a.styles)
	destructive := a.intent == AlertDialogDestructive

	label := a.confirmLabel
	if label == "" {
		label = strs.OK
	}
	confirm := Button(label).OnClick(a.onConfirm).Loading(a.loading)
	if destructive {
		confirm.Danger()
	}
	confirm.handle = NewFocusHandle()
	footer := make([]Element, 0, len(a.actions)+2)
	for _, b := range a.actions {
		footer = append(footer, b)
	}
	var cancel *ButtonElement
	if a.hasCancel {
		cl := a.cancelLabel
		if cl == "" {
			cl = strs.Cancel
		}
		cancel = Button(cl).Secondary().OnClick(a.onCancel)
		cancel.handle = NewFocusHandle()
		footer = append(footer, cancel)
	}
	footer = append(footer, confirm)

	// Escape answers Cancel, or confirms a dialog that has nothing else to answer,
	// unless the confirm button is loading.
	escape := a.onCancel
	if !a.hasCancel {
		escape = a.onConfirm
		if a.loading {
			escape = nil
		}
	}

	ic := alertDialogIcon(a.intent)
	if a.iconSet {
		ic = a.icon
	}
	header := Div().asPart("AlertDialog.Header").Style(st.Header).Shrink(0)
	if ic != nil {
		header.Child(Div().asPart("AlertDialog.Icon").Style(st.Icon).Child(Icon(ic).Size(20)))
	}
	header.Child(Div().asPart("AlertDialog.Title").Style(st.Title).Child(Text(a.title)))

	// The message is the panel's description, linked by its key.
	msgKey := "jamb.alertdialog/message"
	if a.key != "" {
		msgKey = a.key + "/message"
	}
	body := Div().Column().Gap(st.Body.Gap)
	describedBy := ""
	if a.message != "" {
		body.Child(Div().asPart("AlertDialog.Message").Style(st.Message).Child(Text(a.message).Key(msgKey)))
		describedBy = msgKey
	}
	if a.body != nil {
		body.Child(a.body)
	}

	m := &ModalElement{title: a.title, body: body, footer: footer, onClose: escape, key: a.key, size: a.size,
		role: access.RoleAlertDialog, header: header, describedBy: describedBy,
		alert: &ModalStyle{Scrim: st.Scrim, Panel: st.Panel, Title: st.Title, Body: st.Body, Footer: st.Footer}}
	if destructive && cancel != nil {
		// A destructive action is never the answer that a stray Enter gives.
		m.InitialFocus(cancel.handle)
	} else {
		if !destructive {
			m.Default(confirm)
		}
		m.InitialFocus(confirm.handle)
	}
	return m.expand(ctx)
}
