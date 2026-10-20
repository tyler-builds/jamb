// Package syntaxname finds the style of a syntax token name in a theme's syntax
// map, for the code editor and the Markdown view.
package syntaxname

import "strings"

// fallbacks are the names that a theme may not have, with the name whose style
// they take then. The highlighter names CSS selectors "selector", which Zed's
// themes, and so jamb's, do not style; they take the styles that Zed's CSS
// highlighting gives them.
var fallbacks = map[string]string{
	"selector":        "tag",
	"selector.class":  "property",
	"selector.id":     "property",
	"selector.pseudo": "attribute",
}

// Closest finds the style for a token name in a theme's syntax map. It tries the
// name itself, then the name with its last dotted part removed, and so on, so
// "function.method.builtin" falls back to "function.method" and then to
// "function"; when none is in the map, it tries the same for the name's fallback,
// such as "property" for "selector.class". It returns the key it matched, and
// false when no key matches.
func Closest[V any](styles map[string]V, name string) (string, V, bool) {
	if key, v, ok := closest(styles, name); ok {
		return key, v, true
	}
	for n := name; n != ""; n = parent(n) {
		if f, ok := fallbacks[n]; ok {
			return closest(styles, f)
		}
	}
	var zero V
	return "", zero, false
}

// closest is Closest without the fallbacks.
func closest[V any](styles map[string]V, name string) (string, V, bool) {
	for n := name; n != ""; n = parent(n) {
		if v, ok := styles[n]; ok {
			return n, v, true
		}
	}
	var zero V
	return "", zero, false
}

// parent returns name without its last dotted part, or "" when it has one part.
func parent(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[:i]
	}
	return ""
}
