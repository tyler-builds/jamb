package ui

import (
	"strings"
	"testing"
)

// TestFieldMono checks TextInputElement.Mono and TextAreaElement.Mono: the text
// takes the monospace family, over the theme's Field, and of Mono and a Styles
// call that sets a family, the later call wins.
func TestFieldMono(t *testing.T) {
	serif := FieldStyle{Text: Style{FontFamily: "serif"}}
	// field builds a field of widget and applies the calls in order: "mono" for
	// Mono and "styles" for Styles(serif).
	field := func(widget string, sig *Signal[string], calls []string) Element {
		if widget == "TextArea" {
			ta := TextArea(sig).Label("Query")
			for _, c := range calls {
				if c == "mono" {
					ta.Mono()
				} else {
					ta.Styles(serif)
				}
			}
			return ta
		}
		ti := TextInput(sig).Label("Table")
		for _, c := range calls {
			if c == "mono" {
				ti.Mono()
			} else {
				ti.Styles(serif)
			}
		}
		return ti
	}
	for _, widget := range []string{"TextInput", "TextArea"} {
		for _, tc := range []struct {
			name  string
			theme string // the theme's Field text family
			calls string // the calls on the field, in order
			want  string
		}{
			{"the default", "", "", ""},
			{"Mono", "", "mono", "mono"},
			{"Mono over the theme's Field", "serif", "mono", "mono"},
			{"Styles after Mono", "", "mono styles", "serif"},
			{"Mono after Styles", "", "styles mono", "mono"},
		} {
			t.Run(widget+"/"+tc.name, func(t *testing.T) {
				theme := *DarkTheme()
				theme.Field.Text.FontFamily = tc.theme
				sig := NewSignal("select 1")
				r := newThemedRig(&theme, func(ctx *Context) Element {
					return Div().Column().Child(field(widget, sig, strings.Fields(tc.calls)))
				})
				if got := editorNode(t, r.rt).style.FontFamily; got != tc.want {
					t.Errorf("font family %q, want %q", got, tc.want)
				}
			})
		}
	}
}
