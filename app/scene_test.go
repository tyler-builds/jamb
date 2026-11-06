package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/ui"
)

// fakeScenes registers two scenes, Inbox/Empty and Inbox/Full, for the test,
// and counts how often each is made.
func fakeScenes(t *testing.T) map[string]int {
	t.Helper()
	made := map[string]int{}
	names := []string{"Inbox/Empty", "Inbox/Full"}
	oldNames, oldMake := sceneNames, sceneMake
	t.Cleanup(func() { sceneNames, sceneMake = oldNames, oldMake; application.scene = nil })
	RegisterScenes(func() []string { return names }, func(name string) (func(*ui.Context) ui.Element, error) {
		if !slices.Contains(names, name) {
			return nil, fmt.Errorf("there is no scene %q", name)
		}
		made[name]++
		n := made[name]
		return func(*ui.Context) ui.Element { return ui.Text(fmt.Sprintf("%s #%d", name, n)).Key("shown") }, nil
	})
	return made
}

func ownRoot(*ui.Context) ui.Element { return ui.Text("own").Key("shown") }

func shownText(t *testing.T, f *ctlFixture) string {
	t.Helper()
	f.frame()
	var b strings.Builder
	f.rt.Dump(&b, 0)
	for line := range strings.SplitSeq(b.String(), "\n") {
		if strings.Contains(line, "shown") {
			return line
		}
	}
	t.Fatalf("no element keyed shown in\n%s", b.String())
	return ""
}

func TestWithSceneWithoutScenes(t *testing.T) {
	oldNames, oldMake := sceneNames, sceneMake
	t.Cleanup(func() { sceneNames, sceneMake = oldNames, oldMake })
	sceneNames, sceneMake = nil, nil

	t.Setenv("JAMB_SCENE", "")
	root, sw, err := withScene(ownRoot)
	if err != nil || sw != nil || root == nil {
		t.Fatalf("withScene = %v, %v, %v; want the app's root, no switch", root != nil, sw, err)
	}
	t.Setenv("JAMB_SCENE", "Inbox/Empty")
	if _, _, err := withScene(ownRoot); err == nil || !strings.Contains(err.Error(), "has no scenes") {
		t.Fatalf("err = %v, want one saying the program has no scenes", err)
	}
	RegisterScenes(func() []string { return nil }, func(string) (func(*ui.Context) ui.Element, error) { return nil, nil })
	if _, _, err := withScene(ownRoot); err == nil {
		t.Fatal("JAMB_SCENE with no scenes registered is not an error")
	}
}

func TestWithSceneUnknown(t *testing.T) {
	fakeScenes(t)
	t.Setenv("JAMB_SCENE", "Inbox/Nope")
	if _, _, err := withScene(ownRoot); err == nil || !strings.Contains(err.Error(), `JAMB_SCENE: there is no scene "Inbox/Nope"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestScenesFromEnvAndControl(t *testing.T) {
	made := fakeScenes(t)
	t.Setenv("JAMB_SCENE", "Inbox/Empty")
	root, sw, err := withScene(ownRoot)
	if err != nil {
		t.Fatal(err)
	}
	application.scene = sw
	f := newCtlFixture(t, root)
	if got := shownText(t, f); !strings.Contains(got, "Inbox/Empty #1") {
		t.Fatalf("JAMB_SCENE opened %q", got)
	}

	got := f.must(ctlRequest{Op: "scenes"}).(map[string]any)
	if !slices.Equal(got["scenes"].([]string), []string{"Inbox/Empty", "Inbox/Full"}) || got["current"] != "Inbox/Empty" {
		t.Fatalf("scenes: %v", got)
	}

	f.must(ctlRequest{Op: "scene", Name: "Inbox/Full"})
	if got := shownText(t, f); !strings.Contains(got, "Inbox/Full #1") {
		t.Fatalf("scene Inbox/Full shows %q", got)
	}
	// Opening a scene again starts it from fresh state.
	f.must(ctlRequest{Op: "scene", Name: "Inbox/Empty"})
	if got := shownText(t, f); !strings.Contains(got, "Inbox/Empty #2") || made["Inbox/Empty"] != 2 {
		t.Fatalf("reopened Inbox/Empty shows %q, made %v", got, made)
	}
	// No name goes back to the app's own root.
	if got := f.must(ctlRequest{Op: "scene"}).(map[string]any); got["current"] != "" {
		t.Fatalf("scene with no name: %v", got)
	}
	if got := shownText(t, f); !strings.Contains(got, "own") {
		t.Fatalf("scene with no name shows %q", got)
	}
	f.fails(ctlRequest{Op: "scene", Name: "Inbox/Nope"}, "not_found")
}

func TestSceneOpsWithoutScenes(t *testing.T) {
	application.scene = nil
	f := newCtlFixture(t, ownRoot)
	got := f.must(ctlRequest{Op: "scenes"}).(map[string]any)
	if len(got["scenes"].([]string)) != 0 || got["current"] != "" {
		t.Fatalf("scenes: %v", got)
	}
	f.fails(ctlRequest{Op: "scene", Name: "Inbox/Empty"}, "not_found")
}
