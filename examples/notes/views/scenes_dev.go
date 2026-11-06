//go:build jambdev

package views

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tyler-builds/jamb/examples/notes/store"
	"github.com/tyler-builds/jamb/scene"
)

// The notes app's scenes, for jamb dev, jamb ctl and headless runs (see
// docs/tools.md, "Scenes"). Each opens its own notes file in a new temporary
// directory, so a scene never reads or changes the user's notes. This file is
// built only with the jambdev tag, which jamb dev sets.
func init() {
	scene.Add("Notes/Seeded", func() scene.Root { return Root(sceneStore(nil)) })
	scene.Add("Notes/Empty", func() scene.Root {
		return Root(sceneStore(func(s *store.Store) {
			for _, n := range s.Notes.Peek() {
				s.Delete(n)
			}
		}))
	})
	scene.Add("Notes/Many", func() scene.Root {
		return Root(sceneStore(func(s *store.Store) {
			for i := range 200 {
				n := s.Import(fmt.Sprintf("Synthetic note %03d", i+1),
					fmt.Sprintf("Body of synthetic note %d.\n\n- one\n- two", i+1))
				if i%7 == 0 {
					s.AddTag(n, "work")
				}
				if i%25 == 0 {
					s.TogglePin(n)
				}
			}
		}))
	})
	scene.Add("Notes/Search with no results", func() scene.Root {
		return Root(sceneStore(func(s *store.Store) { s.Query.Set("no note has this text") }))
	})
}

// sceneStore opens a store in a new temporary directory, seeded as on a first
// run, and lets setup change it.
func sceneStore(setup func(*store.Store)) *store.Store {
	dir, err := os.MkdirTemp("", "jamb-notes-scene-")
	if err != nil {
		panic(err)
	}
	s, err := store.Open(filepath.Join(dir, "notes.json"))
	if err != nil {
		panic(err)
	}
	if setup != nil {
		setup(s)
	}
	return s
}
