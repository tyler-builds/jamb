package ui

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"weak"

	"github.com/tyler-builds/jamb/internal/devhook"
)

// What jamb dev keeps of the runtime across its restarts, besides signals (see
// devtools/hot): the identity of components, to find their locals again, and
// the state of the view.

func init() {
	devhook.TagComponent = func(el any, site string) {
		if c, ok := el.(*ComponentElement); ok {
			c.site = site
		}
	}
	devhook.ViewCapture = func(rt any) []byte { return rt.(*Runtime).captureView() }
	devhook.ViewRestore = func(rt any, b []byte) { rt.(*Runtime).restoreView(b) }
}

// sitePath names a component instance by its path from the root: a step for
// each component around it and for itself. A step is the component's call in
// the source (see ComponentElement.site), or the type of a composite element,
// with the component's key, or else its place among its siblings of the same
// kind. A component that moves among its siblings keeps its name, and one put
// in another's place does not take that one's name, as long as they come from
// different calls.
func (inst *componentInstance) sitePath() string {
	var steps []string
	for i := inst; i != nil; i = i.parent {
		steps = append(steps, i.step())
	}
	slices.Reverse(steps)
	return strings.Join(steps, "/")
}

func (inst *componentInstance) label() string {
	if c, ok := inst.el.(*ComponentElement); ok {
		if c.site != "" {
			return c.site
		}
		return "component"
	}
	return fmt.Sprintf("%T", inst.el)
}

func (inst *componentInstance) step() string {
	if inst.parent == nil {
		return "root"
	}
	label := inst.label()
	if k, ok := inst.el.(keyed); ok && k.elementKey() != "" {
		return label + "[" + k.elementKey() + "]"
	}
	n := 0
	for _, s := range inst.parent.children {
		if s == inst {
			break
		}
		if k, ok := s.el.(keyed); (!ok || k.elementKey() == "") && s.label() == label {
			n++
		}
	}
	return label + "#" + strconv.Itoa(n+1)
}

// viewState is the view of a window that jamb dev keeps across a restart: the
// focus, where scroll containers are scrolled to, and where the carets of
// text fields are. Elements are named by their runtime IDs, which the same
// tree gives again.
type viewState struct {
	Focus  uint64                `json:"focus,omitempty"`
	Scroll map[uint64][2]float32 `json:"scroll,omitempty"`
	Caret  map[uint64][2]int     `json:"caret,omitempty"` // caret and anchor, in bytes
	// List keeps where a ListOf is scrolled to by the item at the top of its
	// view, since the offset of an item that the list has not measured yet is an
	// estimate, which a new process makes differently.
	List map[uint64]listAnchor `json:"list,omitempty"`
}

// listAnchor is the item at the top of a ListOf's view, by key, and how far
// down the view starts in it.
type listAnchor struct {
	Key string  `json:"key"`
	Off float32 `json:"off"`
}

// captureView returns the window's view state as JSON. Call it on the UI thread.
func (rt *Runtime) captureView() []byte {
	v := viewState{Focus: rt.focusID, Scroll: map[uint64][2]float32{}, Caret: map[uint64][2]int{}, List: map[uint64]listAnchor{}}
	for id, n := range rt.byID {
		if st := n.st; st != nil {
			if vs := st.varList; isScroll(n) && vs != nil && vs.hidx != nil && st.targetY != 0 {
				if a := vs.capture(st, false); a.ok {
					v.List[id] = listAnchor{Key: a.key, Off: a.off}
					continue
				}
			}
			if isScroll(n) && (st.targetX != 0 || st.targetY != 0) {
				v.Scroll[id] = [2]float32{st.targetX, st.targetY}
			}
			if es := st.edit; es != nil && (es.caret != 0 || es.anchor != 0) {
				v.Caret[id] = [2]int{es.caret, es.anchor}
			}
		}
	}
	b, _ := json.Marshal(v)
	return b
}

// restoreView keeps b, a view state, to apply after the first layout, when
// the elements are built and the scroll containers know their content.
func (rt *Runtime) restoreView(b []byte) {
	var v viewState
	if json.Unmarshal(b, &v) == nil {
		rt.pendingView = &v
	}
}

// applyView applies a kept view state to what the tree has of it: a scroll
// container scrolls to where it was, clamped to its content; a text field's
// caret goes back, clamped to its text; and the element that had the focus has
// it again.
func (rt *Runtime) applyView(v *viewState) {
	for id, xy := range v.Scroll {
		n := rt.byID[id]
		if n == nil || n.st == nil || !isScroll(n) {
			continue
		}
		maxX := max(n.lay.ContentW-n.lay.W, 0)
		maxY := max(n.lay.ContentH-n.lay.H, 0)
		st := n.st
		st.targetX, st.targetY = min(max(xy[0], 0), maxX), min(max(xy[1], 0), maxY)
		st.scrollX, st.scrollY = st.targetX, st.targetY
		rt.scrolled = true
	}
	for id, a := range v.List {
		n := rt.byID[id]
		if n == nil || n.st == nil || n.st.varList == nil || n.st.varList.hidx == nil {
			continue
		}
		vs := n.st.varList
		idx, ok := vs.index[a.Key]
		if !ok {
			continue // the item is gone
		}
		t := max(vs.top+float32(vs.hidx.prefix(idx))+a.Off, 0)
		n.st.targetY, n.st.scrollY = t, t
		rt.scrolled = true
	}
	for id, ca := range v.Caret {
		if n := rt.byID[id]; n != nil && n.st != nil && n.st.edit != nil {
			es := n.st.edit
			es.caret, es.anchor = ca[0], ca[1]
			if es.knownSet {
				es.caret, es.anchor = clampToRune(es.known, es.caret), clampToRune(es.known, es.anchor)
			}
		}
	}
	if v.Focus != 0 && rt.byID[v.Focus] != nil {
		rt.setFocus(v.Focus)
	}
	rt.needPaint = true
	rt.requestFrame()
}

// announceRuntime tells jamb dev's hook about a new runtime, which numbers the
// windows to give each its view state back.
func announceRuntime(rt *Runtime) {
	if devhook.Runtime != nil {
		w := weak.Make(rt)
		devhook.Runtime(func() any {
			if r := w.Value(); r != nil {
				return r
			}
			return nil
		})
	}
}
