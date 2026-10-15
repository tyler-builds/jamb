// Package uihost connects the app package to the methods of ui.Runtime that
// only a window host calls: the wake-up function, platform services, the
// cursor, window focus and the window areas of a custom title bar. ui/uitest
// uses it the same way for a headless runtime's clock and clipboard. These
// methods are not part of ui's public API, so apps and tests never see them.
package uihost

// Of returns the host side of a *ui.Runtime. The ui package sets it when it is
// initialized; app and ui/uitest assert the result to the interface of the
// methods they use.
var Of func(rt any) any
