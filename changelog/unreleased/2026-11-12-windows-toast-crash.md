### Fixed

- `app.Notify` no longer crashes an app on Windows with an access violation after it has shown
  notifications for a while. The Windows COM calls now keep their result variables on the heap,
  where a native call can still write them after the goroutine's stack moves, and each
  notification gets the Windows Runtime factories it uses instead of calling through ones kept
  from the first notification.
