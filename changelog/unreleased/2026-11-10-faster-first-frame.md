### Changed

- On Linux, a window shows its first frames drawn on the CPU while the GPU device is still being created,
  and moves to the GPU at the first frame after it is ready, so it appears in about a third of the time
  (45 ms instead of 150 ms with an NVIDIA driver).
- On Linux, the desktop's settings are read from the settings portal in one call, started with the app
  and kept until the desktop reports a change, instead of eight calls made twice while the first window
  opens.
