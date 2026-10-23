# OS integration

Jump lists and single instance, the notification area, OS notifications, opening URLs, and file icons.

## Opening URLs

`ctx.OpenURL(url)` opens an `http`, `https` or `mailto` URL with the system, such as
a help page in the default browser, for a `ui.Link` or a button. Other schemes and
file paths are refused with an error, because opening them could run a program, so
a URL built from text the app does not control stays safe. A headless runtime opens
nothing.

## File icons

`app.FileIcon(path, size)` returns the icon Windows shows for a file, as File Explorer
and Task Manager show it, as an `image.Image` of `size` pixels square:

```go
img, err := app.FileIcon(`C:\Windows\explorer.exe`, 32)
if err == nil {
	icon := ui.Image(ui.ImageFromImage(img)).Size(ui.Px(16), ui.Px(16))
}
```

- **Which icon.** A file that holds icons (an executable, a library or an `.ico` file)
  gives its first icon, at the size asked for, so it is sharp. Any other file gives the
  icon of its type, such as a text file's, at the shell's small or large size and scaled
  to `size`. An executable without an icon of its own gives Windows' generic program icon.
- **Size.** The size is in pixels of the display. For an icon drawn 16 logical pixels
  square, ask for 16 times the display scale, or a larger size that the image scales
  down from.
- **Cost.** It reads the file. Call it off the UI thread for many files, such as a list
  of processes, and keep the images by path. It is safe from any goroutine: the shell
  needs COM, which it starts on the calling thread for the call.
- **Errors.** A path that does not exist, a file Windows has no icon for, and a size of
  zero give an error. It is a Windows feature, and gives an error on other platforms.

## Jump lists and single instance

Windows only; the framework has no window adapter for other platforms yet, so
these settings have no effect there. The settings are set on the `App`, before `Run`:

```go
app.New().
    AppID("JambExamples.Notes").
    JumpList(app.JumpList{
        Tasks:  []app.JumpTask{{Title: "New note", Args: "--new-note"}},
        Recent: true,
    }).
    SingleInstance(func(args []string, dir string) {
        // A second launch of the app handed over args, such as "--new-note".
    }).
    Run(app.Window{...})

// Where the app opens or saves a file:
app.AddRecentDocument(path)
```

- **`AppID`** is the Application User Model ID. Windows uses it to group the
  app's windows on the taskbar, and to key its jump list, its Recent documents
  and the name of its single instance. Use a stable identifier in the form
  `CompanyName.ProductName`, such as `JambExamples.Notes`. It may contain letters,
  digits, `.`, `-` and `_`, must start with a letter or digit, and is at most 128
  characters. The app sets it at startup, before its first window. It is required
  with `JumpList` or `SingleInstance`.
- **`JumpList`** lists the app's own tasks in its taskbar menu, and the Recent
  category when `Recent` is set. The framework adds no tasks. A task has a
  `Title`, which is required, and `Args`, the command line after the executable,
  passed as one string, so quote an argument with spaces as a shell would. Each
  task starts the app's executable, so test tasks with a built executable (see
  [Jump list and packaging](#jump-list-and-packaging)). Build it with
  `-ldflags "-H windowsgui"`, as `jamb package` does: an executable built without
  it, such as the one `go run` makes, is a console program, and Windows opens a
  console window for each launch before the program runs, so a task shows a console
  for a moment.
- **`AddRecentDocument`** adds a file to the Recent category. Call it for each file
  the app opens or saves. A relative path is made absolute first.
- **`SingleInstance`** makes the app one process per user and session. A second
  launch hands its command line to the running app and exits with status 0,
  without a window. The running app calls `onArgs` on its UI thread with the
  arguments after the program name, and the second launch's working directory,
  so `filepath.Join(dir, arg)` resolves a relative path. Its first window is
  brought to the front first. A `JumpList` task is an ordinary launch, and it
  reaches the running app only when `SingleInstance` is set. If the hand-off fails,
  the second launch runs on its own, as a separate process.

`SingleInstance` and `JumpList` are independent. Either one may be set without the
other, and a jump list never turns single instance on.

The notes example uses all four: its "New note" task makes a note in the running
window, and files it imports are added to the Recent documents.

### Separate processes or one process

Without `SingleInstance`, a task or a second launch starts a new process. With it,
the running process receives the command line. The choice changes what the app is
built from:

| | Separate processes (default) | One process (`SingleInstance`) |
|---|---|---|
| A jump list task | Starts a process with its own windows, runtime and state. | Runs in the running app, in `onArgs`. |
| More windows | Each process has its own. The processes do not know about each other. | Opened with `app.Open` in the one process. |
| Shared state | Files, a database or other IPC must carry it between processes. | Ordinary variables and signals. |
| A crash | Ends one process; the others keep running. | Ends every window. |
| Quit | Each process quits when its last window closes. | The process quits when its last window closes. |
| Cost of a launch | A full startup. | A hand-off of a few milliseconds, or a new process if the hand-off fails. |

Use `SingleInstance` when the app is one document or one workspace, and a second
launch should act in it. Leave it off when each launch is independent.

### How the hand-off works

The first launch creates a named mutex, `Local\jamb-<AppID>`, local to the session.
A later launch that finds the mutex taken connects to a named pipe,
`\\.\pipe\jamb-<AppID>-<digest>`, and writes its command line. The digest covers
the user's SID and the session, so two users, or two sessions of one user, never
share a pipe. The pipe grants access only to that user, so another user cannot
send arguments to the app. The request is a four-byte length and a JSON object with
the protocol version, the working directory and the arguments; the answer is one
byte. A launch that cannot reach the running app within a few seconds, or that the
app refuses, runs on its own. The mutex is released when the process exits, so a
crashed app does not block later launches.

### Jump list and packaging

- **`go run`.** `go run` builds the program in a temporary folder, so a task's
  executable disappears when the run ends, and the task fails. Test jump lists and
  single instance with a built executable (`go build`), or with an installed
  build. Two `go run` runs with the same `AppID` also share one instance.
- **Zip and installer.** The app sets its `AppID` at startup, which is enough for
  the taskbar and the jump list. Windows groups the taskbar button with a Start menu
  entry only when that shortcut carries the `AppID`. NSIS's `CreateShortcut` cannot
  set the `System.AppUserModel.ID` property, so the `jamb package` installer runs the
  installed program once with an internal flag, which sets the ID on the installer's
  shortcut and exits (see [Installer](packaging.md#installer)). A zip install has no such
  shortcut; the app sets the ID on its own shortcut at its first `Notify`.
- **MSIX.** A packaged app has an identity from its package, and Windows keys the
  jump list and Recent documents by the package's ID, `<PackageFamilyName>!App`.
  `AppID` is not used for those in a packaged process, so the jump list works
  only under the package ID. `AppID` still names the single instance.
- Use `identityName` from `jamb.json` as the `AppID`. The installer, the MSIX
  manifest and the app then agree on the shortcut's ID and on the notification
  protocol, which is `jamb-` and the `AppID` in lower case. `jamb package` checks
  that the two are equal (see [AppID check](packaging.md#appid-check)).

Windows may show only the first few tasks of a jump list, and it keeps the Recent
category's list itself.

## Notification area, taskbar progress and badges

Windows only. The tray icon and the taskbar calls are Windows features, and the
framework has no window adapter for other platforms yet.

### The notification area icon

A tray icon is made with `app.NewTray` and added with `Show`. Its methods are safe
from any goroutine: they run on the UI thread in call order, so a call made before
`Run` waits for the app to start. The icon needs no window; the first `Show` makes
a hidden window that receives the icon's messages.

```go
var win ui.Window // the window, from app.Open or ctx.Window()

show := win.Show
tray := app.NewTray(app.TrayOptions{
    Icon:          notesIcon, // an image.Image, such as a PNG the app embeds; nil is the executable's icon
    Tooltip:       "Notes",
    OnClick:       show,
    OnDoubleClick: show,
    Menu: func() []app.TrayItem {
        return []app.TrayItem{
            {Label: "Show window", OnClick: show},
            {Label: "Autosave", Checked: autosave.Get(), OnClick: toggleAutosave},
            app.TraySeparator(),
            {Label: "Quit", OnClick: func() { win.Close() }},
        }
    },
})
tray.Show()

tray.SetTooltip("2 notes unsaved")
tray.SetIcon(otherImage) // nil returns to the executable's icon
tray.Hide()              // Show adds the icon again
```

- **Icon.** The image is drawn at the small icon size of the DPI of the tray
  window, and drawn again when the DPI changes. Nil uses the executable's own icon,
  which `jamb package` embeds as resource 1, or the standard application icon when
  the executable has none.
- **Menu.** A native popup menu, so it follows the system theme and assistive
  technology reads it as any system menu. `Menu` runs on the UI thread each time the
  menu opens, so checked items show the current state. A label is literal text: `&`
  is not an access key. `Disabled` shows an item that cannot be chosen, `Checked`
  shows a check mark, and an item with a `Submenu` opens it instead of running
  `OnClick`.
- **Mouse and keyboard.** A click runs `OnClick`. A double click runs `OnClick` for
  its first click, as Windows reports both clicks, and then `OnDoubleClick`. An app
  that wants different actions for the two should make `OnClick` idempotent, or rely
  on `OnDoubleClick` alone and leave `OnClick` nil. A right click, the context menu
  key and Shift+F10 open the menu. Enter and Space, on the focused icon, run
  `OnClick`.
- **Identity.** With `AppID`, Windows is given a GUID, a version 8 UUID made with
  128-bit FNV-1a from a fixed jamb namespace, the `AppID` and the icon's place in the order of
  `NewTray` calls. Windows keeps the user's choice to show or hide the icon by that
  GUID, so the choice survives restarts. Keep the order of `NewTray` calls the same
  from run to run. Windows also ties the GUID to the executable's path: an executable
  that moved is added by its ID instead, and the user's choice does not carry over.
  Without `AppID`, the icon is identified by its ID alone.
- **Explorer restarts.** The icons are added again when the taskbar is made anew,
  and the taskbar state of each window is set again with it. `Shown` stays true
  for an icon that was shown.
- **Shown.** `Shown` reports whether the icon is in the notification area: after
  `Show` added it, and before `Hide`. Like the other methods it runs on the UI
  thread after the calls made before it, so from another goroutine it waits for the
  UI thread to answer.
- **Exit.** The icons are removed when `Run` returns, including when a
  `QuitWhenClosed` window closes the app. A process that ends without `Run`
  returning, such as through `os.Exit`, leaves its icons to Windows, which may keep
  one until the pointer passes over it.
- **Limits.** Each icon has an ID of at most 65535. The tooltip shows at most 127
  UTF-16 units. The icon shows no balloon notifications.

### Hide to the tray

`Window.CloseToTray` is a signal the app owns. While it holds true, the window hides
when the user closes it, and the app keeps running with its tray icon. The window's
taskbar button goes with it. Because the app owns the signal, it can offer the choice
as a setting: bind a `Switch` to it, and the next close follows it. The window hides
only while one of the app's tray icons is shown; without one the user could not bring
it back, so it closes as usual. OnClose is not asked for a close that hides. The
window handle's `Close` still closes the window and asks `OnClose`, so a tray menu's
Quit item closes the windows with `win.Close()`. A close button that the app draws
itself calls `ctx.Window().RequestClose()`, which closes as the window's own close
button does, so `CloseToTray` applies to it; `ui.TitleBarButtons` does this.

```go
closeToTray := ui.NewSignal(settings.CloseToTray) // saved with the app's settings

app.New().Run(app.Window{
    Title:       "Notes",
    CloseToTray: closeToTray,
    OnClose:     func() bool { return confirmUnsaved() },
    Root:        root,
})

// In the settings page:
ui.Switch("Close to the notification area", closeToTray)
```

`Show` restores a hidden or minimized window and brings it to the front, which is
what a tray click does. `Hide` hides the window and its taskbar button. Both are
methods of `ui.Window`, so `ctx.Window().Show()` and `ctx.Window().Hide()` work from
any handle. `Visible()` reports whether the window is shown, whichever way it was
shown or hidden: by `Show` or `Hide`, by a `CloseToTray` close, or by a tray click
that calls `Show`. A minimized window is visible; a window that has closed is not. Windows may refuse to bring a window to the front from the background. A
click on the tray icon normally allows it; check that on Windows (see below). Keep a
tray icon while a window can be hidden: without one, the user has no way to show the
window again.

A hidden or minimized window draws nothing, and its animations stop: an indeterminate
progress bar in a hidden window costs no CPU. Showing the window draws one frame from
its current state, and the animations resume.

### Taskbar progress and badges

`ui.Window` has the taskbar operations, so the handle from `ctx.Window()` or
`app.Open` calls them directly. They are Windows operations.

```go
win := ctx.Window()
win.SetTaskbarProgress(ui.TaskbarProgress{State: ui.TaskbarNormal, Value: 0.4})
win.SetTaskbarBadge(badgeImage, "Attention needed")
win.SetTaskbarBadge(nil, "") // removes the badge
```

- **States.** `ui.TaskbarNone` shows no bar, and it is the state of a new window.
  `ui.TaskbarNormal` shows progress up to `Value`. `ui.TaskbarIndeterminate` shows that
  work is going on, without a value. `ui.TaskbarPaused` and `ui.TaskbarError` show
  progress up to `Value`, in the paused and error colors. `Value` is clamped to 0
  through 1, and NaN is 0.
- **Badge.** The badge is drawn over the taskbar button's icon at the small icon
  size, so keep the image simple. A badge needs a description, which assistive
  technology reads, such as "3 unread". Without one, the badge is not set and a
  message is printed to stderr. Passing nil removes the badge.
- **Timing.** A call made before the taskbar button exists is kept, and applied when
  the button is made. The taskbar makes the button again when the window is shown
  again or when Explorer restarts, and the window's state is applied each time. A
  hidden window has no taskbar button, so it shows nothing.

### Limits and scope

- The taskbar progress and badge show on the window's taskbar button only. The tray
  icon has no progress bar or badge of its own.

### Checks by hand

The headless tests cover the menu model, the GUID derivation, the progress and
badge logic, the taskbar call order and the icon sizes. They do not create an icon,
a menu or a taskbar button. These need a person at a Windows desktop:

- A left click, a double click, and a right click on the icon, each with the mouse.
- The keyboard: move focus to the icon in the notification area, then Enter or
  Space, the context menu key, and Shift+F10. Each of the menu's items, checked and
  disabled, with the keyboard.
- The menu follows the light and dark system themes.
- Explorer restart (for example, end and restart `explorer.exe`): the icon comes back,
  and the taskbar progress and badge are shown again.
- A DPI change: move the window to a monitor with another scale, and change the
  scale in the display settings. The icon stays sharp at the taskbar's size.
- Hide to tray and restore: close the window with its close button, restore it from
  the tray with a click and from the menu, and check that it comes to the front when
  the app is not in the foreground.
- Taskbar progress in each state (normal, indeterminate, paused, error, none), and the
  badge, set and removed, while the window has its taskbar button.
- The icon is gone after the app quits, through `QuitWhenClosed` and through the
  tray's Quit item.
- Idle CPU with a window hidden by `CloseToTray` or `Hide` while an indeterminate
  progress bar or a progress ring runs: the process's CPU time over 60 seconds should
  be near zero, as it is for a minimized window (see BENCHMARKS.md, "Idle work"). Then
  show the window: it draws at once, and the animation resumes.
- Caret blinking and tooltips after a window is shown again: the caret resumes its
  blink, and a tooltip still appears on hover.

## OS notifications

Windows only. `app.Notify` shows a Windows toast, which the Notification Center
keeps. It needs an `AppID`, and the app must be running: call it from a window, an
action, or a goroutine started after `Run`. Without an `AppID` it returns an error.

```go
app.New().
    AppID("JambExamples.Notes").
    SingleInstance(func(args []string, dir string) {}).
    OnNotification(func(n app.NotificationActivation) {
        // n.Action is the clicked button's ID, or "" for a click on the body.
    }).
    Run(app.Window{...})

err := app.Notify(app.Notification{
    Title: "Build finished",
    Body:  "The build finished with 3 warnings.",
    Image: img, // optional hero image, a banner above the text
    Actions: []app.NotificationAction{
        {ID: "open", Label: "Open"},
        {ID: "dismiss", Label: "Dismiss"},
    },
    Tag:  "build", // a later notification with this tag replaces this one
    Args: "build-42", // returned as n.Args when the notification is clicked
})
```

- **Shape.** `Notification` has `Title` (required), `Body`, `Image` (an
  `image.Image`, shown as a hero banner), `Actions` (at most five buttons, each with
  a unique `ID` and a `Label`), `Tag` (at most 64 characters), `Args` and `Silent`.
  `app.RemoveNotification(tag)` removes one notification, and
  `app.ClearNotifications()` removes the app's notifications from the Notification
  Center. Both need an `AppID` too.
- **Errors.** `Notify` returns an error for a notification that cannot be shown:
  an invalid notification, a registration that fails, or a toast that Windows refuses.
  A failure that Windows raises after accepting the toast is printed on stderr.
- **Writing the text.** Windows reads the title and body aloud to screen-reader
  users. Write them as complete sentences, such as "The build finished with 3
  warnings.", not as fragments.
- **Images.** The image is saved as a PNG in the temporary folder, under
  `jamb-notifications`, named by the digest of its bytes, so the same image is
  saved once. Windows may delete the folder's files when it cleans the temporary
  folder, which affects only notifications that are shown again later.

### What is written, and when

The first `Notify` of a process writes the registration below, for the current user
only. Nothing is written at import time, and that process does not write it again.
The paths use the executable that is running, so a moved program needs a new first
`Notify` (a new process) to point the registration at its new location.

- **Start menu shortcut.** Windows shows a toast from a desktop app only when a
  Start menu shortcut carries the app's `AppID` as its `System.AppUserModel.ID`, per
  Microsoft's guidance for desktop apps. Before it writes one, `Notify` looks for
  shortcuts to the running program: every `.lnk` in the user's
  `%APPDATA%\Microsoft\Windows\Start Menu\Programs` folder and in the all-users
  `%ProgramData%\Microsoft\Windows\Start Menu\Programs` folder, including their
  subfolders, whose target is the same executable (paths are compared in full, without
  case). Then:
  - a shortcut that already carries the `AppID` is used as it is, and nothing is written;
  - otherwise the first shortcut that can be written has the `AppID` set on it, and its
    other settings are kept;
  - otherwise `Notify` writes
    `%APPDATA%\Microsoft\Windows\Start Menu\Programs\<name>.lnk`, where `<name>` is the
    program's file name without its extension, with characters Windows does not allow
    removed (`Notes Pro.exe` gives `Notes Pro.lnk`). A shortcut that cannot be written,
    such as one in the all-users folder without rights, is left alone, and the per-user
    shortcut is written instead.

  So a program has one Start menu entry for notifications: the installer's shortcut
  when the installer made one, or this one. The shortcut's target is the program's
  executable.
- **Protocol, when OnNotification is set.** A click on a notification after the app
  has closed starts the app through a URI scheme, `jamb-<appid>` in lower case, such
  as `jamb-jambexamples.notes`. `Notify` writes three values under
  `HKEY_CURRENT_USER\Software\Classes\jamb-<appid>`: the default value
  `URL:jamb-<appid>`, the value `URL Protocol` (empty), and the command under
  `\shell\open\command`, which is `"<exe>" "%1"`.
- **Packaged (MSIX) apps** write nothing: the package's identity is used. A packaged
  app that uses `OnNotification` must declare the same protocol in its package
  manifest. `jamb package --msix` does this when `notificationProtocol` is `true` in
  `jamb.json` (see [jamb.json](packaging.md#jambjson)).

To remove the registration by hand, delete the shortcut and the protocol key:

```bat
del "%APPDATA%\Microsoft\Windows\Start Menu\Programs\Notes Pro.lnk"
reg delete "HKCU\Software\Classes\jamb-jambexamples.notes" /f
```

The next `Notify` writes the registration again. The installer removes both on
uninstall (see [Installer](packaging.md#installer)). Windows keeps a notification it has shown in
the Notification Center until the user or `ClearNotifications` removes it.

### Activation

- **While the app runs.** Windows raises the toast's activated event in the app, which
  calls the handler on the UI thread with the button's or the body's data. The app
  keeps the last 32 toasts alive so that the event is delivered for them. A click on
  an older toast reaches the app through the protocol, as below.
- **After the app has closed.** The click starts the executable through the protocol,
  with the click's data as its command-line argument. The new process is the first
  instance, so it opens its window, and then calls the handler once for that click.
- **With SingleInstance.** `OnNotification` requires `SingleInstance`, and `Run`
  returns an error without it. Without it, a click while the app runs would start
  a second process with a second window. With it, the new process hands the click
  to the running app, which calls the handler and brings its window forward. The
  click is not passed to the `SingleInstance` function. Any other command-line
  arguments of a handed-over launch still go to that function.
- **Duplicates.** A click that reaches the app by two routes is delivered once: each
  click carries a random nonce, and the app remembers the last 64 nonces.

The click's data is an URI with the action ID, the tag, the args and the nonce,
base64-encoded. Any program the user runs can start the protocol with any data, so
the handler must treat `Action`, `Tag` and `Args` as input from outside the app. The
data of one click is limited to 512 characters; the action IDs, the tag and the args
together must fit in that, so keep the args to an ID or a short string.

### Limits

- Windows allows at most five buttons, and a tag of up to 64 characters.
- Windows may hide a notification under Focus Assist or Do Not Disturb, or the user
  may turn the app's notifications off in Settings. `Notify` does not report either.
- Dismissal is not reported. The toast's dismissed event is not subscribed.
- A `go run` program's executable is removed when the run ends. The shortcut and
  the protocol then start a program that no longer exists, so test notifications
  with a built executable.
- The shortcut is per user. An all-users shortcut is given the `AppID` only when the
  user can write it. Otherwise the per-user shortcut is written, and Windows may show
  two entries for the app.

### Hand checks

These cannot be run headless. Check them on Windows 10 and 11 with a built
executable, and with an installed build:

- A notification appears with its title, body, hero image and both buttons.
- A click on the body and on each button, while the app runs, calls the handler once,
  and the window comes forward.
- A click after the app has closed starts it, and the handler is called once the
  window is open.
- A second `Notify` with the same tag replaces the first; `RemoveNotification` removes
  it, and `ClearNotifications` clears the Notification Center.
- With `SingleInstance`, a second launch of the app hands over its arguments, and a
  click reaches the first instance and not a second window.
- Focus Assist and Do Not Disturb hide the notification as Windows documents.
- The shortcut and the protocol key appear as described, and the removal commands
  above remove them.
- After a `setup.exe` install, the app's Start menu folder has one entry for the app,
  and a notification from the installed build shows without a second entry appearing.
- After an uninstall, the installed files, the Start menu entries, the protocol key
  `HKCU\Software\Classes\jamb-<identityName>` and the per-user `<name>.lnk` are gone.
- With `notificationProtocol` set, a click on a notification from a packaged (MSIX)
  build starts the packaged app after it has closed, and the handler is called once.
