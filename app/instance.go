package app

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

// A single instance app has one process that owns its windows. The first launch
// creates a named mutex and serves a named pipe. A later launch finds the mutex
// taken, writes its command line to the pipe and exits without a window. The
// first instance runs the command line on its UI thread, and brings its first
// window forward. Each request is one frame: a four-byte big-endian length, then
// the JSON of a handoff. The answer is one byte.

const (
	// handoffVersion is the protocol version. A server refuses the requests of
	// any other version, so two builds of jamb never misread each other.
	handoffVersion = 1

	// maxHandoff is the largest request a server reads, in bytes.
	maxHandoff = 1 << 20

	// dialTimeout bounds how long a later launch waits for the first instance to
	// start serving its pipe, or to free it when another launch holds it.
	dialTimeout = 3 * time.Second

	// ackTimeout bounds how long a later launch waits for the answer. It is longer
	// than uiTimeout, so an answer that is on its way arrives first.
	ackTimeout = 4 * time.Second

	// uiTimeout bounds how long the first instance waits for its UI thread to take
	// a command line. A busy UI thread then refuses it, and the later launch runs
	// on its own.
	uiTimeout = 2 * time.Second
)

// The answers of a server.
const (
	ackAccepted = 'A'
	ackRefused  = 'R'
)

// errHandoffTimeout is returned when the first instance does not answer in time.
var errHandoffTimeout = errors.New("the running instance did not answer in time")

// handoff is one request: the arguments of a later launch, and its working
// directory, which resolves the relative paths in the arguments.
type handoff struct {
	Version int      `json:"version"`
	Dir     string   `json:"dir"`
	Args    []string `json:"args"`
}

// instanceNames are the names of the objects that one app uses for one user in one
// session, and the user's SID, which the pipe's security descriptor grants access
// to.
type instanceNames struct{ mutex, pipe, sid string }

// instanceNamesFor returns the mutex and pipe names of appID for the user with the
// SID sid in the session. The mutex is local to the session. The pipe is
// machine-wide, so its name carries a digest of the user and the session, which
// keeps two users, or two sessions of one user, from sharing a name. The digest is
// not a secret, so it is a 64-bit FNV-1a hash rather than a cryptographic one: the
// pipe's security descriptor is what keeps other users out.
func instanceNamesFor(appID, sid string, session uint32) (mutex, pipe string) {
	h := fnv.New64a()
	h.Write([]byte(sid + "\x00" + strconv.FormatUint(uint64(session), 10)))
	return `Local\jamb-` + appID, fmt.Sprintf(`\\.\pipe\jamb-%s-%016x`, appID, h.Sum64())
}

// SingleInstance makes the app a single instance per user and session. A later
// launch hands its command line to the first instance and exits with status 0,
// without opening a window. The first instance calls onArgs on its UI thread with
// the arguments after the program name, as os.Args[1:] gives them, and the working
// directory of the later launch, so a relative path can be resolved with
// filepath.Join(dir, arg). Before onArgs runs, the first instance's first window
// is brought to the front. onArgs may be nil.
//
// The first launch keeps its own arguments in os.Args, as usual. SingleInstance
// needs an AppID, which names the instance. A later launch whose hand-off fails
// runs on its own, with its own window and state. It does not depend on JumpList:
// a jump list task starts a new process, and a single instance app receives that
// process's arguments through its hand-off.
func (a *App) SingleInstance(onArgs func(args []string, dir string)) *App {
	a.singleInstance = true
	a.onArgs = onArgs
	return a
}

// joinInstance decides this launch's role. It returns the names the process must
// serve when it is the first instance, and handed reports whether the first
// instance took the command line, in which case the process must return without a
// window. Problems are printed. A launch that cannot join runs on its own, so a
// fault here never stops the app.
func (a *App) joinInstance() (owned *instanceNames, handed bool) {
	names, err := currentInstanceNames(a.appID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jamb: single instance is off for this launch:", err)
		return nil, false
	}
	first, err := claimInstance(names.mutex)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jamb: single instance is off for this launch:", err)
		return nil, false
	}
	if first {
		return &names, false
	}
	handed, err = handOff(names.pipe)
	switch {
	case err != nil:
		fmt.Fprintln(os.Stderr, "jamb: the running instance did not take this launch:", err)
	case !handed:
		fmt.Fprintln(os.Stderr, "jamb: the running instance refused this launch")
	}
	return nil, handed
}

// handOff sends this launch's command line to the first instance at pipe. It
// reports whether the instance took it.
func handOff(pipe string) (bool, error) {
	conn, pid, err := dialHandoff(pipe, dialTimeout)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	// The first instance may bring its window forward when it takes the request.
	// It can only do so if this process allows it, which must come before the
	// request is sent.
	allowForeground(pid)
	dir, _ := os.Getwd()
	return sendHandoff(conn, handoff{Version: handoffVersion, Dir: dir, Args: os.Args[1:]}, ackTimeout)
}

// serveHandoffs answers later launches on a goroutine of its own, through the
// pipe the first instance owns. Each command line is run on the UI thread by
// deliverHandoff.
func (a *App) serveHandoffs(names *instanceNames) {
	onArgs := a.onArgs
	err := servePipe(*names, func(conn io.ReadWriter) {
		// A request that cannot be read is refused by serveHandoff, and that is
		// not a fault of the app.
		_ = serveHandoff(conn, func(h handoff) bool { return deliverHandoff(h, onArgs) })
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "jamb: later launches cannot hand over their command line:", err)
	}
}

// deliverHandoff runs a later launch's command line on the UI thread. It brings
// the first window forward and calls onArgs. It reports false, so the later launch
// runs on its own, when the app has no window left to take the request, which
// means it is quitting, or when the UI thread does not take it in time. The
// claimed flag decides between the two sides: whichever acts first wins, so a
// command line that was refused never runs later.
func deliverHandoff(h handoff, onArgs func(args []string, dir string)) bool {
	var claimed atomic.Bool
	done := make(chan bool, 1)
	postUI(func() {
		if !claimed.CompareAndSwap(false, true) {
			return // the wait gave up first; the later launch runs on its own
		}
		if len(application.windows) == 0 || application.quitting {
			done <- false
			return
		}
		application.windows[0].win.Focus()
		if onArgs != nil {
			onArgs(h.Args, h.Dir)
		}
		done <- true
	})
	select {
	case ok := <-done:
		return ok
	case <-uiDone:
		return false // the UI loop has ended, so nothing will run the request
	case <-time.After(uiTimeout):
		if claimed.CompareAndSwap(false, true) {
			return false
		}
		// The UI thread has the request, and is running it: wait for its answer.
		return <-done
	}
}

// writeHandoff writes h as one frame to w.
func writeHandoff(w io.Writer, h handoff) error {
	body, err := json.Marshal(h)
	if err != nil {
		return err
	}
	if len(body) > maxHandoff {
		return fmt.Errorf("the command line is %d bytes; the limit is %d", len(body), maxHandoff)
	}
	frame := make([]byte, 4, 4+len(body))
	binary.BigEndian.PutUint32(frame, uint32(len(body)))
	_, err = w.Write(append(frame, body...))
	return err
}

// readHandoff reads one frame from r. It refuses a frame that is empty, too large,
// or of another protocol version, before it reads the body.
func readHandoff(r io.Reader) (handoff, error) {
	var head [4]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return handoff{}, err
	}
	n := binary.BigEndian.Uint32(head[:])
	if n == 0 || n > maxHandoff {
		return handoff{}, fmt.Errorf("handoff: a request of %d bytes is not accepted", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return handoff{}, err
	}
	var h handoff
	if err := json.Unmarshal(body, &h); err != nil {
		return handoff{}, fmt.Errorf("handoff: %w", err)
	}
	if h.Version != handoffVersion {
		return handoff{}, fmt.Errorf("handoff: protocol version %d is not %d", h.Version, handoffVersion)
	}
	return h, nil
}

// serveHandoff reads one request from conn and answers it. The answer is accepted
// when deliver reports that the command line was taken, and refused otherwise. A
// request that cannot be read is refused without calling deliver, and the error is
// returned.
func serveHandoff(conn io.ReadWriter, deliver func(handoff) bool) error {
	h, err := readHandoff(conn)
	if err != nil {
		_, _ = conn.Write([]byte{ackRefused})
		return err
	}
	ack := byte(ackRefused)
	if deliver(h) {
		ack = ackAccepted
	}
	_, err = conn.Write([]byte{ack})
	return err
}

// sendHandoff sends h over conn and waits for the answer, for at most timeout. It
// reports whether the server took the command line. A timeout is an error, and
// the caller then runs on its own: the server gives its answer quickly, and a
// stalled server must not hold up the launch.
func sendHandoff(conn io.ReadWriter, h handoff, timeout time.Duration) (bool, error) {
	type result struct {
		ok  bool
		err error
	}
	res := make(chan result, 1)
	go func() {
		if err := writeHandoff(conn, h); err != nil {
			res <- result{false, err}
			return
		}
		var ack [1]byte
		_, err := io.ReadFull(conn, ack[:])
		res <- result{ack[0] == ackAccepted && err == nil, err}
	}()
	select {
	case r := <-res:
		return r.ok, r.err
	case <-time.After(timeout):
		return false, errHandoffTimeout
	}
}
