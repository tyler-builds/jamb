package views

import (
	"fmt"

	"github.com/tyler-builds/jamb/ui"
)

// feedbackSection is the Feedback part of the Components page: progress bars,
// busy rings, toasts, badges, alerts and keyboard shortcuts. Its toasts appear in the window's Toaster and
// are also listed by the bell (see notify).
func feedbackSection(ctx *ui.Context, t *ui.Theme, s *Store) ui.Element {
	count := ui.Local(ctx, "feedback-count", 0)
	alertShown := ui.Local(ctx, "feedback-alert", true)
	// Undo puts the count back, so the toast's action does real work.
	increment := func() {
		before := count.Peek()
		count.Set(before + 1)
		notify(ctx, s, ui.ToastSuccess, fmt.Sprintf("Count is %d", before+1)).
			Action("Undo", func() { count.Set(before) })
	}
	return section(t, "Feedback",
		sample(t, "Determinate", ui.ProgressBar(0.6).Label("Determinate progress").Width(ui.Px(240))),
		sample(t, "Steps, 3 of 8", ui.ProgressBar(3).Min(0).Max(8).Label("Setup steps").
			Format(func(v float64) string { return fmt.Sprintf("%.0f of 8", v) }).Width(ui.Px(240))),
		sample(t, "Indeterminate", ui.ProgressBar(0).Indeterminate().Label("Searching").Width(ui.Px(240))),
		sample(t, "Busy ring, three sizes", ui.Div().Row().Align(ui.Center).Gap(16).
			Child(ui.ProgressRing().Size(ui.RingSmall).Label("Small busy indicator")).
			Child(ui.ProgressRing().Size(ui.RingMedium).Label("Medium busy indicator")).
			Child(ui.ProgressRing().Size(ui.RingLarge).Label("Large busy indicator"))),
		sample(t, "Toasts: one per variant", ui.Div().Row().Wrap().Gap(8).
			Child(ui.Button("Info toast").Secondary().OnClick(func() {
				notify(ctx, s, ui.ToastInfo, "Sync started")
			})).
			Child(ui.Button("Success toast").Secondary().OnClick(func() {
				notify(ctx, s, ui.ToastSuccess, "Changes saved")
			})).
			Child(ui.Button("Warning toast").Secondary().OnClick(func() {
				notify(ctx, s, ui.ToastWarning, "Disk nearly full")
			})).
			Child(ui.Button("Error toast").Secondary().OnClick(func() {
				notify(ctx, s, ui.ToastError, "Upload failed. It stays until you dismiss it.")
			}))),
		sample(t, fmt.Sprintf("Toast with Undo: count %d", count.Get()),
			ui.Button("Increment").Secondary().OnClick(increment)),
		sample(t, "OS notification, with two buttons",
			ui.Button("Send OS notification").Secondary().OnClick(func() { sendOSNotification(ctx, s) })),
		sample(t, "Badges: variants", ui.Div().Row().Wrap().Gap(8).Align(ui.Center).
			Child(ui.BadgeText("Neutral")).
			Child(ui.BadgeText("Accent").Variant(ui.BadgeAccent)).
			Child(ui.BadgeText("Success").Variant(ui.BadgeSuccess)).
			Child(ui.BadgeText("Warning").Variant(ui.BadgeWarning)).
			Child(ui.BadgeText("Danger").Variant(ui.BadgeDanger))),
		sample(t, "Badges: counts and dot", ui.Div().Row().Wrap().Gap(8).Align(ui.Center).
			Child(ui.Badge(6).Variant(ui.BadgeAccent).Noun("items")).
			Child(ui.Badge(150).Variant(ui.BadgeDanger)).
			Child(ui.BadgeDot().Variant(ui.BadgeAccent))),
		sample(t, "Badge on a row", ui.Div().Row().Align(ui.Center).Gap(10).Radius(8).PaddingX(10).PaddingY(6).
			Border(1, t.Border).Label("Projects").Badged(ui.Badge(6).Noun("items")).
			Child(ui.Text("Projects").Medium())),
		sample(t, "Alerts: variants, action and OnClose", alertSamples(alertShown)),
		sample(t, "Keyboard shortcuts", ui.Div().Row().Wrap().Gap(12).Align(ui.Center).
			Child(ui.Kbd("ctrl+shift+p")).
			Child(ui.Kbd("ctrl+k ctrl+t")).
			Child(ui.Kbd("f2").Tooltip("Rename"))),
	)
}

// alertSamples shows an alert of each variant. The success alert has a close
// button, and a button brings it back once it is closed.
func alertSamples(shown *ui.Signal[bool]) ui.Element {
	col := ui.Div().Gap(8).Width(ui.Px(420)).
		Child(ui.Alert("Sync paused").Description("Reconnect to resume syncing.")).
		Child(ui.Alert("Disk nearly full").Warning().Description("Free some space to keep saving drafts.")).
		Child(ui.Alert("Upload failed").Danger().Description("The file is larger than 25 MB.").
			Action(ui.Button("Choose another file").Small().Secondary().OnClick(pressed("Choose another file"))))
	if shown.Get() {
		col.Child(ui.Alert("Changes saved").Success().OnClose(func() { shown.Set(false) }))
	} else {
		col.Child(ui.Button("Show the success alert").Small().Secondary().OnClick(func() { shown.Set(true) }))
	}
	return col
}
