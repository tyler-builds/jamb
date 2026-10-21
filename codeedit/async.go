package codeedit

import "context"

// The hooks of a language server (hover, completion, definition) ask an app's provider for an
// answer, which can take a while: the provider runs on a goroutine of its own, and its answer is
// applied on the UI thread, if it is still wanted. A request is no longer wanted when a newer
// one of its kind starts, when the text changes, when the editor loses focus or shows another
// document, and when it is removed; its context is then canceled, so the provider can stop.

// A Query is what a provider is asked about: the text as it was when the request started, its
// TextVersion, and a byte offset in it, such as the caret's or the pointer's. The text is a copy
// that does not change, so the provider can read it from its goroutine.
type Query struct {
	// Text is the document's text when the request started.
	Text string
	// Version is the document's TextVersion for Text.
	Version uint64
	// Offset is the byte offset in Text that the request is about.
	Offset int
}

// request returns the query about the byte offset off of the document as it is now.
func (vs *viewState) request(off int) Query {
	return Query{Text: vs.doc.Text(), Version: vs.doc.TextVersion(), Offset: off}
}

// asyncSlot runs the requests of one kind for an editor, one at a time: each run cancels the
// request before it. seq tells the latest request from the stale ones, and cancel cancels it.
type asyncSlot[T any] struct {
	seq    uint64
	cancel context.CancelFunc
}

// run starts a request: fn runs on a goroutine with a context that the next run or stop cancels,
// and apply runs on the UI thread with its result, if the request is still the slot's latest and
// its context was not canceled. The result is posted through the context of the editor's latest
// render (see ui.Context.Post), as the highlighter's notices are, so it arrives with a frame.
func (s *asyncSlot[T]) run(vs *viewState, fn func(ctx context.Context) (T, error), apply func(T, error)) {
	s.stop()
	seq := s.seq
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go func() {
		v, err := fn(ctx)
		c := vs.latest.Load()
		if c == nil {
			cancel()
			return
		}
		c.Post(func() {
			if seq != s.seq || ctx.Err() != nil {
				return // a newer request, an edit or a blur made this one stale
			}
			s.cancel = nil
			cancel()
			apply(v, err)
		})
	}()
}

// stop cancels the slot's request, if it has one, and makes any result on its way stale.
func (s *asyncSlot[T]) stop() {
	s.seq++
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// pending reports whether the slot has a request that has not answered.
func (s *asyncSlot[T]) pending() bool { return s.cancel != nil }

// closePopups closes the popups of the editor, and reports whether one was open, for Escape.
func (vs *viewState) closePopups() bool {
	hover := vs.closeHover()
	list := vs.closeCompletion()
	actions := vs.closeActions()
	return hover || list || actions
}

// stopRequests cancels every request of the editor, for a removed editor or another document.
// The lightbulb is hidden too: it is about the document it was asked of.
func (vs *viewState) stopRequests() {
	vs.closePopups()
	vs.hover.slot.stop()
	vs.completion.slot.stop()
	vs.define.stop()
	vs.actions.menu.stop()
	vs.hideLightbulb()
}
