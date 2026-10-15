# Packaging

`jamb package` builds a Windows release of an app from its `jamb.json` (below).
It always writes a zip. Flags add an installer and an MSIX.

```
go run ./cmd/jamb package ./examples/demo                 # zip
go run ./cmd/jamb package --installer ./examples/demo     # zip and setup.exe (needs NSIS)
go run ./cmd/jamb package --msix ./examples/demo          # zip and an unsigned MSIX (needs the Windows SDK)
```

Outputs go to `<app>/dist/`, which git ignores. Each run builds its files in a
temporary folder inside `dist` and moves them there only after every step has
succeeded, so a failed signature leaves no partly signed set.

- `<name>-<version>-windows-<arch>.zip`: the executable at the root, and any
  `LICENSE*` and `NOTICE*` files of the app directory beside it. Always built.
- `<name>-<version>-windows-<arch>-setup.exe`: an installer, built with
  `--installer`. See [Installer](#installer).
- `<name>-<version>-windows-<arch>.msix`: an MSIX package, built with `--msix`.
  It is unsigned unless a sign command signs it. See [Signing](#signing).

`<arch>` is `amd64` or `arm64`.

## jamb.json

Every field is required except `copyright` and `notificationProtocol`. An unknown
field is an error, so a misspelled name stops the build, and every problem is reported
in one message.

| Field | Meaning |
|---|---|
| `name` | The executable's name without `.exe`: starts with a letter, then letters, digits, `-` and `_`, at most 64 characters. |
| `displayName` | The name users see: Start, the taskbar, the installer and the package. It also names the install folder, so it cannot contain `\ / : * ? " < > \|`. |
| `description` | A short description, shown in file properties, the installer and the package. |
| `version` | A semantic version, `MAJOR.MINOR.PATCH`. Each part is 0 to 65535, and Windows stores it as `MAJOR.MINOR.PATCH.0`. Pre-release and build suffixes have no Windows form and are rejected. |
| `publisher` | The publisher's display name, shown as the company in file properties, the installer and the package. |
| `publisherSubject` | The subject of the signing certificate, starting with `CN=`, such as `CN=Example`. The MSIX publisher must equal the certificate's subject exactly, so for a certificate from a certificate authority copy the whole subject. For the Store, use the publisher value that Partner Center shows for the app. |
| `identityName` | The package identity: 3 to 50 characters of letters, digits, `.` and `-`, starting and ending with a letter or digit. Windows uses it to identify the app, and the installer uses it as its registry key, so it must stay the same across versions that should upgrade one another. For the Store, use the identity name that Partner Center shows for the app. |
| `copyright` | Optional. Shown as the legal copyright in file properties and the installer. |
| `notificationProtocol` | Optional, `true` or `false`; the default is `false`. With `true`, the MSIX manifest declares the app's notification protocol, `jamb-` and `identityName` in lower case, as a `windows.protocol` extension. A click on a notification then starts the packaged app after it has closed. The app must use the same `AppID`, which is `identityName`, and `OnNotification`. It has no effect on the zip or the installer. |
| `icon` | A square PNG, at least 48 px, relative to the app directory. 256 px or larger is recommended, and packaging warns below that. It becomes the executable's icon, the installer's icon and the MSIX logos. |

File associations and extra capabilities are not supported yet. The manifest
requests only `runFullTrust`.

## Commands and flags

`jamb package [flags] [package dir]`. The directory defaults to `.`, and flags may
come before or after it.

| Flag | Effect |
|---|---|
| `--arch amd64\|arm64` | The target architecture. Default `amd64`. |
| `--installer` | Build the `setup.exe` installer with NSIS. |
| `--makensis PATH` | The `makensis` program to use. Needs `--installer`. Default: found on PATH or in the NSIS install directory. |
| `--msix` | Build the MSIX package. It is unsigned unless `--sign-command` signs it. |
| `--sign-command CMD` | Run this command once for each file that is built, with `{file}` replaced by the file's path. Default: the `JAMB_SIGN_COMMAND` environment variable. See [Signing](#signing). |
| `--sdk-bin DIR` | The directory with `makeappx.exe`. Default: the newest SDK under `C:\Program Files (x86)\Windows Kits\10\bin`. |

`--cert`, `--test-cert`, `--timestamp` and `--no-msix` were removed. Each one is
refused with the replacement to use.

## What the executable contains

- The icon, in 16, 24, 32, 48, 64, 128 and 256 px.
- Version information: file and product version (`version` with `.0`), company,
  product name, description, original and internal file names, and copyright.
- An application manifest with per-monitor v2 DPI awareness, which matches what
  the framework sets at run time; Common Controls version 6, which `TaskDialog`
  needs; Windows 10 and 11 as the supported OS; long path names; and `asInvoker`,
  so the app never asks for elevation.
- A build with `-trimpath -ldflags "-s -w -H=windowsgui"`, `GOOS=windows`,
  `CGO_ENABLED=0`. The GUI subsystem means no console window opens.

The manifest does not set `activeCodePage` to UTF-8. The framework calls only the
wide-character Win32 functions, so the active code page does not affect it, and
leaving the system's code page alone is the safer default for any ANSI call.

While it builds, `jamb package` writes a resource object,
`jamb_package_windows_<arch>.syso`, into the app's directory and removes it when
packaging ends, on failure and on Ctrl+C too. If a file with that name already
exists, packaging stops rather than overwrite it.

## AppID check

The installer and the MSIX take the app's identity from `identityName`: the installer
gives its Start menu shortcut that ID and removes the notification protocol named after
it when it uninstalls, and with `notificationProtocol` the MSIX declares that protocol.
The app takes its own from `App.AppID("...")` in its Go code. When the two differ,
the installed app's notifications, notification clicks and jump lists do not work, so
`jamb package` checks them.

After it builds the executable, and before it signs it, `jamb package` runs it once
with the internal flag `--jamb-print-appid`. The app answers in `Run`, before it opens
a window, creates a tray icon, takes the single-instance lock, writes a registry key or
shows anything: it writes its AppID to stdout and exits. Code in the app's `main`
before `Run` still runs. The run has no console and is stopped after 10 seconds. The
flag is internal and is not part of the API.

| Result | What happens |
|---|---|
| The AppID equals `identityName` | Packaging goes on. The comparison is exact, case included. |
| They differ, and `--installer` or `--msix` builds a package | Packaging stops with both values, the line of `identityName` in `jamb.json`, and the places in the app's Go files that hold the AppID. |
| They differ, and only the zip is built | A warning. The zip does not use `identityName`. |
| The app sets no `AppID` | A warning. The app then uses none of the features that need the ID (`Notify`, `JumpList` and `SingleInstance` require one), but the installer still gives the shortcut `identityName`. |
| The app is built with a jamb version without the flag | A note, and the app is not run: an older app would open its window. |
| The executable cannot run here: `--arch arm64` on an x64 PC, or a host other than Windows | A warning. Windows on Arm runs both architectures. |
| The app does not answer: it exits without the answer, for example because it reads its own command-line flags before `Run` and refuses the unknown one, or it does not answer within 10 seconds | A warning with the reason, and the app is stopped if it is still running. |

`jamb.json` requires `identityName`, so it is never missing. To fix a mismatch, make the
two equal: change `App.AppID("...")` in the app to the `identityName` value, or change
`identityName` to the app's AppID. The second is offered only when the AppID is also a
valid `identityName`, which allows no `_` and at most 50 characters. `identityName` keys
installer upgrades and the Store listing, so for an app that is already released, change
the Go code.

## Installer

`--installer` compiles `<name>-<version>-windows-<arch>-setup.exe` with NSIS,
from a script that jamb generates from `jamb.json`. The installer:

- installs for the current user only, to `%LOCALAPPDATA%\Programs\<displayName>`.
  It needs no administrator rights, and it writes only to `HKCU` and the user's
  Start menu and desktop. This is the default because it works on managed PCs
  where standard users cannot write to Program Files, and because each user can
  update the app without elevation. A machine-wide Program Files installer is not
  generated; that would need its own script and an administrator prompt.
- adds a Start menu folder with the app and an uninstaller shortcut;
- sets the `AppID` (`identityName`) on the app's Start menu shortcut. The installer runs
  the installed program once with the internal flag `--jamb-stamp-shortcut <AppID>`,
  which sets the ID on the shortcut that carries the program's target, and exits without
  opening a window. The flag is internal and is not part of the API. A failure is not
  fatal: the program sets the ID itself at its first `Notify`. An app built by a jamb
  version without the flag would open its window instead, so rebuild such an app with
  `jamb package`;
- offers a desktop shortcut as an optional component, off by default;
- registers the app in Apps & features under `identityName`, with the display
  name, version, publisher, description, icon and estimated size;
- uninstalls cleanly: it removes the installed files, the shortcuts, the registry
  entry, the notification protocol key `HKCU\Software\Classes\jamb-<identityName>`,
  and the program's own notification shortcut `<name>.lnk` in the user's Start menu
  Programs folder, if the program wrote one. It leaves data the app wrote elsewhere,
  such as under AppData, in place.

Close the app before installing or uninstalling it. Silent installs use `/S`, and
`/D=PATH` (last on the command line) chooses another folder.

## Signing

jamb never needs a signing certificate, and it never reads one or a password. It
is a library shipped as source, so it is not signed. Each app's author chooses how
the app's binaries are signed:

- **Unsigned.** Works. Windows SmartScreen may warn people when they first run the
  app. An unsigned MSIX cannot be installed on a PC.
- **Microsoft Store.** Free for individual developers since September 2025. The
  Store signs the MSIX packages it accepts, so an MSIX can be uploaded unsigned:
  <https://blogs.windows.com/windowsdeveloper/2025/09/10/free-developer-registration-for-individual-developers-on-microsoft-store/>
- **SignPath Foundation.** Free signing for open-source apps with OSI-licensed
  source and automated public builds. Eligibility and setup are at
  <https://signpath.org/>.
- **Azure Artifact Signing, or a purchased certificate.** Paid. Azure Artifact
  Signing is documented at
  <https://learn.microsoft.com/en-us/azure/artifact-signing/overview>.

### The sign command

Set the command with `--sign-command "<command>"`, or with the `JAMB_SIGN_COMMAND`
environment variable. The flag wins when both are set. The environment variable is
used rather than `jamb.json` because the certificate differs between machines and
CI runners, and a committed file should not hold a thumbprint or a path.

jamb runs the command once for each file it builds, and `{file}` is replaced by
that file's path:

1. the executable, before it goes into the zip, the installer or the MSIX;
2. the installer, after it is compiled;
3. the MSIX, after it is packed.

Rules:

- The command runs directly, with no shell. Put a program path with spaces in
  double quotes. Pipes, `&&` and `%VAR%` expansion do not work. For those, call a
  script: `powershell -NoProfile -ExecutionPolicy Bypass -File C:\sign\sign.ps1 {file}`.
  Do not use a `.cmd` or `.bat` file as the program, since cmd.exe reads `&` and
  `%` in the path.
- If the command exits with an error, packaging stops and nothing is written to
  `dist`. The error does not repeat the command, which may name secrets.
- Keep passwords out of the command. Use the Windows certificate store, or a script
  that reads a secret from the environment.
- The command's output is shown in the console, so a signer that asks for a PIN
  can be answered.

Examples, in PowerShell. Replace the placeholders.

Certificate in the Windows certificate store, found by thumbprint, with signtool
from the Windows SDK:

```
$env:JAMB_SIGN_COMMAND = '"C:\Program Files (x86)\Windows Kits\10\bin\<sdk version>\x64\signtool.exe" sign /fd SHA256 /sha1 <THUMBPRINT> /tr http://timestamp.digicert.com /td SHA256 {file}'
go run ./cmd/jamb package --installer --msix ./examples/demo
```

PFX file with its password in an environment variable. signtool accepts a PFX
password only as its `/p` argument, so the password is visible to other processes
of the same user while signtool runs. The certificate store avoids that. Where a
PFX is required, use a script such as `C:\sign\sign.ps1`:

```
param([string]$File)
& "C:\Program Files (x86)\Windows Kits\10\bin\<sdk version>\x64\signtool.exe" sign /fd SHA256 /f "C:\certs\release.pfx" /p $env:CERT_PASSWORD /tr http://timestamp.digicert.com /td SHA256 $File
exit $LASTEXITCODE
```

```
$env:JAMB_SIGN_COMMAND = 'powershell -NoProfile -ExecutionPolicy Bypass -File C:\sign\sign.ps1 {file}'
```

Azure Artifact Signing uses signtool with the Artifact Signing client's `dlib`
and a metadata file. Take the timestamp URL, the DLL path and the metadata file
from the documentation linked above:

```
$env:JAMB_SIGN_COMMAND = '"<signtool.exe>" sign /fd SHA256 /tr <timestamp URL> /td SHA256 /dlib "<Artifact Signing dlib>" /dmdf "<metadata.json>" {file}'
```

SignPath signs through its own service and build integration. Use the command its
documentation gives, as the sign command. See <https://signpath.org/>.

## Testing the MSIX on this PC

An unsigned MSIX cannot be installed, and jamb does not create certificates. For a
local install test, sign the MSIX with a self-signed certificate whose subject is
`publisherSubject`, and trust that certificate on the PC. Microsoft's steps for
creating and trusting a certificate for package signing are at
<https://learn.microsoft.com/en-us/windows/msix/package/create-certificate-package-signing>.
They use `New-SelfSignedCertificate`, and they import the certificate into the
`LocalMachine\TrustedPeople` store from an administrator session.

Trusting a certificate changes what every user of the PC will accept, so remove it
when the test is done.

## Requirements

- **Zip.** Windows 10 or 11 and Go 1.27 or later. Nothing else.
- **`--installer`.** NSIS 3.x, which provides `makensis`. Install it with
  `winget install NSIS.NSIS`, with `scoop install nsis`, or from
  <https://nsis.sourceforge.io/Download>. jamb looks for `makensis` on PATH, in
  `Program Files (x86)\NSIS` and `Program Files\NSIS`, and at the `--makensis`
  path. If it is missing, jamb prints these instructions, skips the installer with
  a warning, and still writes the zip.
- **`--msix`.** The Windows SDK, with its **Windows SDK Signing Tools for Desktop
  Apps** feature, which provides `makeappx.exe`. Get it from
  <https://developer.microsoft.com/windows/downloads/windows-sdk/>. Use
  `--sdk-bin` if the tool is somewhere else. If it is missing, jamb skips the MSIX
  with a warning and still writes the zip.
- **`--sign-command`.** Whatever the command runs.

NSIS also runs on Linux and macOS, so a host other than Windows can compile the
installer. That path has not been tested.

## Packaging dependencies

`cmd/jamb` is its own Go module, listed in `go.work`, so the framework's `go.mod`
and the modules that import it gain nothing from packaging. Its dependencies are
`github.com/tc-hib/winres` (writes the resource object and the icon; ISC-style
license), `golang.org/x/image` (resizes the icon; BSD-3-Clause) and
`github.com/nfnt/resize` (used by winres; ISC-style license). The Windows SDK
tools and NSIS are external programs and are not redistributed. NSIS is under a
zlib-style license.

## Licenses of what your app ships

jamb is licensed under the Apache License, Version 2.0; see [LICENSE](../LICENSE) and [NOTICE](../NOTICE).
A binary built with jamb also contains third-party code. When you distribute the binary, include the
notices of that code with it. Every app built with jamb contains:

- The Go runtime and standard library, as every Go program does, licensed under the BSD 3-Clause
  License. Include the `LICENSE` file of your Go installation (`go env GOROOT`).
- `github.com/go-text/typesetting`, licensed under the Unlicense or the BSD 3-Clause License (SPDX
  `Unlicense OR BSD-3-Clause`). Include its `LICENSE` file, which holds both texts.
- `golang.org/x/image`, licensed under the BSD 3-Clause License. It includes the Go Regular font, which
  jamb embeds as its fallback font, and the Go fonts are under the same license. Include the `LICENSE`
  and `PATENTS` files of the module.
- `golang.org/x/sys` and `golang.org/x/text`, licensed under the BSD 3-Clause License. Include the
  `LICENSE` and `PATENTS` files of each module.

An app that imports `codeedit` also contains:

- `github.com/tetratelabs/wazero`, licensed under the Apache License, Version 2.0. Include its `LICENSE`
  and `NOTICE` files.
- `golang.org/x/sys/windows`, licensed under the BSD 3-Clause License, as above.
- `github.com/tyler-builds/jamb-grammars`, the tree-sitter runtime and grammars for highlighting. The
  module's code is licensed under the Apache License, Version 2.0; the tree-sitter runtime and the
  grammars it embeds are licensed under the MIT License. Include its `LICENSE` and `NOTICE` files, which
  hold the MIT notices.

An app that imports `markdown` also contains:

- `github.com/yuin/goldmark`, the Markdown parser, licensed under the MIT License. Include its `LICENSE`
  file.

An app that imports `markdown/mdcode` contains what an app that imports `codeedit` does, as above, since
it uses the code editor's highlighting.

jamb's own `NOTICE` file also covers material from gpui-component (Apache License, Version 2.0) that is
compiled into jamb, so include it with the app.

To collect these files for a build, [go-licenses](https://github.com/google/go-licenses) can be run from
your app's module. It is a tool, not a dependency of jamb: `go-licenses report ./...` lists each module
with its license, and `go-licenses save ./... --save_path=licenses` copies the license files into a
`licenses` folder.
