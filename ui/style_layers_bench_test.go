package ui

import (
	"strconv"
	"testing"
	"time"
)

// widgetHeavyRoot is a window of rows, each with the widgets that resolve the most
// layers: a button, a check box, a switch, a text field, a badge and a tag.
func widgetHeavyRoot(rows int) func(*Context) Element {
	on, text, check := NewSignal(false), NewSignal(""), NewSignal(false)
	return func(ctx *Context) Element {
		col := Div().Column().Gap(4)
		for i := 0; i < rows; i++ {
			col.Child(Div().Row().Gap(8).Children(
				Button("Save").Key(strconv.Itoa(i)),
				Checkbox("Remember", check),
				Switch("On", on),
				TextInput(text),
				Badge(i),
				Tag("Sample"),
			))
		}
		return col
	}
}

// benchmarkWidgetFrame times a frame of a window of 40 rows of widgets, which is
// rendered again on every frame. With inspect, the inspector is attached, so each
// layer is recorded; without it nothing is.
func benchmarkWidgetFrame(b *testing.B, inspect bool) {
	rt := NewRuntime(widgetHeavyRoot(40), DarkTheme(), nil)
	rt.SetViewport(1200, 900, 1)
	if inspect {
		rt.insp = &inspectState{edits: map[uint64]Style{}}
	}
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.hostInvalidateAll()
		rt.Frame(now)
	}
}

// BenchmarkWidgetFrameNoInspector is a frame of widgets with no inspector attached.
func BenchmarkWidgetFrameNoInspector(b *testing.B) { benchmarkWidgetFrame(b, false) }

// BenchmarkWidgetFrameInspector is the same frame with an inspector attached.
func BenchmarkWidgetFrameInspector(b *testing.B) { benchmarkWidgetFrame(b, true) }
