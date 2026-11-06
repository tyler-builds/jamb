package app

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/tyler-builds/jamb/ui"
)

// sceneNames and sceneMake reach package scene. They are nil until
// RegisterScenes, so a program that does not link the development tools
// carries no scene code.
var (
	sceneNames func() []string
	sceneMake  func(name string) (func(*ui.Context) ui.Element, error)
)

// RegisterScenes connects the app to the scenes that package scene holds, for
// JAMB_SCENE and the control channel's scene ops (see docs/tools.md, "Scenes").
// Package devtools calls it; an app does not need to.
func RegisterScenes(names func() []string, open func(name string) (func(*ui.Context) ui.Element, error)) {
	sceneNames, sceneMake = names, open
}

// sceneShown is the scene the first window shows: name is "" for the app's own
// root. gen tells apart two openings of the same scene, so that each starts
// from fresh state.
type sceneShown struct {
	name string
	root func(*ui.Context) ui.Element
	gen  int
}

// sceneSwitch lets the control channel change the scene of the first window.
type sceneSwitch struct {
	own   func(*ui.Context) ui.Element // the app's own root
	shown *ui.Signal[sceneShown]
}

// withScene returns the root the first window opens with: the scene that
// JAMB_SCENE names, or root. When the program has scenes it wraps the root, so
// that the control channel can open another scene later; the switch is nil
// when it has none.
func withScene(root func(*ui.Context) ui.Element) (func(*ui.Context) ui.Element, *sceneSwitch, error) {
	name := os.Getenv("JAMB_SCENE")
	if sceneMake == nil || len(sceneNames()) == 0 {
		if name != "" {
			return nil, nil, fmt.Errorf("JAMB_SCENE=%q, but this program has no scenes: register them with scene.Add "+
				"in files built with -tags jambdev, and run the app with jamb dev, which sets the tag and links package devtools", name)
		}
		return root, nil, nil
	}
	s := &sceneSwitch{own: root, shown: ui.NewSignal(sceneShown{root: root})}
	if name != "" {
		if err := s.open(name); err != nil {
			return nil, nil, fmt.Errorf("JAMB_SCENE: %w", err)
		}
	}
	return s.render, s, nil
}

// render is the first window's root while the program has scenes.
func (s *sceneSwitch) render(ctx *ui.Context) ui.Element {
	cur := s.shown.Get()
	return ui.Component(cur.root).Key("jamb-scene:" + strconv.Itoa(cur.gen) + ":" + cur.name)
}

// open shows the scene called name, from fresh state, or the app's own root
// again when name is "".
func (s *sceneSwitch) open(name string) error {
	root := s.own
	if name != "" {
		r, err := sceneMake(name)
		if err != nil {
			return err
		}
		root = r
	}
	gen := s.shown.Peek().gen + 1
	s.shown.Set(sceneShown{name: name, root: root, gen: gen})
	return nil
}

// current returns the name of the scene shown, "" for the app's own root.
func (s *sceneSwitch) current() string { return s.shown.Peek().name }

// errNoScenes answers the scene ops of a program without scenes.
var errNoScenes = errors.New("this app has no scenes: register them with scene.Add in files built with -tags jambdev " +
	"(see docs/tools.md, \"Scenes\")")
