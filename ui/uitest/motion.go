package uitest

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

// The motion rules' thresholds, in logical pixels, opacity (0..1) and clock
// time. docs/testing.md lists them with the rules.
const (
	// popOpacity: an element that appears or disappears at this opacity or
	// more, with no transition, pops.
	popOpacity = 0.5
	// popMinArea: an element smaller than this (in square logical pixels, 24 ×
	// 24) does not pop: a check mark or a caret appearing is not motion a
	// person notices.
	popMinArea = 24 * 24
	// cutExitOpacity: an element whose exit fade ends above this opacity, with
	// a last step bigger than the fade's previous one, was cut short.
	cutExitOpacity = 0.05
	// visibleOpacity: below it an element is not seen, so it cannot jump.
	visibleOpacity = 0.05
	// jumpPx: a change of position or size of at least this in one frame can
	// be a jump.
	jumpPx = 12
	// jumpRatio and stillPx: the change is a jump when it is more than
	// jumpRatio times the element's change on the frames before and after it,
	// and those count as at least stillPx.
	jumpRatio = 3
	stillPx   = 1
	// overshootPx: a motion that passes where it comes to rest by more than
	// this overshoots.
	overshootPx = 2
	// restFrames: frames without change that end one motion of an element.
	restFrames = 3
	// settleWithin: the UI still animating this long after the last input
	// never settles.
	settleWithin = time.Second
)

// Kind is the kind of a motion finding (see Recording.Findings).
type Kind string

// The kinds of motion finding. docs/testing.md describes each, with its
// thresholds.
const (
	// PopIn is an element that appears at full opacity and size, with no
	// transition.
	PopIn Kind = "pop-in"
	// PopOut is an element that disappears at full opacity and size, with no
	// transition.
	PopOut Kind = "pop-out"
	// CutExit is an element removed partway through its exit fade, while it
	// is still seen.
	CutExit Kind = "cut-exit"
	// Jump is a change of position or size in one frame, while something
	// animates and not following an input.
	Jump Kind = "jump"
	// Overshoot is a motion that passes where it comes to rest and comes back,
	// with no input during it.
	Overshoot Kind = "overshoot"
	// NeverSettles is a UI still animating at the end of the recording, long
	// after the last input.
	NeverSettles Kind = "never-settles"
)

// Recording is the frames a UI ran since Record, each with how every element
// it drew looked. Findings checks them for motion a person would notice, and
// Check fails the test on it. A recording costs nothing until Record starts
// it: a UI that is not recording samples nothing.
type Recording struct {
	u      *UI
	frames []Frame
	ids    map[uint64]ident
	// tree holds, per element, the runs of frames it was in the tree for,
	// drawn or not: one scrolled off screen is still there.
	tree  map[uint64][]span
	input bool // the next frame follows an input
}

// span is a run of frames, from and to included.
type span struct{ from, to int }

// inTree reports whether element id was in the tree at frame k.
func (r *Recording) inTree(id uint64, k int) bool {
	for _, sp := range r.tree[id] {
		if k >= sp.from && k <= sp.to {
			return true
		}
	}
	return false
}

// ident is how a recording names an element: the keys of its keyed ancestors
// and its own (path), and below the nearest key, the elements without one
// (tail).
type ident struct{ path, tail string }

func (id ident) name() string {
	switch {
	case id.path == "" && id.tail == "":
		return "(root)"
	case id.path == "":
		return id.tail
	case id.tail == "":
		return id.path
	}
	return id.path + "/" + id.tail
}

// Frame is one frame of a Recording.
type Frame struct {
	// At is the UI's clock when the frame ran.
	At time.Time
	// Input reports whether the frame followed an input: a click, a key, typing,
	// a hover, a wheel turn or an accessibility action (see Recording.MarkInput).
	Input bool
	// Settled reports whether the UI was at rest after the frame (see
	// ui.Runtime.Settled).
	Settled bool
	// Elements are the elements the frame drew, parents before their children,
	// then those drawn while their exit plays.
	Elements []Sample
}

// Sample is an element as a frame drew it: where, at what opacity and scale,
// and with what colors (the embedded ui.DrawnStyle).
type Sample struct {
	// Name names the element in findings: the keys of its keyed ancestors and
	// its own, separated by "/", then, for an element with no key, its place
	// below the nearest key, such as `toast/div[0]/text "Saved"`. It is the
	// name of the element when the recording first saw it.
	Name string
	// Key is the element's key, or "".
	Key string
	// Exiting reports an element that was removed and is drawn while its exit
	// plays.
	Exiting bool
	// Visible reports whether some of the element is inside the area it is
	// clipped to.
	Visible bool
	// Looping reports an element that a repeating animation moves (see
	// ui.Animation.Repeat), or one inside it. Going back to its start is the
	// loop, so the rules leave it out.
	Looping bool
	// Blank reports an element that draws nothing of its own: a layout
	// container with no fill, border or shadow. It cannot be seen to pop or
	// jump, so the rules look at what it holds instead.
	Blank bool
	ui.DrawnStyle

	id, parent uint64
	sx, sy     float32 // the scroll offsets of its scrolling ancestors
	ownX, ownY float32 // its own scroll offsets
}

// Record starts recording the frames the UI runs, from the frame it shows now,
// and returns the recording. Every frame that an action, Frame, Advance,
// Settle or WaitFor runs is recorded, until Stop. A new Record stops the
// recording before it.
//
//	rec := u.Record()
//	u.Click("open")
//	u.Settle()
//	rec.Check(t)
func (u *UI) Record() *Recording {
	r := &Recording{u: u, ids: map[uint64]ident{}, tree: map[uint64][]span{}}
	u.rec = r
	r.sample()
	return r
}

// Stop stops recording. Findings and Check still read what was recorded.
func (r *Recording) Stop() {
	if r.u.rec == r {
		r.u.rec = nil
	}
}

// MarkInput marks the next frame recorded as following an input, for a test
// that sends input through Runtime instead of the UI's methods, which mark
// their own.
func (r *Recording) MarkInput() { r.input = true }

// Frames returns the frames recorded so far, the first being the one shown
// when Record was called.
func (r *Recording) Frames() []Frame { return r.frames }

// acted marks the next frame recorded as following an input.
func (u *UI) acted() {
	if u.rec != nil {
		u.rec.input = true
	}
}

// recordFrame records the frame the runtime just ran, when the UI is
// recording.
func (u *UI) recordFrame() {
	if u.rec != nil {
		u.rec.sample()
	}
}

// sample records the latest frame.
func (r *Recording) sample() {
	f := Frame{At: r.u.now, Input: r.input, Settled: r.u.rt.Settled()}
	if n := len(r.frames); n > 0 {
		f.Elements = make([]Sample, 0, len(r.frames[n-1].Elements))
	}
	r.input = false
	k := len(r.frames)
	known := func(id uint64) bool { _, ok := r.ids[id]; return ok }
	r.u.host.Samples(known, func(s uihost.Sample) {
		id, ok := r.ids[s.ID]
		if !ok {
			id = r.ident(s)
			r.ids[s.ID] = id
		}
		if sp := r.tree[s.ID]; len(sp) > 0 && sp[len(sp)-1].to >= k-1 {
			sp[len(sp)-1].to = k
		} else {
			r.tree[s.ID] = append(sp, span{k, k})
		}
		if !s.Drawn {
			return
		}
		f.Elements = append(f.Elements, Sample{
			Name: id.name(), Key: s.Key, Exiting: s.Exiting, Visible: s.Visible, Looping: s.Looping, Blank: s.Blank,
			DrawnStyle: s.Style.(ui.DrawnStyle),
			id:         s.ID, parent: s.Parent, sx: s.ScrollX, sy: s.ScrollY, ownX: s.OwnScrollX, ownY: s.OwnScrollY,
		})
	})
	r.frames = append(r.frames, f)
}

// ident names an element the recording sees for the first time. Its parent,
// sampled before it, has a name already.
func (r *Recording) ident(s uihost.Sample) ident {
	if s.Parent == 0 {
		return ident{path: s.Key}
	}
	p := r.ids[s.Parent]
	if s.Key != "" {
		return ident{path: strings.TrimPrefix(p.path+"/"+s.Key, "/")}
	}
	seg := s.Label
	if seg == "div" {
		seg = fmt.Sprintf("div[%d]", s.Index)
	}
	return ident{path: p.path, tail: strings.TrimPrefix(p.tail+"/"+seg, "/")}
}

// ---- findings ----

// Finding is motion a person would notice, found in a Recording.
type Finding struct {
	// Kind is what was found.
	Kind Kind
	// Element is the name of the element it was found on (see Sample.Name),
	// or "(window)" for a NeverSettles with no element moving.
	Element string
	// Frame is the index in Recording.Frames of the frame it was seen on.
	Frame int
	// At is how far into the recording that frame ran, in clock time.
	At time.Duration
	// Detail says what was seen, with the numbers.
	Detail string
}

// String formats f for a test's log.
func (f Finding) String() string {
	return fmt.Sprintf("%s %s at %v (frame %d): %s", f.Kind, f.Element, f.At, f.Frame, f.Detail)
}

// Allow is a finding a test accepts, with the reason it is right or the issue
// that tracks it: Check passes a finding of Kind on an element whose name
// contains Element. Reason must not be empty.
type Allow struct {
	Kind    Kind
	Element string
	Reason  string
}

// Check fails the test, listing them, when the recording has findings (see
// Findings) that no allow rule covers. It also fails on an allow rule with no
// reason or no kind, and on one that covered no finding, so stale rules come
// out. The findings the rules cover are logged.
func (r *Recording) Check(tb testing.TB, allow ...Allow) {
	tb.Helper()
	used := make([]bool, len(allow))
	for i, a := range allow {
		if a.Kind == "" || strings.TrimSpace(a.Reason) == "" {
			tb.Errorf("uitest: motion allow rule %+v needs a kind and a reason", a)
			used[i] = true
		}
	}
	var bad []string
	for _, f := range r.Findings() {
		i := slices.IndexFunc(allow, func(a Allow) bool {
			return a.Kind == f.Kind && a.Reason != "" && strings.Contains(f.Element, a.Element)
		})
		if i < 0 {
			bad = append(bad, f.String())
			continue
		}
		used[i] = true
		tb.Logf("uitest: allowed motion: %s (%s)", f, allow[i].Reason)
	}
	if len(bad) > 0 {
		tb.Errorf("uitest: %d motion finding(s) a person would notice:\n  %s\n(Recording.Frames has every frame; allow an intended one with uitest.Allow{Kind, Element, Reason})",
			len(bad), strings.Join(bad, "\n  "))
	}
	for i, a := range allow {
		if !used[i] {
			tb.Errorf("uitest: motion allow rule {%s %q} matched no finding; remove it", a.Kind, a.Element)
		}
	}
}

// Findings checks the recorded frames for motion a person would notice and
// returns what it found, in frame order:
//
//   - PopIn, PopOut: an element appears or disappears at full opacity and
//     size with no transition.
//   - CutExit: an element is removed partway through its exit fade.
//   - Jump: a one-frame change of position or size, not following an input,
//     while something animates.
//   - Overshoot: a motion passes where it comes to rest and comes back.
//   - NeverSettles: the UI still animates at the end, long after the last
//     input.
//
// The thresholds are in docs/testing.md. An element moved by a scroll, or
// moving with its parent, is not reported; the parent is. It only reports, so
// a test can log the findings without failing; Check fails on them.
func (r *Recording) Findings() []Finding {
	a := analysis{r: r, frames: r.frames, at: make([]map[uint64]int, len(r.frames))}
	for k, f := range a.frames {
		m := make(map[uint64]int, len(f.Elements))
		for i, s := range f.Elements {
			if _, dup := m[s.id]; !dup {
				m[s.id] = i
			}
		}
		a.at[k] = m
	}
	for k := 1; k < len(a.frames); k++ {
		a.births(k)
		a.deaths(k)
		a.jumps(k)
	}
	a.overshoots()
	a.neverSettles()
	slices.SortStableFunc(a.out, func(x, y Finding) int { return cmp.Compare(x.Frame, y.Frame) })
	return a.out
}

// analysis is one run of Findings.
type analysis struct {
	r      *Recording
	frames []Frame
	at     []map[uint64]int // per frame, each element's index in Elements
	out    []Finding
}

// get returns element id in frame k, or nil.
func (a *analysis) get(k int, id uint64) *Sample {
	if k < 0 || k >= len(a.frames) {
		return nil
	}
	i, ok := a.at[k][id]
	if !ok {
		return nil
	}
	return &a.frames[k].Elements[i]
}

func (a *analysis) add(kind Kind, s *Sample, k int, format string, args ...any) {
	a.out = append(a.out, Finding{Kind: kind, Element: s.Name, Frame: k,
		At: a.frames[k].At.Sub(a.frames[0].At), Detail: fmt.Sprintf(format, args...)})
}

// later returns the first frame after k that ran at a later time, or -1. A
// frame at the same time (an action, then Frame) shows no transition's
// progress.
func (a *analysis) later(k int) int {
	for j := k + 1; j < len(a.frames); j++ {
		if a.frames[j].At.After(a.frames[k].At) {
			return j
		}
	}
	return -1
}

// earlier returns the last frame before k that ran at an earlier time, or -1.
func (a *analysis) earlier(k int) int {
	for j := k - 1; j >= 0; j-- {
		if a.frames[j].At.Before(a.frames[k].At) {
			return j
		}
	}
	return -1
}

// scrolled reports whether a scroll moved the content around s between frames
// j and k: the nearest ancestor of s (as frame from has it) drawn in both
// carries its content by a different scroll offset.
func (a *analysis) scrolled(s *Sample, from, j, k int) bool {
	for p := a.get(from, s.parent); p != nil; p = a.get(from, p.parent) {
		pj, pk := a.get(j, p.id), a.get(k, p.id)
		if pj == nil || pk == nil {
			continue
		}
		dx := (pk.sx + pk.ownX) - (pj.sx + pj.ownX)
		dy := (pk.sy + pk.ownY) - (pj.sy + pj.ownY)
		return abs(dx) > 0.5 || abs(dy) > 0.5
	}
	return false
}

// popStatus is what the pop rules made of an element that came or went, which
// decides how its children are looked at.
type popStatus int

const (
	// popHolder: it draws nothing of its own, or too little or too faintly to
	// pop, so what it holds is looked at, and a pop there is reported on it
	// as the outermost element that came or went.
	popHolder popStatus = iota + 1
	// popDone: it was reported, or it came or went with a transition or a
	// scroll, and what it holds goes with it.
	popDone
)

// pops runs judge on the elements that came (came) or went at frame k, taken
// in tree order from elements: frame k's or the frame before's. A pop is
// reported once, on the outermost element that came or went with it, with how
// many pops it holds.
func (a *analysis) pops(k int, elements []Sample, came bool, judge func(s *Sample) (popStatus, string)) {
	status := map[uint64]popStatus{}
	root := map[uint64]*Sample{} // the outermost element that came or went with each
	type pop struct {
		s      *Sample
		detail string
	}
	var order []*Sample
	found := map[*Sample][]pop{}
	for i := range elements {
		s := &elements[i]
		if came && (s.Exiting || a.drawn(k-1, s.id) || a.r.inTree(s.id, k-1)) ||
			!came && (a.drawn(k, s.id) || a.r.inTree(s.id, k)) || s.Looping {
			continue
		}
		top := s
		switch status[s.parent] {
		case popDone:
			status[s.id] = popDone
			continue
		case popHolder:
			top = root[s.parent]
		}
		root[s.id] = top
		st, detail := judge(s)
		status[s.id] = st
		if detail == "" {
			continue
		}
		if found[top] == nil {
			order = append(order, top)
		}
		found[top] = append(found[top], pop{s, detail})
	}
	for _, top := range order {
		ps := found[top]
		p := ps[0]
		kind, d := PopIn, p.detail
		if !came {
			kind = PopOut
			if strings.HasPrefix(d, "removed at") {
				kind = CutExit
			}
		}
		if p.s != top {
			d = p.s.Name + " " + d
		}
		if len(ps) > 1 {
			d += fmt.Sprintf(" (and %d more inside it)", len(ps)-1)
		}
		a.add(kind, top, k, "%s", d)
	}
}

// drawn reports whether element id was drawn at frame k.
func (a *analysis) drawn(k int, id uint64) bool { return a.get(k, id) != nil }

// births reports the elements that appear at frame k with no transition: they
// were not in the tree, or were hidden by Display(false), at the frame before.
func (a *analysis) births(k int) {
	next := a.later(k)
	a.pops(k, a.frames[k].Elements, true, func(s *Sample) (popStatus, string) {
		if s.Blank || !s.Visible || s.Opacity < popOpacity || area(s) < popMinArea {
			return popHolder, ""
		}
		if a.scrolled(s, k, k-1, k) {
			return popDone, ""
		}
		if n := a.get(next, s.id); n != nil && changes(s, n) {
			return popDone, "" // it enters with a transition
		}
		return popDone, fmt.Sprintf("appeared at opacity %.2f, %.0f×%.0f at (%.0f,%.0f), with no transition",
			s.Opacity, s.Bounds.W, s.Bounds.H, s.Bounds.X, s.Bounds.Y)
	})
}

// deaths reports the elements that disappear at frame k with no transition,
// or partway through their exit fade.
func (a *analysis) deaths(k int) {
	prev := a.earlier(k - 1)
	a.pops(k, a.frames[k-1].Elements, false, func(s *Sample) (popStatus, string) {
		if s.Blank || !s.Visible || area(s) < popMinArea || s.Opacity <= cutExitOpacity {
			return popHolder, ""
		}
		if !s.Exiting && a.scrolled(s, k-1, k-1, k) {
			return popDone, ""
		}
		if s.Opacity >= popOpacity {
			return popDone, fmt.Sprintf("disappeared at opacity %.2f, %.0f×%.0f at (%.0f,%.0f), with no transition",
				s.Opacity, s.Bounds.W, s.Bounds.H, s.Bounds.X, s.Bounds.Y)
		}
		var step float32 // how much it faded on the frame before
		if p := a.get(prev, s.id); p != nil {
			step = p.Opacity - s.Opacity
		}
		if (s.Exiting || step > 0.001) && s.Opacity > step {
			return popDone, fmt.Sprintf("removed at opacity %.2f, while its fade had moved %.2f in the frame before", s.Opacity, step)
		}
		return popHolder, ""
	})
}

// edges are the parts of an element's geometry that the jump and overshoot
// rules follow. Positions are taken against the scroll, so content a scroll
// carries does not move.
var edges = [...]struct {
	name string
	of   func(*Sample) float32
}{
	{"x", func(s *Sample) float32 { return s.Bounds.X + s.sx }},
	{"y", func(s *Sample) float32 { return s.Bounds.Y + s.sy }},
	{"width", func(s *Sample) float32 { return s.Bounds.W }},
	{"height", func(s *Sample) float32 { return s.Bounds.H }},
}

// jumps reports the elements whose position or size changes in one frame at
// frame k, while something animates and not following an input. Of a group
// that moves together, the outermost element that draws something is
// reported.
func (a *analysis) jumps(k int) {
	f, p := &a.frames[k], &a.frames[k-1]
	if f.Input || f.Settled && p.Settled {
		return
	}
	// shown marks, per edge, the elements reported and those moving with one.
	shown := map[uint64][len(edges)]bool{}
	for i := range f.Elements {
		s := &f.Elements[i]
		q := a.get(k-1, s.id)
		if q == nil || s.Exiting != q.Exiting {
			continue
		}
		var mine [len(edges)]bool
		var what []string
		for j, e := range edges {
			step := e.of(s) - e.of(q)
			if abs(step) < jumpPx {
				continue
			}
			if ps, pq := a.get(k, s.parent), a.get(k-1, s.parent); ps != nil && pq != nil &&
				shown[s.parent][j] && abs(e.of(ps)-e.of(pq)-step) < 0.5 {
				mine[j] = true // it moves with its parent, which is reported
				continue
			}
			before, after := a.step(s.id, k-1, e.of), a.step(s.id, k+1, e.of)
			if s.Blank || s.Looping || !seen(s) || !seen(q) || abs(step) <= jumpRatio*max(abs(before), abs(after), stillPx) {
				continue
			}
			mine[j] = true
			what = append(what, fmt.Sprintf("%s %.0f → %.0f (%+.0f; %+.0f before, %+.0f after)", e.name, e.of(q), e.of(s), step, before, after))
		}
		shown[s.id] = mine
		if len(what) > 0 {
			a.add(Jump, s, k, "%s, in one frame while the UI animates, with no input", strings.Join(what, ", "))
		}
	}
}

// step returns how much of of element id changed between frames k-1 and k,
// or 0 when it is not in both.
func (a *analysis) step(id uint64, k int, of func(*Sample) float32) float32 {
	s, q := a.get(k, id), a.get(k-1, id)
	if s == nil || q == nil {
		return 0
	}
	return of(s) - of(q)
}

// overshoots reports the motions that pass where they come to rest by more
// than overshootPx and come back, with no input during them. A motion is a run
// of frames in which an element moves, ended by restFrames frames without
// change.
func (a *analysis) overshoots() {
	type track struct {
		s  *Sample
		ks []int
	}
	tracks := map[uint64]*track{}
	var order []uint64
	for k := range a.frames {
		for i := range a.frames[k].Elements {
			s := &a.frames[k].Elements[i]
			t := tracks[s.id]
			if t == nil {
				t = &track{s: s}
				tracks[s.id] = t
				order = append(order, s.id)
			}
			t.ks = append(t.ks, k)
		}
	}
	for _, id := range order {
		t := tracks[id]
		for _, e := range edges {
			a.overshootOn(id, t.ks, e.name, e.of)
		}
	}
}

func (a *analysis) overshootOn(id uint64, ks []int, edge string, of func(*Sample) float32) {
	start := 0 // index in ks of the frame the current motion starts from
	rest := 0
	moving := false
	for i := 1; i <= len(ks); i++ {
		end := i == len(ks) || ks[i] != ks[i-1]+1 || a.frames[ks[i]].Input
		if !end {
			if abs(of(a.get(ks[i], id))-of(a.get(ks[i-1], id))) < 0.01 {
				rest++
			} else {
				if !moving {
					start = i - 1
				}
				moving, rest = true, 0
			}
			end = rest >= restFrames
		}
		if !end {
			continue
		}
		if moving && rest > 0 {
			a.overshootIn(id, ks[start:i], edge, of)
		}
		moving, rest, start = false, 0, i
	}
}

// overshootIn reports the motion of element id over frames ks, which ends at
// rest, when it passes where it rests.
func (a *analysis) overshootIn(id uint64, ks []int, edge string, of func(*Sample) float32) {
	from, to := of(a.get(ks[0], id)), of(a.get(ks[len(ks)-1], id))
	dir := float32(1)
	if to < from {
		dir = -1
	}
	if abs(to-from) < 1 {
		return
	}
	peak, at := float32(0), 0
	for j, k := range ks {
		if d := (of(a.get(k, id)) - to) * dir; d > peak {
			peak, at = d, j
		}
	}
	s := a.get(ks[at], id)
	if peak <= overshootPx || s.Blank || s.Looping {
		return
	}
	// It moves with its parent, which is reported.
	rides := true
	for j := 1; j < len(ks) && rides; j++ {
		ps, pq := a.get(ks[j], s.parent), a.get(ks[j-1], s.parent)
		rides = ps != nil && pq != nil && !ps.Blank &&
			abs((of(ps)-of(pq))-(of(a.get(ks[j], id))-of(a.get(ks[j-1], id)))) < 0.5
	}
	if rides {
		return
	}
	a.add(Overshoot, s, ks[at], "%s moves from %.0f to rest at %.0f and passes it by %.1f", edge, from, to, peak)
}

// neverSettles reports a UI still animating at the end of the recording,
// settleWithin after the last input.
func (a *analysis) neverSettles() {
	n := len(a.frames) - 1
	if n < 1 || a.frames[n].Settled {
		return
	}
	last := a.frames[0].At
	for _, f := range a.frames {
		if f.Input {
			last = f.At
		}
	}
	if since := a.frames[n].At.Sub(last); since < settleWithin {
		return
	}
	// Name what moved on the last frame that changed anything, outermost
	// first.
	var moved []*Sample
	for k := n; k > 0 && len(moved) == 0; k-- {
		for i := range a.frames[k].Elements {
			s := &a.frames[k].Elements[i]
			if q := a.get(k-1, s.id); q != nil && s.DrawnStyle != q.DrawnStyle && !changed(a, k, s.parent) {
				moved = append(moved, s)
			}
		}
	}
	fd := Finding{Kind: NeverSettles, Element: "(window)", Frame: n, At: a.frames[n].At.Sub(a.frames[0].At)}
	var names []string
	for _, s := range moved[:min(len(moved), 4)] {
		names = append(names, s.Name)
	}
	if len(moved) > 0 {
		fd.Element = moved[0].Name
	}
	fd.Detail = fmt.Sprintf("still animating %v after the last input", a.frames[n].At.Sub(last))
	if len(names) > 0 {
		fd.Detail += "; changing: " + strings.Join(names, ", ")
	}
	a.out = append(a.out, fd)
}

// changed reports whether element id was drawn differently at frame k than
// at k-1.
func changed(a *analysis, k int, id uint64) bool {
	s, q := a.get(k, id), a.get(k-1, id)
	return s != nil && q != nil && s.DrawnStyle != q.DrawnStyle
}

// changes reports whether s, at a later frame, is n with a transition under
// way: its opacity, scale or size differs.
func changes(s, n *Sample) bool {
	return abs(n.Opacity-s.Opacity) > 0.01 || abs(n.Scale-s.Scale) > 0.005 ||
		abs(n.Bounds.W-s.Bounds.W) > 0.5 || abs(n.Bounds.H-s.Bounds.H) > 0.5
}

// seen reports whether a person can see s.
func seen(s *Sample) bool { return s.Visible && s.Opacity > visibleOpacity }

func area(s *Sample) float32 { return s.Bounds.W * s.Bounds.H }

func abs(v float32) float32 { return float32(math.Abs(float64(v))) }
