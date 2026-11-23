### Fixed

- An app with an `AppID` sets it at startup on each Start menu shortcut to its executable that
  lacks it, so a shortcut pinned from the Start menu and the running app are one taskbar button.
  This covers shortcuts the installer's stamp did not reach: one the user made for a zip install,
  one from an older installer, or a stamp that failed.
- The `jamb package` installer no longer gives its shortcut `identityName` as its AppID when the
  AppID check finds that the app sets none, since the pinned shortcut would then be a taskbar
  button apart from the app.
