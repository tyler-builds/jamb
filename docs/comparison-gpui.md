# jamb and GPUI: hello-world comparison

This document compares two builds of the same small app on one Windows machine: jamb (Go) and GPUI (Rust, the UI framework from Zed). The app is a window with a centered greeting and a button that counts its clicks. It measures project size, binary size, build times, startup, idle CPU and memory, and the CPU cost of pointer movement and window resizing.

Measured on 2026-10-12 with `scripts/compare-gpui.ps1`, three runs per measurement. Results are shown as median [minimum–maximum].

## Summary

- **Build time.** jamb's clean release build takes about 9 s. GPUI's takes about 138 s for release, 100 s for debug and 238 s for the size profile. Incremental builds are 2 s for jamb and 4–5 s for GPUI.
- **Binary size.** jamb's stripped release binary is 8.45 MB. GPUI's default release binary is 9.21 MB, and its size-optimized profile is 4.31 MB.
- **Project size.** jamb's app is one 28-line file and uses 76.6 MB of third-party module source. GPUI's app is one 68-line file and resolves 673 packages (about 840 MB of source), with a 1.24 GB release target directory.
- **Toolchain.** jamb needs only the Go toolchain. The app builds with `CGO_ENABLED=0`. GPUI needs Rust 1.95.0, the MSVC linker and the Windows SDK, and its build scripts compile C and assembly helpers.
- **Idle CPU.** jamb's idle CPU was 0.36% of one core (median), with about 180 context switches per second. GPUI's was 1.9%, with about 490 per second. jamb's figure does not match the zero idle CPU in `BENCHMARKS.md`; see [Open items](#open-items).
- **Memory.** jamb's private bytes were higher (101 MB against 73 MB) and its working set was higher (78 MB against 60 MB). GPU dedicated memory was similar (about 19 MB each).
- **Startup.** With the external method used for both apps, GPUI's window was visible sooner (529 ms against 711 ms). jamb's own trace reports the first frame at 193–237 ms after process start. The external method includes process creation, which I did not separate from the rest, so the two jamb figures do not reconcile. See [Open items](#open-items).
- **Interaction.** During pointer movement, jamb used 2.2% of a core against GPUI's 3.4%. During programmatic resizing, jamb used 11.5% against GPUI's 18.8%.
- **Size result that favors GPUI.** The size-optimized GPUI binary is about half the size of jamb's stripped build. This is the one size result in GPUI's favor.

## Setup

| Item | Value |
|---|---|
| Machine | Desktop PC, 12 logical processors, discrete GPU, 32 GB RAM, Windows 11 |
| Display scale | 100% (the 800 x 600 window had an 800 x 600 client area) |
| jamb | Working tree on commit `c5c21b6`, plus the uncommitted `examples/hello`, `scripts/compare-gpui.ps1` and this document; `examples/hello` is its own module in `go.work`; Go go1.27.1 windows/amd64 |
| GPUI | `gpui` and `gpui_platform` 0.2.2 from `https://github.com/zed-industries/zed` at revision `cc053a4a6fa2fd0e8793201ed9099466af1be0b1` (the revision of the gpui-testing project), feature `font-kit` |
| GPUI toolchain | Rust and Cargo 1.95.0 (pinned by `rust-toolchain.toml`), built with `--locked` |
| GPUI lockfile | `Cargo.lock` copied from gpui-testing; cargo pruned it to 673 packages for this app |
| jamb builds | Release: `go build -trimpath -ldflags "-s -w"` (size) and `-s -w -H windowsgui` (timed and run, see Method). Default: `go build` |
| GPUI builds | Release: `cargo build --release`. Size profile: `cargo build --profile size` (inherits release; `opt-level = "z"`, fat LTO, `codegen-units = 1`, `panic = "abort"`, `strip = true`). Debug: `cargo build` |
| Machine load | Other applications were running during the measurements (the machine was not dedicated to the test) |

The two apps look the same. Both windows are 800 x 600, titled "Hello", with a `#0c0c0e` background, "Hello, world!" at 24 px in the system UI font, and a button in jamb's primary-button colors (accent `#6265ed`, hover `#5b5ee7`, 8 px radius, medium weight). The button's padding differs slightly: in the screenshots jamb's button measures 128 x 34 px and GPUI's 134 x 38 px. Both apps were clicked twice in a QA run; both showed "Clicked 2 times".

![jamb hello](screenshots/comparison-jamb-hello.png)

![GPUI hello](screenshots/comparison-gpui-hello.png)

## Method

- **Source size.** Files and lines in the app's own source. Framework size for jamb is the non-test `.go` files of every jamb package in the dependency graph of `examples/hello` (`go list -deps`). For GPUI, the sizes are the crate directories of every package in `Cargo.lock` (from `cargo metadata --locked`), not the whole caches. The zed checkout is a monorepo, so only the crate directories are counted.
- **Dependency sources.** Go: module directories in GOMODCACHE for the build list (`go list -m all`, with `GOWORK=off` so the list is hello's own), and separately for the modules its packages use. Rust: registry crate directories and git crate directories.
- **Build outputs.** Go: the size of a fresh GOCACHE after a cold build. Rust: the size of a fresh `CARGO_TARGET_DIR` after each profile's build. Every build used a temporary cache or target directory, deleted afterwards.
- **Toolchain sizes.** The GOROOT directory for Go. The `1.95.0-x86_64-pc-windows-msvc` toolchain directory for Rust. The MSVC and Windows SDK installs are not measured.
- **Release binary size.** The produced `.exe`. Rust release builds keep debug information in a separate `.pdb`, which is not included.
- **Clean build time.** Dependencies already downloaded; empty build cache or target directory; wall-clock time of the build command. Three runs each.
- **Incremental build time.** After a clean build, the app's main file is changed in two ways, each rebuilt once: a new text literal ("Hello, world!" gets a run number) and then a whitespace-only change (extra trailing newlines). The original file is restored byte for byte afterwards. Three runs each.
- **First build on a new machine (one run).** `go mod download` into a temporary GOMODCACHE (`GOWORK=off`), and `cargo fetch --locked` into a temporary CARGO_HOME (which clones the zed repository). Both caches were deleted afterwards.
- **Startup.** The external method for both apps: the process is started from a fresh copy of its executable in a temporary folder. The script polls for a visible top-level window owned by that process and titled "Hello" (about 5 ms resolution), then captures its client area with `PrintWindow` (`PW_RENDERFULLCONTENT`) until a non-blank frame appears. Times are measured from the process launch call.
- **Idle CPU and context switches.** After an 8 s settle, the app's `TotalProcessorTime` and the sum of `ContextSwitchesPersec` (raw counter `Win32_PerfRawData_PerfProc_Thread` for the process) are read before and after 60 s. Measured with the app focused, then after the taskbar was given focus (the app stays visible).
- **Memory.** Private bytes, working set, thread count and handle count at the end of the unfocused idle sample. GPU dedicated and shared memory come from the `GPU Process Memory` counters for the process.
- **Pointer movement.** For 10 s, the real mouse cursor moves in a Lissajous path over the app's client area, roughly every 16 ms, with no clicks. The cursor is returned to its original position afterwards.
- **Resizing.** For 10 s, the app window is resized programmatically (`SetWindowPos`) between about 560 x 420 and 1040 x 780 logical pixels, roughly every 16 ms. This is not a user drag.
- **Focus.** The app is brought to the front with a quick Alt key press, which Windows needs before it accepts a focus change from a background process. The script records whether the app was focused and unfocused as intended. All runs did.
- **Timer resolution.** Sampling uses `Thread.Sleep`, which has roughly 15 ms resolution on this machine by default, so startup values are accurate to about 15 ms.

## Results

### Project size

| Metric | jamb (Go) | GPUI (Rust) |
|---|---|---|
| App source | 1 file (`main.go`), 28 lines | 1 file (`src/main.rs`), 68 lines |
| App manifests | `go.mod`, `go.sum` | `Cargo.toml`, `Cargo.lock`, `rust-toolchain.toml` |
| Framework source used | 17 jamb packages, 119 non-test files, 40,235 lines, 1.28 MB | Not applicable (GPUI's source is counted with its dependencies) |
| Third-party sources used | 4 modules, 76.6 MB (used by the app's packages) | Registry sources in `Cargo.lock`: 647 crates, 815.0 MB (all platforms) |
| Git-sourced crates | None | 25 crates, 24.6 MB (22 from zed at `cc053a4`; forks of wasm_thread, font-kit and scap) |
| Build list (Go) | 5 modules downloaded (237.7 MB), 3 go.mod-only | Not applicable |
| Dependency count | 53 non-standard packages in the app's graph (jamb's packages, the app's own and third-party); 4 third-party modules used | 673 packages in `Cargo.lock` (647 registry, 25 git, 1 local) |
| Build output after a release build | GOCACHE 84.1 MB (cold) | `target/` 1,243.8 MB |
| Build output after a debug build | GOCACHE 84.2 MB (cold, default flags) | `target/` 2,484.4 MB |
| Build output after the size profile | Not applicable | `target/` 1,158.4 MB |
| Toolchain install | Go GOROOT 235.5 MB | Rust 1.95.0 toolchain 571.3 MB (plus MSVC and Windows SDK, not measured) |
| Download on a new machine (one run) | 5 s, 94 MB in GOMODCACHE | 177 s, 1,794 MB in CARGO_HOME (includes the zed clone) |

### Release binary size

| Build | Size |
|---|---|
| jamb, `go build -trimpath -ldflags "-s -w"` | 8.45 MB |
| jamb, `-s -w -H windowsgui` (the run and screenshot build) | 8.45 MB |
| jamb, default `go build` | 11.4 MB |
| GPUI, `cargo build --release` (`.pdb` not included) | 9.21 MB |
| GPUI, `cargo build --profile size` | 4.31 MB |
| GPUI, debug (`cargo build`) | 24.39 MB |

### Build times (seconds; three runs; dependencies already downloaded)

| Build | Median [min–max] |
|---|---|
| jamb clean release (`-trimpath -ldflags "-s -w -H windowsgui"`, cold GOCACHE) | 9.1 [9.1–12.2] |
| jamb clean default (`go build`, cold GOCACHE) | 9.1 [9.1–10.1] |
| GPUI clean release (`cargo build --release`, empty target) | 137.7 [136.7–139.3] |
| GPUI clean debug (`cargo build`, empty target) | 100.3 [97.2–101.2] |
| GPUI clean size profile (empty target) | 237.7 [161.0–279.9] |
| jamb incremental, release, text change | 2.03 [2.03–2.05] |
| jamb incremental, release, whitespace change | 2.03 [2.03–2.05] |
| jamb incremental, default, text change | 2.04 [2.03–2.07] |
| jamb incremental, default, whitespace change | 2.04 [2.04–2.06] |
| GPUI incremental, release, text change | 4.05 [4.04–14.15] |
| GPUI incremental, release, whitespace change | 4.06 [4.05–4.08] |
| GPUI incremental, debug, text change | 5.07 [5.06–13.17] |
| GPUI incremental, debug, whitespace change | 5.08 [5.07–6.07] |

The size-profile clean builds varied the most of any measurement. Two incremental text-change runs (14.2 s release, 13.2 s debug) were outliers; the other runs were close to the medians.

### Runtime (three runs each; the size profile is GPUI's stripped, size-optimized build)

| Metric | jamb | GPUI release | GPUI size profile |
|---|---|---|---|
| Startup, window visible (external) | 710.7 ms [643.9–745.7] | 529.2 ms [517.1–660.3] | 525.8 ms [440.4–579.6] |
| Startup, first painted frame (external) | 749.7 ms [666.2–787.6] | 571.1 ms [560.0–723.4] | 585.2 ms [481.7–644.5] |
| Startup, jamb's own trace (`JAMB_TRACE=1`, first frame after process start; six manual runs) | 193–237 ms | Not measured | Not measured |
| Idle CPU, focused (60 s) | 0.36% [0.21–0.39] | 1.88% [1.82–1.93] | 1.95% [1.88–2.55] |
| Idle context switches/s, focused | 183 [156–252] | 489 [480–533] | 550 [457–646] |
| Idle CPU, unfocused (60 s) | 0.23% [0.10–0.31] | 2.00% [1.43–2.08] | 2.00% [1.54–2.32] |
| Idle context switches/s, unfocused | 176 [161–182] | 495 [462–649] | 557 [454–638] |
| Private bytes | 100.7 MB [100.5–101.1] | 73.5 MB [73.3–73.8] | 73.6 MB [73.6–73.9] |
| Working set | 78.1 MB [77.7–78.1] | 60.2 MB [59.8–60.3] | 55.4 MB [55.0–55.4] |
| Threads | 35 | 30 | 30 |
| Handles | 464 | 487 [486–487] | 486 [485–487] |
| GPU dedicated memory | 18.6 MB [18.6–18.6] | 19.5 MB [17.5–19.5] | 19.5 MB [17.5–19.5] |
| GPU shared memory | 6.2 MB [6.2–6.4] | 0.6 MB [0.1–1.0] | 1.0 MB [0.9–1.1] |
| Pointer movement, CPU (10 s) | 2.18% [1.25–3.59] | 3.43% [2.50–4.21] | 2.03% [1.56–3.90] |
| Pointer movement, context switches/s | 712 [649–725] | 881 [751–936] | 831 [735–925] |
| Resizing, CPU (10 s) | 11.52% [9.37–11.99] | 18.84% [13.26–19.83] | 14.18% [9.66–14.50] |
| Resizing, context switches/s | 2,411 [2,325–2,529] | 1,820 [1,818–2,110] | 1,818 [1,735–1,963] |

CPU values are percent of one logical core. The GPU memory figures include the Direct3D driver's allocations for the process, which the app does not control.

## Caveats

- **GPUI's dependency tree is larger than a hello world needs.** The 673 packages include text shaping, accessibility, font, image and platform crates that this app does not use directly. `Cargo.lock` lists packages for every platform (for example, `freetype-sys` is locked but is not built for Windows), so the registry and git sizes overstate what a Windows build compiles. The size-profile binary is smaller than jamb's, but the source tree and target directory are much larger.
- **Platform scope differs.** jamb is Windows-only today. GPUI targets macOS, Linux and Windows. jamb's Linux and macOS platform packages are placeholders (per the README), so the example was only compiled for them (`GOOS=linux` and `GOOS=darwin GOARCH=arm64`), not run. Cross-compiling GPUI from Windows was not tested, because it would need the rustup target and its system libraries.
- **GPUI on Windows.** This comparison measures GPUI's Windows backend on this machine and nothing else. It does not assess that backend's maturity or stability.
- **Driver threads.** The Direct3D driver's threads run in the app's process. Process CPU time and context switches include them, and they could not be separated.
- **Startup method.** The external method includes process creation and the script's polling. It is consistent across the two apps, but its absolute values are not comparable with jamb's internal trace (see Open items).
- **Synthetic input.** The pointer and resize phases move the real cursor and resize the app's window for a few seconds each. Each app's cursor position was restored afterwards by the script. The Alt key press that the script uses to bring the app to the front is also synthetic.
- **Focus and occlusion.** The unfocused sample gives focus to the taskbar. The app stays visible, but a window that is not visible at all could behave differently.
- **Measurement noise.** The machine was not idle: other applications were running. Idle CPU ranges were about 0.1 to 0.4% for jamb and 1.4 to 2.6% for GPUI, and the largest run-to-run variation is in the GPUI size-profile clean builds and the pointer and resize samples. The sample size is three.
- **Button look.** The button colors and radius match jamb's primary button (GPUI's `rounded_lg` is 8 px), but the padding is not identical (see Setup). The text is GPUI's default system font, which is also what jamb uses on Windows.
- **Build flags.** The jamb run and screenshot binary uses `-H windowsgui`, so no console window appears. The size figure in the table uses exactly `-trimpath -ldflags "-s -w"`, and it is the same 8.45 MB. GPUI's release build uses the `windows_subsystem` attribute for the same reason.

## Toolchain requirements

| | jamb (Go) | GPUI (Rust) |
|---|---|---|
| Install | Go 1.27 or later (one installer) | rustup, then Rust 1.95.0 (pinned); the first build also needs the MSVC linker from Visual Studio 2022 C++ build tools and the Windows SDK |
| C compiler or cgo | Not needed; the example has no `import "C"` and builds with `CGO_ENABLED=0` | Needed on Windows: the dependency graph includes `cc` build scripts (for example `psm`/`stacker`, which compile small C and assembly pieces) and `embed-resource`, which calls the MSVC resource compiler |
| Network on first build | `go mod download` of about 94 MB | About 1.8 GB, including a clone of the zed repository |
| Build steps | `go build` (one command) | `cargo build` (one command) |
| Cross-compiling from Windows | Compiles for linux/amd64 and darwin/arm64 (compile only, see Caveats) | Not tested |

## Open items

- **jamb's idle CPU.** `BENCHMARKS.md` reports zero CPU over 10 s for the demo. This hello app used 0.36% of a core over 60 s (about 180 context switches per second). The difference may come from the app, the sample length, or the machine's load. It needs a separate look before the figure is quoted.
- **Startup gap.** jamb's own trace puts the first frame at 193–237 ms after process start. The external method saw the painted window at 666–788 ms. The 400+ ms difference is outside jamb's clock; it is most likely process creation or the script's polling, which I did not isolate. GPUI has no equivalent trace, so the two startup numbers are comparable only as the external method gives them.
- **GPUI idle wake-ups.** GPUI used about 2% of a core and about 500 context switches per second while idle. The cause was not investigated.

## Reproducing

```
powershell -ExecutionPolicy Bypass -File scripts\compare-gpui.ps1 -OutFile results.json
```

The script expects the jamb repository (default: its parent folder) and the gpui-hello project next to it (`-GpuiRoot` to change). `-Runs`, `-IdleSeconds`, `-SettleSeconds` and `-MoveSeconds` set the lengths; `-SkipGo`, `-SkipRust`, `-SkipInteraction` and `-IncludeDownload` select parts. The full run with three runs and the download took 56 minutes on this machine. Without the download it takes about 53 minutes. Most of that time is the GPUI clean builds (about 24 minutes) and the runtime samples (about 24 minutes, dominated by the two 60-second idle samples per app and run). The script writes only to temporary folders and to `docs/screenshots`, and it removes its temporary folders and processes when it exits.
