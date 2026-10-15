// Package keymap sets key bindings from JSON files, so users can change
// shortcuts without code. It uses only the exported API of package ui.
//
// A keymap file is a JSON array of sections, applied in order: a later
// section, or a later key within one, wins, as code order does. A section has
// an optional context (a BindIn expression; none means global) and its
// bindings. A value is an action name, null (NoAction) or {"unbind": name}
// (Unbind):
//
//	[
//	  {"bindings": {"ctrl+shift+n": "notes.new"}},
//	  {"context": "Editor", "bindings": {
//	    "ctrl+d": "editor.delete-forward",
//	    "ctrl+a": null,
//	    "ctrl+z": {"unbind": "editor.undo"}
//	  }}
//	]
package keymap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/tyler-builds/jamb/ui"
)

// entry is one "keys": value pair of a section, in file order.
type entry struct {
	keys  string
	value any
}

// member is one member of a JSON object, with its value left raw.
type member struct {
	key string
	raw json.RawMessage
}

// Parse reads a keymap file (see the package documentation) and returns its
// bindings in file order. actions are the actions the file may name, in
// addition to the text-field actions (ui.EditorActions) and ui.NoAction, which
// are always available. Every problem is reported, naming its section (1-based),
// context and keys; the valid bindings are returned along with the errors,
// joined.
func Parse(data []byte, actions []*ui.Action) ([]ui.Binding, error) {
	var sections []json.RawMessage
	if err := json.Unmarshal(data, &sections); err != nil {
		return nil, &formatError{fmt.Errorf("a keymap is a JSON array of sections: %w", err)}
	}
	known := map[string]*ui.Action{ui.NoAction.Name(): ui.NoAction}
	for _, a := range append(ui.EditorActions(), actions...) {
		known[a.Name()] = a
	}
	var out []ui.Binding
	var errs []error
	for i, raw := range sections {
		expr, entries, err := decodeSection(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("keymap section %d: %w", i+1, err))
		}
		label := fmt.Sprintf("keymap section %d (no context)", i+1)
		if expr != "" {
			label = fmt.Sprintf("keymap section %d (context %q)", i+1, expr)
		}
		// A section whose context does not parse keeps reporting its keys,
		// but none of its bindings can be made.
		valid := true
		bindExpr := expr
		if expr != "" {
			// A probe key: ParseBinding checks the expression first, so this
			// reports only a bad context.
			if _, err := ui.ParseBinding(expr, "f1", ui.NoAction); err != nil {
				errs = append(errs, fmt.Errorf("%s: context: %w", label, err))
				valid, bindExpr = false, ""
			}
		}
		for _, e := range entries {
			b, err := binding(e, bindExpr, known)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %q: %w", label, e.keys, err))
				continue
			}
			if valid {
				out = append(out, b)
			}
		}
	}
	return out, errors.Join(errs...)
}

// Load reads the keymap file at path and parses it as Parse does. A missing
// file is not an error: it returns no bindings and a nil error. Other errors
// name the file.
func Load(path string, actions []*ui.Action) ([]ui.Binding, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	bs, err := Parse(data, actions)
	if err != nil {
		return bs, fmt.Errorf("%s: %w", path, err)
	}
	return bs, nil
}

// File is a keymap file applied to a runtime as its own binding layer, named
// "keymap:"+Path, which keeps its place among the bindings (see
// ui.Runtime.SetBindings). Actions are the actions the file may name.
type File struct {
	// Path is the path of the keymap file.
	Path string
	// Actions lists the actions that the file may name.
	Actions []*ui.Action

	last    []byte // the bytes of the last successful read
	read    bool   // last holds a read of the file
	applied bool   // the layer has bindings in the runtime
}

// Apply reads the file and applies it to rt, which must run on the UI thread.
// It reads the file on each call and does nothing when the bytes are the same
// as last time, so it is cheap to call on every change notification.
//
// A missing file removes the layer. A file that cannot be read, or that is not
// a JSON array, keeps the layer as it was and returns the error, so a
// half-saved file does not wipe the user's keymap. Otherwise the layer is
// replaced by the file's valid bindings, and err joins the problems with the
// entries that were skipped; changed reports whether the bindings changed (or
// the file could not be read).
func (f *File) Apply(rt *ui.Runtime) (changed bool, err error) {
	layer := "keymap:" + f.Path
	data, err := os.ReadFile(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		f.last, f.read = nil, false
		if !f.applied {
			return false, nil
		}
		f.applied = false
		rt.SetBindings(layer, nil)
		return true, nil
	}
	if err != nil {
		return true, fmt.Errorf("%s: %w", f.Path, err)
	}
	if f.read && bytes.Equal(f.last, data) {
		return false, nil
	}
	f.last, f.read = data, true
	bs, err := Parse(data, f.Actions)
	var format *formatError
	if errors.As(err, &format) {
		return true, fmt.Errorf("%s: %w", f.Path, err)
	}
	rt.SetBindings(layer, bs)
	f.applied = len(bs) > 0
	if err != nil {
		err = fmt.Errorf("%s: %w", f.Path, err)
	}
	return true, err
}

// formatError is Parse's error when the file is not a JSON array at all, so no
// section can be read. Files with bad sections are not this.
type formatError struct{ err error }

func (e *formatError) Error() string { return e.err.Error() }
func (e *formatError) Unwrap() error { return e.err }

// decodeSection reads a section's context and bindings. Problems in the section
// are returned joined, and the entries that can be read are still returned.
func decodeSection(raw json.RawMessage) (string, []entry, error) {
	members, err := objectMembers(raw)
	if err != nil {
		return "", nil, errors.New(`a section is an object with "context" and "bindings"`)
	}
	var expr string
	var entries []entry
	var errs []error
	for _, m := range members {
		switch m.key {
		case "context":
			if err := json.Unmarshal(m.raw, &expr); err != nil {
				errs = append(errs, errors.New(`"context" must be a string`))
			}
		case "bindings":
			es, err := decodeBindings(m.raw)
			if err != nil {
				errs = append(errs, err)
			}
			entries = append(entries, es...)
		default:
			errs = append(errs, fmt.Errorf("unknown field %q; a section has \"context\" and \"bindings\"", m.key))
		}
	}
	return expr, entries, errors.Join(errs...)
}

// decodeBindings reads the bindings object of a section, in file order.
func decodeBindings(raw json.RawMessage) ([]entry, error) {
	members, err := objectMembers(raw)
	if err != nil {
		return nil, errors.New(`"bindings" must be an object from keys to actions`)
	}
	out := make([]entry, 0, len(members))
	for _, m := range members {
		var v any
		if err := json.Unmarshal(m.raw, &v); err != nil {
			return nil, err
		}
		out = append(out, entry{keys: m.key, value: v})
	}
	return out, nil
}

// objectMembers returns the members of a JSON object in file order. A map
// would lose the order, and the order decides which of two bindings wins.
func objectMembers(raw json.RawMessage) ([]member, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, errors.New("expected a JSON object")
	}
	var out []member
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, errors.New("expected a key string")
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, member{key: key, raw: v})
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return nil, err
	}
	return out, nil
}

// binding makes the binding for one entry in its section's context. A bad
// keys string is reported before a bad action name.
func binding(e entry, expr string, known map[string]*ui.Action) (ui.Binding, error) {
	a, err := action(e.value, known)
	if err != nil {
		if _, kerr := ui.ParseBinding(expr, e.keys, ui.NoAction); kerr != nil {
			return ui.Binding{}, kerr
		}
		return ui.Binding{}, err
	}
	return ui.ParseBinding(expr, e.keys, a)
}

// action says which action a value names: an action name, null for NoAction,
// or {"unbind": name} for Unbind of that action.
func action(v any, known map[string]*ui.Action) (*ui.Action, error) {
	switch v := v.(type) {
	case nil:
		return ui.NoAction, nil
	case string:
		return lookupAction(v, known)
	case map[string]any:
		name, ok := v["unbind"].(string)
		if !ok || len(v) != 1 {
			return nil, errors.New(`an unbind is written {"unbind": "action name"}`)
		}
		a, err := lookupAction(name, known)
		if err != nil {
			return nil, err
		}
		return ui.Unbind(a), nil
	}
	return nil, errors.New(`the value must be an action name, null, or {"unbind": "action name"}`)
}

// lookupAction finds the action a file names.
func lookupAction(name string, known map[string]*ui.Action) (*ui.Action, error) {
	a, ok := known[name]
	if !ok {
		return nil, fmt.Errorf("unknown action %q", name)
	}
	return a, nil
}
