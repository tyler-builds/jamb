package ui

import "testing"

// OpenURL opens http, https and mailto URLs through the platform, refuses every
// other scheme, and opens nothing headless.
func TestOpenURL(t *testing.T) {
	var ctx *Context
	rt := newTestRuntime(func(c *Context) Element { ctx = c; return Div() }, 200, 100, 1)
	frame(rt)
	if err := ctx.OpenURL("https://example.com/docs"); err != nil {
		t.Fatalf("headless: %v", err)
	}
	var opened []string
	rt.services.OpenURL = func(u string) error { opened = append(opened, u); return nil }
	for _, ok := range []string{"https://example.com/docs?q=1", "http://localhost:8080/", "mailto:team@example.com", "HTTPS://EXAMPLE.COM"} {
		if err := ctx.OpenURL(ok); err != nil {
			t.Errorf("%q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "notes.txt", `C:\Windows\System32\calc.exe`, "file:///C:/Windows/notepad.exe",
		"javascript:alert(1)", "ms-settings:display", "https://", "mailto:"} {
		if err := ctx.OpenURL(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
	if len(opened) != 4 {
		t.Fatalf("opened %v, want the four accepted URLs", opened)
	}
}
