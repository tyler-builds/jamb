package ui

import "testing"

// A runtime with a theme pair follows the system's app mode; SetTheme pins one
// theme; high contrast is built from the theme in use.
func TestRuntimeFollowsTheSystemAppMode(t *testing.T) {
	light, dark := LightTheme(), DarkTheme()
	rt := newTestRuntime(func(ctx *Context) Element { return Div() }, 200, 100, 1)
	rt.SetThemes(light, dark)
	rt.SetSystemSettings(SystemSettings{Dark: false})
	if rt.Theme() != light {
		t.Fatal("light app mode: the runtime does not use the light theme")
	}
	rt.SetSystemSettings(SystemSettings{Dark: true})
	if rt.Theme() != dark || rt.AppTheme() != dark {
		t.Fatal("dark app mode: the runtime does not use the dark theme")
	}
	rt.SetSystemSettings(SystemSettings{Dark: true, HighContrast: true, Colors: HighContrastWhite()})
	if rt.Theme().Name != "high contrast" || rt.AppTheme() != dark {
		t.Fatalf("high contrast: theme %q, app theme dark %v", rt.Theme().Name, rt.AppTheme() == dark)
	}
	pinned := LightTheme()
	rt.SetTheme(pinned)
	rt.SetSystemSettings(SystemSettings{Dark: true})
	if rt.Theme() != pinned {
		t.Fatal("after SetTheme the runtime still follows the system")
	}
}

// SystemDark reports the system's app mode to components.
func TestContextSystemDark(t *testing.T) {
	var seen bool
	rt := newTestRuntime(func(ctx *Context) Element { seen = ctx.SystemDark(); return Div() }, 200, 100, 1)
	rt.SetSystemSettings(SystemSettings{Dark: true})
	frame(rt)
	if !seen {
		t.Fatal("SystemDark is false in dark app mode")
	}
}
