package ui

import (
	"strings"
	"testing"
	"time"
)

// toastWarnings renders root in a runtime that records its warnings, and shows two
// toasts. It returns the warnings.
func toastWarnings(root func(*Context) Element) []string {
	var warnings []string
	rt := NewRuntime(root, DarkTheme(), nil)
	rt.SetWarnFunc(func(s string) { warnings = append(warnings, s) })
	rt.SetViewport(400, 300, 1)
	rt.Frame(time.Now())
	rt.toast("", ToastInfo, "Saved")
	rt.toast("", ToastInfo, "Saved again")
	return warnings
}

// A toast in a window with no Toaster is not shown, so the runtime says so once,
// and says how to add one.
func TestToastWithoutToasterWarnsOnce(t *testing.T) {
	warnings := toastWarnings(func(ctx *Context) Element {
		return Div().Child(Text("No toaster here"))
	})
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %q", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "ui.Toaster()") {
		t.Errorf("warning %q does not say to add ui.Toaster()", warnings[0])
	}
}

// With a Toaster in the window, a toast is shown and nothing is reported.
func TestToastWithToasterDoesNotWarn(t *testing.T) {
	warnings := toastWarnings(func(ctx *Context) Element {
		return Div().Children(Text("With a toaster"), Toaster())
	})
	if len(warnings) != 0 {
		t.Errorf("got warnings %q, want none", warnings)
	}
}
