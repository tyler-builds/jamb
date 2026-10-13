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
	invalidate()
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
// Treat the value as immutable. To change a slice, map or struct pointer
// held in a signal, build a new one and Set it (slices.Clone, append to a
// copy, ...). Changing it in place does not notify anyone, and can race with
// a render reading it.
type Signal[T any] struct {
	mu    sync.Mutex
	value T
	subs  []observer
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

// Set stores v and schedules subscribers to re-render.
func (s *Signal[T]) Set(v T) {
	s.mu.Lock()
	s.value = v
	subs := append([]observer(nil), s.subs...)
	s.mu.Unlock()
	for _, inst := range subs {
		inst.invalidate()
	}
}

// Update sets the value to fn(current) atomically.
func (s *Signal[T]) Update(fn func(T) T) {
	s.mu.Lock()
	s.value = fn(s.value)
	subs := append([]observer(nil), s.subs...)
	s.mu.Unlock()
	for _, inst := range subs {
		inst.invalidate()
	}
}

func (s *Signal[T]) unsubscribe(o observer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sub := range s.subs {
		if sub == o {
			s.subs[i] = s.subs[len(s.subs)-1]
			s.subs = s.subs[:len(s.subs)-1]
			return
		}
	}
}
