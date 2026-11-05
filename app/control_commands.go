package app

import (
	"errors"
	"strings"

	"github.com/tyler-builds/jamb/ui"
)

// ctlActionInfo is an action as the actions op lists it.
type ctlActionInfo struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Keys   string `json:"keys,omitempty"` // its key binding, such as "Ctrl+N"
	Hidden bool   `json:"hidden,omitempty"`
}

// opActions lists the actions that the window handles where its focus is, as
// the command palette does, deepest element first: the actions dispatch can run.
func (c *control) opActions(w *ctlWin, req ctlRequest, reply ctlReply) {
	out := []ctlActionInfo{}
	for _, a := range w.rt.AvailableActions() {
		out = append(out, ctlActionInfo{Name: a.Name(), Label: a.Label(), Keys: w.rt.KeysFor(a), Hidden: a.Hidden()})
	}
	reply(map[string]any{"actions": out}, nil)
}

// opDispatch runs an action on the window's focus path, as its key binding or
// the command palette does.
func (c *control) opDispatch(w *ctlWin, req ctlRequest, reply ctlReply) {
	if req.Action == "" {
		reply(nil, ctlErr("bad_request", "dispatch needs action, an action's name; the actions op lists them"))
		return
	}
	var names []string
	for _, a := range w.rt.AvailableActions() {
		if a.Name() == req.Action {
			handled := w.rt.Dispatch(a)
			w.wake()
			reply(map[string]any{"handled": handled}, nil)
			return
		}
		names = append(names, a.Name())
	}
	reply(nil, ctlErr("not_found", "no action %q is handled where the focus is; these are: %s", req.Action, strings.Join(names, ", ")))
}

type ctlCommandInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Input       map[string]any `json:"input"`
	Output      map[string]any `json:"output"`
}

// opCommands lists the app's commands (see ui.NewCommand) with the JSON
// Schemas of their input and output.
func (c *control) opCommands(req ctlRequest, reply ctlReply) {
	out := []ctlCommandInfo{}
	for _, cmd := range ui.Commands() {
		out = append(out, ctlCommandInfo{Name: cmd.Name(), Description: cmd.Description(),
			Input: cmd.InputSchema(), Output: cmd.OutputSchema()})
	}
	reply(map[string]any{"commands": out}, nil)
}

// opRun runs a command with args as its input and returns its output.
func (c *control) opRun(req ctlRequest, reply ctlReply) {
	if req.Command == "" {
		reply(nil, ctlErr("bad_request", "run needs command, a command's name; the commands op lists them"))
		return
	}
	cmd := ui.CommandNamed(req.Command)
	if cmd == nil {
		var names []string
		for _, x := range ui.Commands() {
			names = append(names, x.Name())
		}
		reply(nil, ctlErr("not_found", "there is no command %q; the commands are: %s", req.Command, strings.Join(names, ", ")))
		return
	}
	out, err := cmd.RunJSON(req.Args)
	var ce *ui.CommandError
	switch {
	case errors.As(err, &ce):
		reply(nil, ctlErr("bad_request", "%s: %v; the commands op gives its input schema", req.Command, ce))
	case err != nil:
		reply(nil, ctlErr("failed", "%s: %v", req.Command, err))
	default:
		reply(map[string]any{"result": out}, nil)
	}
	c.wakeAll()
}

// wakeAll asks every window for a frame, for what a command changed.
func (c *control) wakeAll() {
	for _, w := range c.windows() {
		w.wake()
	}
}
