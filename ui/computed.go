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
	value T
	valid bool       // value is the function's result for the current signals
	gen   uint64     // counts invalidations, so a result computed meanwhile is not kept as valid
	subs  []observer // the components (and Computeds) that read the value

	depMu sync.Mutex
	deps  []subscribable // the signals (and Computeds) the last computation read
	tr    tracker
}

// NewComputed creates a Computed whose value is fn's result. fn runs the first
// time the value is read, and again after a signal it read changes; it should
// only read signals and compute, with no other effects.
func NewComputed[T any](fn func() T) *Computed[T] {
	c := &Computed[T]{fn: fn}
	c.tr.o = c
	return c
}

// Get returns the value, computing it if a signal it depends on has changed.
// During a render it subscribes the rendering component.
func (c *Computed[T]) Get() T {
	if t := tracking.Load(); t != nil {
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
	}
	return c.Peek()
}

// Peek returns the value without subscribing, computing it if needed.
func (c *Computed[T]) Peek() T {
	c.mu.Lock()
	if c.valid {
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
	// A signal that changed while fn ran makes this result stale already: it is
	// returned, but the next read computes again.
	if c.gen == gen {
		c.value, c.valid = v, true
	}
	c.mu.Unlock()
	return v
}

// compute runs fn with this Computed's reads recorded. The tracker is restored
// even if fn panics.
func (c *Computed[T]) compute() T {
	prev := tracking.Swap(&c.tr)
	defer tracking.Store(prev)
	return c.fn()
}

// invalidate is called when a signal the value was computed from changes. The
// next read computes it again; its readers are told now, so they re-render.
func (c *Computed[T]) invalidate() {
	c.mu.Lock()
	c.valid = false
	c.gen++
	subs := append([]observer(nil), c.subs...)
	c.mu.Unlock()
	for _, o := range subs {
		o.invalidate()
	}
}

func (c *Computed[T]) addDep(s subscribable) {
	c.depMu.Lock()
	c.deps = append(c.deps, s)
	c.depMu.Unlock()
}

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
			c.subs[i] = c.subs[len(c.subs)-1]
			c.subs = c.subs[:len(c.subs)-1]
			return
		}
	}
}
