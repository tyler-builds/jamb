package app

import (
	"fmt"
	"os"
	"slices"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/ui"
)

// Changes describes work in a window that closing the window would lose. A
// Window's Unsaved returns it (see Window.Unsaved).
type Changes struct {
	// Name is what is unsaved, as the prompt names it: a document's name, such
	// as "Report.txt", or a count, such as "3 notes". Empty asks about "your
	// changes".
	Name string
	// Save saves the changes and returns nil when they are saved. It runs on a
	// goroutine of its own, so the window keeps drawing and the prompt shows
	// that it is saving; signals are safe to read there (use Peek). A nil Save
	// means the changes cannot be saved, and the prompt offers only to discard
	// them.
	Save func() error
}

// The close of windows, or the quit of the app, asks about unsaved changes
// before anything closes:
//
//   - When windows are asked to close (a window's close button, Close or
//     RequestClose on its handle, a QuitWhenClosed window, or Quit), every
//     OnClose is asked first, as before. If all of them let the windows close,
//     the windows whose Unsaved reports changes are asked about them, one at a
//     time, each in its own window, which comes to the front.
//   - The prompt offers Save, Don't save and Cancel; Discard and Cancel when the
//     changes cannot be saved. Cancel keeps every window open, also those whose
//     changes were already answered. A failed save keeps the prompt open with
//     the error and offers Try again.
//   - Nothing closes until every window has its answer. An answer is checked
//     again before the windows close: a saved window that still reports changes,
//     and a window answered with Don't save that was edited afterwards (it has
//     had a key press, text or a click since), are asked again.
//   - One close is asked about at a time. A close or a quit asked for while the
//     prompt is open brings the prompt to the front and is otherwise dropped.
//
// The prompt is an AlertDialog in the window's host layer (see
// ui.Runtime.SetHostLayer), so it follows the window's theme and strings and
// assistive technology, tests and the control channel see it. A window whose
// root element is not a Div has no host layer, and is asked with the system's
// message box instead.

// quitPost runs a function on the UI thread; tests replace it.
var quitPost = postUI

// closeOp is a close or a quit waiting for the answers about unsaved changes.
type closeOp struct {
	hosts []*host // the windows that close
	done  func()  // closes them, once every one is answered
	// discarded holds, for each window answered with Don't save, its input
	// count at the answer (see host.inputSeq).
	discarded map[*host]uint64
	asking    *host // the window whose prompt is open
}

// closePrompt is the state of a window's prompt.
type closePrompt struct {
	changes Changes
	saving  bool
	err     string // the error of the last save, which failed
}

// guardClose asks about the unsaved changes of hosts before they close. It
// returns false when there are none, and the caller closes them now. Otherwise
// it returns true, and runs done once every window is answered, or never, when
// the user cancels.
func guardClose(hosts []*host, done func()) bool {
	if op := application.closing; op != nil {
		if op.asking != nil {
			op.asking.win.Focus()
		}
		return true
	}
	op := &closeOp{hosts: slices.Clone(hosts), done: done, discarded: map[*host]uint64{}}
	// The close is in progress before the first question, so that an answer
	// that comes at once, as a message box's may, finds it.
	application.closing = op
	if !op.ask() {
		application.closing = nil
		return false
	}
	return true // done runs, or ran, once every window is answered; or never, after Cancel
}

// unsaved returns the window's unsaved changes, or nil.
func (h *host) unsaved() *Changes {
	if h.cfg.Unsaved == nil {
		return nil
	}
	return h.cfg.Unsaved()
}

// ask opens the prompt of the next window that has changes without a current
// answer. It returns false when every window is answered.
func (op *closeOp) ask() bool {
	for _, h := range op.hosts {
		if !slices.Contains(application.windows, h) {
			continue // it closed some other way
		}
		c := h.unsaved()
		if c == nil {
			continue // nothing to lose, or saved
		}
		if seq, ok := op.discarded[h]; ok && seq == h.inputSeq {
			continue // discarded, and not edited since
		}
		delete(op.discarded, h)
		op.asking = h
		h.showPrompt(&closePrompt{changes: *c})
		h.win.Focus()
		return true
	}
	op.asking = nil
	return false
}

// advance moves on from the window just answered.
func (op *closeOp) advance() {
	if h := op.asking; h != nil && h.prompt != nil {
		h.prompt.Set(nil)
	}
	if op.ask() {
		return
	}
	application.closing = nil
	op.done()
}

// cancel ends the close; every window stays open.
func (op *closeOp) cancel() {
	if h := op.asking; h != nil && h.prompt != nil {
		h.prompt.Set(nil)
	}
	application.closing = nil
}

// forget drops a window that is destroyed while the close waits.
func (op *closeOp) forget(h *host) {
	op.hosts = slices.DeleteFunc(op.hosts, func(o *host) bool { return o == h })
	delete(op.discarded, h)
	if op.asking == h {
		op.advance()
	}
}

// promptCurrent returns the close in progress and h's prompt, and reports
// whether h's prompt is the one it waits for.
func (h *host) promptCurrent() (*closeOp, *closePrompt, bool) {
	op := application.closing
	if op == nil || op.asking != h || h.prompt == nil {
		return nil, nil, false
	}
	p := h.prompt.Peek()
	return op, p, p != nil
}

// promptSave runs the changes' Save, off the UI thread.
func (h *host) promptSave() {
	op, p, ok := h.promptCurrent()
	if !ok || p.saving || p.changes.Save == nil {
		return
	}
	next := *p
	next.saving, next.err = true, ""
	h.showPrompt(&next)
	save := p.changes.Save
	go func() {
		err := save()
		quitPost(func() {
			if application.closing != op || op.asking != h {
				return
			}
			if err != nil {
				failed := next
				failed.saving, failed.err = false, err.Error()
				h.showPrompt(&failed)
				return
			}
			op.advance()
		})
	}()
}

// promptDiscard answers Don't save, or Discard.
func (h *host) promptDiscard() {
	op, p, ok := h.promptCurrent()
	if !ok || p.saving {
		return
	}
	op.discarded[h] = h.inputSeq
	op.advance()
}

// promptCancel answers Cancel. A save in progress cannot be cancelled.
func (h *host) promptCancel() {
	op, p, ok := h.promptCurrent()
	if !ok || p.saving {
		return
	}
	op.cancel()
}

// showPrompt shows the prompt in the window's host layer, or in a message box
// when the window has none.
func (h *host) showPrompt(p *closePrompt) {
	if h.rt.HostLayerShown() {
		h.prompt.Set(p)
		return
	}
	h.prompt.Set(p) // the state the answers read
	if p.saving {
		return // the box closed with its answer; the save runs
	}
	s := h.cfg.Strings
	if s == nil {
		s = ui.EnglishStrings()
	}
	o := dialog.MessageOptions{Title: h.cfg.Title, Icon: dialog.IconWarning, Buttons: dialog.ButtonsYesNoCancel}
	switch {
	case p.changes.Save == nil:
		o.Text, o.Buttons, o.Default = s.DiscardChanges(p.changes.Name)+"\n\n"+s.UnsavedNoSave, dialog.ButtonsOKCancel, dialog.Cancel
	case p.err != "":
		o.Text = s.SaveChanges(p.changes.Name) + "\n\n" + s.SaveFailed(p.err)
	default:
		o.Text = s.SaveChanges(p.changes.Name) + "\n\n" + s.UnsavedLost
	}
	h.win.ShowMessageBox(o, func(b dialog.Button) {
		switch {
		case b == dialog.Yes:
			h.promptSave()
		case b == dialog.No, b == dialog.OK && p.changes.Save == nil:
			h.promptDiscard()
		case b == dialog.NoButton:
			fmt.Fprintln(os.Stderr, "jamb: the window has unsaved changes and cannot ask about them: its root element "+
				"is not a Div, and this platform has no message box; it stays open")
			h.promptCancel()
		default:
			h.promptCancel()
		}
	})
}

// closeLayer renders the prompt about unsaved changes in the window's host layer.
func (h *host) closeLayer(ctx *ui.Context) ui.Element {
	p := h.prompt.Get()
	if p == nil {
		return nil
	}
	s := ctx.Strings()
	var d *ui.AlertDialogElement
	if p.changes.Save == nil {
		d = ui.AlertDialog(s.DiscardChanges(p.changes.Name), s.UnsavedNoSave).
			Intent(ui.AlertDialogDestructive).
			Confirm(s.Discard, h.promptDiscard)
	} else {
		msg, confirm := s.UnsavedLost, s.Save
		if p.err != "" {
			msg, confirm = s.SaveFailed(p.err), s.TryAgain
		}
		d = ui.AlertDialog(s.SaveChanges(p.changes.Name), msg).
			Intent(ui.AlertDialogWarning).
			Confirm(confirm, h.promptSave).
			ConfirmLoading(p.saving).
			Action(ui.Button(s.DontSave).Secondary().Key("jamb.unsaved.dontsave").Disabled(p.saving).OnClick(h.promptDiscard))
	}
	return d.Key("jamb.unsaved").Cancel(s.Cancel, h.promptCancel)
}
