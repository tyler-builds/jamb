package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/input"
)

func TestLocalStateLivesWithItsComponent(t *testing.T) {
	show := NewSignal(true)
	parentTick := NewSignal(0)
	rt := newTestRuntime(func(ctx *Context) Element {
		parentTick.Get()
		if !show.Get() {
			return Text("hidden")
		}
		return Component(func(ctx *Context) Element {
			count := Local(ctx, "count", 0)
			if count.Get() > 5 {
				// Locals are found by name, so a conditional call is fine.
				Local(ctx, "extra", "x")
			}
			return Div().Key("counter").Size(Px(100), Px(30)).
				OnClick(func() { count.Set(count.Peek() + 1) }).
				Child(Textf("count=%d", count.Get()))
		})
	}, 400, 300, 1)
	for i := 0; i < 3; i++ {
		if err := rt.ClickKey("counter"); err != nil {
			t.Fatal(err)
		}
	}
	if findText(rt.tree, "count=3") == nil {
		t.Fatal("local state did not count clicks")
	}
	parentTick.Set(1) // parent re-renders: the child keeps its local state
	frame(rt)
	if findText(rt.tree, "count=3") == nil {
		t.Error("local state was lost when the parent re-rendered")
	}
	show.Set(false) // the component goes away...
	frame(rt)
	show.Set(true) // ...and comes back fresh
	frame(rt)
	if findText(rt.tree, "count=0") == nil {
		t.Error("a re-created component should start with fresh local state")
	}
}

func TestLocalTypeMismatchExplains(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil || !strings.Contains(r.(string), `named "n"`) {
			t.Errorf("want an explanatory panic, got %v", r)
		}
	}()
	newTestRuntime(func(ctx *Context) Element {
		return Component(func(ctx *Context) Element {
			Local(ctx, "n", 0)
			Local(ctx, "n", "oops")
			return nil
		})
	}, 100, 100, 1)
}

func TestFindClickKeyAndDumpShowFields(t *testing.T) {
	v := NewSignal("hello")
	clicks := 0
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Padding(10).Gap(8).
			Child(Button("Save").Key("save").OnClick(func() { clicks++ })).
			Child(TextInput(v).Key("name").Placeholder("Your name").Small())
	}, 400, 300, 1)
	if err := rt.ClickKey("save"); err != nil || clicks != 1 {
		t.Fatalf("ClickKey(save): err=%v clicks=%d", err, clicks)
	}
	if err := rt.ClickKey("nope"); err == nil || !strings.Contains(err.Error(), `"nope"`) {
		t.Errorf("missing key should explain itself, got %v", err)
	}
	r, ok := rt.Find("name")
	if !ok || r.H != 28 {
		t.Errorf("Small text field: found=%v height=%v, want 28", ok, r.H)
	}
	if err := rt.ClickKey("name"); err != nil {
		t.Fatal(err)
	}
	rt.Press(input.KeyEnd, 0)
	rt.Type(" world")
	if v.Peek() != "hello world" {
		t.Errorf("value %q", v.Peek())
	}
	var buf bytes.Buffer
	rt.Dump(&buf, 0)
	if d := buf.String(); !strings.Contains(d, "key=name") || !strings.Contains(d, `placeholder="Your name"`) {
		t.Errorf("Dump does not identify the text field:\n%s", d)
	}
}

func TestWrapFlowsChildren(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		row := Div().Row().Wrap().Gap(4).Width(Px(100))
		for i := 0; i < 5; i++ {
			row.Child(Div().Keyf("c%d", i).Size(Px(30), Px(20)))
		}
		return Div().Align(Start).Child(row)
	}, 400, 300, 1)
	a, _ := rt.Find("c0")
	d, _ := rt.Find("c3")
	if d.Y != a.Y+24 || d.X != a.X {
		t.Errorf("4th child at %v,%v; want it to start a second line at %v,%v", d.X, d.Y, a.X, a.Y+24)
	}
}
