package scene

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/ui"
)

func reset(t *testing.T) {
	t.Helper()
	mu.Lock()
	old := scenes
	scenes = map[string]func() Root{}
	mu.Unlock()
	t.Cleanup(func() { mu.Lock(); scenes = old; mu.Unlock() })
}

func TestAddAndMake(t *testing.T) {
	reset(t)
	made := 0
	Add("Inbox/Many", func() Root {
		made++
		return func(*ui.Context) ui.Element { return ui.Text("many") }
	})
	Add("Inbox/Empty", func() Root { return func(*ui.Context) ui.Element { return ui.Text("empty") } })
	if got := strings.Join(Names(), ","); got != "Inbox/Empty,Inbox/Many" {
		t.Fatalf("Names() = %s", got)
	}
	for range 2 {
		if root, err := Make("Inbox/Many"); err != nil || root == nil {
			t.Fatalf("Make = %v, %v", root != nil, err)
		}
	}
	if made != 2 {
		t.Errorf("made %d times, want once per Make", made)
	}
	if _, err := Make("Inbox/None"); err == nil || !strings.Contains(err.Error(), "the scenes are Inbox/Empty, Inbox/Many") {
		t.Errorf("Make of a missing scene: %v", err)
	}
}

func TestMakeWithNoScenes(t *testing.T) {
	reset(t)
	if _, err := Make("x"); err == nil || !strings.Contains(err.Error(), "-tags jambdev") {
		t.Errorf("err = %v, want a hint about the build tag", err)
	}
}

func TestMakeNilRoot(t *testing.T) {
	reset(t)
	Add("Nil", func() Root { return nil })
	if _, err := Make("Nil"); err == nil {
		t.Error("a scene that makes no root is not an error")
	}
}

func TestAddPanics(t *testing.T) {
	reset(t)
	root := func() Root { return func(*ui.Context) ui.Element { return nil } }
	Add("Taken", root)
	for _, tc := range []struct {
		name    string
		newRoot func() Root
		want    string
	}{
		{"", root, "needs a name"},
		{" Padded", root, "space"},
		{"/Lead", root, "empty part"},
		{"Trail/", root, "empty part"},
		{"A//B", root, "empty part"},
		{"Taken", root, "registered twice"},
		{"NoFunc", nil, "no function"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil || !strings.Contains(r.(string), tc.want) {
					t.Fatalf("panic = %v, want one containing %q", r, tc.want)
				}
			}()
			Add(tc.name, tc.newRoot)
		})
	}
}
