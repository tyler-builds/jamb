package syntaxname

import "testing"

func TestClosest(t *testing.T) {
	styles := map[string]int{"function": 1, "function.method": 2, "keyword": 3, "property": 4, "tag": 5}
	tests := []struct {
		name string
		key  string
		ok   bool
	}{
		{"function.method.builtin", "function.method", true},
		{"function.builtin", "function", true},
		{"function.method", "function.method", true},
		{"keyword.control.import", "keyword", true},
		{"selector.class", "property", true},
		{"selector", "tag", true},
		{"selector.pseudo", "", false}, // its fallback, attribute, is not in the map
		{"variable", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		key, _, ok := Closest(styles, tt.name)
		if key != tt.key || ok != tt.ok {
			t.Errorf("Closest(%q) = %q, %v; want %q, %v", tt.name, key, ok, tt.key, tt.ok)
		}
	}
	// A theme that styles selectors keeps its own style.
	if key, _, _ := Closest(map[string]int{"selector": 1, "property": 2}, "selector.class"); key != "selector" {
		t.Errorf("a theme's own selector style lost to the fallback: %q", key)
	}
}
