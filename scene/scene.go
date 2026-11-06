// Package scene names starting states of an app's UI, such as an empty inbox, a
// long list or an error, so that development tools can open the app straight
// into one: `JAMB_SCENE=Inbox/Empty jamb dev`, `jamb ctl scene name=Inbox/Empty`,
// or a test that renders the scene's root.
//
// A scene makes its own state, from synthetic data, and returns the root of
// the window it shows. Register scenes from init functions in files built only
// with the jambdev build tag, which `jamb dev` sets, so that a release build
// compiles none of them, however many there are:
//
//	//go:build jambdev
//
//	package views
//
//	import "github.com/tyler-builds/jamb/scene"
//
//	func init() {
//		scene.Add("Inbox/Empty", func() scene.Root {
//			return Root(store.NewMemory())
//		})
//		scene.Add("Inbox/Many", func() scene.Root {
//			return Root(store.NewMemory(store.Sample(500)...))
//		})
//	}
//
// The scene replaces the root of the app's first window. The window's other
// settings, such as its size, bindings and theme, stay the app's own. See
// docs/tools.md, "Scenes".
package scene

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/tyler-builds/jamb/ui"
)

// Root renders a window, as app.Window.Root does.
type Root = func(ctx *ui.Context) ui.Element

var (
	mu     sync.Mutex
	scenes = map[string]func() Root{}
)

// Add registers a scene under name. Slashes in the name group scenes, as in
// "Settings/Accounts/Signed out". newRoot runs each time the scene is opened, so
// each opening starts from fresh state. Add panics when name is empty, when
// it has a slash at either end or an empty group, or when it is registered
// already.
func Add(name string, newRoot func() Root) {
	if err := checkName(name); err != nil {
		panic("scene.Add: " + err.Error())
	}
	if newRoot == nil {
		panic(fmt.Sprintf("scene.Add: scene %q has no function to make its root", name))
	}
	mu.Lock()
	defer mu.Unlock()
	if _, dup := scenes[name]; dup {
		panic(fmt.Sprintf("scene.Add: scene %q is registered twice", name))
	}
	scenes[name] = newRoot
}

func checkName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("a scene needs a name")
	}
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("scene name %q starts or ends with a space", name)
	}
	for part := range strings.SplitSeq(name, "/") {
		if strings.TrimSpace(part) == "" {
			return fmt.Errorf("scene name %q has an empty part between slashes", name)
		}
	}
	return nil
}

// Names returns the names of the registered scenes, sorted.
func Names() []string {
	mu.Lock()
	defer mu.Unlock()
	names := make([]string, 0, len(scenes))
	for n := range scenes {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// Make opens the scene called name: it runs the scene's make function and
// returns the root it made. The error lists the scenes when there is none of
// that name.
func Make(name string) (Root, error) {
	mu.Lock()
	newRoot, ok := scenes[name]
	mu.Unlock()
	if !ok {
		names := Names()
		if len(names) == 0 {
			return nil, fmt.Errorf("there is no scene %q: no scenes are registered (build with -tags jambdev, as jamb dev does)", name)
		}
		return nil, fmt.Errorf("there is no scene %q; the scenes are %s", name, strings.Join(names, ", "))
	}
	root := newRoot()
	if root == nil {
		return nil, fmt.Errorf("scene %q made no root", name)
	}
	return root, nil
}
