package main

import (
	"github.com/tyler-builds/jamb/app"

	// The visual inspector: Ctrl+Shift+I opens it, here in a release build too
	// (see App.DevTools).
	_ "github.com/tyler-builds/jamb/devtools"
	"github.com/tyler-builds/jamb/examples/demo/views"
)

func main() {
	store := views.NewStore()

	// The AppID names the app to Windows, for its OS notifications and single
	// instance. A click on a notification reaches the running demo, not a new window.
	application := app.New().
		AppID("JambExamples.Demo").
		SingleInstance(nil).
		DevTools(true).
		OnNotification(views.NotificationClicked(store))
	application.Run(app.Window{
		Title:     "Jamb Demo",
		Width:     1200,
		Height:    760,
		MinWidth:  760,
		MinHeight: 480,
		// The app draws its own title bar (see views.App).
		Chrome: app.ChromeCustom,
		// Closing the main window quits, even with project windows open.
		QuitWhenClosed: true,
		// With the notification area icon on, closing the window hides it (see Settings).
		CloseToTray: store.CloseToTray,
		Root:        views.App(store),
		Theme:       views.DemoDarkTheme,
		// Keyboard shortcuts for the main window (see views.Bindings).
		Bindings: views.Bindings(),
	})
}
