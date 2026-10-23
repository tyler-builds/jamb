package views

import (
	"fmt"
	"time"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// inboxLimit is how many notifications the bell keeps.
const inboxLimit = 20

// inboxItem is one notification: a toast the app showed, kept for the bell.
type inboxItem struct {
	Text    string
	Variant ui.ToastVariant
	At      time.Time
	Read    bool
}

// notify shows a toast and keeps it in the bell's list. The toast is what the
// user sees and hears; the list is what the bell shows later. The toast is
// returned so that the caller can add an action to it.
func notify(ctx *ui.Context, s *Store, v ui.ToastVariant, text string) *ui.Toast {
	s.addInbox(v, text)
	return ctx.Toast(v, text)
}

// addInbox puts a notification at the top of the list, unread.
func (s *Store) addInbox(v ui.ToastVariant, text string) {
	s.Inbox.Update(func(items []inboxItem) []inboxItem {
		items = append([]inboxItem{{Text: text, Variant: v, At: time.Now()}}, items...)
		if len(items) > inboxLimit {
			items = items[:inboxLimit]
		}
		return items
	})
}

// unreadCount is the number of notifications the user has not read.
func unreadCount(items []inboxItem) int {
	n := 0
	for _, it := range items {
		if !it.Read {
			n++
		}
	}
	return n
}

// inboxButton is the bell in the header. Its badge counts the unread
// notifications, and its name says the count too. A click opens the list.
func inboxButton(ctx *ui.Context, s *Store) ui.Element {
	t := ctx.Theme()
	items := s.Inbox.Get()
	unread := unreadCount(items)
	open := ui.Local(ctx, "open", false)
	close := func() { open.Set(false) }

	name := "Notifications"
	if unread > 0 {
		name = fmt.Sprintf("Notifications, %d unread", unread)
	}
	bell := ui.IconButton(icons.Bell).Key("bell").Label(name).
		OnClick(func() { open.Set(!open.Peek()) })
	if !open.Get() {
		// No tooltip under the list, which would show behind it.
		bell.Tooltip("Notifications")
	}
	// The badge is drawn over the bell's corner. Its text is in the bell's name,
	// so the badge is hidden from assistive technology.
	badge := ui.Div().AccessHidden().Absolute().Right(-2).Top(-4).Child(ui.Badge(unread).Variant(ui.BadgeDanger))
	wrap := ui.Div().Shrink(0).Child(bell).Child(badge)
	if !open.Get() {
		return wrap
	}
	rows := ui.Div().Column().Gap(2)
	if len(items) == 0 {
		rows.Child(ui.Div().Padding(8).Child(ui.Text("No notifications").FontSize(13).TextColor(t.TextMuted)))
	}
	for _, it := range items {
		rows.Child(inboxRow(t, it))
	}
	list := ui.Div().Key("inbox").Overlay(ui.BelowEnd).Width(ui.Px(320)).Padding(8).Gap(6).
		Radius(10).Background(t.Surface).Border(1, t.Border).Shadow(ui.ShadowLg).
		Enter(ui.Fade().Slide(0, -6).Grow(0.98), ui.Animation{Duration: 140 * time.Millisecond}).
		Exit(ui.Fade().Grow(0.98), ui.Animation{Duration: 90 * time.Millisecond}).
		OnClickOutside(close).
		Child(ui.Div().Row().Align(ui.Center).Justify(ui.SpaceBetween).PaddingX(4).
			Child(ui.Text("Notifications").FontSize(13).Semibold()).
			Child(ui.Button("Mark all read").Key("inbox-read").Ghost().Small().Disabled(unread == 0).
				OnClick(func() { s.markAllRead() }))).
		Child(rows)
	return wrap.Child(list)
}

// inboxRow is one notification in the list: its icon, its text and when it
// happened. An unread one is in bold and has a dot.
func inboxRow(t *ui.Theme, it inboxItem) ui.Element {
	ic, c := toastIconFor(t, it.Variant)
	row := ui.Div().Row().Align(ui.Center).Gap(10).PaddingX(4).PaddingY(6).Radius(6).
		Child(ui.Icon(ic).Size(15).Color(c)).
		Child(ui.Div().Grow(1).MinWidth(ui.Px(0)).Gap(1).
			Child(ui.Text(it.Text).FontSize(13).FontWeight(weightOf(it.Read)).TextColor(textOf(t, it.Read))).
			Child(ui.Text(ago(it.At)).FontSize(11).TextColor(t.TextSubtle)))
	if !it.Read {
		row.Child(ui.BadgeDot().Variant(ui.BadgeAccent))
	}
	return row
}

func weightOf(read bool) ui.FontWeight {
	if read {
		return ui.FontRegular
	}
	return ui.FontSemibold
}

func textOf(t *ui.Theme, read bool) ui.Color {
	if read {
		return t.TextMuted
	}
	return t.Text
}

// markAllRead marks every notification read, so the badge goes to zero.
func (s *Store) markAllRead() {
	s.Inbox.Update(func(items []inboxItem) []inboxItem {
		out := make([]inboxItem, len(items))
		for i, it := range items {
			it.Read = true
			out[i] = it
		}
		return out
	})
}

// toastIconFor is the icon and the color of a toast variant in the demo's
// lists. It matches the icons the framework's toasts use.
func toastIconFor(t *ui.Theme, v ui.ToastVariant) (*icons.Icon, ui.Color) {
	switch v {
	case ui.ToastSuccess:
		return icons.Check, t.Success
	case ui.ToastWarning:
		return icons.Alert, t.Warning
	case ui.ToastError:
		return icons.X, t.Danger
	}
	return icons.Info, t.Accent
}
