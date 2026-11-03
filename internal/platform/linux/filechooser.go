//go:build linux

package linux

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/tyler-builds/jamb/dialog"
)

// File dialogs are the desktop's own, through the file chooser portal
// (org.freedesktop.portal.FileChooser): the portal shows the dialog and
// answers with a Response signal on a request object.

const (
	portalFileChooser = "org.freedesktop.portal.FileChooser"
	portalRequest     = "org.freedesktop.portal.Request"
)

// portalFilter is the portal's filter: a name and (kind, pattern) pairs, where
// kind 0 is a glob and 1 a MIME type.
type portalFilter struct {
	Name     string
	Patterns []portalPattern
}

type portalPattern struct {
	Kind    uint32
	Pattern string
}

// fileChooserOptions builds the options of OpenFile or SaveFile.
func fileChooserOptions(kind dialog.FileKind, o dialog.FileOptions, token string) map[string]dbus.Variant {
	opts := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token),
		"modal":        dbus.MakeVariant(true),
	}
	if kind == dialog.Folder {
		opts["directory"] = dbus.MakeVariant(true)
	}
	if kind == dialog.Open && o.Multiple {
		opts["multiple"] = dbus.MakeVariant(true)
	}
	if kind != dialog.Folder && len(o.Filters) > 0 {
		filters := make([]portalFilter, 0, len(o.Filters))
		for _, f := range o.Filters {
			pf := portalFilter{Name: f.Name}
			for _, p := range f.Patterns {
				pf.Patterns = append(pf.Patterns, portalPattern{0, p})
			}
			filters = append(filters, pf)
		}
		opts["filters"] = dbus.MakeVariant(filters)
		opts["current_filter"] = dbus.MakeVariant(filters[0])
	}
	if o.Folder != "" {
		// A byte string with its terminating NUL, as the portal takes paths.
		opts["current_folder"] = dbus.MakeVariant(append([]byte(o.Folder), 0))
	}
	if kind == dialog.Save && o.Name != "" {
		opts["current_name"] = dbus.MakeVariant(o.Name)
	}
	return opts
}

// requestPath is the object path the portal gives the request with the token:
// /org/freedesktop/portal/desktop/request/SENDER/TOKEN, where SENDER is the
// connection's unique name without its colon and with dots as underscores.
// Knowing it lets the Response signal be watched before the call is made, so
// it cannot be missed.
func requestPath(uniqueName, token string) dbus.ObjectPath {
	sender := strings.ReplaceAll(strings.TrimPrefix(uniqueName, ":"), ".", "_")
	return dbus.ObjectPath(portalPath + "/request/" + sender + "/" + token)
}

// showFileDialog asks the portal for a file dialog and calls done on the UI
// thread with the result.
func showFileDialog(kind dialog.FileKind, o dialog.FileOptions, done func(dialog.FileResult)) {
	go func() {
		res := runFileDialog(kind, o)
		Post(func() { done(res) })
	}()
}

func runFileDialog(kind dialog.FileKind, o dialog.FileOptions) dialog.FileResult {
	conn, err := sessionBus()
	if err != nil {
		return dialog.FileResult{Err: fmt.Errorf("platform/linux: no session bus for the file dialog: %w", err)}
	}
	token := fmt.Sprintf("jamb%d", rand.Uint64())
	path := requestPath(conn.Names()[0], token)
	match := []dbus.MatchOption{dbus.WithMatchObjectPath(path), dbus.WithMatchInterface(portalRequest), dbus.WithMatchMember("Response")}
	if err := conn.AddMatchSignal(match...); err != nil {
		return dialog.FileResult{Err: err}
	}
	defer conn.RemoveMatchSignal(match...)
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	title := o.Title
	if title == "" {
		title = map[dialog.FileKind]string{dialog.Open: "Open", dialog.Save: "Save", dialog.Folder: "Choose a folder"}[kind]
	}
	method := portalFileChooser + ".OpenFile"
	if kind == dialog.Save {
		method = portalFileChooser + ".SaveFile"
	}
	var handle dbus.ObjectPath
	obj := conn.Object(portalName, portalPath)
	if err := obj.Call(method, 0, "", title, fileChooserOptions(kind, o, token)).Store(&handle); err != nil {
		return dialog.FileResult{Err: fmt.Errorf("platform/linux: the file chooser portal: %w", err)}
	}
	if handle != path {
		// An older portal chose its own path; watch that one instead.
		conn.RemoveMatchSignal(match...)
		match[0] = dbus.WithMatchObjectPath(handle)
		conn.AddMatchSignal(match...)
		path = handle
	}
	for sig := range signals {
		if sig.Path != path || sig.Name != portalRequest+".Response" || len(sig.Body) < 2 {
			continue
		}
		return fileResult(kind, o, sig.Body[0], sig.Body[1])
	}
	return dialog.FileResult{Err: errors.New("platform/linux: the session bus closed")}
}

// fileResult reads the Response signal: response 0 is a choice, 1 a cancel,
// anything else a failure; results holds the chosen URIs.
func fileResult(kind dialog.FileKind, o dialog.FileOptions, response, results any) dialog.FileResult {
	code, _ := response.(uint32)
	switch code {
	case 0:
	case 1:
		return dialog.FileResult{Canceled: true}
	default:
		return dialog.FileResult{Err: errors.New("platform/linux: the file dialog failed")}
	}
	m, _ := results.(map[string]dbus.Variant)
	uris, _ := m["uris"].Value().([]string)
	var paths []string
	for _, u := range uris {
		p, err := pathOfURI(u)
		if err != nil {
			return dialog.FileResult{Err: err}
		}
		if kind == dialog.Save && o.Extension != "" && filepath.Ext(p) == "" {
			p += "." + strings.TrimPrefix(o.Extension, ".")
		}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		return dialog.FileResult{Canceled: true}
	}
	return dialog.FileResult{Paths: paths}
}

// pathOfURI returns the local path of a file:// URI.
func pathOfURI(u string) (string, error) {
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme != "file" {
		return "", fmt.Errorf("platform/linux: the file dialog returned %q, which is not a local file", u)
	}
	return parsed.Path, nil
}
