package views

import (
	"github.com/tyler-builds/jamb/app"
	"github.com/tyler-builds/jamb/ui"
)

// sendOSNotification shows a sample OS notification with two buttons. The
// notification's clicks come back to NotificationClicked. A notification that cannot
// be shown is reported in the window.
func sendOSNotification(ctx *ui.Context, s *Store) {
	err := app.Notify(app.Notification{
		Title: "Sample build finished",
		Body:  "The sample build finished with 3 warnings. Open the report, or dismiss this notification.",
		Actions: []app.NotificationAction{
			{ID: "open", Label: "Open"},
			{ID: "dismiss", Label: "Dismiss"},
		},
		Tag:  "demo-build",
		Args: "sample-build",
	})
	if err != nil {
		notify(ctx, s, ui.ToastError, "The OS notification was not shown: "+err.Error())
	}
}

// NotificationClicked returns the handler of the OS notifications that the demo
// sends. It lists the click in the inbox, with the button that was clicked. The
// click's data comes from outside the app, so only the demo's own button IDs are
// named; any other ID is reported as unknown.
func NotificationClicked(s *Store) func(n app.NotificationActivation) {
	return func(n app.NotificationActivation) {
		where := "the notification body"
		switch n.Action {
		case "":
		case "open", "dismiss":
			where = "the " + n.Action + " button"
		default:
			where = "an unknown button"
		}
		s.addInbox(ui.ToastInfo, "OS notification clicked on "+where)
	}
}
