#Requires -Version 5.1
<#
.SYNOPSIS
    Reproduces the jamb and GPUI hello-world comparison in docs/comparison-gpui.md.

.DESCRIPTION
    Builds the two identical hello apps (examples/hello in this repository, and the
    gpui-hello project next to it) and measures:

      - project size: source files and lines, dependency sources, dependency counts,
        toolchain install sizes;
      - binary size (Go stripped and default; Rust release and debug);
      - incremental build times (a text change, then a whitespace change);
      - with -IncludeCleanBuilds, clean build times (fresh build cache or target
        directory, dependencies already downloaded) and the build output sizes;
      - optionally, the one-time dependency download into temporary caches;
      - startup to the first painted frame, for the first launch of a new copy of the
        executable and for warm launches of the same file, idle CPU and memory (focused
        and unfocused),
        CPU during continuous pointer movement, and CPU during continuous resizing.

    By default the builds use the normal Go build cache and gpui-hello's own target
    folder (see -GpuiTargetDir), so only the first run compiles GPUI from scratch.
    With -IncludeCleanBuilds, every build uses a temporary cache or target directory.
    Every temporary directory and process the script starts is removed on exit,
    including after a failure. The only source edits are to examples/hello/main.go
    and gpui-hello's src\main.rs, and each is restored from a byte-for-byte backup
    in a finally block.

    The pointer and resize phases move the real mouse cursor for a few seconds at a
    time and resize the app window. Run the script only when that is acceptable. The
    cursor is put back where it was afterwards.

    Requirements: Windows 10 or later, Go 1.27 or later, Rust 1.95.0 through rustup,
    the Visual Studio 2022 C++ tools and the Windows SDK, and the gpui-hello project
    with its Cargo.lock. Close other heavy work first; the numbers are only meaningful
    on an otherwise idle machine.

.PARAMETER JambRoot
    Root of the jamb repository. Default: the parent of this script's folder.

.PARAMETER GpuiRoot
    The gpui-hello project. Default: a sibling folder named gpui-hello.

.PARAMETER Runs
    Repetitions of every runtime measurement and incremental build, and with
    -IncludeCleanBuilds of every clean build. Default 3.

.PARAMETER WarmLaunches
    Launches of the same executable file per runtime run, after its first launch.
    Their times are the warm startup figures; the last one stays open for the idle,
    memory, pointer and resize samples. Default 5.

.PARAMETER SettleSeconds
    Pause after a window is painted, and after a focus change, before idle sampling.
    Default 8.

.PARAMETER IdleSeconds
    Length of each idle sample, focused and unfocused. Default 60.

.PARAMETER MoveSeconds
    Length of the pointer-movement and resize samples. Default 10.

.PARAMETER SkipGo
    Skip the jamb (Go) builds and runtime measurements.

.PARAMETER SkipRust
    Skip the GPUI (Rust) builds and runtime measurements.

.PARAMETER SkipInteraction
    Skip the pointer-movement and resize samples. Use it when the cursor must not move.

.PARAMETER IncludeCleanBuilds
    Also time clean builds: each build runs -Runs times with an empty build cache or
    target directory, and that directory's size is recorded. GPUI's release and
    debug clean builds take about 4 minutes per run on the reference machine.

.PARAMETER GpuiTargetDir
    Cargo target directory for the default builds. Default: gpui-hello's own target
    folder, so it stays warm between runs of the script.

.PARAMETER IncludeDownload
    Also measure the first-time dependency download into temporary GOMODCACHE and
    CARGO_HOME directories. Slow on the Rust side (the zed git repository is cloned).

.PARAMETER ShotDir
    Where the two screenshots are saved. Default: docs\screenshots in this repository.

.PARAMETER OutFile
    Optional path for the results as JSON.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\compare-gpui.ps1 -OutFile $env:TEMP\results.json
#>
[CmdletBinding()]
param(
    [string]$JambRoot = '',
    [string]$GpuiRoot = '',
    [int]$Runs = 3,
    [ValidateRange(1, 100)][int]$WarmLaunches = 5,
    [int]$SettleSeconds = 8,
    [int]$IdleSeconds = 60,
    [int]$MoveSeconds = 10,
    [switch]$SkipGo,
    [switch]$SkipRust,
    [switch]$SkipInteraction,
    [switch]$IncludeCleanBuilds,
    [string]$GpuiTargetDir = '',
    [switch]$IncludeDownload,
    [string]$ShotDir = '',
    [string]$OutFile = ''
)

$ErrorActionPreference = 'Stop'
$total = [Diagnostics.Stopwatch]::StartNew()

if (-not $JambRoot) { $JambRoot = Split-Path -Parent $PSScriptRoot }
$JambRoot = (Resolve-Path -LiteralPath $JambRoot).Path
if (-not $GpuiRoot) { $GpuiRoot = Join-Path (Split-Path -Parent $JambRoot) 'gpui-hello' }
$GpuiRoot = (Resolve-Path -LiteralPath $GpuiRoot).Path
if (-not $GpuiTargetDir) { $GpuiTargetDir = Join-Path $GpuiRoot 'target' }
if (-not $ShotDir) { $ShotDir = Join-Path $JambRoot 'docs\screenshots' }
$goHello = Join-Path $JambRoot 'examples\hello'
$goMain = Join-Path $goHello 'main.go'
$rustMain = Join-Path $GpuiRoot 'src\main.rs'

$work = Join-Path ([IO.Path]::GetTempPath()) ('compare-gpui-' + [guid]::NewGuid().ToString('N').Substring(0, 8))
New-Item -ItemType Directory -Path $work | Out-Null
New-Item -ItemType Directory -Path (Join-Path $work 'logs'), (Join-Path $work 'bin') | Out-Null
$started = New-Object System.Collections.ArrayList
$R = [ordered]@{ buildRetries = @() }

if (-not ('Cmp.Win32' -as [type])) {
    Add-Type -AssemblyName System.Drawing
    Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Drawing;
using System.Drawing.Imaging;
using System.Runtime.InteropServices;
using System.Text;

namespace Cmp {
public static class Win32 {
    public delegate bool EnumProc(IntPtr hWnd, IntPtr lParam);

    [DllImport("user32.dll")] public static extern bool EnumWindows(EnumProc callback, IntPtr lParam);
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint pid);
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern int GetWindowText(IntPtr hWnd, StringBuilder text, int max);
    [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr hWnd, out RECT rect);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
    [DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr hWnd, ref POINT point);
    [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr hWnd, IntPtr hdc, uint flags);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll")] public static extern bool GetCursorPos(out POINT point);
    [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr hWnd, IntPtr after, int x, int y, int cx, int cy, uint flags);
    [DllImport("user32.dll")] public static extern uint GetDpiForWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern void keybd_event(byte vk, byte scan, uint flags, UIntPtr extra);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern IntPtr FindWindow(string className, string title);

    // Windows only lets the foreground process move focus. A quick Alt press, which
    // the app ignores, makes the system accept the request.
    public static bool ForceForeground(IntPtr hWnd) {
        keybd_event(0x12, 0, 0, UIntPtr.Zero);
        keybd_event(0x12, 0, 2, UIntPtr.Zero);
        return SetForegroundWindow(hWnd);
    }

    [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
    [StructLayout(LayoutKind.Sequential)] public struct POINT { public int X, Y; }

    // Top-level windows owned by a process, visible or not.
    public static List<IntPtr> WindowsOf(uint pid) {
        var found = new List<IntPtr>();
        EnumWindows((hWnd, l) => {
            uint p;
            GetWindowThreadProcessId(hWnd, out p);
            if (p == pid) found.Add(hWnd);
            return true;
        }, IntPtr.Zero);
        return found;
    }

    public static string TitleOf(IntPtr hWnd) {
        var sb = new StringBuilder(256);
        GetWindowText(hWnd, sb, sb.Capacity);
        return sb.ToString();
    }

    // Client area of the window in screen coordinates.
    public static Rectangle ClientScreenRect(IntPtr hWnd) {
        RECT cr;
        GetClientRect(hWnd, out cr);
        POINT origin = new POINT { X = 0, Y = 0 };
        ClientToScreen(hWnd, ref origin);
        return new Rectangle(origin.X, origin.Y, cr.Right - cr.Left, cr.Bottom - cr.Top);
    }

    // Renders the window with PW_RENDERFULLCONTENT and returns only its client area.
    public static Bitmap CaptureClient(IntPtr hWnd) {
        RECT wr;
        GetWindowRect(hWnd, out wr);
        int ww = wr.Right - wr.Left, wh = wr.Bottom - wr.Top;
        var full = new Bitmap(ww, wh, PixelFormat.Format32bppArgb);
        using (var g = Graphics.FromImage(full)) {
            IntPtr dc = g.GetHdc();
            PrintWindow(hWnd, dc, 2);
            g.ReleaseHdc(dc);
        }
        Rectangle client = ClientScreenRect(hWnd);
        var clip = new Rectangle(client.Left - wr.Left, client.Top - wr.Top, client.Width, client.Height);
        var result = full.Clone(clip, PixelFormat.Format32bppArgb);
        full.Dispose();
        return result;
    }

    // Number of distinct colours on a sparse grid of the bitmap. One colour means blank.
    public static int DistinctColors(Bitmap bmp) {
        var seen = new HashSet<int>();
        var rect = new Rectangle(0, 0, bmp.Width, bmp.Height);
        var data = bmp.LockBits(rect, ImageLockMode.ReadOnly, PixelFormat.Format32bppArgb);
        try {
            int stepX = Math.Max(1, bmp.Width / 160), stepY = Math.Max(1, bmp.Height / 120);
            int[] row = new int[bmp.Width];
            for (int y = 0; y < bmp.Height; y += stepY) {
                Marshal.Copy(IntPtr.Add(data.Scan0, y * data.Stride), row, 0, bmp.Width);
                for (int x = 0; x < bmp.Width; x += stepX) seen.Add(row[x]);
            }
        } finally {
            bmp.UnlockBits(data);
        }
        return seen.Count;
    }
}
}
'@ -ReferencedAssemblies System.Drawing
}

function Get-DirBytes([string]$Path) {
    if ([string]::IsNullOrEmpty($Path) -or -not (Test-Path -LiteralPath $Path)) { return [int64]0 }
    [int64]$sum = 0
    foreach ($f in [IO.Directory]::EnumerateFiles($Path, '*', [IO.SearchOption]::AllDirectories)) {
        $sum += (New-Object IO.FileInfo -ArgumentList $f).Length
    }
    return $sum
}

function Get-Summary([double[]]$Values) {
    $s = @($Values | Sort-Object)
    $n = $s.Count
    if ($n -eq 0) { return $null }
    if ($n % 2 -eq 1) { $med = $s[[int](($n - 1) / 2)] } else { $med = ($s[$n / 2 - 1] + $s[$n / 2]) / 2 }
    return [ordered]@{ median = [math]::Round($med, 2); min = [math]::Round($s[0], 2); max = [math]::Round($s[$n - 1], 2); n = $n; values = @($Values | ForEach-Object { [math]::Round($_, 2) }) }
}

function MB([double]$Bytes) { return [math]::Round($Bytes / 1MB, 2) }

# Runs a build command in a generated batch file, so quoting survives PowerShell.
# Returns the wall-clock seconds. A failed build is retried once, because the MSVC
# linker occasionally fails on a file an antivirus is still scanning; the retry is
# recorded in buildRetries and its time is the one returned. A second failure
# throws with the log path.
function Invoke-Build([string]$Dir, [string]$Command, [hashtable]$EnvVars, [string]$Name) {
    try {
        return Invoke-BuildOnce $Dir $Command $EnvVars $Name
    } catch {
        $R.buildRetries += @("$Name ($($_.Exception.Message))")
        return Invoke-BuildOnce $Dir $Command $EnvVars "$Name-retry"
    }
}

function Invoke-BuildOnce([string]$Dir, [string]$Command, [hashtable]$EnvVars, [string]$Name) {
    $log = Join-Path $work "logs\$Name.log"
    $bat = Join-Path $work "logs\$Name.cmd"
    Set-Content -LiteralPath $bat -Encoding ASCII -Value @(
        '@echo off',
        "cd /d `"$Dir`"",
        "$Command > `"$log`" 2>&1",
        'exit /b %ERRORLEVEL%')
    $old = @{}
    foreach ($k in $EnvVars.Keys) {
        $old[$k] = [Environment]::GetEnvironmentVariable($k)
        [Environment]::SetEnvironmentVariable($k, $EnvVars[$k])
    }
    try {
        $sw = [Diagnostics.Stopwatch]::StartNew()
        $p = Start-Process -FilePath 'cmd.exe' -ArgumentList '/d', '/c', "`"$bat`"" -Wait -PassThru -WindowStyle Hidden
        $sw.Stop()
    } finally {
        foreach ($k in $EnvVars.Keys) { [Environment]::SetEnvironmentVariable($k, $old[$k]) }
    }
    if ($p.ExitCode -ne 0) { throw "Build failed (exit $($p.ExitCode)): $Command. Log: $log" }
    return $sw.Elapsed.TotalSeconds
}

# Replaces a file's bytes and returns nothing; used for the temporary source edits.
function Write-Bytes([string]$Path, [byte[]]$Bytes) { [IO.File]::WriteAllBytes($Path, $Bytes) }

function Get-ContextSwitches([int]$ProcessId) {
    $rows = Get-CimInstance -ClassName Win32_PerfRawData_PerfProc_Thread -Filter "IDProcess=$ProcessId"
    return [double](($rows | Measure-Object -Property ContextSwitchesPersec -Sum).Sum)
}

function Get-GpuMemory([int]$ProcessId) {
    try {
        $prefix = "pid_$($ProcessId)_"
        $ded = (Get-Counter -Counter '\GPU Process Memory(*)\Dedicated Usage' -ErrorAction Stop).CounterSamples |
            Where-Object { $_.InstanceName -like "$prefix*" } | Measure-Object -Property CookedValue -Sum
        $shared = (Get-Counter -Counter '\GPU Process Memory(*)\Shared Usage' -ErrorAction Stop).CounterSamples |
            Where-Object { $_.InstanceName -like "$prefix*" } | Measure-Object -Property CookedValue -Sum
        $dedMB = $null; $sharedMB = $null
        if ($ded.Count -gt 0) { $dedMB = MB $ded.Sum }
        if ($shared.Count -gt 0) { $sharedMB = MB $shared.Sum }
        return [ordered]@{ dedicatedMB = $dedMB; sharedMB = $sharedMB }
    } catch {
        return [ordered]@{ dedicatedMB = $null; sharedMB = $null; note = 'GPU Process Memory counters not available' }
    }
}

# CPU (percent of one core) and context switches per second over the action.
function Measure-Cpu($Proc, [scriptblock]$Action) {
    $Proc.Refresh()
    $cpu0 = $Proc.TotalProcessorTime.TotalMilliseconds
    $cs0 = Get-ContextSwitches $Proc.Id
    $sw = [Diagnostics.Stopwatch]::StartNew()
    & $Action
    $sw.Stop()
    $Proc.Refresh()
    $cpu1 = $Proc.TotalProcessorTime.TotalMilliseconds
    $cs1 = Get-ContextSwitches $Proc.Id
    $sec = $sw.Elapsed.TotalSeconds
    return [ordered]@{
        cpuPct = [math]::Round(($cpu1 - $cpu0) / 1000 / $sec * 100, 3)
        ctxPerSec = [math]::Round(($cs1 - $cs0) / $sec, 1)
        seconds = [math]::Round($sec, 2)
    }
}

function Move-Pointer([IntPtr]$Hwnd, [int]$Seconds) {
    $r = [Cmp.Win32]::ClientScreenRect($Hwnd)
    $cx = $r.Left + $r.Width / 2
    $cy = $r.Top + $r.Height / 2
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ($sw.Elapsed.TotalSeconds -lt $Seconds) {
        $t = $sw.Elapsed.TotalSeconds
        $x = $cx + 0.4 * $r.Width * [math]::Sin(2 * [math]::PI * $t / 3.1)
        $y = $cy + 0.4 * $r.Height * [math]::Sin(2 * [math]::PI * $t / 2.3)
        [void][Cmp.Win32]::SetCursorPos([int]$x, [int]$y)
        [Threading.Thread]::Sleep(16)
    }
}

function Resize-Window([IntPtr]$Hwnd, [int]$Seconds) {
    # SWP_NOMOVE | SWP_NOZORDER | SWP_NOACTIVATE
    $flags = [uint32](0x0002 -bor 0x0004 -bor 0x0010)
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ($sw.Elapsed.TotalSeconds -lt $Seconds) {
        $t = $sw.Elapsed.TotalSeconds
        $w = [int](800 + 240 * [math]::Sin(2 * [math]::PI * $t / 4))
        $h = [int](600 + 180 * [math]::Sin(2 * [math]::PI * $t / 5))
        [void][Cmp.Win32]::SetWindowPos($Hwnd, [IntPtr]::Zero, 0, 0, $w, $h, $flags)
        [Threading.Thread]::Sleep(16)
    }
    [void][Cmp.Win32]::SetWindowPos($Hwnd, [IntPtr]::Zero, 0, 0, 800, 600, $flags)
}

# Starts the app and waits for a visible window titled "Hello" with a non-blank
# client area. Times are from the launch call. The caller disposes of the capture.
function Start-Hello($App, [string]$Exe) {
    $clock = [Diagnostics.Stopwatch]::StartNew()
    $p = Start-Process -FilePath $Exe -PassThru
    [void]$started.Add($p)
    $deadline = (Get-Date).AddSeconds(60)
    $hwnd = [IntPtr]::Zero
    while ($hwnd -eq [IntPtr]::Zero -and (Get-Date) -lt $deadline) {
        foreach ($w in [Cmp.Win32]::WindowsOf([uint32]$p.Id)) {
            if ([Cmp.Win32]::IsWindowVisible($w) -and [Cmp.Win32]::TitleOf($w) -eq 'Hello') { $hwnd = $w; break }
        }
        if ($hwnd -eq [IntPtr]::Zero) { [Threading.Thread]::Sleep(5) }
    }
    if ($hwnd -eq [IntPtr]::Zero) { throw "$($App.key): no visible 'Hello' window within 60 s" }
    $visibleMs = $clock.Elapsed.TotalMilliseconds

    $bmp = $null
    $distinct = 0
    while ((Get-Date) -lt $deadline) {
        $bmp = [Cmp.Win32]::CaptureClient($hwnd)
        $distinct = [Cmp.Win32]::DistinctColors($bmp)
        if ($distinct -gt 1) { break }
        $bmp.Dispose()
        [Threading.Thread]::Sleep(5)
    }
    $paintedMs = $clock.Elapsed.TotalMilliseconds
    if ($distinct -le 1) { throw "$($App.key): window never showed content" }
    return [pscustomobject]@{ proc = $p; hwnd = $hwnd; bmp = $bmp; visibleMs = $visibleMs; paintedMs = $paintedMs }
}

function Stop-Hello($Launch) {
    $Launch.bmp.Dispose()
    Stop-Process -Id $Launch.proc.Id -Force -ErrorAction SilentlyContinue
    [void]$Launch.proc.WaitForExit(10000)
    [Threading.Thread]::Sleep(500)
}

# Copies the app into its own temporary folder and launches the copy: once as a new
# file (an antivirus scans a file it has not seen before it runs), then
# $WarmLaunches more times from the same file, as on every later start. The last
# launch stays open for the idle, pointer and resize samples. Returns one result.
function Measure-App($App, [int]$Run) {
    $dir = Join-Path $work "run\$($App.key)-$Run"
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    $exe = Join-Path $dir (Split-Path -Leaf $App.exe)
    Copy-Item -LiteralPath $App.exe -Destination $exe

    $first = Start-Hello $App $exe
    Stop-Hello $first
    $warmVisible = @(); $warmPainted = @()
    for ($k = 1; $k -le $WarmLaunches; $k++) {
        $launch = Start-Hello $App $exe
        $warmVisible += $launch.visibleMs
        $warmPainted += $launch.paintedMs
        if ($k -lt $WarmLaunches) { Stop-Hello $launch }
    }
    $p = $launch.proc
    $hwnd = $launch.hwnd
    $bmp = $launch.bmp
    if ($App.shot) { $bmp.Save((Join-Path $ShotDir $App.shot), [Drawing.Imaging.ImageFormat]::Png) }
    $client = "$($bmp.Width)x$($bmp.Height)"
    $bmp.Dispose()
    $dpi = [Cmp.Win32]::GetDpiForWindow($hwnd)

    [Threading.Thread]::Sleep($SettleSeconds * 1000)
    [void][Cmp.Win32]::ForceForeground($hwnd)
    [Threading.Thread]::Sleep(500)
    $focusedOk = ([Cmp.Win32]::GetForegroundWindow() -eq $hwnd)
    $idleFocused = Measure-Cpu $p { [Threading.Thread]::Sleep($IdleSeconds * 1000) }

    # The taskbar takes the focus; the app stays visible and is no longer active.
    $taskbar = [Cmp.Win32]::FindWindow('Shell_TrayWnd', $null)
    [void][Cmp.Win32]::ForceForeground($taskbar)
    [Threading.Thread]::Sleep(500)
    [Threading.Thread]::Sleep($SettleSeconds * 1000)
    $unfocusedOk = ([Cmp.Win32]::GetForegroundWindow() -ne $hwnd)
    $idleUnfocused = Measure-Cpu $p { [Threading.Thread]::Sleep($IdleSeconds * 1000) }

    $p.Refresh()
    $memory = [ordered]@{
        privateMB = MB $p.PrivateMemorySize64
        workingSetMB = MB $p.WorkingSet64
        threads = $p.Threads.Count
        handles = $p.HandleCount
    }
    $gpu = Get-GpuMemory $p.Id
    $memory.gpuDedicatedMB = $gpu.dedicatedMB
    $memory.gpuSharedMB = $gpu.sharedMB

    $pointer = $null
    $resize = $null
    if (-not $SkipInteraction) {
        $cursor = New-Object Cmp.Win32+POINT
        [void][Cmp.Win32]::GetCursorPos([ref]$cursor)
        [void][Cmp.Win32]::ForceForeground($hwnd)
        [Threading.Thread]::Sleep(500)
        $pointer = Measure-Cpu $p { Move-Pointer $hwnd $MoveSeconds }
        $resize = Measure-Cpu $p { Resize-Window $hwnd $MoveSeconds }
        [void][Cmp.Win32]::SetCursorPos($cursor.X, $cursor.Y)
    }

    Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
    return [ordered]@{
        app = $App.key
        run = $Run
        firstVisibleMs = [math]::Round($first.visibleMs, 1)
        firstPaintedMs = [math]::Round($first.paintedMs, 1)
        warmVisibleMs = @($warmVisible | ForEach-Object { [math]::Round($_, 1) })
        warmPaintedMs = @($warmPainted | ForEach-Object { [math]::Round($_, 1) })
        client = $client
        dpi = $dpi
        focusedOk = $focusedOk
        unfocusedOk = $unfocusedOk
        idleFocused = $idleFocused
        idleUnfocused = $idleUnfocused
        memory = $memory
        pointer = $pointer
        resize = $resize
    }
}

try {
    # Environment
    Push-Location $GpuiRoot
    try {
        $rustcVersion = (& rustc -V) -join ' '
        $cargoVersion = (& cargo -V) -join ' '
    } finally { Pop-Location }
    $R.environment = [ordered]@{
        os = (Get-CimInstance Win32_OperatingSystem).Caption
        logicalCores = [Environment]::ProcessorCount
        ramGB = [math]::Round((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1GB, 1)
        goVersion = ((& go version) -join ' ')
        rustcVersion = $rustcVersion
        cargoVersion = $cargoVersion
        runs = $Runs
    }

    $gpuiPins = Select-String -LiteralPath (Join-Path $GpuiRoot 'Cargo.toml') -Pattern 'rev = "([0-9a-f]+)"' | ForEach-Object { $_.Matches[0].Groups[1].Value } | Select-Object -First 1
    $R.environment.gpuiRev = $gpuiPins

    # Project size and dependencies: jamb
    $goFiles = Get-ChildItem -LiteralPath $goHello -Filter '*.go' -File
    $R.jambSource = [ordered]@{
        files = @($goFiles).Count
        lines = [int](($goFiles | ForEach-Object { [IO.File]::ReadAllLines($_.FullName).Length }) | Measure-Object -Sum).Sum
        manifests = 'go.mod, go.sum'
    }
    # GOWORK=off so the module lists are hello's own, not the whole workspace's.
    $savedWork = $env:GOWORK
    $env:GOWORK = 'off'
    Push-Location $goHello
    try {
        $depLines = @(& go list -deps -f '{{if not .Standard}}{{.ImportPath}}|{{with .Module}}{{.Path}}{{end}}|{{.Dir}}{{end}}' .)
        $buildList = @(& go list -m -f '{{.Path}}|{{.Version}}|{{.Dir}}' all)
    } finally {
        Pop-Location
        $env:GOWORK = $savedWork
    }
    $jambModule = 'github.com/tyler-builds/jamb'
    $helloModule = "$jambModule/examples/hello"
    $jambDirs = @{}
    $usedModules = @{}
    foreach ($line in $depLines) {
        $parts = $line -split '\|'
        if ($parts[1] -eq $jambModule -or $parts[1] -like "$jambModule/*") {
            if ($parts[1] -ne $helloModule) { $jambDirs[$parts[2]] = $true }
        } elseif ($parts[1]) { $usedModules[$parts[1]] = $true }
    }
    $jambFiles = 0; $jambBytes = [int64]0; $jambLines = 0
    foreach ($d in $jambDirs.Keys) {
        $goFilesInDir = @(Get-ChildItem -LiteralPath $d -Filter '*.go' -File | Where-Object { $_.Name -notlike '*_test.go' })
        foreach ($f in $goFilesInDir) {
            $jambFiles++; $jambBytes += $f.Length; $jambLines += [IO.File]::ReadAllLines($f.FullName).Length
        }
    }
    $R.jambDependencySource = [ordered]@{
        packagesUsedByHello = $depLines.Count
        jambPackageDirs = $jambDirs.Count
        jambNonTestGoFiles = $jambFiles
        jambNonTestBytesMB = MB $jambBytes
        jambNonTestLines = $jambLines
        thirdPartyModulesUsed = $usedModules.Count
    }
    $buildListBytes = [int64]0
    $thirdParty = 0
    $goModOnly = 0
    foreach ($line in $buildList) {
        $parts = $line -split '\|'
        if ($parts[0] -eq $jambModule -or $parts[0] -eq "$jambModule/examples/hello") { continue }
        if (-not $parts[2]) { $goModOnly++; continue }
        $thirdParty++
        $buildListBytes += Get-DirBytes $parts[2]
    }
    $usedBytes = [int64]0
    foreach ($line in $buildList) {
        $parts = $line -split '\|'
        if ($usedModules.ContainsKey($parts[0])) { $usedBytes += Get-DirBytes $parts[2] }
    }
    $R.jambDependencySource.buildListModules = $thirdParty
    $R.jambDependencySource.buildListGoModOnly = $goModOnly
    $R.jambDependencySource.buildListSourceMB = MB $buildListBytes
    $R.jambDependencySource.usedModulesSourceMB = MB $usedBytes
    $R.jambDependencySource.goModuleCacheNote = 'module directories in GOMODCACHE (go list -m all, excluding jamb and the hello module)'
    $R.jambGoRoot = [ordered]@{ goRootMB = MB (Get-DirBytes ((& go env GOROOT).Trim())) }

    # Project size and dependencies: GPUI
    $rustFiles = Get-ChildItem -LiteralPath (Join-Path $GpuiRoot 'src') -Filter '*.rs' -File
    $R.gpuiSource = [ordered]@{
        files = @($rustFiles).Count
        lines = [int](($rustFiles | ForEach-Object { [IO.File]::ReadAllLines($_.FullName).Length }) | Measure-Object -Sum).Sum
        manifests = 'Cargo.toml, Cargo.lock, rust-toolchain.toml'
    }
    $lockPath = Join-Path $GpuiRoot 'Cargo.lock'
    $lockPackages = (Select-String -LiteralPath $lockPath -Pattern '^\[\[package\]\]').Count
    $meta = (& cargo metadata --format-version 1 --locked --manifest-path (Join-Path $GpuiRoot 'Cargo.toml') | Out-String) | ConvertFrom-Json
    $registry = @($meta.packages | Where-Object { $_.source -like 'registry+*' })
    $git = @($meta.packages | Where-Object { $_.source -like 'git+*' })
    $local = @($meta.packages | Where-Object { -not $_.source })
    $registryBytes = [int64]0
    foreach ($pkg in $registry) { $registryBytes += Get-DirBytes (Split-Path -Parent $pkg.manifest_path) }
    $gitBytes = [int64]0
    foreach ($pkg in $git) { $gitBytes += Get-DirBytes (Split-Path -Parent $pkg.manifest_path) }
    $R.gpuiDependencySource = [ordered]@{
        cargoLockPackages = $lockPackages
        resolvedRegistryCrates = $registry.Count
        resolvedGitCrates = $git.Count
        localCrates = $local.Count
        registrySourceMB = MB $registryBytes
        gitCrateDirsSourceMB = MB $gitBytes
        note = 'crate directories only (the zed git checkout is a monorepo; only the crates in the graph are counted)'
    }
    $gitCheckouts = @($git | ForEach-Object { if ($_.source -match '#([0-9a-f]+)$') { $Matches[1] } } | Sort-Object -Unique)
    $R.gpuiDependencySource.gitRevisionsInGraph = $gitCheckouts.Count
    $rustupHome = if ($env:RUSTUP_HOME) { $env:RUSTUP_HOME } else { Join-Path $env:USERPROFILE '.rustup' }
    $R.gpuiToolchain = [ordered]@{ toolchain1_95_0MB = MB (Get-DirBytes (Join-Path $rustupHome 'toolchains\1.95.0-x86_64-pc-windows-msvc')) }

    # Builds and incremental builds: jamb. By default one build each with the normal
    # Go build cache; with -IncludeCleanBuilds, $Runs timed builds each with an empty cache.
    $goRelFlags = '-trimpath -ldflags "-s -w -H windowsgui"'
    if (-not $SkipGo) {
        $R.jambBuild = [ordered]@{}
        $goEnv = @{ rel = @{}; dbg = @{} }
        if ($IncludeCleanBuilds) {
            $goRelCache = @(); $goDbgCache = @(); $cleanRel = @(); $cleanDbg = @()
            for ($i = 1; $i -le $Runs; $i++) {
                $cache = Join-Path $work "gocache-rel-$i"
                $cleanRel += Invoke-Build $JambRoot "go build $goRelFlags -o `"$work\bin\jamb-rel.exe`" ./examples/hello" @{ GOCACHE = $cache } "go-clean-rel-$i"
                $goRelCache += MB (Get-DirBytes $cache)
                $cache = Join-Path $work "gocache-dbg-$i"
                $cleanDbg += Invoke-Build $JambRoot "go build -o `"$work\bin\jamb-default.exe`" ./examples/hello" @{ GOCACHE = $cache } "go-clean-dbg-$i"
                $goDbgCache += MB (Get-DirBytes $cache)
                if ($i -lt $Runs) { Remove-Item -LiteralPath (Join-Path $work "gocache-dbg-$i") -Recurse -Force; Remove-Item -LiteralPath (Join-Path $work "gocache-rel-$i") -Recurse -Force }
            }
            $goEnv.rel = @{ GOCACHE = (Join-Path $work "gocache-rel-$Runs") }
            $goEnv.dbg = @{ GOCACHE = (Join-Path $work "gocache-dbg-$Runs") }
            $R.jambBuild.cleanReleaseSeconds = Get-Summary $cleanRel
            $R.jambBuild.cleanDefaultSeconds = Get-Summary $cleanDbg
            $R.jambBuild.goCacheAfterReleaseMB = Get-Summary $goRelCache
            $R.jambBuild.goCacheAfterDefaultMB = Get-Summary $goDbgCache
        } else {
            Invoke-Build $JambRoot "go build $goRelFlags -o `"$work\bin\jamb-rel.exe`" ./examples/hello" @{} 'go-rel' | Out-Null
            Invoke-Build $JambRoot "go build -o `"$work\bin\jamb-default.exe`" ./examples/hello" @{} 'go-dbg' | Out-Null
        }
        $specBytesCmd = "go build -trimpath -ldflags `"-s -w`" -o `"$work\bin\jamb-spec.exe`" ./examples/hello"
        Invoke-Build $JambRoot $specBytesCmd $goEnv.rel 'go-spec' | Out-Null

        # Incremental: a text change (new literal), then whitespace-only changes, on a warm cache.
        $origGo = [IO.File]::ReadAllBytes($goMain)
        $origText = [Text.Encoding]::UTF8.GetString($origGo)
        $incRelText = @(); $incRelWs = @(); $incDbgText = @(); $incDbgWs = @()
        try {
            foreach ($mode in @('rel', 'dbg')) {
                $cmd = if ($mode -eq 'rel') { "go build $goRelFlags -o `"$work\bin\inc-jamb.exe`" ./examples/hello" } else { "go build -o `"$work\bin\inc-jamb.exe`" ./examples/hello" }
                for ($i = 1; $i -le $Runs; $i++) {
                    Write-Bytes $goMain ([Text.Encoding]::UTF8.GetBytes($origText.Replace('Hello, world!', "Hello, world! $i")))
                    $t = Invoke-Build $JambRoot $cmd $goEnv[$mode] "go-inc-text-$mode-$i"
                    if ($mode -eq 'rel') { $incRelText += $t } else { $incDbgText += $t }
                    Write-Bytes $goMain ([Text.Encoding]::UTF8.GetBytes($origText + ("`n" * $i)))
                    $t = Invoke-Build $JambRoot $cmd $goEnv[$mode] "go-inc-ws-$mode-$i"
                    if ($mode -eq 'rel') { $incRelWs += $t } else { $incDbgWs += $t }
                }
            }
        } finally {
            Write-Bytes $goMain $origGo
        }
        $R.jambBuild.incrementalReleaseTextSeconds = Get-Summary $incRelText
        $R.jambBuild.incrementalReleaseWhitespaceSeconds = Get-Summary $incRelWs
        $R.jambBuild.incrementalDefaultTextSeconds = Get-Summary $incDbgText
        $R.jambBuild.incrementalDefaultWhitespaceSeconds = Get-Summary $incDbgWs
        $R.jambBinary = [ordered]@{
            releaseStrippedMB = MB (Get-Item -LiteralPath "$work\bin\jamb-spec.exe").Length
            releaseGuiSubsystemMB = MB (Get-Item -LiteralPath "$work\bin\jamb-rel.exe").Length
            defaultMB = MB (Get-Item -LiteralPath "$work\bin\jamb-default.exe").Length
            note = 'releaseStrippedMB uses -trimpath -ldflags "-s -w" exactly; releaseGuiSubsystemMB adds -H windowsgui, which is what the runtime phase launches'
        }
        Get-ChildItem -LiteralPath $work -Filter 'gocache-*' -Directory | Remove-Item -Recurse -Force
    }

    # Builds and incremental builds: GPUI, release and debug profiles. By default one
    # build each in $GpuiTargetDir; with -IncludeCleanBuilds, $Runs timed builds each
    # in an empty target directory.
    if (-not $SkipRust) {
        $R.gpuiBuild = [ordered]@{}
        $lastTarget = @{}
        $flags = @{ rel = '--release'; dbg = '' }
        foreach ($mode in @('rel', 'dbg')) {
            if ($IncludeCleanBuilds) {
                $times = @(); $sizes = @()
                for ($i = 1; $i -le $Runs; $i++) {
                    $target = Join-Path $work "target-$mode-$i"
                    $times += Invoke-Build $GpuiRoot "cargo build $($flags[$mode]) --locked" @{ CARGO_TARGET_DIR = $target } "rust-clean-$mode-$i"
                    $sizes += MB (Get-DirBytes $target)
                    if ($i -lt $Runs) { Remove-Item -LiteralPath $target -Recurse -Force } else { $lastTarget[$mode] = $target }
                }
                $label = if ($mode -eq 'rel') { 'Release' } else { 'Debug' }
                $R.gpuiBuild["clean$($label)Seconds"] = Get-Summary $times
                $R.gpuiBuild["targetAfter$($label)MB"] = Get-Summary $sizes
            } else {
                Invoke-Build $GpuiRoot "cargo build $($flags[$mode]) --locked" @{ CARGO_TARGET_DIR = $GpuiTargetDir } "rust-$mode" | Out-Null
                $lastTarget[$mode] = $GpuiTargetDir
            }
        }
        Copy-Item -LiteralPath (Join-Path $lastTarget['rel'] 'release\gpui-hello.exe') -Destination "$work\bin\gpui-rel.exe"
        Copy-Item -LiteralPath (Join-Path $lastTarget['dbg'] 'debug\gpui-hello.exe') -Destination "$work\bin\gpui-default.exe"
        $R.gpuiBinary = [ordered]@{
            releaseMB = MB (Get-Item -LiteralPath "$work\bin\gpui-rel.exe").Length
            debugMB = MB (Get-Item -LiteralPath "$work\bin\gpui-default.exe").Length
            note = 'Windows release builds keep debug info in a separate .pdb, so the .exe sizes exclude it'
        }

        # Incremental on a warm target: a text change, then whitespace.
        $origRust = [IO.File]::ReadAllBytes($rustMain)
        $origRustText = [Text.Encoding]::UTF8.GetString($origRust)
        $incRelText = @(); $incRelWs = @(); $incDbgText = @(); $incDbgWs = @()
        try {
            foreach ($mode in @('rel', 'dbg')) {
                $targetEnv = @{ CARGO_TARGET_DIR = $lastTarget[$mode] }
                for ($i = 1; $i -le $Runs; $i++) {
                    Write-Bytes $rustMain ([Text.Encoding]::UTF8.GetBytes($origRustText.Replace('"Hello, world!"', "`"Hello, world! $i`"")))
                    $t = Invoke-Build $GpuiRoot "cargo build $($flags[$mode]) --locked" $targetEnv "rust-inc-text-$mode-$i"
                    if ($mode -eq 'rel') { $incRelText += $t } else { $incDbgText += $t }
                    Write-Bytes $rustMain ([Text.Encoding]::UTF8.GetBytes($origRustText + ("`n" * $i)))
                    $t = Invoke-Build $GpuiRoot "cargo build $($flags[$mode]) --locked" $targetEnv "rust-inc-ws-$mode-$i"
                    if ($mode -eq 'rel') { $incRelWs += $t } else { $incDbgWs += $t }
                }
            }
        } finally {
            Write-Bytes $rustMain $origRust
        }
        $R.gpuiBuild.incrementalReleaseTextSeconds = Get-Summary $incRelText
        $R.gpuiBuild.incrementalReleaseWhitespaceSeconds = Get-Summary $incRelWs
        $R.gpuiBuild.incrementalDebugTextSeconds = Get-Summary $incDbgText
        $R.gpuiBuild.incrementalDebugWhitespaceSeconds = Get-Summary $incDbgWs
    }

    # First build on a new machine: dependency download into temporary caches
    if ($IncludeDownload) {
        $dl = [ordered]@{}
        if (-not $SkipGo) {
            $modcache = Join-Path $work 'gomodcache'
            $sw = [Diagnostics.Stopwatch]::StartNew()
            Invoke-Build $goHello 'go mod download' @{ GOMODCACHE = $modcache; GOWORK = 'off' } 'go-download' | Out-Null
            $dl.jambDownloadSeconds = [math]::Round($sw.Elapsed.TotalSeconds, 1)
            $dl.jambDownloadedMB = MB (Get-DirBytes $modcache)
        }
        if (-not $SkipRust) {
            $cargoHome = Join-Path $work 'cargohome'
            try {
                $t = Invoke-Build $GpuiRoot 'cargo fetch --locked' @{ CARGO_HOME = $cargoHome } 'rust-download'
                $dl.gpuiDownloadSeconds = [math]::Round($t, 1)
                $dl.gpuiDownloadedMB = MB (Get-DirBytes $cargoHome)
            } catch {
                $dl.gpuiDownloadNote = "did not complete: $($_.Exception.Message)"
            }
        }
        $R.firstBuildDownload = $dl
    }

    # Runtime
    $apps = @()
    if (-not $SkipGo) {
        $apps += [pscustomobject]@{ key = 'jamb'; exe = "$work\bin\jamb-rel.exe"; shot = 'comparison-jamb-hello.png' }
    }
    if (-not $SkipRust) {
        $apps += [pscustomobject]@{ key = 'gpui'; exe = "$work\bin\gpui-rel.exe"; shot = 'comparison-gpui-hello.png' }
    }
    # Both apps are dark: GPUI's hello sets its colors, and jamb's hello follows
    # Windows' app mode, so it is pinned to the dark mode for the run.
    $env:JAMB_DARK = '1'
    $runtime = @()
    for ($run = 1; $run -le $Runs; $run++) {
        foreach ($app in $apps) {
            Write-Host "runtime: $($app.key) run $run"
            $runtime += Measure-App $app $run
        }
    }
    $R.runtime = $runtime
    $summaries = [ordered]@{}
    foreach ($app in $apps) {
        $rows = @($runtime | Where-Object { $_.app -eq $app.key })
        $summaries[$app.key] = [ordered]@{
            firstVisibleMs = Get-Summary ([double[]]($rows | ForEach-Object { $_.firstVisibleMs }))
            firstPaintedMs = Get-Summary ([double[]]($rows | ForEach-Object { $_.firstPaintedMs }))
            warmVisibleMs = Get-Summary ([double[]]($rows | ForEach-Object { $_.warmVisibleMs }))
            warmPaintedMs = Get-Summary ([double[]]($rows | ForEach-Object { $_.warmPaintedMs }))
            idleFocusedCpuPct = Get-Summary ([double[]]($rows | ForEach-Object { $_.idleFocused.cpuPct }))
            idleFocusedCtxPerSec = Get-Summary ([double[]]($rows | ForEach-Object { $_.idleFocused.ctxPerSec }))
            idleUnfocusedCpuPct = Get-Summary ([double[]]($rows | ForEach-Object { $_.idleUnfocused.cpuPct }))
            idleUnfocusedCtxPerSec = Get-Summary ([double[]]($rows | ForEach-Object { $_.idleUnfocused.ctxPerSec }))
            privateMB = Get-Summary ([double[]]($rows | ForEach-Object { $_.memory.privateMB }))
            workingSetMB = Get-Summary ([double[]]($rows | ForEach-Object { $_.memory.workingSetMB }))
            threads = Get-Summary ([double[]]($rows | ForEach-Object { $_.memory.threads }))
            handles = Get-Summary ([double[]]($rows | ForEach-Object { $_.memory.handles }))
            gpuDedicatedMB = Get-Summary ([double[]]($rows | Where-Object { $null -ne $_.memory.gpuDedicatedMB } | ForEach-Object { $_.memory.gpuDedicatedMB }))
            gpuSharedMB = Get-Summary ([double[]]($rows | Where-Object { $null -ne $_.memory.gpuSharedMB } | ForEach-Object { $_.memory.gpuSharedMB }))
        }
        if (-not $SkipInteraction) {
            $summaries[$app.key].pointerCpuPct = Get-Summary ([double[]]($rows | ForEach-Object { $_.pointer.cpuPct }))
            $summaries[$app.key].pointerCtxPerSec = Get-Summary ([double[]]($rows | ForEach-Object { $_.pointer.ctxPerSec }))
            $summaries[$app.key].resizeCpuPct = Get-Summary ([double[]]($rows | ForEach-Object { $_.resize.cpuPct }))
            $summaries[$app.key].resizeCtxPerSec = Get-Summary ([double[]]($rows | ForEach-Object { $_.resize.ctxPerSec }))
        }
    }
    $R.runtimeSummary = $summaries
    $R.totalSeconds = [math]::Round($total.Elapsed.TotalSeconds, 0)
    $json = $R | ConvertTo-Json -Depth 8
    if ($OutFile) { Set-Content -LiteralPath $OutFile -Value $json -Encoding UTF8 }
    Write-Host $json
} finally {
    foreach ($p in $started) {
        if (-not $p.HasExited) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
    }
    if (Test-Path -LiteralPath $work) { Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue }
}
