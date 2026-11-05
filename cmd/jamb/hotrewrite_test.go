package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const hotSample = `package views

import (
	jui "github.com/tyler-builds/jamb/ui"
)

var theme = jui.NewSignal("dark")

type Form struct{ Name, Email *jui.Signal[string] }

func NewForm() *Form {
	return &Form{Name: jui.NewSignal(""), Email: jui.NewSignal[string]("")}
}

func (f *Form) Root(ctx *jui.Context) jui.Element {
	clicks := jui.NewSignal(0)
	clicks = jui.NewSignal(1)
	use(jui.NewSignal(true), jui.NewSignal(2))
	return jui.Div().Children(jui.Component(f.Root).Key("a"), jui.Component(f.Root))
}

func use(...any) {}
`

func TestRewriteFile(t *testing.T) {
	got, changed, err := rewriteFile([]byte(hotSample), "views")
	if err != nil || !changed {
		t.Fatalf("rewrite: %v %v", changed, err)
	}
	out := string(got)
	for _, want := range []string{
		`package views; import jambhot__ "github.com/tyler-builds/jamb/devtools/hot"`,
		`var theme = jambhot__.Signal(jui.NewSignal("dark"), "views:theme")`,
		`Name: jambhot__.Signal(jui.NewSignal(""), "views:NewForm/Form.Name")`,
		`Email: jambhot__.Signal(jui.NewSignal[string](""), "views:NewForm/Form.Email")`,
		`clicks := jambhot__.Signal(jui.NewSignal(0), "views:Form.Root/clicks")`,
		`clicks = jambhot__.Signal(jui.NewSignal(1), "views:Form.Root/clicks~2")`,
		`use(jambhot__.Signal(jui.NewSignal(true), "views:Form.Root/#1"), jambhot__.Signal(jui.NewSignal(2), "views:Form.Root/#2"))`,
		`jambhot__.Component(jui.Component(f.Root), "views:Form.Root/c1").Key("a"), jambhot__.Component(jui.Component(f.Root), "views:Form.Root/c2")`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s\nin\n%s", want, out)
		}
	}
	// Every line stays where it was, and the copy still parses.
	if bytes.Count(got, []byte("\n")) != strings.Count(hotSample, "\n") {
		t.Error("the rewrite moved lines")
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "", got, 0); err != nil {
		t.Errorf("the copy does not parse: %v", err)
	}

	// A file without ui, or with a blank or dot import of it, is left alone.
	for _, src := range []string{
		"package x\n\nfunc f() {}\n",
		"package x\n\nimport _ \"github.com/tyler-builds/jamb/ui\"\n",
		"package x\n\nimport . \"github.com/tyler-builds/jamb/ui\"\n\nvar s = NewSignal(1)\n",
	} {
		if _, changed, _ := rewriteFile([]byte(src), "x"); changed {
			t.Errorf("rewrote %q", src)
		}
	}
}
