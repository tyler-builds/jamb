//go:build !js

package devtools

// The state that survives jamb dev's restarts. jamb dev builds and runs apps
// for the desktop, so a web build, where every byte is downloaded, leaves the
// package out.
import _ "github.com/tyler-builds/jamb/devtools/hot"
