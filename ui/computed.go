package ui

import "sync"

// Computed is a value derived from signals: the result of a function that reads
// them. It remembers the result, and computes it again only when it is read after
// one of the signals it read has changed. Reading it with Get inside a
// component's render subscribes the component, as a Signal does, so the
// component re-renders when the value may have changed.
//
// Create a Computed once, like a signal (a package variable, a field of the app's
// store, or ui.Local), not in every render: it stays subscribed to the signals
// it reads.
//
//	total := ui.NewComputed(func() int {
//		sum := 0
//		for _, item := range cart.Get() {
//			sum += item.Price
//		}
//		return sum
//	})
//	ui.Textf("Total: %d", total.Get())
//
// Read a Computed on the UI thread: in a render, an event handler or a function
// passed to Post. Recording which signals the function reads uses the same
// process-wide tracker as rendering, so a computation on another goroutine
// could take a rendering component's reads. Off the UI thread, read the signals
// themselves.
type Computed[T any] struct {
	fn func() T

	mu    sync.Mutex
	value T                 // the last result, which the readers have seen
	has   bool              // value holds a result
	valid bool              // value is the function's result for the current signals
	maybe bool              // a Computed the value was computed from may have changed
	gen   uint64            // counts invalidations, so a result computed meanwhile is not kept as valid
	subs  []observer        // the components (and Computeds) that read the value
	equal func(a, b T) bool // set by EqualFunc

	depMu sync.Mutex
	deps  []subscribable // the signals (and Computeds) the last computation read
	tr    tracker
}

// NewComputed creates a Computed whose value is fn's result. fn runs the first
// time the value is read, and again after a signal it read changes; it should
// only read signals and compute, with no other effects.
//
// When a signal it read changes, its readers are not re-rendered at once: the
// value is computed again before the next frame, and the readers re-render
// only if it is not the same as before, by == as for a Signal (see
// Signal.EqualFunc and EqualFunc). A Computed that turns a list into a count,
// or a selection into "is this row selected", re-renders only the readers
// whose result changed.
func NewComputed[T any](fn func() T) *Computed[T] {
	c := &Computed[T]{fn: fn}
	c.tr.o = c
	return c
}

// EqualFunc sets how a new result is compared with the last one, to tell
// whether the readers need to re-render; it returns c. The default is the
// comparison a Signal makes (see Signal.EqualFunc).
func (c *Computed[T]) EqualFunc(equal func(a, b T) bool) *Computed[T] {
	c.mu.Lock()
	c.equal = equal
	c.mu.Unlock()
	return c
}

// Get returns the value, computing it if a signal it depends on has changed.
// During a render it subscribes the rendering component.
func (c *Computed[T]) Get() T {
	t := tracking.Load()
	if t == nil {
		return c.Peek()
	}
	c.mu.Lock()
	found := false
	for _, sub := range c.subs {
		if sub == t.o {
			found = true
			break
		}
	}
	if !found {
		c.subs = append(c.subs, t.o)
		t.o.addDep(c)
	}
	c.mu.Unlock()
	// The reader is told of no change the computation finds: it reads the new value now.
	return c.get(t.o)
}

// Peek returns the value without subscribing, computing it if needed.
func (c *Computed[T]) Peek() T { return c.get(nil) }

// refresh brings the value up to date, telling the readers if it changed.
func (c *Computed[T]) refresh() { c.get(nil) }

// get returns the value, computing it if needed. If the result is not the same
// as the last one, the readers other than reader re-render.
func (c *Computed[T]) get(reader observer) T {
	c.mu.Lock()
	if c.valid && c.maybe {
		// Only a Computed it read may have changed: bring those up to date, and
		// keep the value if none of them did (none invalidated c).
		c.maybe = false
		c.mu.Unlock()
		c.refreshDeps()
		c.mu.Lock()
	}
	if c.valid && !c.maybe {
		v := c.value
		c.mu.Unlock()
		return v
	}
	gen := c.gen
	c.mu.Unlock()

	// Recompute with this Computed as the observer, so the signals fn reads
	// become its dependencies. The lock is not held while fn runs: fn may read
	// other Computeds, and a change may invalidate this one meanwhile.
	c.dropDeps()
	v := c.compute()

	c.mu.Lock()
	changed := !c.has || !c.same(c.value, v)
	// The readers see v, so it is what the next result is compared with. A
	// signal that changed while fn ran makes it stale already: it is returned,
	// but the next read computes again.
	c.value, c.has = v, true
	if c.gen == gen {
		c.valid, c.maybe = true, false
	}
	var subs []observer
	if changed {
		subs = append(subs, c.subs...)
	}
	c.mu.Unlock()
	for _, o := range subs {
		if o != reader {
			o.invalidate()
		}
	}
	return v
}

func (c *Computed[T]) same(a, b T) bool {
	if c.equal != nil {
		return c.equal(a, b)
	}
	return same(a, b)
}

// compute runs fn with this Computed's reads recorded. The tracker is restored
// even if fn panics.
func (c *Computed[T]) compute() T {
	prev := tracking.Swap(&c.tr)
	defer tracking.Store(prev)
	return c.fn()
}

// invalidate is called when a signal the value was computed from changes, or a
// Computed it was computed from changed. The next read computes it again; its
// readers are told that it may have changed, and find out whether it did
// before they re-render.
func (c *Computed[T]) invalidate() {
	c.mu.Lock()
	c.valid = false
	c.gen++
	subs := append([]observer(nil), c.subs...)
	c.mu.Unlock()
	for _, o := range subs {
		o.stale()
	}
}

// stale is called when a Computed the value was computed from may have
// changed. The next read finds out, and computes the value again if it did.
func (c *Computed[T]) stale() {
	c.mu.Lock()
	c.maybe = true
	subs := append([]observer(nil), c.subs...)
	c.mu.Unlock()
	for _, o := range subs {
		o.stale()
	}
}

func (c *Computed[T]) addDep(s subscribable) {
	c.depMu.Lock()
	c.deps = append(c.deps, s)
	c.depMu.Unlock()
}

// refreshDeps brings the Computeds the last computation read up to date. One
// that changed invalidates c.
func (c *Computed[T]) refreshDeps() {
	c.depMu.Lock()
	deps := append([]subscribable(nil), c.deps...)
	c.depMu.Unlock()
	refreshAll(deps)
}

// refreshAll brings the Computeds among deps up to date.
func refreshAll(deps []subscribable) {
	for _, d := range deps {
		if r, ok := d.(refresher); ok {
			r.refresh()
		}
	}
}

// refresher is a dependency whose value is computed: a Computed.
type refresher interface{ refresh() }

// dropDeps unsubscribes from the signals the last computation read.
func (c *Computed[T]) dropDeps() {
	c.depMu.Lock()
	deps := c.deps
	c.deps = nil
	c.depMu.Unlock()
	for _, d := range deps {
		d.unsubscribe(c)
	}
}

func (c *Computed[T]) unsubscribe(o observer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, sub := range c.subs {
		if sub == o {
			last := len(c.subs) - 1
			c.subs[i] = c.subs[last]
			c.subs[last] = nil // the spare capacity must not keep the observer alive
			c.subs = c.subs[:last]
			return
		}
	}
}
