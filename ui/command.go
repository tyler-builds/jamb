package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"
)

// Command is an operation of the app that a coding agent or a script calls with
// arguments and gets a result from, through the control channel (`jamb ctl run`,
// see docs/tools.md). Its input and output are Go types, and the channel
// describes them to the caller with JSON Schemas made from those types.
//
// An action (see NewAction) is what a user runs from a key, a menu or the
// command palette, on the focused element; it takes nothing and returns nothing.
// A command is app-wide, takes typed input and returns typed output. The control
// channel offers both: an agent runs the app's actions as a user would, and its
// commands where clicking through the UI would be slow or ambiguous.
type Command struct {
	name, description string
	in, out           reflect.Type
	run               func(in any) (any, error)
}

var commands = struct {
	sync.Mutex
	byName map[string]*Command
}{byName: map[string]*Command{}}

// NewCommand registers a command. name identifies it, as an action's does (for
// example "notes.create"); description tells an agent what it does. run is
// called on the UI thread with the decoded input, and its output is encoded as
// JSON. In and Out are any types that encoding/json reads and writes: structs
// with json tags describe best, and a field's `desc` tag becomes its
// description in the schema. A field without omitempty is required.
//
//	type CreateIn struct {
//		Title string   `json:"title" desc:"The note's title"`
//		Tags  []string `json:"tags,omitempty"`
//	}
//	var CreateNote = ui.NewCommand("notes.create", "Create a note and select it",
//		func(in CreateIn) (NoteRef, error) { ... })
//
// Create commands once, as package variables, as actions are. A second command
// with the same name panics.
func NewCommand[In, Out any](name, description string, run func(In) (Out, error)) *Command {
	if name == "" || run == nil {
		panic("ui.NewCommand: a command needs a name and a run function")
	}
	c := &Command{
		name: name, description: description,
		in: reflect.TypeFor[In](), out: reflect.TypeFor[Out](),
		run: func(in any) (any, error) { return run(in.(In)) },
	}
	commands.Lock()
	defer commands.Unlock()
	if _, dup := commands.byName[name]; dup {
		panic(fmt.Sprintf("ui.NewCommand: a command named %q already exists", name))
	}
	commands.byName[name] = c
	return c
}

// Commands returns the registered commands, sorted by name.
func Commands() []*Command {
	commands.Lock()
	defer commands.Unlock()
	out := make([]*Command, 0, len(commands.byName))
	for _, c := range commands.byName {
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b *Command) int { return strings.Compare(a.name, b.name) })
	return out
}

// CommandNamed returns the command with the name, or nil.
func CommandNamed(name string) *Command {
	commands.Lock()
	defer commands.Unlock()
	return commands.byName[name]
}

// Name returns the command's name.
func (c *Command) Name() string { return c.name }

// Description returns what the command does.
func (c *Command) Description() string { return c.description }

// InputSchema returns the JSON Schema (draft 2020-12) of the command's input.
func (c *Command) InputSchema() map[string]any { return schemaOf(c.in) }

// OutputSchema returns the JSON Schema (draft 2020-12) of the command's output.
func (c *Command) OutputSchema() map[string]any { return schemaOf(c.out) }

// CommandError is an input that the command could not decode, which RunJSON
// returns so that a caller can tell it from a failure of the command itself.
type CommandError struct{ Err error }

func (e *CommandError) Error() string { return "invalid input: " + e.Err.Error() }
func (e *CommandError) Unwrap() error { return e.Err }

// RunJSON decodes args as the command's input, runs the command and returns its
// output. Call it on the UI thread. Empty args mean an empty object. An input
// with a field the type lacks, a required field missing or a value of the wrong
// type is a *CommandError.
func (c *Command) RunJSON(args []byte) (any, error) {
	if len(bytes.TrimSpace(args)) == 0 {
		args = []byte("{}")
	}
	in := reflect.New(c.in)
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.DisallowUnknownFields()
	if err := dec.Decode(in.Interface()); err != nil {
		return nil, &CommandError{err}
	}
	if err := checkRequired(c.in, args); err != nil {
		return nil, &CommandError{err}
	}
	return c.run(in.Elem().Interface())
}

// checkRequired reports the first required field of a struct input that args
// leaves out.
func checkRequired(t reflect.Type, args []byte) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	var given map[string]json.RawMessage
	if json.Unmarshal(args, &given) != nil {
		return nil // not an object; Decode has said what is wrong
	}
	for _, f := range jsonFields(t) {
		if f.required {
			if _, ok := given[f.name]; !ok {
				return fmt.Errorf("%q is required", f.name)
			}
		}
	}
	return nil
}

// jsonField is a struct field as encoding/json sees it.
type jsonField struct {
	name     string
	typ      reflect.Type
	required bool
	desc     string
}

// jsonFields returns the fields that encoding/json encodes, with embedded
// structs' fields promoted.
func jsonFields(t reflect.Type) []jsonField {
	var out []jsonField
	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if f.Anonymous && name == "" {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				out = append(out, jsonFields(ft)...)
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		omit := slices.Contains(strings.Split(opts, ","), "omitempty") || slices.Contains(strings.Split(opts, ","), "omitzero")
		out = append(out, jsonField{name: name, typ: f.Type, required: !omit && f.Type.Kind() != reflect.Pointer, desc: f.Tag.Get("desc")})
	}
	return out
}

var (
	timeType      = reflect.TypeFor[time.Time]()
	durationType  = reflect.TypeFor[time.Duration]()
	rawType       = reflect.TypeFor[json.RawMessage]()
	marshalerType = reflect.TypeFor[json.Marshaler]()
)

// schemaOf returns the JSON Schema of the values of t, as encoding/json writes
// them.
func schemaOf(t reflect.Type) map[string]any {
	s := schemaFor(t, map[reflect.Type]bool{})
	s["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	return s
}

func schemaFor(t reflect.Type, seen map[reflect.Type]bool) map[string]any {
	switch {
	case t == timeType:
		return map[string]any{"type": "string", "format": "date-time"}
	case t == durationType:
		return map[string]any{"type": "integer", "description": "nanoseconds"}
	case t == rawType:
		return map[string]any{}
	case t.Kind() != reflect.Pointer && t.Implements(marshalerType),
		t.Kind() != reflect.Pointer && reflect.PointerTo(t).Implements(marshalerType):
		return map[string]any{} // it writes its own JSON
	}
	switch t.Kind() {
	case reflect.Pointer:
		return schemaFor(t.Elem(), seen)
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 && t.Kind() == reflect.Slice {
			return map[string]any{"type": "string", "contentEncoding": "base64"}
		}
		return map[string]any{"type": "array", "items": schemaFor(t.Elem(), seen)}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": schemaFor(t.Elem(), seen)}
	case reflect.Struct:
		if seen[t] {
			return map[string]any{"type": "object", "description": "a " + t.Name() + ", as above"}
		}
		seen[t] = true
		defer delete(seen, t)
		props := map[string]any{}
		required := []string{}
		for _, f := range jsonFields(t) {
			p := schemaFor(f.typ, seen)
			if f.desc != "" {
				p["description"] = f.desc
			}
			props[f.name] = p
			if f.required {
				required = append(required, f.name)
			}
		}
		s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
		if len(required) > 0 {
			s["required"] = required
		}
		return s
	}
	return map[string]any{} // an interface: any value
}
