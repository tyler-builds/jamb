package app

import (
	"fmt"
	"hash/fnv"
	"image"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// ctlNode is a node of the accessibility tree as the control channel reports
// it. Bounds are logical pixels from the window's top left, the coordinates
// that click and scroll take.
type ctlNode struct {
	ID          ctlID    `json:"id"`
	Parent      ctlID    `json:"parent,omitempty"`
	Role        string   `json:"role"`
	Name        string   `json:"name,omitempty"`
	Key         string   `json:"key,omitempty"`
	Value       string   `json:"value,omitempty"`
	Description string   `json:"description,omitempty"`
	States      []string `json:"states,omitempty"`
	Bounds      ctlRect  `json:"bounds"`
	Children    int      `json:"children,omitempty"`
	// PosInSet and SetSize place an item in its set, such as a row of a list:
	// a virtualized list builds only the rows near its view, so set_size tells
	// how many there are to scroll to.
	PosInSet int `json:"pos_in_set,omitempty"`
	SetSize  int `json:"set_size,omitempty"`
	// Scroll is set on a scroll container whose content is larger than its view.
	Scroll *ctlScroll `json:"scroll,omitempty"`
	// Truncated is set when the name, value or description was longer than
	// ctlMaxValue bytes and was cut.
	Truncated bool `json:"truncated,omitempty"`
}

// ctlScroll is where a scroll container is scrolled to, in percent of its
// range on each axis (0 at the start, 100 at the end), on the axes it can
// scroll on.
type ctlScroll struct {
	X *float64 `json:"x,omitempty"`
	Y *float64 `json:"y,omitempty"`
}

func ctlScrollOf(s access.Scroll) *ctlScroll {
	if !s.Container || (!s.CanX && !s.CanY) {
		return nil
	}
	out := &ctlScroll{}
	if s.CanX {
		out.X = &s.X
	}
	if s.CanY {
		out.Y = &s.Y
	}
	return out
}

type ctlRect struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	W float32 `json:"w"`
	H float32 `json:"h"`
}

func ctlNodeOf(n *access.Node, scale float32) ctlNode {
	name, c1 := ctlCut(n.Name)
	value, c2 := ctlCut(n.Value)
	desc, c3 := ctlCut(n.Description)
	b := n.Bounds
	return ctlNode{
		ID: ctlID(n.ID), Parent: ctlID(n.Parent), Role: n.Role.String(),
		Name: name, Key: n.Key, Value: value, Description: desc,
		States:    strings.Fields(n.States.String()),
		Bounds:    ctlRect{X: b.X / scale, Y: b.Y / scale, W: b.W / scale, H: b.H / scale},
		Children:  len(n.Children),
		PosInSet:  n.PosInSet,
		SetSize:   n.SetSize,
		Scroll:    ctlScrollOf(n.Scroll),
		Truncated: c1 || c2 || c3,
	}
}

// ctlID is a node's ID. It is written as a string, because IDs use all 64
// bits and a JSON number loses precision past 53 in many readers; a request
// may give it as a string or a number.
type ctlID uint64

func (id ctlID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatUint(uint64(id), 10) + `"`), nil
}

func (id *ctlID) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("node must be a node id, such as \"%d\"", access.RootID)
	}
	*id = ctlID(n)
	return nil
}

// ctlCut cuts s to at most ctlMaxValue bytes, on a character boundary.
func ctlCut(s string) (string, bool) {
	if len(s) <= ctlMaxValue {
		return s, false
	}
	i := ctlMaxValue
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return s[:i], true
}

// ctlRevision identifies the content of a tree, so that a client paging
// through it can tell that it changed in between.
func ctlRevision(t *access.Tree) string {
	h := fnv.New64a()
	h.Write([]byte(t.Dump()))
	return strconv.FormatUint(h.Sum64(), 16)
}

// ctlQuery is what find and wait match nodes by: all of the parts that are set.
type ctlQuery struct {
	node            ctlID
	key, role, name string
	contains        string // lower case
}

func (r ctlRequest) query() ctlQuery {
	return ctlQuery{node: r.Node, key: r.Key, role: r.Role, name: r.Name, contains: strings.ToLower(r.Contains)}
}

func (q ctlQuery) empty() bool { return q == ctlQuery{} }

func (q ctlQuery) match(n *access.Node) bool {
	switch {
	case q.node != 0 && ctlID(n.ID) != q.node,
		q.key != "" && n.Key != q.key,
		q.role != "" && !strings.EqualFold(n.Role.String(), q.role),
		q.name != "" && n.Name != q.name:
		return false
	case q.contains != "":
		return strings.Contains(strings.ToLower(n.Name), q.contains) ||
			strings.Contains(strings.ToLower(n.Value), q.contains)
	}
	return true
}

// matches returns the nodes of t that q matches, in reading order.
func (q ctlQuery) matches(t *access.Tree) []*access.Node {
	var found []*access.Node
	t.Walk(func(n *access.Node) bool {
		if q.match(n) {
			found = append(found, n)
		}
		return true
	})
	return found
}

func ctlLimit(n int) (int, *ctlError) {
	switch {
	case n == 0:
		return ctlDefaultLimit, nil
	case n < 0 || n > ctlMaxLimit:
		return 0, ctlErr("bad_request", "limit must be 1 to %d", ctlMaxLimit)
	}
	return n, nil
}

type ctlTree struct {
	Revision string    `json:"revision"`
	Total    int       `json:"total"`
	Offset   int       `json:"offset"`
	Next     int       `json:"next,omitempty"` // the offset of the next page; absent on the last
	Focus    ctlID     `json:"focus,omitempty"`
	Nodes    []ctlNode `json:"nodes"`
}

// opTree returns the window's accessibility tree, depth first in reading
// order, a page at a time: limit nodes from offset. A later page passes the
// revision of the first, and fails with stale when the tree changed.
func (c *control) opTree(w *ctlWin, req ctlRequest, reply ctlReply) {
	limit, err := ctlLimit(req.Limit)
	if err != nil {
		reply(nil, err)
		return
	}
	if req.Offset < 0 {
		reply(nil, ctlErr("bad_request", "offset must not be negative"))
		return
	}
	t := w.rt.AccessTree()
	rev := ctlRevision(t)
	if req.Revision != "" && req.Revision != rev {
		reply(nil, ctlErr("stale", "the tree changed after revision %s; read it again from offset 0", req.Revision))
		return
	}
	out := ctlTree{Revision: rev, Offset: req.Offset, Focus: ctlID(t.Focus), Nodes: []ctlNode{}}
	t.Walk(func(n *access.Node) bool {
		if out.Total >= req.Offset && len(out.Nodes) < limit {
			out.Nodes = append(out.Nodes, ctlNodeOf(n, w.scale))
		}
		out.Total++
		return true
	})
	if end := req.Offset + len(out.Nodes); end < out.Total {
		out.Next = end
	}
	reply(out, nil)
}

type ctlFound struct {
	Total int       `json:"total"`
	Nodes []ctlNode `json:"nodes"`
}

func ctlFoundOf(found []*access.Node, limit int, scale float32) ctlFound {
	out := ctlFound{Total: len(found), Nodes: []ctlNode{}}
	for _, n := range found[:min(limit, len(found))] {
		out.Nodes = append(out.Nodes, ctlNodeOf(n, scale))
	}
	return out
}

// opFind returns the nodes that match the request's node, key, role, name and
// contains, in reading order, up to limit.
func (c *control) opFind(w *ctlWin, req ctlRequest, reply ctlReply) {
	q := req.query()
	if q.empty() {
		reply(nil, ctlErr("bad_request", "find needs node, key, role, name or contains"))
		return
	}
	limit, err := ctlLimit(req.Limit)
	if err != nil {
		reply(nil, err)
		return
	}
	reply(ctlFoundOf(q.matches(w.rt.AccessTree()), limit, w.scale), nil)
}

// point returns where an input op acts: x and y when they are given; or else
// the center of the node the query matches, the first on screen if any is,
// with the node; or, for a key that matches no node, the center of the keyed
// element's layout. A point outside the window is an error, except for a
// node, which target may scroll into view.
func (c *control) point(w *ctlWin, req ctlRequest) (x, y float32, n *access.Node, err *ctlError) {
	q := req.query()
	switch {
	case (req.X == nil) != (req.Y == nil):
		return 0, 0, nil, ctlErr("bad_request", "give both x and y, or neither")
	case req.X != nil && !q.empty():
		return 0, 0, nil, ctlErr("bad_request", "give x and y, or a node to find, not both")
	case req.X != nil:
		x, y = *req.X, *req.Y
	case q.empty():
		return 0, 0, nil, ctlErr("bad_request", "%s needs x and y, or node, key, role, name or contains", req.Op)
	default:
		if found := q.matches(w.rt.AccessTree()); len(found) > 0 {
			n = found[0]
			for _, f := range found {
				if ctlOnScreen(w, f) {
					n = f
					break
				}
			}
			b := n.Bounds
			return (b.X + b.W/2) / w.scale, (b.Y + b.H/2) / w.scale, n, nil
		}
		r, ok := w.rt.Find(req.Key)
		if req.Key == "" || q != (ctlQuery{key: req.Key}) || !ok {
			return 0, 0, nil, ctlErr("not_found", "no element matches. A list builds only the rows near its view, so a row far from it is not in the tree: scroll the list (its rows tell set_size) and try again")
		}
		z := w.rt.Zoom()
		x, y = (r.X+r.W/2)*z, (r.Y+r.H/2)*z
	}
	if !ctlInWindow(w, x, y) {
		return 0, 0, nil, ctlErr("not_visible", "(%g, %g) is outside the window, which is %g×%g; scroll it into view first", x, y, w.w, w.h)
	}
	return x, y, nil, nil
}

func ctlInWindow(w *ctlWin, x, y float32) bool { return x >= 0 && y >= 0 && x < w.w && y < w.h }

// ctlOnScreen reports whether a node can be clicked where its bounds say: it
// has a size, no scroll container clips it away, and its center is in the window.
func ctlOnScreen(w *ctlWin, n *access.Node) bool {
	b := n.Bounds
	return !n.States.Has(access.StateOffscreen) && b.W > 0 && b.H > 0 &&
		ctlInWindow(w, (b.X+b.W/2)/w.scale, (b.Y+b.H/2)/w.scale)
}

// ctlAction is an input op waiting for its target to be ready (see target).
type ctlAction struct {
	window int
	req    ctlRequest
	reply  ctlReply
	act    func(w *ctlWin, x, y float32)
	last   access.Rect // the target's bounds in the frame before
	seen   bool        // last is set
}

// target runs act at the point the request names, once its node is ready: on
// screen, scrolled into view if it was not, and still, its bounds the same in
// two frames in a row, so that a click does not land where a moving target
// was. An idle window's visible target is ready at once. A target that is not
// ready within timeout_ms fails with not_visible.
func (c *control) target(w *ctlWin, index int, req ctlRequest, reply ctlReply, act func(w *ctlWin, x, y float32)) {
	d, err := ctlTimeout(req.TimeoutMS)
	if err != nil {
		reply(nil, err)
		return
	}
	x, y, n, err := c.point(w, req)
	if err != nil {
		reply(nil, err)
		return
	}
	if n == nil || (ctlOnScreen(w, n) && c.atRestNow(w)) {
		act(w, x, y)
		return
	}
	a := &ctlAction{window: index, req: req, reply: reply, act: act}
	c.actions = append(c.actions, a)
	c.pump(index, w)
	c.advance(a, w)
	w.redraw()
	c.after(d, func() {
		c.post(func() {
			if i := slices.Index(c.actions, a); i >= 0 {
				c.actions = slices.Delete(c.actions, i, i+1)
				a.reply(nil, ctlErr("not_visible", "the target was not on screen and still within %v", d))
			}
		})
	})
}

// advance moves a waiting action on by a frame. It reports whether the action
// is done, answered or acted on.
func (c *control) advance(a *ctlAction, w *ctlWin) bool {
	x, y, n, err := c.point(w, a.req)
	switch {
	case err != nil:
		a.reply(nil, err)
		return true
	case n == nil:
		a.act(w, x, y)
		return true
	case !ctlOnScreen(w, n):
		w.rt.ScrollIntoView(n.ID)
		a.seen = false
		return false
	case a.seen && n.Bounds == a.last:
		a.act(w, x, y)
		return true
	}
	a.last, a.seen = n.Bounds, true
	return false
}

// ctlAccepted answers an input op: the input reached the app. Its effects
// show in a later frame; wait for idle to see them.
type ctlAccepted struct {
	Accepted bool     `json:"accepted"`
	X        *float32 `json:"x,omitempty"`
	Y        *float32 `json:"y,omitempty"`
}

func accepted(x, y float32) ctlAccepted { return ctlAccepted{Accepted: true, X: &x, Y: &y} }

func ctlButton(s string) (input.MouseButton, bool) {
	switch s {
	case "", "left":
		return input.ButtonLeft, true
	case "right":
		return input.ButtonRight, true
	case "middle":
		return input.ButtonMiddle, true
	}
	return 0, false
}

// opClick moves the pointer to the target and clicks it count times.
func (c *control) opClick(w *ctlWin, index int, req ctlRequest, reply ctlReply) {
	b, ok := ctlButton(req.Button)
	if !ok {
		reply(nil, ctlErr("bad_request", "button must be left, right or middle"))
		return
	}
	count := req.Count
	if count == 0 {
		count = 1
	}
	if count < 1 || count > 3 {
		reply(nil, ctlErr("bad_request", "count must be 1 to 3"))
		return
	}
	c.target(w, index, req, reply, func(w *ctlWin, x, y float32) {
		w.rt.PointerMove(x, y, 0)
		for n := 1; n <= count; n++ {
			w.rt.PointerDown(x, y, b, n, 0)
			w.rt.PointerUp(x, y, b, 0)
		}
		w.wake()
		reply(accepted(x, y), nil)
	})
}

// opType types text into the focused element.
func (c *control) opType(w *ctlWin, req ctlRequest, reply ctlReply) {
	switch {
	case req.Text == "":
		reply(nil, ctlErr("bad_request", "type needs text"))
		return
	case len(req.Text) > ctlMaxText:
		reply(nil, ctlErr("bad_request", "text is longer than %d bytes", ctlMaxText))
		return
	}
	for _, r := range req.Text {
		w.rt.TextInput(string(r))
	}
	w.wake()
	reply(ctlAccepted{Accepted: true}, nil)
}

// opKey presses and releases keys, in order. Keys are named as in JAMB_SCRIPT,
// such as enter, ctrl+s or ctrl+shift+left, and separated by spaces.
func (c *control) opKey(w *ctlWin, req ctlRequest, reply ctlReply) {
	specs := strings.Fields(req.Keys)
	if len(specs) == 0 {
		reply(nil, ctlErr("bad_request", "key needs keys, such as \"ctrl+s\""))
		return
	}
	type press struct {
		k    input.Key
		mods input.Modifiers
	}
	presses := make([]press, 0, len(specs))
	for _, s := range specs {
		k, mods, ok := parseKey(s)
		if !ok {
			reply(nil, ctlErr("bad_request", "unknown key %q; see JAMB_SCRIPT in docs/tools.md for the names", s))
			return
		}
		presses = append(presses, press{k, mods})
	}
	for _, p := range presses {
		w.rt.KeyDown(p.k, p.mods, false)
		w.rt.KeyUp(p.k, p.mods)
	}
	w.wake()
	reply(ctlAccepted{Accepted: true}, nil)
}

// opScroll scrolls by dx and dy logical pixels at the target.
func (c *control) opScroll(w *ctlWin, index int, req ctlRequest, reply ctlReply) {
	if req.DX == 0 && req.DY == 0 {
		reply(nil, ctlErr("bad_request", "scroll needs dx or dy"))
		return
	}
	c.target(w, index, req, reply, func(w *ctlWin, x, y float32) {
		w.rt.PointerMove(x, y, 0)
		w.rt.Scroll(x, y, req.DX, req.DY, false, 0)
		w.wake()
		reply(accepted(x, y), nil)
	})
}

// opMove moves the pointer to the target, as a hover does.
func (c *control) opMove(w *ctlWin, index int, req ctlRequest, reply ctlReply) {
	c.target(w, index, req, reply, func(w *ctlWin, x, y float32) {
		w.rt.PointerMove(x, y, 0)
		w.wake()
		reply(accepted(x, y), nil)
	})
}

// ctlDragSteps is how many moves a drag makes between press and release, so
// that what follows the pointer sees it travel.
const ctlDragSteps = 8

// opDrag presses the left button at the target, moves by dx and dy, and
// releases it there.
func (c *control) opDrag(w *ctlWin, index int, req ctlRequest, reply ctlReply) {
	if req.DX == 0 && req.DY == 0 {
		reply(nil, ctlErr("bad_request", "drag needs dx or dy"))
		return
	}
	c.target(w, index, req, reply, func(w *ctlWin, x, y float32) {
		w.rt.PointerMove(x, y, 0)
		w.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
		for i := 1; i <= ctlDragSteps; i++ {
			f := float32(i) / ctlDragSteps
			w.rt.PointerMove(x+req.DX*f, y+req.DY*f, 0)
		}
		w.rt.PointerUp(x+req.DX, y+req.DY, input.ButtonLeft, 0)
		w.wake()
		reply(accepted(x+req.DX, y+req.DY), nil)
	})
}

func ctlTimeout(ms int) (time.Duration, *ctlError) {
	d := time.Duration(ms) * time.Millisecond
	switch {
	case ms == 0:
		return ctlWaitDefault, nil
	case ms < 0 || d > ctlWaitMax:
		return 0, ctlErr("bad_request", "timeout_ms must be 1 to %d", ctlWaitMax.Milliseconds())
	}
	return d, nil
}

type ctlShot struct {
	Path   string `json:"path"`
	Width  int    `json:"width"`  // physical pixels
	Height int    `json:"height"` // physical pixels
}

// opShot writes the window's next frame as a PNG: to path, which must be
// absolute, or else to a new file in the control directory.
func (c *control) opShot(w *ctlWin, req ctlRequest, reply ctlReply) {
	d, err := ctlTimeout(req.TimeoutMS)
	if err != nil {
		reply(nil, err)
		return
	}
	path := req.Path
	if path == "" {
		c.shots++
		path = filepath.Join(c.dir, fmt.Sprintf("%d-shot-%d.png", c.pid, c.shots))
	} else if !filepath.IsAbs(path) {
		reply(nil, ctlErr("bad_request", "path must be absolute"))
		return
	}
	if !w.visible {
		reply(nil, ctlErr("not_visible", "the window draws no frames: it is hidden, minimized or covered by other windows "+
			"(GNOME stops a covered window drawing); bring it to the front"))
		return
	}
	w.capture(func(img *image.RGBA, err error) {
		if err != nil {
			reply(nil, ctlErr("failed", "the frame was not captured: %v", err))
			return
		}
		go func() { // encoding is slow; the UI thread goes on
			if err := savePNG(path, img); err != nil {
				reply(nil, ctlErr("failed", "%v", err))
				return
			}
			reply(ctlShot{Path: path, Width: img.Bounds().Dx(), Height: img.Bounds().Dy()}, nil)
		}()
	})
	w.redraw()
	c.after(d, func() {
		reply(nil, ctlErr("timeout", "no frame was drawn within %v; a window covered by others may draw none", d))
	})
}

// ctlWait is a wait in progress.
type ctlWait struct {
	window int
	kind   string // idle, found or gone
	q      ctlQuery
	limit  int
	start  time.Time
	quiet  int // frames in a row with nothing to do
	reply  ctlReply
}

type ctlWaited struct {
	ElapsedMS int64     `json:"elapsed_ms"`
	Found     *ctlFound `json:"found,omitempty"` // the matches, when waiting for found
}

// opWait answers when the window is idle (it has settled, as JAMB_SCRIPT's idle
// step means, and no Query is fetching), when a node matching the query is
// found, or when none is left (gone).
func (c *control) opWait(w *ctlWin, index int, req ctlRequest, reply ctlReply) {
	d, err := ctlTimeout(req.TimeoutMS)
	if err != nil {
		reply(nil, err)
		return
	}
	wt := &ctlWait{window: index, kind: req.For, q: req.query(), start: c.now(), reply: reply}
	switch req.For {
	case "idle":
		if !wt.q.empty() {
			reply(nil, ctlErr("bad_request", "waiting for idle takes no node to find"))
			return
		}
	case "found", "gone":
		if wt.q.empty() {
			reply(nil, ctlErr("bad_request", "waiting for %s needs node, key, role, name or contains", req.For))
			return
		}
		if wt.limit, err = ctlLimit(req.Limit); err != nil {
			reply(nil, err)
			return
		}
		if c.check(wt, w) {
			return
		}
	default:
		reply(nil, ctlErr("bad_request", "for must be idle, found or gone"))
		return
	}
	c.waits = append(c.waits, wt)
	c.pump(index, w)
	if wt.kind == "idle" {
		w.redraw() // the frames that follow tell whether the window has settled
	}
	c.after(d, func() { c.post(func() { c.expire(wt, d) }) })
}

// check answers a found or gone wait if the window's tree satisfies it.
func (c *control) check(wt *ctlWait, w *ctlWin) bool {
	found := wt.q.matches(w.rt.AccessTree())
	switch {
	case wt.kind == "found" && len(found) > 0:
		f := ctlFoundOf(found, wt.limit, w.scale)
		wt.reply(ctlWaited{ElapsedMS: c.now().Sub(wt.start).Milliseconds(), Found: &f}, nil)
		return true
	case wt.kind == "gone" && len(found) == 0:
		wt.reply(ctlWaited{ElapsedMS: c.now().Sub(wt.start).Milliseconds()}, nil)
		return true
	}
	return false
}

// frame advances the waits of window index after it drew a frame. It reports
// whether they need another frame.
func (c *control) frame(index int, w *ctlWin) bool {
	for _, d := range slices.Clone(c.deferred) {
		if d.window == index {
			c.runDeferred(d, w)
		}
	}
	more := false
	for _, a := range slices.Clone(c.actions) {
		if a.window != index {
			continue
		}
		if c.advance(a, w) {
			c.actions = slices.DeleteFunc(c.actions, func(b *ctlAction) bool { return b == a })
		} else {
			more = true
		}
	}
	c.waits = slices.DeleteFunc(c.waits, func(wt *ctlWait) bool {
		if wt.window != index {
			return false
		}
		if wt.kind != "idle" {
			return c.check(wt, w)
		}
		switch {
		case !c.atRestNow(w):
			wt.quiet = 0
			more = true
			return false
		case w.rt.Fetching():
			// The result comes back through Post, which wakes a frame.
			wt.quiet = 0
			return false
		}
		if wt.quiet++; wt.quiet < ctlQuietFrames {
			more = true
			return false
		}
		wt.reply(ctlWaited{ElapsedMS: c.now().Sub(wt.start).Milliseconds()}, nil)
		return true
	})
	return more
}

// expire ends a wait that is still in progress with a timeout.
func (c *control) expire(wt *ctlWait, d time.Duration) {
	i := slices.Index(c.waits, wt)
	if i < 0 {
		return // answered
	}
	c.waits = slices.Delete(c.waits, i, i+1)
	what := map[string]string{"idle": "the window did not settle", "found": "nothing matched", "gone": "a match was still there"}[wt.kind]
	wt.reply(nil, ctlErr("timeout", "%s after %v", what, d))
}

// ctlPumpEvery is how often pump runs a covered window's undrawn frame.
const ctlPumpEvery = 16 * time.Millisecond

// pump runs undrawn frames of window index while requests wait for its
// frames (waits, input waiting for its target, reads waiting for input to be
// drawn) and the window draws none of its own. A hidden or minimized window
// draws nothing, GNOME stops a window drawing while other windows cover it,
// which is where an app often is while an agent or jamb dev drives it, and a
// window that has just opened may not be asked to draw yet. The frames build
// and lay out the tree, run its animations and settle it, so the requests
// answer as they do on screen. A window that drew a frame of its own since the
// last tick is left to its own frames.
func (c *control) pump(index int, w *ctlWin) {
	if c.pumping[index] {
		return
	}
	if c.pumping == nil {
		c.pumping = map[int]bool{}
	}
	c.pumping[index] = true
	var tick func()
	tick = func() {
		c.post(func() {
			wins := c.windows()
			if index > len(wins) || !c.waitingOn(index) {
				delete(c.pumping, index)
				return
			}
			if w := wins[index-1]; !w.visible || c.now().Sub(c.drew[index]) > ctlPumpStale {
				w.offscreen()
				c.frame(index, w)
			}
			c.after(ctlPumpEvery, tick)
		})
	}
	c.after(ctlPumpEvery, tick)
}

// ctlPumpStale is how long a window may go without drawing before pump draws
// for it.
const ctlPumpStale = 100 * time.Millisecond

// waitingOn reports whether requests wait for frames of window index.
func (c *control) waitingOn(index int) bool {
	for _, w := range c.waits {
		if w.window == index {
			return true
		}
	}
	for _, a := range c.actions {
		if a.window == index {
			return true
		}
	}
	for _, d := range c.deferred {
		if d.window == index {
			return true
		}
	}
	return false
}
