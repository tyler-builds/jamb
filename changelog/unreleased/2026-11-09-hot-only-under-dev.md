### Fixed

- An app that imports package `devtools` no longer encodes every `ui.Local` as it is made when
  `jamb dev` did not start it. The demo's Components page, whose data grid holds 100,000 rows in
  a local, opens in 0.4 s in a browser instead of 3 s, and the page uses half the memory or less.
  A web build leaves the package that keeps state across restarts (`devtools/hot`) out.
