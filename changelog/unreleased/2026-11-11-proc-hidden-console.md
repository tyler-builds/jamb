### Added

- The `proc` package starts background programs without a console window on Windows:
  `proc.Command`, `proc.CommandContext` and `proc.Hide`. A GUI app that runs a console
  program such as `git` through it no longer flashes a console window for each run.
