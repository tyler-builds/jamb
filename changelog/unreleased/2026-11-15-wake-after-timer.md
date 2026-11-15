### Fixed

- A text field's caret keeps blinking, and a timer that waits behind another runs on time, when a
  timer that is due sooner, such as a search's debounce after a key, takes the window's wake-up.
  A window keeps only the earliest wake-up it was asked for, and the app asks again for the next
  one after each frame.
