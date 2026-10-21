package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// typingBenchApp is a window with one text field, keyed "field": a single-line
// TextInput holding value, or a multi-line TextArea. The field is focused.
func typingBenchApp(area bool, value string) (*Runtime, *Signal[string]) {
	text := NewSignal(value)
	rt := NewRuntime(func(ctx *Context) Element {
		col := Div().Column().Padding(16).Gap(8).Height(Full)
		if area {
			return col.Child(TextArea(text).Key("field").Grow(1))
		}
		return col.Child(TextInput(text).Key("field"))
	}, DarkTheme(), benchText)
	rt.SetViewport(800, 600, 1)
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)
	// A click focuses the field, as a user does.
	f := rt.findKey(rt.tree, "field").hit
	x, y := f.X+f.W/2, f.Y+f.H/2
	rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	rt.PointerUp(x, y, input.ButtonLeft, 0)
	if area {
		// Down 100 times, so that the text is typed in the middle of the document.
		for range 100 {
			rt.KeyDown(input.KeyDown, 0, false)
			rt.KeyUp(input.KeyDown, 0)
		}
	}
	rt.Frame(now.Add(time.Second))
	return rt, text
}

// BenchmarkTextInputTyping types one character per iteration into a focused
// field and runs a frame, as the app does for each key. The single-line field
// starts empty; the TextArea starts with about 10 KB of text, 200 lines long.
// The text is reset every 128 characters, so both cases stay at their size.
func BenchmarkTextInputTyping(b *testing.B) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = fmt.Sprintf("Line %d: the quick brown fox jumps over the lazy dog.", i)
	}
	long := strings.Join(lines, "\n")
	cases := []struct {
		name  string
		area  bool
		start string
	}{
		{"field", false, ""},
		{"area-10KB", true, long},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			rt, text := typingBenchApp(c.area, c.start)
			now := time.Unix(1_000_000, 0)
			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				if i%128 == 127 {
					text.Set(c.start)
				}
				rt.TextInput("a")
				now = now.Add(16 * time.Millisecond)
				rt.Frame(now)
			}
		})
	}
}
