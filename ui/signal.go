package ui

import (
	"sync"
	"sync/atomic"
)

// tracking is the observer whose reads are being recorded: the component
// currently rendering, or a Computed recomputing its value. Signals read then
// become its dependencies.
//
// Threading model: all rendering happens on the single UI thread, one
// component at a time, so one process-wide tracker is enough. Signals may be
// read and written from any goroutine. A background goroutine that calls Get
// at the moment a component is rendering may subscribe that component too;
// the only effect is one extra re-render later, and Peek avoids it. Rendering
// several windows on different threads would need a per-thread tracker
// instead of this global.
var tracking atomic.Pointer[tracker]

// observer reads signals and is told when they change: a component instance,
// or a Computed.
type observer interface {
	// invalidate tells the observer that a value it read changed.
	invalidate()
	// stale tells it that a Computed it read may have changed: the Computed
	// computes its value again when it is next read, and invalidates the
	// observer then if the value changed.
	stale()
	addDep(subscribable)
}

// tracker holds the observer that signal reads are recorded for. Each observer
// keeps its own, so starting to track one does not allocate.
type tracker struct{ o observer }

type subscribable interface {
	unsubscribe(observer)
}

// Signal is a reactive value. Reading it with Get inside a component's render
// function subscribes that component; Set re-renders every subscriber on the
// next frame. Signals are safe to use from any goroutine; off the UI thread,
// prefer Peek to Get.
//
// A write that leaves the value as it was, by ==, notifies no one: setting a
// number, string, boolean, pointer, or struct of those, to the value it has
// already re-renders nothing. Values that == cannot compare, such as slices
// and maps, always notify. EqualFunc replaces the comparison.
//
// Treat the value as immutable. To change a slice, map or struct pointer
// held in a signal, build a new one and Set it (slices.Clone, append to a
// copy, ...). Changing it in place does not notify anyone, and can race with
// a render reading it; nor does setting the same pointer again.
type Signal[T any] struct {
	mu    sync.Mutex
	value T
	subs  []observer
	equal func(a, b T) bool // set by EqualFunc
}

// NewSignal creates a signal holding v.
func NewSignal[T any](v T) *Signal[T] { return &Signal[T]{value: v} }

// Get returns the value and, during rendering, subscribes the rendering component.
func (s *Signal[T]) Get() T {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t := tracking.Load(); t != nil {
		found := false
		for _, sub := range s.subs {
			if sub == t.o {
				found = true
				break
			}
		}
		if !found {
			s.subs = append(s.subs, t.o)
			// Lock order is always signal, then observer.
			t.o.addDep(s)
		}
	}
	return s.value
}

// Peek returns the value without subscribing.
func (s *Signal[T]) Peek() T {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.value
}

// Set stores v and schedules subscribers to re-render, unless v is the same
// as the value (see EqualFunc).
func (s *Signal[T]) Set(v T) {
	s.mu.Lock()
	if s.same(s.value, v) {
		s.mu.Unlock()
		return
	}
	s.value = v
	subs := append([]observer(nil), s.subs...)
	s.mu.Unlock()
	for _, inst := range subs {
		inst.invalidate()
	}
}

// Update sets the value to fn(current) atomically. Like Set, it notifies no
// one when fn returns the same value.
func (s *Signal[T]) Update(fn func(T) T) {
	s.mu.Lock()
	v := fn(s.value)
	if s.same(s.value, v) {
		s.mu.Unlock()
		return
	}
	s.value = v
	subs := append([]observer(nil), s.subs...)
	s.mu.Unlock()
	for _, inst := range subs {
		inst.invalidate()
	}
}

// EqualFunc sets how Set and Update tell whether a value is the same as the
// current one, and so need not re-render the subscribers; it returns s. By
// default values are compared with ==, and types that == cannot compare
// (slices, maps, functions) are never the same. Pass a function that compares
// what the readers show, such as slices.Equal for a slice of strings, or one
// that always returns false to notify on every write. It runs with the signal
// locked, so it must not read or write s.
func (s *Signal[T]) EqualFunc(equal func(a, b T) bool) *Signal[T] {
	s.mu.Lock()
	s.equal = equal
	s.mu.Unlock()
	return s
}

func (s *Signal[T]) same(a, b T) bool {
	if s.equal != nil {
		return s.equal(a, b)
	}
	return same(a, b)
}

func (s *Signal[T]) unsubscribe(o observer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sub := range s.subs {
		if sub == o {
			last := len(s.subs) - 1
			s.subs[i] = s.subs[last]
			s.subs[last] = nil // the spare capacity must not keep the observer alive
			s.subs = s.subs[:last]
			return
		}
	}
}
