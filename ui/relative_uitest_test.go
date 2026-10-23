package ui_test

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// A TimeAgo follows the window's clock: Advance drives its refreshes, so its text moves
// from minutes to hours, and then to "yesterday" once the time is on the day before.
func TestTimeAgoFollowsTheWindowClock(t *testing.T) {
	start := time.Date(2026, time.October, 23, 12, 0, 0, 0, time.Local)
	at := start.Add(-50 * time.Minute)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Key("root").Child(ui.TimeAgo(at).Key("ago"))
	}, uitest.Start(start))
	if !u.ByName("50 minutes ago").Exists() {
		t.Fatal("the TimeAgo does not show 50 minutes ago at the start")
	}
	u.Advance(10 * time.Minute)
	if !u.ByName("1 hour ago").Exists() {
		t.Fatal("after 10 minutes the TimeAgo does not show 1 hour ago")
	}
	u.Advance(24 * time.Hour)
	if !u.ByName("yesterday").Exists() {
		t.Fatal("a day later the TimeAgo does not show yesterday")
	}
}
