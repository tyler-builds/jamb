package ui

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type cmdTag struct {
	Name string `json:"name"`
}

type cmdIn struct {
	Title string    `json:"title" desc:"The title"`
	Body  string    `json:"body,omitempty"`
	Tags  []cmdTag  `json:"tags,omitempty"`
	At    time.Time `json:"at,omitzero"`
	Count *int      `json:"count"`
	skip  int
	Gone  string `json:"-"`
}

type cmdOut struct {
	ID    string         `json:"id"`
	Extra map[string]int `json:"extra,omitempty"`
}

func TestCommandSchemas(t *testing.T) {
	c := NewCommand("test.schema", "Makes a thing", func(in cmdIn) (cmdOut, error) { return cmdOut{}, nil })
	in := c.InputSchema()
	b, _ := json.Marshal(in)
	want := `{"$schema":"https://json-schema.org/draft/2020-12/schema","additionalProperties":false,` +
		`"properties":{"at":{"format":"date-time","type":"string"},"body":{"type":"string"},` +
		`"count":{"type":"integer"},"tags":{"items":{"additionalProperties":false,"properties":{"name":{"type":"string"}},` +
		`"required":["name"],"type":"object"},"type":"array"},"title":{"description":"The title","type":"string"}},` +
		`"required":["title"],"type":"object"}`
	if string(b) != want {
		t.Errorf("input schema:\n%s\nwant\n%s", b, want)
	}
	out := c.OutputSchema()
	if props := out["properties"].(map[string]any); !reflect.DeepEqual(props["extra"],
		map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer"}}) {
		t.Errorf("output schema: %v", out)
	}
	if CommandNamed("test.schema") != c || !containsCommand(Commands(), c) {
		t.Error("the command is not registered")
	}
}

func containsCommand(cs []*Command, c *Command) bool {
	for _, x := range cs {
		if x == c {
			return true
		}
	}
	return false
}

func TestCommandRunJSON(t *testing.T) {
	var got cmdIn
	c := NewCommand("test.run", "", func(in cmdIn) (cmdOut, error) {
		got = in
		if in.Title == "fail" {
			return cmdOut{}, errors.New("it failed")
		}
		return cmdOut{ID: "n1"}, nil
	})
	out, err := c.RunJSON([]byte(`{"title":"T","tags":[{"name":"a"}]}`))
	if err != nil || out.(cmdOut).ID != "n1" || got.Title != "T" || got.Tags[0].Name != "a" {
		t.Fatalf("run: %+v %v (input %+v)", out, err, got)
	}
	for args, want := range map[string]string{
		`{}`:                           `"title" is required`,
		``:                             `"title" is required`,
		`{"title":"T","colour":"red"}`: `unknown field "colour"`,
		`{"title":3}`:                  `cannot unmarshal number`,
	} {
		_, err := c.RunJSON([]byte(args))
		var ce *CommandError
		if !errors.As(err, &ce) || !strings.Contains(err.Error(), want) {
			t.Errorf("args %q: %v; want an input error saying %s", args, err, want)
		}
	}
	if _, err := c.RunJSON([]byte(`{"title":"fail"}`)); err == nil || errors.As(err, new(*CommandError)) {
		t.Errorf("a failing command: %v; want its own error", err)
	}

	// A command that takes nothing.
	ping := NewCommand("test.ping", "", func(struct{}) (string, error) { return "pong", nil })
	if out, err := ping.RunJSON(nil); err != nil || out != "pong" {
		t.Errorf("ping: %v %v", out, err)
	}
}

func TestCommandDuplicateName(t *testing.T) {
	NewCommand("test.dup", "", func(struct{}) (int, error) { return 0, nil })
	defer func() {
		if recover() == nil {
			t.Error("a second command with the same name did not panic")
		}
	}()
	NewCommand("test.dup", "", func(struct{}) (int, error) { return 0, nil })
}
