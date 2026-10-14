package ui

import (
	"reflect"
	"slices"
	"strings"

	"github.com/tyler-builds/jamb/internal/inspect"
)

// The visual inspector names the layer that set each field of a style (see
// Fields in inspect.go). A widget part's style is merged from layers: the
// widget's built-in default, the theme's part, and the Styles of the instance
// (see layered). While an inspector is attached to the window, each layer is
// named on the style it is merged from, field by field, and Merge carries the
// names along with the values. The resolved style of an element then knows the
// layer of every field, state layers and live edits included. Without an
// inspector nothing is recorded: the records are nil and Merge does not touch
// them.

// styleProv records, by field name, the layers that set a style's fields, the
// lowest first. The last entry of a field is the layer that its value came from,
// if the value is the one the entry records.
type styleProv map[string][]provEntry

// provEntry is one layer that set a field: its name, and the value the field had
// after the layer was merged, as the inspector shows it.
type provEntry struct {
	layer string
	value string
}

var styleType = reflect.TypeOf(Style{})

// Layer returns s with the name of the layer recorded on each style that s holds:
// s itself when it is a Style, or each Style field of a struct, such as a widget's
// part styles. When no inspector is attached to the window of ctx, s is returned
// unchanged.
//
// A widget that builds a part's style from layers calls Layer on each layer,
// before it merges the layer, so that the inspector can name the layer each value
// came from. The names the inspector shows are "built-in default" (the widget's own
// default), "theme" (the theme's style for the part) and "instance Styles" (the
// Styles of the element); a widget may name another layer, such as "high contrast".
// The merged style keeps the names, so the inspector reports the layer of every
// field. A style that is already merged from named layers keeps its names.
func Layer[S any](ctx *Context, s S, name string) S {
	if ctx == nil || ctx.rt.insp == nil {
		return s
	}
	return recordLayer(s, name)
}

// recordLayer is Layer while an inspector is attached. It is kept apart from Layer
// because taking the address of its argument moves the argument to the heap, which
// Layer must not do when nothing is recorded.
func recordLayer[S any](s S, name string) S {
	tagLayer(reflect.ValueOf(&s).Elem(), name)
	return s
}

// tagLayer records the layer name on each Style in v, which must be settable. A
// style that already has records is a resolved style (a widget's, merged from its
// own layers), so its records stay: they name the layers it came from.
func tagLayer(v reflect.Value, name string) {
	if v.Type() == styleType {
		if s := v.Interface().(Style); s.prov == nil {
			v.Set(reflect.ValueOf(tagStyle(s, name)))
		}
		return
	}
	if v.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < v.NumField(); i++ {
		if f := v.Field(i); f.CanSet() {
			tagLayer(f, name)
		}
	}
}

// tagStyle returns s with each field that it sets recorded as set by the layer.
func tagStyle(s Style, layer string) Style {
	p := styleProv{}
	v := reflect.ValueOf(s)
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		if f := t.Field(i); f.IsExported() && !v.Field(i).IsZero() {
			p[f.Name] = []provEntry{{layer, formatStyleValue(v.Field(i))}}
		}
	}
	s.prov = p
	return s
}

// mergeProv returns the records of r, the merge of a style that has the records sp
// with a style that has the records op. Each field that the merge sets gets op's
// records after sp's, and the value of its last record is the value the merge
// left. A field that the merge zeroes (a clear, or a border width that resets its
// edges) gets a record of the layer that cleared it.
func mergeProv(sp, op styleProv, r Style) styleProv {
	out := make(styleProv, len(sp)+len(op))
	for k, v := range sp {
		out[k] = slices.Clip(v)
	}
	rv := reflect.ValueOf(r)
	t := rv.Type()
	by := "element method" // the layer that zeroed a field: the first one of op's
	for i := 0; i < t.NumField(); i++ {
		if oe := op[t.Field(i).Name]; len(oe) > 0 {
			by = oe[len(oe)-1].layer
			break
		}
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if oe := op[f.Name]; len(oe) > 0 {
			entries := append(slices.Clip(out[f.Name]), oe...)
			entries[len(entries)-1].value = formatStyleValue(rv.Field(i))
			out[f.Name] = entries
		} else if chain := sp[f.Name]; len(chain) > 0 && rv.Field(i).IsZero() {
			if cur := formatStyleValue(rv.Field(i)); chain[len(chain)-1].value != cur {
				out[f.Name] = append(slices.Clip(chain), provEntry{"cleared by " + by, cur})
			}
		}
	}
	return out
}

// inState returns s, the style that an element takes in a state (hover, pressed,
// and so on), for the merge in resolveStyle. With an inspector attached, its
// layers are named as the state's: "state: hover (theme)", for example.
func (rt *Runtime) inState(s Style, state string) Style {
	if rt.insp == nil {
		return s
	}
	name := inspect.SourceState + state
	if s.prov == nil {
		return tagStyle(s, name)
	}
	return renameLayers(s, name)
}

// recordSizing names the sizing method as the layer of the fields that it set, on
// a style that has records. The sizes are applied after the records of the other
// layers, so the sizing method is the last layer of its fields.
func (s *Style) recordSizing(set sizeSet) {
	if s.prov == nil {
		return
	}
	v := reflect.ValueOf(*s)
	for name, bit := range sizingFields {
		if set&bit != 0 {
			f := v.FieldByName(name)
			s.prov[name] = append(slices.Clip(s.prov[name]), provEntry{inspect.SourceSizing, formatStyleValue(f)})
		}
	}
}

// layer is one layer of a widget's part styles, with the name the inspector
// gives it.
type layer[S any] struct {
	name string
	s    S
}

// themeLayer is the theme's style for a part, as a layer for layered.
func themeLayer[S any](s S) layer[S] { return layer[S]{inspect.SourceTheme, s} }

// instanceLayer is the Styles of an element, as a layer for layered.
func instanceLayer[S any](s S) layer[S] { return layer[S]{inspect.SourceInstance, s} }

// layered returns the look of a widget's part styles: base, with each layer
// merged over it in order, as the widget's merge does. Under high contrast each
// layer is first passed through plain, which drops what the system colors replace.
// The inspector names base as the built-in default, and each layer by its name
// (see Layer).
func layered[S any](ctx *Context, base S, merge func(S, S) S, plain func(S) S, layers ...layer[S]) S {
	hc := ctx.Theme().HighContrast
	st := base
	// Without an inspector the layers are merged as they always were: a layer is
	// copied only when it is recorded.
	rec := ctx.rt.insp != nil
	if rec {
		st = recordLayer(st, inspect.SourceDefault)
	}
	for _, l := range layers {
		o := l.s
		if hc {
			o = plain(o)
		}
		if rec {
			o = recordLayer(o, l.name)
		}
		st = merge(st, o)
	}
	if rec {
		st = nameStates(st)
	}
	return st
}

// appliedStates names the states that a widget applies with a Style of its own
// (see Style in the widget's part styles), by the suffix of the field: the field
// BoxChecked of a check box is the checked state of its box. The states that are
// pointer states (Hover, FocusVisible, and so on) are named when they apply (see
// inState), and are not here.
var appliedStates = map[string]string{
	"Checked": "checked", "Mixed": "mixed", "Disabled": "disabled", "Invalid": "invalid",
	"Selected": "selected", "Highlighted": "highlighted", "Open": "open", "Current": "current",
	"Today": "today", "InRange": "in range", "OutsideMonth": "outside month", "Sorted": "sorted",
	"Alternate": "alternate", "Editing": "editing",
}

// nameStates names the layers of the applied states of a resolved widget style (see
// appliedStates), so the inspector reports "state: disabled (theme)" for a value that
// the disabled style of the part set, and not the layer of the part.
func nameStates[S any](st S) S {
	nameStateFields(reflect.ValueOf(&st).Elem())
	return st
}

// nameStateFields names the applied-state fields of the struct v (settable), and of
// the structs it holds.
func nameStateFields(v reflect.Value) {
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if !f.CanSet() {
			continue
		}
		if f.Type() == styleType {
			for suffix, label := range appliedStates {
				if strings.HasSuffix(v.Type().Field(i).Name, suffix) {
					f.Set(reflect.ValueOf(renameLayers(f.Interface().(Style), "state: "+label)))
				}
			}
		} else if f.Kind() == reflect.Struct {
			nameStateFields(f)
		}
	}
}

// renameLayers returns s with its layers named after the state, such as "state:
// hover", and the layer each came from in brackets: "state: hover (theme)". A layer
// that is already a state's keeps its name, so a style that a widget resolved and
// that another widget merges again is not named twice.
func renameLayers(s Style, name string) Style {
	if s.prov == nil {
		return s
	}
	p := make(styleProv, len(s.prov))
	for k, entries := range s.prov {
		es := make([]provEntry, len(entries))
		for i, e := range entries {
			es[i] = e
			if !strings.HasPrefix(e.layer, inspect.SourceState) {
				es[i].layer = name + " (" + e.layer + ")"
			}
		}
		p[k] = es
	}
	s.prov = p
	return s
}
