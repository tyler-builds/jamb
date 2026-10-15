package ui

import (
	"slices"
	"testing"
)

// parseStack parses each KeyContext spec, root first.
func parseStack(t *testing.T, specs ...string) []keyCtx {
	t.Helper()
	var out []keyCtx
	for _, s := range specs {
		kc, err := parseKeyCtx(s)
		if err != nil {
			t.Fatalf("parseKeyCtx(%q): %v", s, err)
		}
		out = append(out, kc)
	}
	return out
}

// canon writes a predicate in a fully parenthesized form, to compare parses.
func (p *contextPredicate) canon() string {
	switch p.op {
	case predIdent:
		return p.name
	case predEq:
		return p.name + " == " + p.value
	case predNeq:
		return p.name + " != " + p.value
	case predNot:
		return "!" + p.a.canon()
	case predDescendant:
		return "(" + p.a.canon() + " > " + p.b.canon() + ")"
	case predAnd:
		return "(" + p.a.canon() + " && " + p.b.canon() + ")"
	case predOr:
		return "(" + p.a.canon() + " || " + p.b.canon() + ")"
	}
	return "?"
}

func TestKeyContextParse(t *testing.T) {
	tests := []struct {
		spec string
		want keyCtx
	}{
		{"baz foo=bar", keyCtx{{"baz", ""}, {"foo", "bar"}}},
		{"baz foo = bar", keyCtx{{"baz", ""}, {"foo", "bar"}}},
		{"  baz foo   =   bar baz", keyCtx{{"baz", ""}, {"foo", "bar"}}},
		{" baz foo = bar", keyCtx{{"baz", ""}, {"foo", "bar"}}},
		{"Editor mode=full", keyCtx{{"Editor", ""}, {"mode", "full"}}},
		{"my-ctx_2 x-1=a_b", keyCtx{{"my-ctx_2", ""}, {"x-1", "a_b"}}},
		{"A A=1 B=2 B=3", keyCtx{{"A", ""}, {"B", "2"}}}, // a repeated key keeps its first entry
		{"", nil},
	}
	for _, tc := range tests {
		got, err := parseKeyCtx(tc.spec)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("parseKeyCtx(%q) = %v, %v; want %v", tc.spec, got, err, tc.want)
		}
	}
}

func TestKeyContextPanicsOnBadSpec(t *testing.T) {
	tests := []struct{ spec, want string }{
		{"Editor mode=", `ui.KeyContext("Editor mode="): "mode=" has no value`},
		{"Editor mode =", `ui.KeyContext("Editor mode ="): "mode=" has no value`},
		{"Editor!", `ui.KeyContext("Editor!"): unexpected '!'; entries are identifiers or key=value`},
		{"=full", `ui.KeyContext("=full"): unexpected '='; entries are identifiers or key=value`},
	}
	for _, tc := range tests {
		msg := panicMsg(func() { Div().KeyContext(tc.spec) })
		if msg != tc.want {
			t.Errorf("KeyContext(%q) panic = %q, want %q", tc.spec, msg, tc.want)
		}
	}
	d := Div().KeyContext("Editor mode=full")
	if d.keyContext != "Editor mode=full" || len(d.keyCtx) != 2 {
		t.Errorf("KeyContext kept %q and %v", d.keyContext, d.keyCtx)
	}
}

func TestContextPredicateParse(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"abc12", "abc12"},
		{"_1a", "_1a"},
		{"!abc", "!abc"},
		{" ! ! abc", "!!abc"},
		{"a == b", "a == b"},
		{"c!=d", "c != d"},
		{"a || b", "(a || b)"},
		{"a || !b && c", "(a || (!b && c))"},
		{"a && b || c&&d", "((a && b) || (c && d))"},
		{"a == b && c || d == e && f", "((a == b && c) || (d == e && f))"},
		{"a && b && c && d", "(((a && b) && c) && d)"},
		{"a && (b == c || d != e)", "(a && (b == c || d != e))"},
		{" ( a || b ) ", "(a || b)"},
		{"a || b && c", "(a || (b && c))"},
		{"a > b || c", "(a > (b || c))"}, // > binds loosest, as in GPUI
		{"Workspace > Pane > Editor", "((Workspace > Pane) > Editor)"},
		{"Pane > (Pane > Editor)", "(Pane > (Pane > Editor))"},
		{"!(a && b)", "!(a && b)"},
		{"!editor && workspace", "(!editor && workspace)"},
		{"Pane > !Workspace", "(Pane > !Workspace)"},
		{"Palette > Editor && mode == full", "(Palette > (Editor && mode == full))"},
	}
	for _, tc := range tests {
		p, err := parseContextPredicate(tc.expr)
		if err != nil {
			t.Errorf("parse %q: %v", tc.expr, err)
			continue
		}
		if got := p.canon(); got != tc.want {
			t.Errorf("parse %q = %s, want %s", tc.expr, got, tc.want)
		}
	}
}

func TestContextPredicateParseErrors(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"", "unexpected end of expression"},
		{"Editor &&", "unexpected end of expression"},
		{"a >", "unexpected end of expression"},
		{"c == !d", "operands of == must be identifiers"},
		{"(a == b) == c", "operands of == must be identifiers"},
		{"(a || b", "expected a ')'"},
		{"a)", "unexpected character ')'"},
		{"a b", "unexpected character 'b'"},
		{"a @ b", "unexpected character '@'"},
	}
	for _, tc := range tests {
		msg := panicMsg(func() { BindIn(tc.expr, "enter", NewAction("test.p", "P")) })
		want := `ui.BindIn("` + tc.expr + `"): ` + tc.want
		if msg != want {
			t.Errorf("BindIn(%q) panic = %q, want %q", tc.expr, msg, want)
		}
	}
}

func TestContextPredicateChildOperator(t *testing.T) {
	predicate, _ := parseContextPredicate("parent > child")
	parent := parseStack(t, "parent")[0]
	child := parseStack(t, "child")[0]
	other := parseStack(t, "other")[0]
	eval := func(p *contextPredicate, stack ...keyCtx) bool { return p.eval(stack, stack) }

	if !eval(predicate, parent, child) {
		t.Error("parent > child should match [parent child]")
	}
	if !eval(predicate, parseStack(t, "grandparent")[0], parent, child) {
		t.Error("parent > child should match below a grandparent")
	}
	if eval(predicate, other, child) {
		t.Error("parent > child should not match [other child]")
	}
	if !eval(predicate, parent, other, child) {
		t.Error("parent > child should match [parent other child]")
	}
	if eval(predicate) || eval(predicate, child) || eval(predicate, parent) {
		t.Error("parent > child should not match an empty, child-only or parent-only stack")
	}
	zany, _ := parseContextPredicate("child > child")
	if eval(zany, child) || !eval(zany, child, child) {
		t.Error("child > child should need two child contexts")
	}
}

func TestContextPredicateNotOperator(t *testing.T) {
	notEditor, _ := parseContextPredicate("!editor")
	editor := parseStack(t, "editor")[0]
	workspace := parseStack(t, "workspace")[0]
	parent := parseStack(t, "parent")[0]
	child := parseStack(t, "child")[0]
	eval := func(p *contextPredicate, stack ...keyCtx) bool { return p.eval(stack, stack) }

	if !eval(notEditor, workspace) || eval(notEditor, editor) {
		t.Error("!editor on single contexts")
	}
	if eval(notEditor, editor, workspace) || eval(notEditor, workspace, editor) {
		t.Error("!editor should not match when editor is anywhere on the stack")
	}
	complexNot, _ := parseContextPredicate("!editor && workspace")
	if !eval(complexNot, workspace) || eval(complexNot, editor, workspace) {
		t.Error("!editor && workspace")
	}
	notMode, _ := parseContextPredicate("!(mode == full)")
	if eval(notMode, parseStack(t, "mode=full")[0]) || !eval(notMode, parseStack(t, "mode=partial")[0]) {
		t.Error("!(mode == full)")
	}
	notDesc, _ := parseContextPredicate("!(parent > child)")
	if !eval(notDesc, parent) || !eval(notDesc, child) || eval(notDesc, parent, child) {
		t.Error("!(parent > child)")
	}
	notChild, _ := parseContextPredicate("parent > !child")
	if eval(notChild, parent) || eval(notChild, child) || eval(notChild, parent, child) {
		t.Error("parent > !child")
	}
	doubleNot, _ := parseContextPredicate("!!editor")
	if !eval(doubleNot, editor) || eval(doubleNot, workspace) {
		t.Error("!!editor")
	}

	// Workspace > Pane > Editor, and GPUI's cases for descendants.
	wpe := parseStack(t, "Workspace", "Pane", "Editor")
	pane, editorCtx := parseStack(t, "Pane")[0], parseStack(t, "Editor")[0]
	ws := parseStack(t, "Workspace")[0]
	paneOverPaneEditor, _ := parseContextPredicate("Pane > (Pane > Editor)")
	if eval(paneOverPaneEditor, wpe...) {
		t.Error("Pane > (Pane > Editor) should not match Workspace > Pane > Editor")
	}
	if p, _ := parseContextPredicate("Workspace > Pane > Editor"); !eval(p, wpe...) {
		t.Error("Workspace > Pane > Editor should match")
	}
	pairThenEditor, _ := parseContextPredicate("(Pane > Pane) > Editor")
	if eval(pairThenEditor, wpe...) {
		t.Error("(Pane > Pane) > Editor should not match")
	}
	paneNotWorkspace, _ := parseContextPredicate("Pane > !Workspace")
	if !eval(paneNotWorkspace, pane, editorCtx) || eval(paneNotWorkspace, pane, ws) {
		t.Error("Pane > !Workspace")
	}
	notWorkspace, _ := parseContextPredicate("!Workspace")
	if eval(notWorkspace, ws) || !eval(notWorkspace, pane) || !eval(notWorkspace, editorCtx) || eval(notWorkspace, wpe...) {
		t.Error("!Workspace")
	}
}

// The predicates here are checked against the stack from docs/input.md's
// example: a Workspace, a Pane and an editor in full mode.
func TestContextDepth(t *testing.T) {
	stack := parseStack(t, "Workspace", "Pane", "Editor mode=full")
	tests := []struct {
		expr  string
		depth int
		ok    bool
	}{
		{"Editor", 3, true},
		{"Pane", 2, true},
		{"Workspace", 1, true},
		{"Workspace > Editor", 3, true},
		{"Pane > Editor && mode == full", 3, true},
		{"Editor && mode == single_line", 0, false},
		{"!Terminal", 3, true},
		{"!Pane", 0, false},
		{"mode != single_line", 3, true},
		{"mode == single_line", 0, false},
		{"Terminal", 0, false},
	}
	for _, tc := range tests {
		p, err := parseContextPredicate(tc.expr)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.expr, err)
		}
		depth, ok := p.depthOf(stack)
		if ok != tc.ok || (ok && depth != tc.depth) {
			t.Errorf("depth_of(%q) = %d, %v; want %d, %v", tc.expr, depth, ok, tc.depth, tc.ok)
		}
	}
	// Without any contexts, nothing matches, not even !Terminal (GPUI's depth_of).
	p, _ := parseContextPredicate("!Terminal")
	if _, ok := p.depthOf(nil); ok {
		t.Error("!Terminal matched an empty stack")
	}
}

func TestContextStackFromPath(t *testing.T) {
	inner := Div().KeyContext("Inner")
	root := &node{div: Div().KeyContext("Outer")}
	mid := &node{div: Div()}
	leaf := &node{div: inner}
	// path is focused first, root last, as focusPath returns it.
	stack := contextStack([]*node{leaf, mid, root})
	if len(stack) != 2 || !stack[0].contains("Outer") || !stack[1].contains("Inner") {
		t.Errorf("contextStack = %v, want [Outer Inner] root first", stack)
	}
}
