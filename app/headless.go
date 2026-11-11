package app

import (
	"fmt"
	"os"
	"strconv"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/platform/headless"
	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/render/soft"
)

// headlessOn runs the app with no window on the screen: JAMB_HEADLESS=1. Its
// windows exist in memory, the CPU renderer draws them, and the platform's
// settings are the defaults, so a run looks the same on every machine (see
// platform/headless and docs/tools.md, "Headless runs").
var headlessOn = os.Getenv("JAMB_HEADLESS") == "1"

func init() {
	if !headlessOn {
		return
	}
	if s := os.Getenv("JAMB_HEADLESS_SCALE"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil || v < 0.5 || v > 4 {
			fmt.Fprintf(os.Stderr, "jamb: JAMB_HEADLESS_SCALE=%q is not a scale from 0.5 to 4; using 1\n", s)
		} else {
			headless.Scale = float32(v)
		}
	}
}

// The platform functions the app calls: the headless platform's in a headless
// run, the operating system's (os...) otherwise.

func runPlatform(start func() error) error {
	if headlessOn {
		return headless.Run(start)
	}
	return osRunPlatform(start)
}

func openPlatform(opts platform.WindowOptions, h platform.Handler) (platform.Window, error) {
	if headlessOn {
		return headless.Open(opts, h)
	}
	return osOpenPlatform(opts, h)
}

func postUI(fn func()) {
	if headlessOn {
		headless.Post(fn)
		return
	}
	osPostUI(fn)
}

func onUIThread() bool {
	if headlessOn {
		return headless.OnUIThread()
	}
	return osOnUIThread()
}

func quitUI() {
	if headlessOn {
		headless.Quit()
		return
	}
	osQuitUI()
}

func releaseWindowSession() {
	if !headlessOn {
		osReleaseWindowSession()
	}
}

func prepareDevice() {
	if !headlessOn {
		osPrepareDevice()
	}
}

// headlessTarget is a headless window: the CPU renderer draws into it.
type headlessTarget interface {
	Target() soft.Target
}

func newRenderer(w platform.Window, width, height int) (render.Renderer, error) {
	if headlessOn {
		t, ok := w.(headlessTarget)
		if !ok {
			return nil, fmt.Errorf("app: headless window %T has no target for the CPU renderer", w)
		}
		return soft.New(t.Target()), nil
	}
	return osNewRenderer(w, width, height)
}

func releaseDevice() {
	if !headlessOn {
		osReleaseDevice()
	}
}

func rebuildDevice() error {
	if headlessOn {
		return nil
	}
	return osRebuildDevice()
}

func gpuAdapter() string {
	if headlessOn {
		return "CPU (headless)"
	}
	return osGpuAdapter()
}

// settingsPrefetch delivers the desktop's settings that prefetchSettings
// started reading, until the first window takes them.
var settingsPrefetch chan platform.Settings

// prefetchSettings starts reading the desktop's settings on another goroutine:
// on Linux they come from the settings portal over D-Bus, which takes long
// enough to overlap with creating the first window.
func prefetchSettings() {
	if headlessOn {
		return
	}
	c := make(chan platform.Settings, 1)
	settingsPrefetch = c
	go func() { c <- osReadPlatformSettings() }()
}

// settingsCache holds the desktop's settings once read, until they change.
var settingsCache *platform.Settings

// readPlatformSettings returns the desktop's settings, reading them only the
// first time and after forgetPlatformSettings.
func readPlatformSettings() platform.Settings {
	if headlessOn {
		return platform.DefaultSettings()
	}
	if settingsCache != nil {
		return *settingsCache
	}
	var s platform.Settings
	if c := settingsPrefetch; c != nil {
		settingsPrefetch = nil
		s = <-c
	} else {
		s = osReadPlatformSettings()
	}
	settingsCache = &s
	return s
}

// forgetPlatformSettings makes the next readPlatformSettings read them again,
// when the desktop reports that one changed.
func forgetPlatformSettings() { settingsCache = nil }

func clockNow() int64 {
	if headlessOn {
		return headless.Clock()
	}
	return osClockNow()
}

// headlessNotify reports a notification on stderr instead of showing it.
func headlessNotify(n Notification) error {
	if err := n.check(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "jamb: headless: notification %q: %q\n", n.Title, n.Body)
	return nil
}
