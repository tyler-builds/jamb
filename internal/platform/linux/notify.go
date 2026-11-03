//go:build linux

package linux

import (
	"fmt"
	"image"
	"image/draw"
	"sync"

	"github.com/godbus/dbus/v5"
)

// Notifications go to the desktop's notification server
// (org.freedesktop.Notifications), which KDE, GNOME and the other desktops run
// on the session bus. Clicks arrive as ActionInvoked signals while the app runs.

const (
	notifyName  = "org.freedesktop.Notifications"
	notifyPath  = "/org/freedesktop/Notifications"
	notifyIface = "org.freedesktop.Notifications"
	// defaultAction is the action of a click on the notification's body.
	defaultAction = "default"
	// maxNotifyImage bounds the larger side of an image sent with a
	// notification; servers show them small, and the pixels travel in the call.
	maxNotifyImage = 512
)

// Notification is a desktop notification.
type Notification struct {
	AppName string // the app's name, which the server shows and groups by
	Title   string
	Body    string
	Image   image.Image // shown beside the text; may be nil
	// Actions are (key, label) pairs of the buttons. A click on the body
	// reports DefaultAction.
	Actions [][2]string
	Silent  bool
	// Replaces is the id of a notification this one replaces, or 0.
	Replaces uint32
}

// DefaultAction is the action key that a click on a notification's body
// reports.
const DefaultAction = defaultAction

var notifyState struct {
	mu       sync.Mutex
	watching bool
	onAction func(id uint32, action string)
	onClosed func(id uint32)
}

// SetNotificationHandlers sets the functions called, on the UI thread, when the
// user clicks a notification (its body or a button) and when a notification
// closes.
func SetNotificationHandlers(onAction func(id uint32, action string), onClosed func(id uint32)) {
	notifyState.mu.Lock()
	notifyState.onAction, notifyState.onClosed = onAction, onClosed
	notifyState.mu.Unlock()
}

// ShowNotification shows n and returns its id. It is safe from any goroutine.
func ShowNotification(n Notification) (uint32, error) {
	conn, err := sessionBus()
	if err != nil {
		return 0, fmt.Errorf("no session bus: %w", err)
	}
	watchNotifications(conn)
	actions := []string{defaultAction, ""}
	for _, a := range n.Actions {
		actions = append(actions, a[0], a[1])
	}
	hints := map[string]dbus.Variant{}
	if n.Silent {
		hints["suppress-sound"] = dbus.MakeVariant(true)
	}
	if n.AppName != "" {
		hints["desktop-entry"] = dbus.MakeVariant(n.AppName)
	}
	if n.Image != nil {
		hints["image-data"] = dbus.MakeVariant(imageData(n.Image))
	}
	var id uint32
	err = conn.Object(notifyName, notifyPath).Call(notifyIface+".Notify", 0,
		n.AppName, n.Replaces, "", n.Title, n.Body, actions, hints, int32(-1)).Store(&id)
	if err != nil {
		return 0, fmt.Errorf("the notification server: %w", err)
	}
	return id, nil
}

// CloseNotification removes a notification from the screen and the history.
func CloseNotification(id uint32) error {
	conn, err := sessionBus()
	if err != nil {
		return fmt.Errorf("no session bus: %w", err)
	}
	return conn.Object(notifyName, notifyPath).Call(notifyIface+".CloseNotification", 0, id).Err
}

// notifyImage is the image-data hint: width, height, row stride, alpha,
// bits per sample, channels, and straight-alpha RGBA bytes.
type notifyImage struct {
	Width, Height, Stride int32
	HasAlpha              bool
	Bits, Channels        int32
	Data                  []byte
}

func imageData(img image.Image) notifyImage {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if s := max(w, h); s > maxNotifyImage {
		w, h = max(w*maxNotifyImage/s, 1), max(h*maxNotifyImage/s, 1)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w == b.Dx() && h == b.Dy() {
		draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	} else {
		// Nearest neighbor is enough for a thumbnail the server shrinks again.
		for y := range h {
			for x := range w {
				dst.Set(x, y, img.At(b.Min.X+x*b.Dx()/w, b.Min.Y+y*b.Dy()/h))
			}
		}
	}
	return notifyImage{Width: int32(w), Height: int32(h), Stride: int32(dst.Stride), HasAlpha: true, Bits: 8, Channels: 4, Data: dst.Pix}
}

// watchNotifications delivers the server's ActionInvoked and
// NotificationClosed signals to the handlers, once per process.
func watchNotifications(conn *dbus.Conn) {
	notifyState.mu.Lock()
	defer notifyState.mu.Unlock()
	if notifyState.watching {
		return
	}
	notifyState.watching = true
	conn.AddMatchSignal(dbus.WithMatchInterface(notifyIface), dbus.WithMatchObjectPath(notifyPath))
	ch := make(chan *dbus.Signal, 16)
	conn.Signal(ch)
	go func() {
		for sig := range ch {
			if sig.Path != notifyPath || len(sig.Body) < 2 {
				continue
			}
			id, _ := sig.Body[0].(uint32)
			notifyState.mu.Lock()
			onAction, onClosed := notifyState.onAction, notifyState.onClosed
			notifyState.mu.Unlock()
			switch sig.Name {
			case notifyIface + ".ActionInvoked":
				action, _ := sig.Body[1].(string)
				if onAction != nil {
					Post(func() { onAction(id, action) })
				}
			case notifyIface + ".NotificationClosed":
				if onClosed != nil {
					Post(func() { onClosed(id) })
				}
			}
		}
	}()
}
