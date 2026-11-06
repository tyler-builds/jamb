package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// devtoolsLinkName is the name of the file that `jamb dev` adds to the app's main
// package. It is never written into the app's directory: go build reads it from
// the overlay that devtoolsOverlay writes, so the app's source is not changed.
const devtoolsLinkName = "jamb_devtools_link.go"

// devtoolsImport is the package that links the visual inspector into an app.
const devtoolsImport = "github.com/tyler-builds/jamb/devtools"

// devtoolsLinkSource links the visual inspector into the app, as a blank import
// of its devtools package, so the app does not have to import it itself.
const devtoolsLinkSource = "package main\n\nimport _ \"" + devtoolsImport + "\"\n"

// devtoolsOverlay writes the devtools link into work and returns the path of the
// overlay file for go build -overlay. The overlay adds the link to the package in
// dir, which is the app's main package.
func devtoolsOverlay(work, dir string) (string, error) {
	src := filepath.Join(work, devtoolsLinkName)
	if err := os.WriteFile(src, []byte(devtoolsLinkSource), 0o644); err != nil {
		return "", err
	}
	// The overlay format is {"Replace": {"<path>": "<replacement>"}}; a path that
	// is not on disk is added to its package.
	b, err := json.Marshal(map[string]map[string]string{
		"Replace": {filepath.Join(dir, devtoolsLinkName): src},
	})
	if err != nil {
		return "", err
	}
	overlay := filepath.Join(work, "overlay.json")
	if err := os.WriteFile(overlay, b, 0o644); err != nil {
		return "", err
	}
	return overlay, nil
}

// devTag is the build tag that jamb dev sets, so that files an app builds only
// for development, such as its scenes (see package scene), are compiled in.
const devTag = "jambdev"

// devTags returns the -tags value for jamb dev's builds: devTag and the tags
// that goflags, the GOFLAGS environment variable, sets, since a -tags flag on
// the command line replaces the one in GOFLAGS.
func devTags(goflags string) string {
	tags := []string{devTag}
	for _, f := range strings.Fields(goflags) {
		v, ok := strings.CutPrefix(f, "-tags=")
		if !ok {
			v, ok = strings.CutPrefix(f, "--tags=")
		}
		if !ok {
			continue
		}
		for _, t := range strings.Split(v, ",") {
			if t = strings.TrimSpace(t); t != "" && !slices.Contains(tags, t) {
				tags = append(tags, t)
			}
		}
	}
	return strings.Join(tags, ",")
}
