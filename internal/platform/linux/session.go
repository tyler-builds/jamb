//go:build linux

package linux

import (
	"fmt"
	"os"
	"slices"
	"strconv"

	"github.com/tyler-builds/jamb/internal/platform/linux/wl"
)

// Window placement across jamb dev's restarts, with xdg-session-management-v1
// (wayland-protocols staging): a Wayland client cannot place its windows, but it
// can ask the compositor to put them back where a session had them.
//
// Under jamb dev, JAMB_WL_SESSION_FILE names a file that holds the session's
// ID. The app opens that session, or a new one, names each window by the order
// it opened in (window-1, window-2, ...), and asks the compositor to restore it
// before its first commit; a name the session does not know is added. When the
// compositor makes the session, its ID goes to the file, for the next build.
// ReleaseSession, called as jamb dev takes the app's state, lets go of the
// session, so that the build that replaces this one can take it over while this
// one's windows are still on screen.

// winSession is the window session of the process, or nil.
type winSession struct {
	file   string
	proxy  *wl.Proxy
	opened int // windows named so far
}

// openSession opens the session named by JAMB_WL_SESSION_FILE, if the
// compositor manages sessions; otherwise it does nothing.
func (d *display) openSession() {
	file := os.Getenv("JAMB_WL_SESSION_FILE")
	if file == "" || d.sessions == nil {
		return
	}
	s := &winSession{file: file}
	var id *string
	reason := uint32(1) // launch
	if b, err := os.ReadFile(file); err == nil && len(b) > 0 {
		v := string(b)
		id, reason = &v, 2 // recover: the same app, after a restart
	}
	trace := os.Getenv("JAMB_TRACE") == "1"
	if trace {
		if id != nil {
			fmt.Fprintf(os.Stderr, "jamb: window session: taking over session %s\n", *id)
		} else {
			fmt.Fprintln(os.Stderr, "jamb: window session: asking for a new session")
		}
	}
	s.proxy = d.conn.NewProxy("xdg_session_v1", 1, func(op uint16, dec *wl.Decoder) {
		if trace {
			fmt.Fprintf(os.Stderr, "jamb: window session: event %s\n", [...]string{"created", "restored", "replaced"}[min(op, 2)])
		}
		if op == 0 { // created(session_id)
			id := dec.String()
			tmp := s.file + ".tmp"
			if os.WriteFile(tmp, []byte(id), 0o600) == nil {
				os.Rename(tmp, s.file)
			}
		}
	})
	d.sessions.Send(1, wl.NewID{P: s.proxy}, reason, id) // get_session
	d.session = s
}

// placeWindow asks the session to put w where the window of the same name was.
// It must come before w's first commit.
func (d *display) placeWindow(w *window) {
	s := d.session
	if s == nil || s.proxy == nil {
		return
	}
	s.opened++
	name := "window-" + strconv.Itoa(s.opened)
	ts := d.conn.NewProxy("xdg_toplevel_session_v1", 1, func(op uint16, dec *wl.Decoder) {
		if op == 0 && os.Getenv("JAMB_TRACE") == "1" { // restored
			fmt.Fprintf(os.Stderr, "jamb: the compositor put %s back where it was\n", name)
		}
	})
	s.proxy.Send(3, wl.NewID{P: ts}, w.toplevel, name) // restore_toplevel
	w.toplevelSession = ts
}

// ReleaseSession closes the windows and lets go of the window session, so that
// another process takes it over with the windows where they were: KWin keeps
// a window's place in its session when the window's toplevel ends, and only
// while its entry in the session lives, so the windows go first, then their
// entries, then the session. The windows are gone from the screen at once;
// the process is about to be replaced. It runs on the UI thread, at once when
// called there, and waits until the compositor has handled it. Before the
// loop runs, or without one, as in tests, it does nothing.
func ReleaseSession() {
	if uiThread.Load() == 0 {
		return // no loop: no windows, and nothing would run the release
	}
	if OnUIThread() {
		releaseSession()
		return
	}
	done := make(chan struct{})
	Post(func() { releaseSession(); close(done) })
	<-done
}

func releaseSession() {
	d := disp
	if d == nil || d.session == nil || d.session.proxy == nil {
		return
	}
	for _, w := range slices.Clone(d.windows) {
		ts := w.toplevelSession
		if ts == nil {
			continue
		}
		w.destroy() // the compositor keeps where the window was
		ts.Send(0)  // destroy
		ts.Destroyed()
		w.toplevelSession = nil
	}
	d.session.proxy.Send(0) // destroy: the session stays for the next process
	d.session.proxy.Destroyed()
	d.session.proxy = nil
	d.conn.Roundtrip()
}
