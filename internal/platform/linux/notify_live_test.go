//go:build linux

package linux

import (
	"image"
	"image/color"
	"os"
	"testing"
	"time"
)

// TestNotificationLive shows a notification on the desktop, replaces it, and
// closes it. It needs a session bus with a notification server, so it runs only
// with JAMB_LIVE_TESTS=1.
func TestNotificationLive(t *testing.T) {
	if os.Getenv("JAMB_LIVE_TESTS") == "" {
		t.Skip("set JAMB_LIVE_TESTS=1 to show a notification")
	}
	img := image.NewRGBA(image.Rect(0, 0, 900, 300))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(1, 1, color.RGBA{255, 0, 0, 255})
	id, err := ShowNotification(Notification{AppName: "jamb-test", Title: "jamb test notification",
		Body: "This is a test from the jamb Linux adapter.", Image: img, Actions: [][2]string{{"open", "Open"}}, Silent: true})
	if err != nil {
		t.Fatal(err)
	}
	again, err := ShowNotification(Notification{AppName: "jamb-test", Title: "jamb test notification (replaced)",
		Body: "It replaced the first one.", Replaces: id, Silent: true})
	if err != nil {
		t.Fatal(err)
	}
	if again != id {
		t.Errorf("replacing gave id %d, want %d", again, id)
	}
	time.Sleep(1500 * time.Millisecond)
	if err := CloseNotification(id); err != nil {
		t.Error(err)
	}
}
