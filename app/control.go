package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/tyler-builds/jamb/internal/devhook"
	"github.com/tyler-builds/jamb/ui"
)

// The control channel lets another program, such as a coding agent or `jamb ctl`,
// read and drive a running app: list its windows, read their accessibility
// trees, find elements, click, type, press keys, scroll, take screenshots and
// wait for the UI to settle or for an element to appear. It is JAMB_SCRIPT made
// interactive, and every answer is JSON.
//
// It is left out of a program unless the program links package devtools, which
// calls RegisterControl (as `jamb dev` does for every app it builds), so a
// release build carries none of its code. A linked program opens it only when
// JAMB_CONTROL=1, which `jamb dev` sets.
//
// The app listens on a Unix domain socket (on Windows too, which has them since
// Windows 10 1803) named after its process ID, in a directory only its user can
// read: JAMB_CONTROL_DIR, or jamb-UID in the temporary directory. Beside the
// socket a PID.json file describes the app, so that a client can find it. The
// app removes both when it ends.
//
// The protocol is one JSON object per line each way. A request names an op and
// its arguments; the answer carries the request's id back, and either a result
// or an error with a code and a message:
//
//	{"id":1,"op":"find","role":"button","name":"Save"}
//	{"id":1,"result":{"total":1,"nodes":[{"id":42,"role":"button","name":"Save",...}]}}
//	{"id":2,"op":"click","key":"missing"}
//	{"id":2,"error":{"code":"not_found","message":"no element has the key \"missing\""}}
//
// A connection answers its requests in order, one at a time; a client that wants
// to wait and act at once opens a second connection. See docs/tools.md for the ops.

// ctlProtocol is the version of the protocol, reported by the info op. It
// changes only when a change would break a client.
const ctlProtocol = 1

// Limits that keep one request or answer bounded.
const (
	ctlMaxLine      = 1 << 20 // bytes in one request line
	ctlMaxText      = 16 << 10
	ctlDefaultLimit = 100 // nodes in one tree or find answer
	ctlMaxLimit     = 256
	ctlMaxValue     = 512 // bytes of a node's value or name before it is cut
	ctlWaitDefault  = 5 * time.Second
	ctlWaitMax      = 60 * time.Second
	ctlQuietFrames  = 3 // frames with nothing to do before a window counts as idle
)

// controlStart opens the control channel. It is nil until RegisterControl.
var controlStart func()

// RegisterControl makes the control channel available in this program: with
// JAMB_CONTROL=1 in its environment, the app opens it at start (see
// docs/tools.md, "Control channel"). Package devtools calls it; an app does not
// need to.
func RegisterControl() { controlStart = startControl }

// startControl opens the control channel on the UI thread, before the first
// window, and tells the host to report frames to it.
func startControl() {
	c, err := listenControl(controlDir(), os.Getpid())
	if err != nil {
		fmt.Fprintln(os.Stderr, "jamb: the control channel did not open:", err)
		return
	}
	c.post = postUI
	c.windows = hostWindows
	application.controlFrame = c.hostFrame
	application.controlStop = c.close
	go c.serve()
	fmt.Fprintf(os.Stderr, "jamb: control channel open: jamb ctl -app %d info\n", c.pid)
}

// controlDir is the directory of the sockets and their descriptions.
func controlDir() string {
	if d := os.Getenv("JAMB_CONTROL_DIR"); d != "" {
		return d
	}
	name := "jamb-control"
	if uid := os.Getuid(); uid >= 0 {
		name = "jamb-" + strconv.Itoa(uid)
	}
	return filepath.Join(os.TempDir(), name)
}

// ctlWin is what a request acts on: one window, seen through the few things
// the ops need, so that tests can supply a headless runtime.
type ctlWin struct {
	rt      *ui.Runtime
	title   string
	w, h    float32 // the size in logical pixels
	scale   float32 // physical pixels per logical pixel
	visible bool
	// wake asks for a frame when the input left the runtime with work to draw.
	wake func()
	// redraw draws a frame whether or not anything changed.
	redraw func()
	// capture hands the next frame drawn to fn (see render.Renderer.CaptureNextFrame).
	capture func(fn func(*image.RGBA, error))
	// offscreen runs a frame of the runtime without drawing it: it builds and
	// lays out the tree, for a window that draws no frames because it is
	// hidden, minimized or, on GNOME, covered by other windows.
	offscreen func()
}

// control is an open control channel. Its waits are touched only on the UI thread.
type control struct {
	ln     net.Listener
	dir    string
	pid    int
	socket string
	desc   string // the PID.json file

	post     func(func())     // runs a function on the UI thread
	windows  func() []*ctlWin // the open windows, in the order they opened; UI thread only
	quit     chan struct{}    // closed by close
	once     sync.Once
	waits    []*ctlWait                  // UI thread only
	actions  []*ctlAction                // input ops waiting for their targets; UI thread only
	deferred []*ctlDeferred              // requests waiting for a frame; UI thread only
	pumping  map[int]bool                // windows whose undrawn frames pump runs
	drew     map[int]time.Time           // when each window last drew a frame
	shots    int                         // screenshots taken, which name the files of those without a path
	now      func() time.Time            // the clock of waits, replaced by tests
	after    func(time.Duration, func()) // runs a function later, replaced by tests
}

// ctlDescription is the PID.json file beside the socket.
type ctlDescription struct {
	Protocol int    `json:"protocol"`
	PID      int    `json:"pid"`
	Socket   string `json:"socket"`
	Exe      string `json:"exe"`
	Dir      string `json:"dir"` // the app's working directory
	Started  string `json:"started"`
}

// listenControl opens the socket and writes its description.
func listenControl(dir string, pid int) (*control, error) {
	if err := privateDir(dir); err != nil {
		return nil, err
	}
	pruneControl(dir)
	socket := filepath.Join(dir, strconv.Itoa(pid)+".sock")
	os.Remove(socket) // left by an earlier process with the same ID
	ln, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	os.Chmod(socket, 0o600)
	c := &control{ln: ln, dir: dir, pid: pid, socket: socket, desc: filepath.Join(dir, strconv.Itoa(pid)+".json"),
		quit: make(chan struct{}), now: time.Now, after: func(d time.Duration, fn func()) { time.AfterFunc(d, fn) }}
	exe, _ := os.Executable()
	wd, _ := os.Getwd()
	b, _ := json.MarshalIndent(ctlDescription{Protocol: ctlProtocol, PID: pid, Socket: socket, Exe: exe, Dir: wd,
		Started: time.Now().UTC().Format(time.RFC3339)}, "", "  ")
	if err := writeFileAtomic(c.desc, append(b, '\n')); err != nil {
		ln.Close()
		os.Remove(socket)
		return nil, err
	}
	return c, nil
}

// pruneControl removes the sockets and descriptions that apps which ended
// without closing their channel left in dir: those whose socket no longer
// answers. (`jamb dev` stops an app by killing it.)
func pruneControl(dir string) {
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		var d ctlDescription
		if err != nil || json.Unmarshal(b, &d) != nil || d.Socket == "" {
			continue
		}
		if c, err := net.DialTimeout("unix", d.Socket, 200*time.Millisecond); err == nil {
			c.Close()
			continue
		}
		os.Remove(d.Socket)
		os.Remove(f)
	}
}

// privateDir makes dir, if it is missing, readable by this user only, and
// refuses one that is a symbolic link or that others may use.
func privateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	return checkPrivate(dir, fi)
}

func writeFileAtomic(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// close stops listening and removes the socket and its description. Requests
// still waiting get a quit error.
func (c *control) close() {
	c.once.Do(func() {
		close(c.quit)
		c.ln.Close()
		os.Remove(c.socket)
		os.Remove(c.desc)
	})
}

func (c *control) serve() {
	for {
		conn, err := c.ln.Accept()
		if err != nil {
			return // closed
		}
		go c.handle(conn)
	}
}

// ctlRequest is a request. One struct holds the arguments of every op; each op
// reads the ones it documents.
type ctlRequest struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Op     string          `json:"op"`
	Window int             `json:"window,omitempty"` // 1-based, in the order windows opened; 0 is the first

	// What a node is found by: find and wait match all that are set, and click,
	// scroll, move and drag take the first match as their target.
	Node ctlID  `json:"node,omitempty"` // the node's id, from tree or find
	Key  string `json:"key,omitempty"`  // the element's key, exactly
	Role string `json:"role,omitempty"` // such as button; any case
	Name string `json:"name,omitempty"` // the accessible name, exactly
	// Contains matches a name or value that contains it, in any case.
	Contains string `json:"contains,omitempty"`

	X      *float32 `json:"x,omitempty"` // logical pixels, from the window's top left
	Y      *float32 `json:"y,omitempty"`
	DX     float32  `json:"dx,omitempty"` // scroll and drag, in logical pixels
	DY     float32  `json:"dy,omitempty"`
	Button string   `json:"button,omitempty"` // left (the default), right or middle
	Count  int      `json:"count,omitempty"`  // clicks: 2 double-clicks, 3 triple-clicks
	Keys   string   `json:"keys,omitempty"`   // key: such as "ctrl+s", or several separated by spaces
	Text   string   `json:"text,omitempty"`   // type

	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
	Revision string `json:"revision,omitempty"` // tree: the revision the offset continues

	For       string `json:"for,omitempty"`        // wait: idle, found or gone
	TimeoutMS int    `json:"timeout_ms,omitempty"` // wait, shot, and how long an input op's target may take to be ready

	Path string `json:"path,omitempty"` // shot: where to write the PNG; absolute

	Action  string          `json:"action,omitempty"`  // dispatch: the action's name
	Command string          `json:"command,omitempty"` // run: the command's name
	Args    json.RawMessage `json:"args,omitempty"`    // run: the command's input, a JSON value

	Freeze bool `json:"freeze,omitempty"` // checkpoint: drop input from now on, as the app is about to stop
}

type ctlResponse struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Result any             `json:"result,omitempty"`
	Error  *ctlError       `json:"error,omitempty"`
}

// ctlError is a failed request. Its codes are stable; its messages are for people.
type ctlError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ctlError) Error() string { return e.Code + ": " + e.Message }

func ctlErr(code, format string, args ...any) *ctlError {
	return &ctlError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// handle answers the requests of one connection, in order.
func (c *control) handle(conn net.Conn) {
	defer conn.Close()
	go func() {
		<-c.quit
		conn.Close()
	}()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64<<10), ctlMaxLine)
	enc := json.NewEncoder(conn)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var req ctlRequest
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		var resp ctlResponse
		if err := dec.Decode(&req); err != nil {
			resp.Error = ctlErr("bad_request", "%v", err)
		} else {
			resp = c.do(req)
		}
		resp.ID = req.ID
		if err := enc.Encode(resp); err != nil {
			return
		}
	}
	if errors.Is(sc.Err(), bufio.ErrTooLong) {
		enc.Encode(ctlResponse{Error: ctlErr("bad_request", "a request is longer than %d bytes", ctlMaxLine)})
	}
}

// do runs a request on the UI thread and waits for its answer.
func (c *control) do(req ctlRequest) ctlResponse {
	if !slices.Contains(ctlOps, req.Op) {
		return ctlResponse{Error: ctlErr("unknown_op", "there is no op %q; the ops are %s", req.Op, ctlOpList())}
	}
	ch := make(chan ctlResponse, 1)
	var once sync.Once
	reply := func(result any, err *ctlError) {
		once.Do(func() {
			if err != nil {
				ch <- ctlResponse{Error: err}
			} else {
				ch <- ctlResponse{Result: result}
			}
		})
	}
	c.post(func() { c.run(req, reply) })
	select {
	case r := <-ch:
		return r
	case <-c.quit:
		return ctlResponse{Error: ctlErr("quit", "the app quit")}
	}
}

// ctlReply answers a request, once; later calls do nothing.
type ctlReply func(result any, err *ctlError)

// ctlOps are the ops. They are a list and run's switch rather than a map of
// functions, so that a program that never opens the channel links none of them.
var ctlOps = []string{"actions", "checkpoint", "click", "commands", "dispatch", "drag", "find", "info", "key", "move", "run", "scene", "scenes", "scroll", "shot", "tree", "type", "wait"}

func ctlOpList() string {
	b, _ := json.Marshal(ctlOps)
	return string(b)
}

// run runs a request on the UI thread.
func (c *control) run(req ctlRequest, reply ctlReply) {
	wins := c.windows()
	switch req.Op {
	case "info":
		reply(c.info(wins), nil)
		return
	case "commands":
		c.opCommands(req, reply)
		return
	case "run":
		c.opRun(req, reply)
		return
	case "checkpoint":
		c.opCheckpoint(req, reply)
		return
	case "scenes":
		c.opScenes(reply)
		return
	case "scene":
		c.opScene(req, reply)
		return
	}
	i := req.Window
	if i == 0 {
		i = 1
	}
	if i < 1 || i > len(wins) {
		reply(nil, ctlErr("no_window", "there is no window %d; the app has %d open", req.Window, len(wins)))
		return
	}
	w := wins[i-1]
	if !w.visible && w.rt.NeedsFrame() {
		// The window draws no frames of its own (see pump): one undrawn frame
		// brings its tree up to the input before this request.
		w.offscreen()
	}
	if w.visible && w.rt.NeedsFrame() {
		// Input sent before this request has not been drawn yet, and the tree
		// would not show it: run the request after the next frame, so that every
		// request sees the effects of the ones before it.
		d := &ctlDeferred{window: i, req: req, reply: reply}
		c.deferred = append(c.deferred, d)
		w.wake()
		c.pump(i, w)
		// A window that stops drawing, such as one covered by others, runs it anyway.
		c.after(ctlFreshWait, func() { c.post(func() { c.runDeferred(d, nil) }) })
		return
	}
	c.exec(w, i, req, reply)
}

// ctlFreshWait is how long a request waits for the frame that shows the input
// before it, before it runs without it.
const ctlFreshWait = time.Second

// ctlDeferred is a request waiting for its window's next frame.
type ctlDeferred struct {
	window int
	req    ctlRequest
	reply  ctlReply
}

// runDeferred runs a deferred request that is still waiting, with w, or with
// the window as it is now when w is nil.
func (c *control) runDeferred(d *ctlDeferred, w *ctlWin) {
	i := slices.Index(c.deferred, d)
	if i < 0 {
		return // already run
	}
	c.deferred = slices.Delete(c.deferred, i, i+1)
	if w == nil {
		wins := c.windows()
		if d.window > len(wins) {
			d.reply(nil, ctlErr("no_window", "window %d closed", d.window))
			return
		}
		w = wins[d.window-1]
	}
	c.exec(w, d.window, d.req, d.reply)
}

// exec runs a request on window i.
func (c *control) exec(w *ctlWin, i int, req ctlRequest, reply ctlReply) {
	switch req.Op {
	case "tree":
		c.opTree(w, req, reply)
	case "find":
		c.opFind(w, req, reply)
	case "click":
		c.opClick(w, i, req, reply)
	case "type":
		c.opType(w, req, reply)
	case "key":
		c.opKey(w, req, reply)
	case "scroll":
		c.opScroll(w, i, req, reply)
	case "move":
		c.opMove(w, i, req, reply)
	case "drag":
		c.opDrag(w, i, req, reply)
	case "actions":
		c.opActions(w, req, reply)
	case "dispatch":
		c.opDispatch(w, req, reply)
	case "shot":
		c.opShot(w, req, reply)
	case "wait":
		c.opWait(w, i, req, reply)
	}
}

type ctlInfo struct {
	Protocol int             `json:"protocol"`
	PID      int             `json:"pid"`
	Exe      string          `json:"exe"`
	Windows  []ctlWindowInfo `json:"windows"`
}

type ctlWindowInfo struct {
	Window  int     `json:"window"`
	Title   string  `json:"title"`
	Width   float32 `json:"width"`
	Height  float32 `json:"height"`
	Scale   float32 `json:"scale"`
	Visible bool    `json:"visible"`
}

func (c *control) info(wins []*ctlWin) ctlInfo {
	exe, _ := os.Executable()
	in := ctlInfo{Protocol: ctlProtocol, PID: c.pid, Exe: exe, Windows: []ctlWindowInfo{}}
	for i, w := range wins {
		in.Windows = append(in.Windows, ctlWindowInfo{Window: i + 1, Title: w.title, Width: w.w, Height: w.h,
			Scale: w.scale, Visible: w.visible})
	}
	return in
}

// hostFrame is called by every window after it drew a frame. It reports
// whether the control channel needs another frame of that window.
func (c *control) hostFrame(h *host) bool {
	i := slices.Index(application.windows, h)
	if i < 0 {
		return false
	}
	if c.drew == nil {
		c.drew = map[int]time.Time{}
	}
	c.drew[i+1] = c.now()
	return c.frame(i+1, h.ctlWin())
}

// hostWindows returns the open windows.
func hostWindows() []*ctlWin {
	wins := make([]*ctlWin, 0, len(application.windows))
	for _, h := range application.windows {
		wins = append(wins, h.ctlWin())
	}
	return wins
}

func (h *host) ctlWin() *ctlWin {
	w, ht := h.win.Size()
	s := h.win.Scale()
	if s <= 0 {
		s = 1
	}
	return &ctlWin{
		rt:      h.rt,
		title:   h.cfg.Title,
		w:       float32(w) / s,
		h:       float32(ht) / s,
		scale:   s,
		visible: h.win.Visible(),
		wake: func() {
			h.win.SetCursor(h.rh.Cursor())
			if h.rt.NeedsFrame() {
				h.win.Wake()
			}
		},
		redraw: func() {
			h.rh.Invalidate()
			h.win.Wake()
		},
		capture:   h.r.CaptureNextFrame,
		offscreen: func() { h.rh.Frame(time.Now()) },
	}
}

// opCheckpoint writes the state that survives a restart to path, for jamb dev,
// which asks for it before it stops the app (see devtools/hot), and then lets
// the windows save as they close.
func (c *control) opCheckpoint(req ctlRequest, reply ctlReply) {
	if devhook.Checkpoint == nil {
		reply(nil, ctlErr("failed", "the program keeps no state across restarts: it does not link package devtools/hot"))
		return
	}
	if req.Path == "" || !filepath.IsAbs(req.Path) {
		reply(nil, ctlErr("bad_request", "checkpoint needs path, an absolute file name"))
		return
	}
	n, err := devhook.Checkpoint(req.Path)
	if err != nil {
		reply(nil, ctlErr("failed", "%v", err))
		return
	}
	if req.Freeze {
		// The new build takes over from this state: input from now on would be
		// lost with this process, so the windows take none.
		application.frozen = true
		releaseWindowSession() // the new build takes over where the windows are
	}
	// jamb dev stops the app next, as a close does: what the app saves as its
	// windows close is saved, so that data it writes on a timer is not older on
	// disk than in the checkpoint. A window that would rather stay open is
	// stopped all the same; its state is in the checkpoint.
	for _, h := range application.windows {
		if h.rh != nil {
			h.rh.FlushTimers()
		}
		if h.cfg.OnClose != nil {
			h.cfg.OnClose()
		}
	}
	reply(map[string]any{"values": n}, nil)
}
