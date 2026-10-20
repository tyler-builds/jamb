package highlight

import "github.com/tyler-builds/jamb/internal/syntaxname"

// shift returns the spans moved by the edit e. A span that lies inside the
// replaced range vanishes. A span that starts or ends inside it is cut at the
// start of the edit. The input slice is not modified.
func shift(spans []Span, e EditRecord) []Span {
	delta := len(e.Text) - (e.OldEnd - e.Start)
	out := make([]Span, 0, len(spans))
	for _, sp := range spans {
		sp.Start = moved(sp.Start, e, delta)
		sp.End = moved(sp.End, e, delta)
		if sp.Start < sp.End {
			out = append(out, sp)
		}
	}
	return out
}

// moved maps the position p through the edit e, whose length change is delta. A
// position at or before the edit's start stays where it is. A position at or
// after its old end moves by delta. A position inside the replaced range moves
// to the start of the edit.
func moved(p int, e EditRecord, delta int) int {
	switch {
	case p <= e.Start:
		return p
	case p >= e.OldEnd:
		return p + delta
	default:
		return e.Start
	}
}

// Closest finds the style for a capture name in a theme's syntax map. It tries
// the name itself, then the name with its last dotted part removed, and so on,
// so "function.method.builtin" falls back to "function.method" and then to
// "function". When none is in the map, it tries the same for the few names that
// Zed's themes do not style, such as "selector.class" for CSS, with the name
// whose style they take instead ("property"). It returns the key it matched, and
// false when no key matches.
func Closest[V any](styles map[string]V, capture string) (string, V, bool) {
	return syntaxname.Closest(styles, capture)
}
