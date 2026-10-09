package ui

import (
	"sync"
	"sync/atomic"
)

// tracking is the component currently rendering; signals read during its
// render become its dependencies.
//
// Threading model: all rendering happens on the single UI thread, one
// component at a time, so one process-wide tracker is enough. Signals may be
// read and written from any goroutine. A background goroutine that calls Get
// at the moment a component is rendering may subscribe that component too;
// the only effect is one extra re-render later, and Peek avoids it. Rendering
// several windows on different threads would need a per-thread tracker
// instead of this global.
var tracking atomic.Pointer[componentInstance]

type subscribable interface {
	unsubscribe(*componentInstance)
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
	subs  []*componentInstance
}

// NewSignal creates a signal holding v.
func NewSignal[T any](v T) *Signal[T] { return &Signal[T]{value: v} }

// Get returns the value and, during rendering, subscribes the rendering component.
func (s *Signal[T]) Get() T {
	s.mu.Lock()
	defer s.mu.Unlock()
	if inst := tracking.Load(); inst != nil {
		found := false
		for _, sub := range s.subs {
			if sub == inst {
				found = true
				break
			}
		}
		if !found {
			s.subs = append(s.subs, inst)
			// Lock order is always signal, then instance.
			inst.addDep(s)
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
	subs := append([]*componentInstance(nil), s.subs...)
	s.mu.Unlock()
	for _, inst := range subs {
		inst.invalidate()
	}
}

// Update sets the value to fn(current) atomically.
func (s *Signal[T]) Update(fn func(T) T) {
	s.mu.Lock()
	s.value = fn(s.value)
	subs := append([]*componentInstance(nil), s.subs...)
	s.mu.Unlock()
	for _, inst := range subs {
		inst.invalidate()
	}
}

func (s *Signal[T]) unsubscribe(inst *componentInstance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sub := range s.subs {
		if sub == inst {
			s.subs[i] = s.subs[len(s.subs)-1]
			s.subs = s.subs[:len(s.subs)-1]
			return
		}
	}
}
