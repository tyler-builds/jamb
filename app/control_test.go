package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/devhook"
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/ui"
)

// ctlFixture is a control channel over one headless window, with a clock and
// timers of its own.
type ctlFixture struct {
	t       *testing.T
	c       *control
	w       *ctlWin
	rt      *ui.Runtime
	now     time.Time
	timers  []func()
	redraws int
	shot    func(fn func(*image.RGBA, error)) // what capture does; nil captures nothing
}

func newCtlFixture(t *testing.T, root func(ctx *ui.Context) ui.Element) *ctlFixture {
	t.Helper()
	f := &ctlFixture{t: t, now: time.Unix(1000, 0)}
	f.rt = ui.NewRuntime(root, nil, nil)
	f.rt.SetViewport(400, 300, 2) // scale 2: the tree's physical pixels are halved
	f.frame()
	f.w = &ctlWin{rt: f.rt, title: "Test", w: 400, h: 300, scale: 2, visible: true,
		wake:   func() {},
		redraw: func() { f.redraws++ },
		capture: func(fn func(*image.RGBA, error)) {
			if f.shot != nil {
				f.shot(fn)
			}
		},
	}
	f.c = &control{dir: t.TempDir(), pid: 7, quit: make(chan struct{}),
		post:    func(fn func()) { fn() },
		windows: func() []*ctlWin { return []*ctlWin{f.w} },
		now:     func() time.Time { return f.now },
		after:   func(_ time.Duration, fn func()) { f.timers = append(f.timers, fn) },
	}
	return f
}

func (f *ctlFixture) frame() { f.rt.Frame(f.now) }

type ctlGot struct {
	result any
	err    *ctlError
}

// call runs a request and returns its answer, or ok false while it waits.
func (f *ctlFixture) call(req ctlRequest) (ctlGot, chan ctlGot, bool) {
	ch := make(chan ctlGot, 4)
	f.c.run(req, func(r any, e *ctlError) { ch <- ctlGot{r, e} })
	if len(f.c.deferred) > 0 {
		// Earlier input is still to be drawn: the window draws its next frame.
		f.frame()
		f.c.frame(1, f.w)
	}
	select {
	case g := <-ch:
		return g, ch, true
	default:
		return ctlGot{}, ch, false
	}
}

// must runs a request that must answer at once, with no error.
func (f *ctlFixture) must(req ctlRequest) any {
	f.t.Helper()
	g, _, ok := f.call(req)
	if !ok {
		f.t.Fatalf("%s: no answer", req.Op)
	}
	if g.err != nil {
		f.t.Fatalf("%s: %v", req.Op, g.err)
	}
	return g.result
}

// fails runs a request that must fail at once with code.
func (f *ctlFixture) fails(req ctlRequest, code string) {
	f.t.Helper()
	g, _, ok := f.call(req)
	if !ok || g.err == nil || g.err.Code != code {
		f.t.Fatalf("%s: got %+v, %v; want error %s", req.Op, g.result, g.err, code)
	}
}

func ptr(v float32) *float32 { return &v }

func TestControlFindAndClick(t *testing.T) {
	clicks := 0
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Padding(20).Children(
			ui.Button("Save").Key("save").OnClick(func() { clicks++ }),
			ui.Button("Cancel").Key("cancel"),
		)
	})
	found := f.must(ctlRequest{Op: "find", Role: "Button", Name: "Save"}).(ctlFound)
	if found.Total != 1 || found.Nodes[0].Key != "save" {
		t.Fatalf("find: %+v", found)
	}
	// Bounds are logical pixels: the button sits inside the 20-pixel padding,
	// not at twice that.
	if b := found.Nodes[0].Bounds; b.X < 19 || b.X > 21 || b.W <= 0 || b.X+b.W > 400 {
		t.Errorf("bounds %+v are not logical pixels", b)
	}
	if got := f.must(ctlRequest{Op: "find", Contains: "CANC"}).(ctlFound); got.Total != 1 || got.Nodes[0].Name != "Cancel" {
		t.Errorf("find contains: %+v", got)
	}

	f.must(ctlRequest{Op: "click", Key: "save"})
	f.frame()
	if clicks != 1 {
		t.Fatalf("clicks = %d after a click on the key; want 1", clicks)
	}
	b := found.Nodes[0].Bounds
	f.must(ctlRequest{Op: "click", X: ptr(b.X + b.W/2), Y: ptr(b.Y + b.H/2), Count: 2})
	f.frame()
	if clicks != 3 {
		t.Errorf("clicks = %d after a double click; want 3", clicks)
	}

	f.fails(ctlRequest{Op: "click", Key: "missing"}, "not_found")
	f.fails(ctlRequest{Op: "click", X: ptr(500), Y: ptr(10)}, "not_visible")
	f.fails(ctlRequest{Op: "click", X: ptr(5)}, "bad_request")
	f.fails(ctlRequest{Op: "click"}, "bad_request")
	f.fails(ctlRequest{Op: "click", Key: "save", Button: "fourth"}, "bad_request")
	f.fails(ctlRequest{Op: "find"}, "bad_request")
	f.fails(ctlRequest{Op: "find", Key: "save", Window: 2}, "no_window")
}

func TestControlMoveAndDrag(t *testing.T) {
	var entered bool
	var downX, upX float32
	var moves int
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element {
		// A plain Div is not in the accessibility tree: it is found by its key's layout.
		return ui.Div().Padding(20).Child(ui.Div().Key("h").Width(ui.Px(60)).Height(ui.Px(30)).
			OnMouseEnter(func() { entered = true }).
			OnMouseDown(func(e *ui.MouseEvent) { downX = e.X }).
			OnMouseMove(func(e *ui.MouseEvent) { moves++ }).
			OnMouseUp(func(e *ui.MouseEvent) { upX = e.X }))
	})
	f.must(ctlRequest{Op: "move", Key: "h"})
	f.frame()
	if !entered {
		t.Error("move did not hover the element")
	}
	got := f.must(ctlRequest{Op: "drag", Key: "h", DX: 25}).(ctlAccepted)
	f.frame()
	if upX-downX != 25 || moves < ctlDragSteps {
		t.Errorf("drag went from %g to %g in %d moves; want 25 pixels in %d or more", downX, upX, moves, ctlDragSteps)
	}
	if got.X == nil || *got.X != 75 {
		t.Errorf("drag reported its end at %+v; want x 75", got)
	}
	f.fails(ctlRequest{Op: "drag", Key: "h"}, "bad_request")
}

// A click on a node scrolled out of view scrolls it in, waits for it to stop
// moving, and clicks it; it never clicks where an off-screen node's empty
// bounds are.
func TestControlClickScrollsIntoView(t *testing.T) {
	clicked := ""
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element {
		list := ui.Div().Column().ScrollY().Height(ui.Px(100)).Key("list")
		for i := range 30 {
			name := fmt.Sprintf("Item %d", i)
			list.Child(ui.Button(name).OnClick(func() { clicked = name }))
		}
		return ui.Div().Padding(10).Child(list)
	})
	if n := f.must(ctlRequest{Op: "find", Name: "Item 25"}).(ctlFound); n.Total != 1 || ctlOnScreen(f.w, f.rt.AccessTree().Node(access.ID(n.Nodes[0].ID))) {
		t.Fatalf("Item 25 should start out of view: %+v", n)
	}
	_, ch, ok := f.call(ctlRequest{Op: "click", Name: "Item 25"})
	for i := 0; !ok && i < 200; i++ {
		f.now = f.now.Add(16 * time.Millisecond)
		f.frame()
		f.c.frame(1, f.w)
		select {
		case g := <-ch:
			if g.err != nil {
				t.Fatalf("click: %v", g.err)
			}
			ok = true
		default:
		}
	}
	if !ok || clicked != "Item 25" {
		t.Fatalf("clicked %q (answered %v); want Item 25", clicked, ok)
	}
	if len(f.c.actions) != 0 {
		t.Errorf("%d actions left", len(f.c.actions))
	}

	// One that cannot come into view, in a window that draws nothing, times out.
	_, ch, _ = f.call(ctlRequest{Op: "click", Name: "Item 0", TimeoutMS: 50})
	f.timers[len(f.timers)-1]()
	if g := <-ch; g.err == nil || g.err.Code != "not_visible" {
		t.Errorf("a click that never became ready: %+v %v", g.result, g.err)
	}
}

func TestControlTreePages(t *testing.T) {
	more := ui.NewSignal(false)
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element {
		d := ui.Div().Column().Children(ui.Button("A"), ui.Button("B"), ui.Button("C"))
		if more.Get() {
			d.Child(ui.Button("D"))
		}
		return d
	})
	first := f.must(ctlRequest{Op: "tree", Limit: 2}).(ctlTree)
	if len(first.Nodes) != 2 || first.Next != 2 || first.Total < 4 || first.Nodes[0].Role != "window" {
		t.Fatalf("first page: %+v", first)
	}
	second := f.must(ctlRequest{Op: "tree", Limit: 2, Offset: first.Next, Revision: first.Revision}).(ctlTree)
	if second.Offset != 2 || second.Revision != first.Revision {
		t.Fatalf("second page: %+v", second)
	}
	more.Set(true)
	f.frame()
	f.fails(ctlRequest{Op: "tree", Offset: 2, Revision: first.Revision}, "stale")
	f.fails(ctlRequest{Op: "tree", Limit: ctlMaxLimit + 1}, "bad_request")
}

func TestControlTypeAndKeys(t *testing.T) {
	name := ui.NewSignal("")
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Padding(10).Child(ui.TextInput(name).Key("name").Label("Name"))
	})
	f.must(ctlRequest{Op: "click", Key: "name"})
	f.frame()
	f.must(ctlRequest{Op: "type", Text: "hé!"})
	f.frame()
	if got := name.Peek(); got != "hé!" {
		t.Fatalf("typed %q; want %q", got, "hé!")
	}
	f.must(ctlRequest{Op: "key", Keys: "backspace backspace"})
	f.frame()
	if got := name.Peek(); got != "h" {
		t.Errorf("after two backspaces %q; want %q", got, "h")
	}
	if n := f.must(ctlRequest{Op: "find", Key: "name"}).(ctlFound); n.Total != 1 || n.Nodes[0].Value != "h" || n.Nodes[0].Role != "text field" {
		t.Errorf("the field's node: %+v", n)
	}
	f.fails(ctlRequest{Op: "key", Keys: "enter nosuchkey"}, "bad_request")
	f.fails(ctlRequest{Op: "type", Text: strings.Repeat("x", ctlMaxText+1)}, "bad_request")
	f.fails(ctlRequest{Op: "type"}, "bad_request")
}

// A request sees the effects of the input sent before it, even when the
// window has not drawn them yet.
func TestControlReadsAfterInput(t *testing.T) {
	q := ui.NewSignal("")
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element {
		d := ui.Div().Column().Child(ui.TextInput(q).Key("q").Label("Search"))
		for _, s := range []string{"apple", "banana", "cherry"} {
			if strings.Contains(s, q.Get()) {
				d.Child(ui.Button(s))
			}
		}
		return d
	})
	f.must(ctlRequest{Op: "click", Key: "q"})
	f.frame()
	ch := make(chan ctlGot, 4)
	reply := func(r any, e *ctlError) { ch <- ctlGot{r, e} }
	f.must(ctlRequest{Op: "type", Text: "an"})
	f.c.run(ctlRequest{Op: "find", Role: "button"}, reply)
	if len(ch) != 0 || len(f.c.deferred) != 1 {
		t.Fatalf("find ran before the typed text was drawn")
	}
	f.frame()
	f.c.frame(1, f.w)
	if g := <-ch; g.err != nil || g.result.(ctlFound).Total != 1 {
		t.Fatalf("find after typing: %+v %v; want only banana", g.result, g.err)
	}

	// A window that draws no frame runs it after ctlFreshWait, without it.
	f.must(ctlRequest{Op: "type", Text: "x"})
	f.c.run(ctlRequest{Op: "find", Role: "button"}, reply)
	if len(f.c.deferred) != 1 {
		t.Fatal("find after typing was not deferred")
	}
	f.timers[len(f.timers)-1]()
	if g := <-ch; g.err != nil {
		t.Fatalf("find after the window drew nothing: %v", g.err)
	}
	if len(f.c.deferred) != 0 {
		t.Errorf("%d requests still deferred", len(f.c.deferred))
	}
}

func TestControlWaits(t *testing.T) {
	shown := ui.NewSignal(false)
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element {
		d := ui.Div()
		if shown.Get() {
			d.Child(ui.Button("Done").Key("done"))
		}
		return d
	})

	// found: waits until a frame shows the element.
	_, ch, ok := f.call(ctlRequest{Op: "wait", For: "found", Key: "done"})
	if ok {
		t.Fatal("wait for found answered before the element existed")
	}
	if f.c.frame(1, f.w) {
		t.Error("a found wait asked for frames; changes bring their own")
	}
	shown.Set(true)
	f.now = f.now.Add(250 * time.Millisecond)
	f.frame()
	f.c.frame(1, f.w)
	g := <-ch
	if w, _ := g.result.(ctlWaited); g.err != nil || w.Found == nil || w.Found.Nodes[0].Key != "done" || w.ElapsedMS != 250 {
		t.Fatalf("wait for found: %+v %v", g.result, g.err)
	}
	// Already true: answers at once.
	f.must(ctlRequest{Op: "wait", For: "found", Key: "done"})

	// gone, timing out: the timer answers.
	_, ch, ok = f.call(ctlRequest{Op: "wait", For: "gone", Key: "done", TimeoutMS: 100})
	if ok || len(f.timers) == 0 {
		t.Fatal("wait for gone did not wait")
	}
	f.timers[len(f.timers)-1]()
	if g := <-ch; g.err == nil || g.err.Code != "timeout" {
		t.Fatalf("wait for gone after its timeout: %+v %v", g.result, g.err)
	}
	if len(f.c.waits) != 0 {
		t.Errorf("%d waits left after the timeout", len(f.c.waits))
	}

	// idle: asks for a frame, then answers after ctlQuietFrames quiet frames.
	_, ch, ok = f.call(ctlRequest{Op: "wait", For: "idle"})
	if ok || f.redraws == 0 {
		t.Fatalf("wait for idle answered at once (%v) or drew no frame (%d)", ok, f.redraws)
	}
	for i := 1; i < ctlQuietFrames; i++ {
		if !f.c.frame(1, f.w) {
			t.Fatalf("quiet frame %d did not ask for another", i)
		}
	}
	f.c.frame(1, f.w)
	if g := <-ch; g.err != nil {
		t.Fatalf("wait for idle: %v", g.err)
	}
	// A late timer of an answered wait does nothing.
	f.timers[len(f.timers)-1]()

	f.fails(ctlRequest{Op: "wait", For: "found"}, "bad_request")
	f.fails(ctlRequest{Op: "wait", For: "idle", Key: "done"}, "bad_request")
	f.fails(ctlRequest{Op: "wait", For: "soon"}, "bad_request")
	f.fails(ctlRequest{Op: "wait", For: "idle", TimeoutMS: int(ctlWaitMax.Milliseconds()) + 1}, "bad_request")
}

func TestControlShot(t *testing.T) {
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element { return ui.Div() })
	f.shot = func(fn func(*image.RGBA, error)) { fn(image.NewRGBA(image.Rect(0, 0, 8, 6)), nil) }
	_, ch, _ := f.call(ctlRequest{Op: "shot"})
	g := <-ch // written on another goroutine
	s, _ := g.result.(ctlShot)
	if g.err != nil || s.Width != 8 || s.Height != 6 || filepath.Dir(s.Path) != f.c.dir {
		t.Fatalf("shot: %+v %v", g.result, g.err)
	}
	if _, err := os.Stat(s.Path); err != nil {
		t.Errorf("the PNG: %v", err)
	}
	if f.redraws != 1 {
		t.Errorf("shot drew %d frames; want 1", f.redraws)
	}
	f.fails(ctlRequest{Op: "shot", Path: "relative.png"}, "bad_request")
	f.w.visible = false
	f.fails(ctlRequest{Op: "shot"}, "not_visible")
}

// The socket: one JSON line each way, ids returned, bad lines answered, and the
// socket and its description removed at close.
func TestControlSocket(t *testing.T) {
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element { return ui.Button("Go").Key("go") })
	dir, err := os.MkdirTemp("", "jc") // short: socket paths are limited to about 100 bytes
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	c, err := listenControl(filepath.Join(dir, "ctl"), 42)
	if err != nil {
		t.Fatal(err)
	}
	ui := make(chan func())
	go func() { // the UI thread
		for fn := range ui {
			fn()
		}
	}()
	defer close(ui)
	c.post = func(fn func()) { ui <- fn }
	c.windows = f.c.windows
	go c.serve()

	// A description that a killed app left is pruned when the next app opens.
	stale, _ := json.Marshal(ctlDescription{PID: 9, Socket: filepath.Join(dir, "ctl", "9.sock")})
	os.WriteFile(filepath.Join(dir, "ctl", "9.json"), stale, 0o600)
	c2, err := listenControl(filepath.Join(dir, "ctl"), 43)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ctl", "9.json")); !os.IsNotExist(err) {
		t.Errorf("the stale description is left: %v", err)
	}
	if _, err := os.Stat(c.desc); err != nil {
		t.Errorf("a live app's description was pruned: %v", err)
	}
	c2.close()

	var desc ctlDescription
	if b, err := os.ReadFile(c.desc); err != nil || json.Unmarshal(b, &desc) != nil || desc.Socket != c.socket || desc.PID != 42 {
		t.Fatalf("description: %s %v", b, err)
	}
	conn, err := net.Dial("unix", c.socket)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	in := bufio.NewScanner(conn)
	ask := func(line string) map[string]any {
		t.Helper()
		if _, err := conn.Write([]byte(line + "\n")); err != nil {
			t.Fatal(err)
		}
		if !in.Scan() {
			t.Fatalf("no answer to %s: %v", line, in.Err())
		}
		var m map[string]any
		if err := json.Unmarshal(in.Bytes(), &m); err != nil {
			t.Fatalf("answer %s: %v", in.Bytes(), err)
		}
		return m
	}
	code := func(m map[string]any) any {
		e, _ := m["error"].(map[string]any)
		return e["code"]
	}

	m := ask(`{"id":"a","op":"info"}`)
	r, _ := m["result"].(map[string]any)
	if m["id"] != "a" || r["pid"] != float64(42) || len(r["windows"].([]any)) != 1 {
		t.Errorf("info: %v", m)
	}
	m = ask(`{"id":2,"op":"find","key":"go"}`)
	r, _ = m["result"].(map[string]any)
	nodes, _ := r["nodes"].([]any)
	if m["id"] != float64(2) || len(nodes) != 1 {
		t.Fatalf("find: %v", m)
	}
	// IDs are strings, and a request may give one as a string or a number.
	id, ok := nodes[0].(map[string]any)["id"].(string)
	if !ok {
		t.Fatalf("the node's id is not a string: %v", nodes[0])
	}
	for _, node := range []string{`"` + id + `"`, id} {
		if m := ask(`{"op":"find","node":` + node + `}`); m["error"] != nil {
			t.Errorf("find node %s: %v", node, m)
		}
	}
	if m := ask(`{"op":"find","node":"x"}`); code(m) != "bad_request" {
		t.Errorf("a bad node id: %v", m)
	}
	if m := ask(`{"id":3,"op":"fly"}`); code(m) != "unknown_op" || m["id"] != float64(3) {
		t.Errorf("unknown op: %v", m)
	}
	if m := ask(`{"op":"find","colour":"red"}`); code(m) != "bad_request" {
		t.Errorf("unknown field: %v", m)
	}
	if m := ask(`not json`); code(m) != "bad_request" {
		t.Errorf("not JSON: %v", m)
	}

	c.close()
	for _, p := range []string{c.socket, c.desc} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s is left after close: %v", p, err)
		}
	}
}

var testSaveAction = ui.NewAction("test.save", "Save")

type ctlNoteIn struct {
	Title string `json:"title" desc:"The title"`
}

var ctlNotes = ui.NewSignal([]string{})

var testCreate = ui.NewCommand("test.notes.create", "Create a note", func(in ctlNoteIn) (map[string]int, error) {
	if in.Title == "" {
		return nil, errors.New("a note needs a title")
	}
	ctlNotes.Set(append(slices.Clone(ctlNotes.Peek()), in.Title))
	return map[string]int{"count": len(ctlNotes.Peek())}, nil
})

func TestControlActionsAndCommands(t *testing.T) {
	saved := 0
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Padding(10).OnAction(testSaveAction, func() { saved++ }).
			Child(ui.Button("Focus me").Key("b"))
	})
	f.rt.AddBindings(ui.Bind("ctrl+s", testSaveAction))
	acts := f.must(ctlRequest{Op: "actions"}).(map[string]any)["actions"].([]ctlActionInfo)
	if len(acts) != 1 || acts[0].Name != "test.save" || acts[0].Keys != "Ctrl+S" {
		t.Fatalf("actions: %+v", acts)
	}
	if got := f.must(ctlRequest{Op: "dispatch", Action: "test.save"}).(map[string]any); got["handled"] != true || saved != 1 {
		t.Fatalf("dispatch: %v, saved %d", got, saved)
	}
	f.fails(ctlRequest{Op: "dispatch", Action: "test.nope"}, "not_found")
	f.fails(ctlRequest{Op: "dispatch"}, "bad_request")

	cmds := f.must(ctlRequest{Op: "commands"}).(map[string]any)["commands"].([]ctlCommandInfo)
	var found *ctlCommandInfo
	for i := range cmds {
		if cmds[i].Name == testCreate.Name() {
			found = &cmds[i]
		}
	}
	if found == nil || found.Description != "Create a note" || found.Input["required"] == nil {
		t.Fatalf("commands: %+v", cmds)
	}
	got := f.must(ctlRequest{Op: "run", Command: "test.notes.create", Args: []byte(`{"title":"Hi"}`)}).(map[string]any)
	if got["result"].(map[string]int)["count"] != 1 || ctlNotes.Peek()[0] != "Hi" {
		t.Fatalf("run: %v; notes %v", got, ctlNotes.Peek())
	}
	f.fails(ctlRequest{Op: "run", Command: "test.notes.create", Args: []byte(`{"name":"x"}`)}, "bad_request")
	f.fails(ctlRequest{Op: "run", Command: "test.notes.create", Args: []byte(`{"title":""}`)}, "failed")
	f.fails(ctlRequest{Op: "run", Command: "test.missing"}, "not_found")
	f.fails(ctlRequest{Op: "run"}, "bad_request")
}

// A checkpoint for jamb dev's swap freezes the app: the build that replaces it
// has its state, so the input it would take now would be lost.
func TestControlCheckpointFreezes(t *testing.T) {
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element { return ui.Div() })
	saved := ""
	old := devhook.Checkpoint
	devhook.Checkpoint = func(path string) (int, error) { saved = path; return 3, nil }
	t.Cleanup(func() { devhook.Checkpoint = old; application.frozen = false })
	path := filepath.Join(t.TempDir(), "state.json")
	got := f.must(ctlRequest{Op: "checkpoint", Path: path}).(map[string]any)
	if got["values"] != 3 || saved != path || application.frozen {
		t.Fatalf("checkpoint: %v, saved to %q, frozen %v", got, saved, application.frozen)
	}
	f.must(ctlRequest{Op: "checkpoint", Path: path, Freeze: true})
	if !application.frozen {
		t.Error("a checkpoint with freeze left the app taking input")
	}
	if !isInput(platform.EventKeyDown) || isInput(platform.EventResize) {
		t.Error("isInput: key down is input, a resize is not")
	}
	f.fails(ctlRequest{Op: "checkpoint", Path: "relative.json"}, "bad_request")
}
