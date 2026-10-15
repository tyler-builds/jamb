package app

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	"image/png"
	"io"
	"math/rand/v2"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/internal/notifyscheme"
)

// Notification is an operating system notification: a Windows toast, shown in the
// Notification Center. Notify shows it; see the README's OS notifications section.
//
// Write Title and Body as complete sentences. Windows reads them aloud with screen
// readers, so "3 warnings in the build" is read as written, and "3 warnings" alone
// is not a sentence a listener can place.
type Notification struct {
	// Title is the first line, in bold. It is required.
	Title string
	// Body is the text below the title. It may be empty.
	Body string
	// Image is shown as a hero image: a banner above the text, which Windows scales
	// to fit the notification. It is saved as a PNG file in the temporary folder, as
	// Windows reads images from files; the file is named by a digest of its contents,
	// so the same image is saved once.
	Image image.Image
	// Actions are the buttons, in order: at most five. A click on a button reports
	// the button's ID to OnNotification as NotificationActivation.Action.
	Actions []NotificationAction
	// Tag identifies the notification. A later notification with the same tag
	// replaces the one that is shown, and RemoveNotification removes it. It is at
	// most 64 characters, and empty for none.
	Tag string
	// Args is returned unchanged to OnNotification as NotificationActivation.Args
	// when the notification is activated. Put an ID or a short string in it, not a
	// secret: it is stored in the notification's activation data.
	Args string
	// Silent shows the notification without its sound.
	Silent bool
}

// NotificationAction is a button of a Notification.
type NotificationAction struct {
	// ID is what OnNotification receives when the button is clicked. It is required
	// and unique within the notification.
	ID string
	// Label is the text of the button. It is required.
	Label string
}

// NotificationActivation is a click on a notification, reported to OnNotification.
type NotificationActivation struct {
	// Action is the ID of the button that was clicked, or empty for a click on the
	// body of the notification.
	Action string
	// Tag and Args are the Tag and Args of the notification.
	Tag string
	// Args is the Args of the notification, as Tag describes.
	Args string
}

// The limits of a notification. The toast limits are Windows's: five buttons, and
// tags of up to 64 characters. The activation limit is jamb's: the data a click
// carries back, as a URI, must fit in maxActivationLen characters.
const (
	maxNotificationActions = 5
	maxNotificationTag     = 64
	maxActivationLen       = 512
	maxActivationJSON      = 1024
	activationVersion      = 1
	activationHost         = "activate"
	maxRecentActivations   = 64
	notificationImageDir   = "jamb-notifications"
)

// OnNotification sets the function that is called on the UI thread when the user
// clicks a notification that the app sent, on the body or on a button. It needs
// SingleInstance and an AppID: a click on a notification starts the app's program
// with the click's data, and SingleInstance hands that to the running app, which
// then calls fn in place of starting a second window. A click after the app has
// closed starts the app and calls fn once its window is open.
//
// The data of a click is passed through the notification system, which any program
// of the user can also send. fn must treat Action, Tag and Args as input from
// outside the app, and check them before it acts on them.
func (a *App) OnNotification(fn func(n NotificationActivation)) *App {
	a.notifyHandler = fn
	return a
}

// check reports a notification that cannot be shown, before anything is written.
func (n Notification) check() error {
	if strings.TrimSpace(n.Title) == "" {
		return errors.New("app.Notify: Title is required")
	}
	if utf8.RuneCountInString(n.Tag) > maxNotificationTag {
		return fmt.Errorf("app.Notify: Tag is %d characters; the limit is %d", utf8.RuneCountInString(n.Tag), maxNotificationTag)
	}
	if len(n.Actions) > maxNotificationActions {
		return fmt.Errorf("app.Notify: %d actions; the limit is %d", len(n.Actions), maxNotificationActions)
	}
	seen := map[string]bool{}
	for i, act := range n.Actions {
		if act.ID == "" {
			return fmt.Errorf("app.Notify: action %d has no ID", i+1)
		}
		if strings.TrimSpace(act.Label) == "" {
			return fmt.Errorf("app.Notify: action %q has no Label", act.ID)
		}
		if seen[act.ID] {
			return fmt.Errorf("app.Notify: action ID %q is used twice", act.ID)
		}
		seen[act.ID] = true
	}
	if n.Image != nil && n.Image.Bounds().Empty() {
		return errors.New("app.Notify: Image is empty")
	}
	return nil
}

// activationData is what a click carries back: the button, the tag, the args and a
// nonce that tells one click from another, so a click that reaches the app twice,
// by two routes, is delivered once.
type activationData struct {
	V      int    `json:"v"`
	Action string `json:"a,omitempty"`
	Tag    string `json:"t,omitempty"`
	Args   string `json:"g,omitempty"`
	Nonce  string `json:"n"`
}

// encodeActivation returns the URI that carries d for the scheme:
// scheme://activate?d=<base64url of the JSON>. It is an error when the URI is longer
// than maxActivationLen.
func encodeActivation(scheme string, d activationData) (string, error) {
	d.V = activationVersion
	body, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	uri := scheme + "://" + activationHost + "?d=" + base64.RawURLEncoding.EncodeToString(body)
	if len(uri) > maxActivationLen {
		return "", fmt.Errorf("app.Notify: the action IDs, Tag and Args make %d characters of activation data; the limit is %d, so shorten them", len(uri), maxActivationLen)
	}
	return uri, nil
}

// decodeActivation reads the URI that encodeActivation made for scheme. It refuses
// anything else, so data from another program or a damaged URI is never delivered.
func decodeActivation(scheme, raw string) (activationData, error) {
	var d activationData
	if len(raw) > maxActivationLen {
		return d, fmt.Errorf("the activation is %d characters; the limit is %d", len(raw), maxActivationLen)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return d, fmt.Errorf("the activation is not a URI: %w", err)
	}
	if u.Scheme != scheme || u.Host != activationHost {
		return d, errors.New("the activation is not one of this app's notifications")
	}
	body, err := base64.RawURLEncoding.DecodeString(u.Query().Get("d"))
	if err != nil || len(body) == 0 || len(body) > maxActivationJSON {
		return d, errors.New("the activation data is missing or damaged")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, fmt.Errorf("the activation data is damaged: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return d, errors.New("the activation data has trailing text")
	}
	if d.V != activationVersion {
		return d, fmt.Errorf("the activation data is version %d; this app reads version %d", d.V, activationVersion)
	}
	if d.Nonce == "" || len(d.Nonce) > 64 {
		return d, errors.New("the activation has no valid nonce")
	}
	return d, nil
}

// splitActivation takes the activation URI of scheme out of args. It returns the
// URI, the other arguments, and whether an activation was found. A later launch
// hands over its arguments as they were on its command line, so an activation
// arrives as one of them.
func splitActivation(scheme string, args []string) (raw string, rest []string, found bool) {
	prefix := scheme + "://"
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			if !found {
				raw = a
				found = true
			}
			continue
		}
		rest = append(rest, a)
	}
	return raw, rest, found
}

// activationLog remembers the nonces of the recent activations, so one click is
// delivered once. It is only used on the UI thread.
type activationLog struct {
	seen  map[string]bool
	order []string
}

// first reports whether nonce is new, and records it.
func (l *activationLog) first(nonce string) bool {
	if l.seen == nil {
		l.seen = map[string]bool{}
	}
	if l.seen[nonce] {
		return false
	}
	l.seen[nonce] = true
	l.order = append(l.order, nonce)
	if len(l.order) > maxRecentActivations {
		delete(l.seen, l.order[0])
		l.order = l.order[1:]
	}
	return true
}

// notifySettings is what the notification calls need from the running app. It is
// set on the UI thread when the app starts, and read from any goroutine.
type notifySettings struct {
	appID    string
	scheme   string
	packaged bool
	handler  func(NotificationActivation)
}

var (
	notifyCurrent     atomic.Pointer[notifySettings]
	recentActivations activationLog // only used on the UI thread
)

// notificationArgs takes a notification activation out of args, and posts it to the
// handler on the UI thread. It returns the other arguments, and whether there was an
// activation. Without a handler, args are returned unchanged.
func (a *App) notificationArgs(args []string) (rest []string, found bool) {
	if a.notifyHandler == nil || a.appID == "" {
		return args, false
	}
	raw, rest, found := splitActivation(notifyscheme.Activation(a.appID), args)
	if found {
		postUI(func() { deliverActivation(raw) })
	}
	return rest, found
}

// handoffArgs returns the function that a later launch's arguments are passed to.
// It takes a notification activation out first, and calls the SingleInstance
// function with the rest. The function is not called when only an activation was
// given, because the window is brought forward either way.
func (a *App) handoffArgs() func(args []string, dir string) {
	onArgs := a.onArgs
	return func(args []string, dir string) {
		rest, found := a.notificationArgs(args)
		if found && len(rest) == 0 {
			return
		}
		if onArgs != nil {
			onArgs(rest, dir)
		}
	}
}

// deliverActivation decodes a click and calls the handler, once per nonce. A click
// that cannot be decoded is reported on stderr and ignored. It runs on the UI thread.
func deliverActivation(raw string) {
	s := notifyCurrent.Load()
	if s == nil || s.handler == nil {
		return
	}
	d, err := decodeActivation(s.scheme, raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jamb: notification click ignored:", err)
		return
	}
	if !recentActivations.first(d.Nonce) {
		return
	}
	s.handler(NotificationActivation{Action: d.Action, Tag: d.Tag, Args: d.Args})
}

// newNonce returns a random 16-character hexadecimal nonce. It tells one click from
// another, so that a click delivered twice is handled once; it is not a secret, so it
// comes from math/rand/v2 and the package links no crypto package (whose import would
// add the 32 MB buffer of Go's FIPS module to every app).
func newNonce() (string, error) {
	return fmt.Sprintf("%016x", rand.Uint64()), nil
}

// toastAction is a button of a toast: its label and the arguments a click gives back.
type toastAction struct {
	Label     string
	Arguments string
}

// toastContent is what toastXML builds from. Protocol makes a click start the app
// through its registered scheme, with Launch as the URI, rather than a foreground
// activation; see the README.
type toastContent struct {
	Title    string
	Body     string
	ImageURI string
	Protocol bool
	Launch   string
	Actions  []toastAction
	Silent   bool
}

// toastXML returns the toast content XML of c. Every text and attribute is escaped,
// so no title, body, label or argument can add markup.
func toastXML(c toastContent) string {
	var b strings.Builder
	b.WriteString("<toast")
	if c.Protocol {
		b.WriteString(` activationType="protocol" launch="` + xmlText(c.Launch) + `"`)
	}
	b.WriteString(`><visual><binding template="ToastGeneric">`)
	if c.ImageURI != "" {
		b.WriteString(`<image placement="hero" src="` + xmlText(c.ImageURI) + `"/>`)
	}
	b.WriteString("<text>" + xmlText(c.Title) + "</text>")
	if c.Body != "" {
		b.WriteString("<text>" + xmlText(c.Body) + "</text>")
	}
	b.WriteString("</binding></visual>")
	if len(c.Actions) > 0 {
		b.WriteString("<actions>")
		for _, a := range c.Actions {
			b.WriteString(`<action content="` + xmlText(a.Label) + `" arguments="` + xmlText(a.Arguments) + `"`)
			if c.Protocol {
				b.WriteString(` activationType="protocol"`)
			}
			b.WriteString("/>")
		}
		b.WriteString("</actions>")
	}
	if c.Silent {
		b.WriteString(`<audio silent="true"/>`)
	}
	b.WriteString("</toast>")
	return b.String()
}

// xmlText escapes s for an XML text node or a double-quoted attribute. Characters
// that XML cannot hold are replaced by U+FFFD.
func xmlText(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// notificationImagePath returns the file that holds a hero image with the PNG data
// in dir: a file named by the 128-bit FNV-1a hash of the data. Two images that collide
// would share a file, which only shows the wrong banner, so a non-cryptographic hash
// serves.
func notificationImagePath(dir string, data []byte) string {
	h := fnv.New128a()
	h.Write(data)
	return filepath.Join(dir, hex.EncodeToString(h.Sum(nil))+".png")
}

// writeNotificationImage saves img as a PNG in dir, unless the same image is already
// there, and returns the file's path. The file is written under a temporary name and
// renamed, so a reader never sees part of it.
func writeNotificationImage(dir string, img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("app.Notify: encoding the image: %w", err)
	}
	path := notificationImagePath(dir, buf.Bytes())
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

// fileURI returns the file URI of the absolute path, as the toast image source
// takes it: file:///C:/dir/name.png, with spaces and other characters escaped.
func fileURI(path string) string {
	u := url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(path)}
	return u.String()
}

// shortcutName returns the name of the Start menu shortcut of the program exe: its
// file name without the extension, with the characters Windows does not allow in
// names removed. When nothing is left, the AppID is used.
func shortcutName(exe, appID string) string {
	name := strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	var b strings.Builder
	for _, r := range name {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 0x20 {
			continue
		}
		b.WriteRune(r)
	}
	if s := strings.Trim(b.String(), " ."); s != "" {
		return s
	}
	return appID
}

// regValue is a value under HKEY_CURRENT_USER: the key's path, the value's name (empty
// for the default value), and a string.
type regValue struct {
	Key   string
	Name  string
	Value string
}

// shortcutCandidate is a Start menu shortcut found in the Programs folders, with what
// the registration needs to know about it.
type shortcutCandidate struct {
	Path     string // full path of the .lnk file
	Target   string // the program the shortcut starts
	AppID    string // its System.AppUserModel.ID, empty when it has none
	Writable bool   // whether the file can be written, so that the AppID can be set on it
}

// shortcutAction is what the registration does with the program's Start menu shortcut.
type shortcutAction int

const (
	// shortcutKeep: a shortcut to the program already carries the AppID, so nothing is
	// written.
	shortcutKeep shortcutAction = iota
	// shortcutStamp: the AppID is set on an existing shortcut.
	shortcutStamp
	// shortcutCreate: the program's own shortcut is written.
	shortcutCreate
)

// chooseShortcut returns what the registration does with the Start menu shortcut of
// the program exe, and the shortcut's path. found holds the shortcuts in the Programs
// folders, in search order: the per-user folder first, then the all-users one. created
// is the path of the shortcut that would be written. Only a shortcut whose target is
// exe counts, and the rules are these:
//
//   - a shortcut that already carries appID is kept, whether or not it is writable,
//     because nothing needs writing;
//   - otherwise the first writable shortcut has appID set on it;
//   - otherwise the shortcut at created is written, so that a shortcut that cannot be
//     changed does not stop the registration.
//
// It is pure, so the rules are tested without the Start menu.
func chooseShortcut(appID, exe, created string, found []shortcutCandidate) (shortcutAction, string) {
	stamp := ""
	for _, c := range found {
		if !samePath(c.Target, exe) {
			continue
		}
		if c.AppID == appID {
			return shortcutKeep, c.Path
		}
		if c.Writable && stamp == "" {
			stamp = c.Path
		}
	}
	if stamp != "" {
		return shortcutStamp, stamp
	}
	return shortcutCreate, created
}

// samePath reports whether a and b name the same file: both are made absolute and
// cleaned, and compared without case, as Windows file names are.
func samePath(a, b string) bool {
	return normalPath(a) == normalPath(b)
}

// normalPath returns the absolute, cleaned form of p in lower case. An empty path stays
// empty, rather than becoming the working directory.
func normalPath(p string) string {
	if p == "" {
		return ""
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return strings.ToLower(filepath.Clean(p))
}

// notifyPlan is what the first notification of a process registers for the user: the
// AppID on the program's Start menu shortcut, which is set on an existing shortcut or
// created, and, when the app has an OnNotification handler, the protocol that a click
// starts the app through.
type notifyPlan struct {
	Action   shortcutAction
	Shortcut string // full path of the .lnk file that is kept, stamped or created
	Target   string // the program the shortcut starts
	AppID    string
	Values   []regValue
}

// planNotifyRegistration returns what registration does for the program exe, with the
// Start menu's per-user Programs folder programs and the shortcuts found in the Programs
// folders (see chooseShortcut). It writes nothing; the tests use it to check the
// registration without touching the registry or the Start menu.
//
// The protocol is registered as HKCU\Software\Classes\<scheme>, with its URL Protocol
// value, and its command under \shell\open\command, which runs the program with the
// URI as its only argument.
func planNotifyRegistration(appID, exe, programs string, found []shortcutCandidate, protocol bool) notifyPlan {
	action, path := chooseShortcut(appID, exe, filepath.Join(programs, shortcutName(exe, appID)+".lnk"), found)
	p := notifyPlan{
		Action:   action,
		Shortcut: path,
		Target:   exe,
		AppID:    appID,
	}
	if !protocol {
		return p
	}
	scheme := notifyscheme.Activation(appID)
	key := `Software\Classes\` + scheme
	p.Values = []regValue{
		{Key: key, Name: "", Value: "URL:" + scheme},
		{Key: key, Name: "URL Protocol", Value: ""},
		{Key: key + `\shell\open\command`, Name: "", Value: `"` + exe + `" "%1"`},
	}
	return p
}

// notificationDir is the folder that hero images are saved in.
func notificationDir() string {
	return filepath.Join(os.TempDir(), notificationImageDir)
}
