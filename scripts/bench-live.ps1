#Requires -Version 5.1
<#
.SYNOPSIS
    Measures jamb's live performance on the secondary monitor, for BENCHMARKS.md.

.DESCRIPTION
    Builds examples/hello and examples/demo and measures them in windows that open on
    the non-primary monitor at -Position, without activation:

      - binary sizes: the stripped build (-trimpath -ldflags "-s -w -H windowsgui") and
        the default build (-H windowsgui), which is built but not run;
      - startup: a fresh copy of each executable, then warm launches of that copy. Times
        from the launch call to a visible window and to the first painted frame, and the
        app's own JAMB_TRACE line ("first frame presented ... after process start");
      - idle: after settling, CPU as a percentage of one core over -IdleSeconds, context
        switches per second, private bytes, working set, threads, handles and GPU memory;
      - scenarios: a demo run under JAMB_FRAMESTATS, with a JAMB_SCRIPT loop of about
        -ScenarioSeconds. The loops scroll the Activity list, move the pointer over the
        Activity rows, type in the Activity filter, run a continuous animation (the easing
        rows of the Components page's Motion section), switch pages, and type and scroll
        in the code editor;
      - resize: SetWindowPos on the demo window for -ScenarioSeconds, under JAMB_FRAMESTATS,
        with CPU and context switches;
      - stress: JAMB_STRESS=rebuild and paint, each with vsync on and with JAMB_VSYNC=0;
      - a second window: a project window, opened from the Projects page, and the idle
        CPU of the two windows;
      - soak (only with -SoakMinutes above 0): a JAMB_HEAPLOG run of a loop that scrolls,
        switches pages and types, with the start, end and maximum of each memory counter
        and the least-squares slope per hour.

    Scripted input (JAMB_SCRIPT) is delivered inside the app, not by the operating system,
    so the mouse cursor, the keyboard focus and the window activation are never touched.
    The scenario coordinates are logical pixels in the demo's default 1200 x 760 window.
    They come from the layout of views.App, found with ui.Runtime.Find in a headless
    layout pass at that size, and each one is commented where it is used.

    Nothing runs on the primary monitor: every window is checked to lie wholly off it,
    and the run stops if one does not. The script refuses to start while a demo process
    is already running, because the demo's single-instance check would hand the launch
    to that process. Every process the script starts is stopped in a finally block.

    Output: OutDir\raw\run<N>\ holds the raw results of each run (JSON, and the frame
    CSVs and stderr of the scenario runs), OutDir\summary.json holds the medians and
    ranges with the machine information, and OutDir\summary.md holds the same as tables.
    Each figure is a median across the runs, with the range in brackets.

    Requirements: Windows 10 or later, Go 1.27 or later, a secondary monitor, and the
    jamb repository. Run it with nothing else heavy on the machine, and with the video
    on the primary monitor if that is the scenario being measured.

.PARAMETER Runs
    Repetitions of every measurement. Default 3.

.PARAMETER Position
    Logical top-left corner of the first window of each app, as X,Y. It must lie on a
    non-primary monitor. Default -1600,150.

.PARAMETER SoakMinutes
    Length of the soak run. Default 0, which skips it.

.PARAMETER OutDir
    Where the results are written. Default: a folder named jamb-bench-live with the date
    and time, in %TEMP%.

.PARAMETER Skip
    Measurements to skip: startup, idle, scenarios, resize, stress, multiwindow, soak.

.PARAMETER Note
    A note recorded in the results, for example "video playing on primary monitor".

.PARAMETER WarmLaunches
    Warm launches per run, after the first launch of a fresh copy. Default 3.

.PARAMETER SettleSeconds
    Pause after a window is painted, before idle samples. Default 8.

.PARAMETER IdleSeconds
    Length of each idle sample, and of the second-window sample. Default 60.

.PARAMETER ScenarioSeconds
    Length of each scenario, resize and stress-style sample. Default 10.

.PARAMETER JambRoot
    Root of the jamb repository. Default: the parent of this script's folder.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\bench-live.ps1 -Note "video playing on primary monitor"

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\bench-live.ps1 -Runs 1 -Skip idle,soak -OutDir $env:TEMP\live
#>
[CmdletBinding()]
param(
    [ValidateRange(1, 20)][int]$Runs = 3,
    [string]$Position = '-1600,150',
    [ValidateRange(0, 1440)][int]$SoakMinutes = 0,
    [string]$OutDir = '',
    [ValidateSet('startup', 'idle', 'scenarios', 'resize', 'stress', 'multiwindow', 'soak')][string[]]$Skip = @(),
    [string]$Note = '',
    [ValidateRange(1, 20)][int]$WarmLaunches = 3,
    [ValidateRange(0, 600)][int]$SettleSeconds = 8,
    [ValidateRange(5, 600)][int]$IdleSeconds = 60,
    [ValidateRange(3, 120)][int]$ScenarioSeconds = 10,
    [string]$JambRoot = ''
)

$ErrorActionPreference = 'Stop'
$total = [Diagnostics.Stopwatch]::StartNew()

if (-not $JambRoot) { $JambRoot = Split-Path -Parent $PSScriptRoot }
$JambRoot = (Resolve-Path -LiteralPath $JambRoot).Path
if (-not $OutDir) { $OutDir = Join-Path $env:TEMP ('jamb-bench-live-' + (Get-Date -Format 'yyyy-MM-dd-HHmm')) }
New-Item -ItemType Directory -Path $OutDir -Force | Out-Null
$OutDir = (Resolve-Path -LiteralPath $OutDir).Path
$rawDir = Join-Path $OutDir 'raw'
New-Item -ItemType Directory -Path $rawDir -Force | Out-Null
$work = Join-Path ([IO.Path]::GetTempPath()) ('bench-live-' + [guid]::NewGuid().ToString('N').Substring(0, 8))
New-Item -ItemType Directory -Path (Join-Path $work 'bin'), (Join-Path $work 'logs'), (Join-Path $work 'run') | Out-Null
$started = New-Object System.Collections.ArrayList
$warnings = New-Object System.Collections.ArrayList
$skipSet = @($Skip)

. (Join-Path $PSScriptRoot 'bench-common.ps1')

function Wanted([string]$Name) { return -not ($skipSet -contains $Name) }

function Warn([string]$Message) {
    [void]$warnings.Add($Message)
    Write-Warning $Message
}

function Save-Json([string]$Path, $Object) {
    $json = $Object | ConvertTo-Json -Depth 12
    [IO.File]::WriteAllText($Path, $json, (New-Object Text.UTF8Encoding($false)))
}

# The apps read JAMB_* and HELLO_* variables. Clear the inherited ones, so that each
# launch sets only its own.
foreach ($name in @([Environment]::GetEnvironmentVariables().Keys)) {
    if ($name -like 'JAMB_*' -or $name -like 'HELLO_*') { [Environment]::SetEnvironmentVariable($name, $null) }
}

$monitors = Get-MonitorInfo
[void](Get-SecondaryPoint $Position $monitors)

# The demo's single-instance check would hand this run's launches to a demo that is
# already running, and activate it.
$running = @(Get-Process -Name 'demo' -ErrorAction SilentlyContinue)
if ($running.Count -gt 0) {
    throw "A demo process is running (PID $($running[0].Id)). Close it first: its single-instance check would take this run's launches."
}

try {
    # Machine information
    $cpuInfo = Get-CimInstance -ClassName Win32_Processor | Select-Object -First 1
    $osInfo = Get-CimInstance -ClassName Win32_OperatingSystem
    $gpus = @(Get-CimInstance -ClassName Win32_VideoController | ForEach-Object { [ordered]@{ name = $_.Name; driver = $_.DriverVersion } })
    $plan = ((& powercfg /getactivescheme) -join ' ')
    $planName = $plan
    if ($plan -match '\((.+)\)\s*$') { $planName = $Matches[1] }
    $commit = ((& git -C $JambRoot rev-parse --short HEAD) | Out-String).Trim()
    $R = [ordered]@{}
    $R.environment = [ordered]@{
        date = (Get-Date).ToString('yyyy-MM-dd')
        commit = $commit
        note = $Note
        cpu = "$($cpuInfo.Name.Trim()), $($cpuInfo.NumberOfLogicalProcessors) logical processors"
        gpus = $gpus
        ramGB = [math]::Round((Get-CimInstance -ClassName Win32_ComputerSystem).TotalPhysicalMemory / 1GB, 1)
        os = "$($osInfo.Caption), build $($osInfo.BuildNumber) ($($osInfo.Version))"
        goVersion = ((& go version) -join ' ')
        powerPlan = $planName
        monitors = $monitors
        position = $Position
        runs = $Runs
        warmLaunches = $WarmLaunches
        settleSeconds = $SettleSeconds
        idleSeconds = $IdleSeconds
        scenarioSeconds = $ScenarioSeconds
        soakMinutes = $SoakMinutes
        skipped = @($Skip)
    }

    # Binary sizes: the stripped build, the default build, both with -H windowsgui.
    $bin = Join-Path $work 'bin'
    $buildSpecs = @(
        [pscustomobject]@{ key = 'hello'; pkg = './examples/hello'; stripped = $true },
        [pscustomobject]@{ key = 'demo'; pkg = './examples/demo'; stripped = $true },
        [pscustomobject]@{ key = 'hello-default'; pkg = './examples/hello'; stripped = $false },
        [pscustomobject]@{ key = 'demo-default'; pkg = './examples/demo'; stripped = $false }
    )
    $R.binaries = [ordered]@{}
    foreach ($b in $buildSpecs) {
        $exe = Join-Path $bin "$($b.key).exe"
        if ($b.stripped) { $flags = '-trimpath -ldflags "-s -w -H windowsgui"' } else { $flags = '-ldflags "-H windowsgui"' }
        $sw = [Diagnostics.Stopwatch]::StartNew()
        $log = Join-Path $work "logs\build-$($b.key).log"
        $bat = Join-Path $work "logs\build-$($b.key).cmd"
        Set-Content -LiteralPath $bat -Encoding ASCII -Value @(
            '@echo off',
            "cd /d `"$JambRoot`"",
            "go build $flags -o `"$exe`" $($b.pkg) > `"$log`" 2>&1",
            'exit /b %ERRORLEVEL%')
        $p = Start-Process -FilePath 'cmd.exe' -ArgumentList '/d', '/c', "`"$bat`"" -Wait -PassThru -WindowStyle Hidden
        if ($p.ExitCode -ne 0) { throw "Build of $($b.key) failed (exit $($p.ExitCode)). Log: $log" }
        $sw.Stop()
        $R.binaries[$b.key] = [ordered]@{
            MB = MB (Get-Item -LiteralPath $exe).Length
            buildSeconds = [math]::Round($sw.Elapsed.TotalSeconds, 1)
            flags = $flags
        }
    }

    # The two apps. The demo is the one with its own window title and scenarios.
    $apps = @(
        [pscustomobject]@{ key = 'hello'; title = 'Hello'; exe = (Join-Path $bin 'hello.exe') },
        [pscustomobject]@{ key = 'demo'; title = 'Jamb Demo'; exe = (Join-Path $bin 'demo.exe') }
    )
    $demoApp = $apps[1]

    # Every launch sets the position and the no-activation switch. JAMB_DARK pins the
    # hello app to the dark mode, as in compare-gpui.ps1.
    $baseEnv = @{ JAMB_POSITION = $Position; JAMB_NOACTIVATE = '1'; JAMB_DARK = '1' }
    function New-AppEnv([hashtable]$Extra = @{}) {
        $e = $baseEnv.Clone()
        foreach ($k in $Extra.Keys) { $e[$k] = $Extra[$k] }
        return $e
    }

    # A fresh copy of an app, in its own folder, so that the first launch is of a file
    # the antivirus has not scanned.
    function Copy-App($App, [string]$Name) {
        $dir = Join-Path $work "run\$Name"
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        $exe = Join-Path $dir (Split-Path -Leaf $App.exe)
        Copy-Item -LiteralPath $App.exe -Destination $exe
        return $exe
    }

    # Starts an app and waits for its visible window with the app's title, which must lie
    # off the primary monitor, and for a painted client area. Times are from the launch call.
    function Open-App($App, [string]$Exe, [hashtable]$Vars, [string]$ErrPath) {
        $clock = [Diagnostics.Stopwatch]::StartNew()
        $p = Start-Recorded $Exe $Vars '' $ErrPath
        [void]$started.Add($p)
        $deadline = (Get-Date).AddSeconds(60)
        $hwnd = [IntPtr]::Zero
        while ($hwnd -eq [IntPtr]::Zero -and (Get-Date) -lt $deadline) {
            foreach ($w in [Cmp.Win32]::WindowsOf([uint32]$p.Id)) {
                if ([Cmp.Win32]::IsWindowVisible($w) -and [Cmp.Win32]::TitleOf($w) -eq $App.title) { $hwnd = $w; break }
            }
            if ($hwnd -eq [IntPtr]::Zero) { [Threading.Thread]::Sleep(5) }
        }
        if ($hwnd -eq [IntPtr]::Zero) { throw "$($App.key): no visible '$($App.title)' window within 60 s. Stderr: $ErrPath" }
        $visibleMs = $clock.Elapsed.TotalMilliseconds
        Assert-OffPrimary ([Cmp.Win32]::WindowScreenRect($hwnd)) $monitors "$($App.key) window"

        $distinct = 0
        while ((Get-Date) -lt $deadline) {
            $bmp = [Cmp.Win32]::CaptureClient($hwnd)
            $distinct = [Cmp.Win32]::DistinctColors($bmp)
            $bmp.Dispose()
            if ($distinct -gt 1) { break }
            [Threading.Thread]::Sleep(5)
        }
        $paintedMs = $clock.Elapsed.TotalMilliseconds
        if ($distinct -le 1) { throw "$($App.key): window never showed content" }
        return [pscustomobject]@{ app = $App.key; proc = $p; hwnd = $hwnd; visibleMs = $visibleMs; paintedMs = $paintedMs; err = $ErrPath }
    }

    # Closes an app the way the close button does (WM_CLOSE), and stops it if it does not
    # exit within 20 s. Returns whether it had to be stopped, which loses its frame CSV.
    function Close-App($Launch) {
        $forced = $false
        if (-not $Launch.proc.HasExited) { [void][Cmp.Win32]::PostMessage($Launch.hwnd, 0x0010, [IntPtr]::Zero, [IntPtr]::Zero) }
        if (-not $Launch.proc.WaitForExit(20000)) {
            Stop-Process -Id $Launch.proc.Id -Force -ErrorAction SilentlyContinue
            [void]$Launch.proc.WaitForExit(5000)
            $forced = $true
        }
        return $forced
    }

    # The first-frame line of JAMB_TRACE, in milliseconds, or $null.
    function Read-FirstFrameMs([string]$ErrPath) {
        $text = Read-SharedText $ErrPath
        if ($text -match 'first frame presented (\S+) after process start') { return Convert-DurationMs $Matches[1] }
        return $null
    }

    # Checks every visible window of a process, and returns them.
    function Assert-Windows([int]$ProcessId, [string]$What) {
        $windows = @(Get-VisibleWindowRects $ProcessId)
        foreach ($w in $windows) { Assert-OffPrimary $w.rect $monitors "$What window '$($w.title)'" }
        return $windows
    }

    # Startup: a fresh copy, then warm launches of it, all with JAMB_TRACE so that the
    # app's own first-frame time is recorded.
    function Measure-Startup($App, [int]$Run) {
        $exe = Copy-App $App "startup-$($App.key)-$Run"
        $dir = Split-Path -Parent $exe
        $traceEnv = New-AppEnv @{ JAMB_TRACE = '1' }
        $first = Open-App $App $exe $traceEnv (Join-Path $dir 'first.err')
        $firstTrace = Read-FirstFrameMs $first.err
        [void](Close-App $first)
        $warm = @()
        for ($k = 1; $k -le $WarmLaunches; $k++) {
            $launch = Open-App $App $exe $traceEnv (Join-Path $dir "warm$k.err")
            $warm += [ordered]@{
                visibleMs = [math]::Round($launch.visibleMs, 1)
                paintedMs = [math]::Round($launch.paintedMs, 1)
                traceMs = (Read-FirstFrameMs $launch.err)
            }
            [void](Close-App $launch)
        }
        return [ordered]@{
            app = $App.key; run = $Run
            firstVisibleMs = [math]::Round($first.visibleMs, 1)
            firstPaintedMs = [math]::Round($first.paintedMs, 1)
            firstTraceMs = $firstTrace
            warm = $warm
        }
    }

    # Idle, unfocused: the window is never activated. A fresh launch settles, then the
    # process is sampled for -IdleSeconds.
    function Measure-Idle($App, [int]$Run) {
        $exe = Copy-App $App "idle-$($App.key)-$Run"
        $launch = Open-App $App $exe (New-AppEnv) (Join-Path (Split-Path -Parent $exe) 'idle.err')
        [Threading.Thread]::Sleep($SettleSeconds * 1000)
        $cpu = Measure-Cpu $launch.proc { [Threading.Thread]::Sleep($IdleSeconds * 1000) }
        $launch.proc.Refresh()
        $memory = [ordered]@{
            privateMB = MB $launch.proc.PrivateMemorySize64
            workingSetMB = MB $launch.proc.WorkingSet64
            threads = $launch.proc.Threads.Count
            handles = $launch.proc.HandleCount
        }
        $gpu = Get-GpuMemory $launch.proc.Id
        $memory.gpuDedicatedMB = $gpu.dedicatedMB
        $memory.gpuSharedMB = $gpu.sharedMB
        $windows = @(Assert-Windows $launch.proc.Id $App.key)
        $activated = ([Cmp.Win32]::GetForegroundWindow() -eq $launch.hwnd)
        $forced = Close-App $launch
        return [ordered]@{
            app = $App.key; run = $Run; idle = $cpu; memory = $memory
            windows = $windows.Count; appActivated = $activated; closedForced = $forced
        }
    }

    # Scenarios: each script is a loop of about ScenarioSeconds. The coordinates are
    # logical pixels in the demo's default window, 1200 x 760, from ui.Runtime.Find on
    # views.App at that size:
    #   sidebar: nav-Dashboard y 102-138, nav-Projects 140-176, nav-Activity 178-214,
    #     nav-Components 216-252, nav-Markdown 254-290, nav-Settings 330-366; each item is
    #     x 12-232, so the clicks use x 122 and the centre of each item's y range.
    #   Activity page: the event list "events" is x 284-1161, y 259-701, so (700, 480) is
    #     inside it; the filter field's box is x 277-537, y 198-232, so (407, 215) is in it.
    #   Components page: the filter field "components-filter" is x 277-597, y 105-139, so
    #     (437, 122) is in it. Filtering with "motion" leaves the Motion section (y 257),
    #     whose easing rows (x 418-678, y 311-400) animate forever (Animate with Repeat).
    #     Filtering with "code editor" leaves only the Code editor section, whose
    #     "code-editor" is x 296-1149, y 358-618, so (600, 450) is in it and (722, 488) is
    #     its centre.
    #   Projects page: the first project card "1" is x 277-1168, y 198-303, and its "Open in
    #     window" button is x 1083-1117, y 221-255, so (1100, 238) is in it.
    # The default window size was measured at 100 percent text scale, which the demo's
    # layoutScale keeps at 1.
    $nav = @{
        Dashboard = '122 120'; Projects = '122 158'; Activity = '122 196'
        Components = '122 234'; Markdown = '122 272'; Settings = '122 348'
    }
    $pointerScript = 'click 122 196; wait 800; mark'
    foreach ($y in 270..690 | Where-Object { ($_ - 270) % 20 -eq 0 }) { $pointerScript += "; move 700 $y; wait 16" }
    $pointerScript += '; repeat 0'
    $scenarioDefs = @(
        [pscustomobject]@{ name = 'activity-scroll'
            script = "click $($nav.Activity); wait 800; mark; scroll 700 480 -120; wait 16; repeat 0" },
        [pscustomobject]@{ name = 'pointer-moves'; script = $pointerScript },
        [pscustomobject]@{ name = 'typing'
            script = "click $($nav.Activity); wait 800; click 407 215; wait 300; mark; type e; wait 100; key backspace; wait 100; repeat 0" },
        [pscustomobject]@{ name = 'animation'
            script = "click $($nav.Components); wait 800; click 437 122; wait 300; type motion; wait 1000; mark; wait 1000; repeat 0" },
        [pscustomobject]@{ name = 'page-switch'
            script = "mark; click $($nav.Dashboard); wait 300; click $($nav.Projects); wait 300; click $($nav.Activity); wait 300; click $($nav.Components); wait 300; click $($nav.Markdown); wait 300; click $($nav.Settings); wait 300; repeat 0" },
        [pscustomobject]@{ name = 'code-editor'
            script = "click $($nav.Components); wait 800; click 437 122; wait 300; type code editor; wait 800; click 600 450; wait 300; mark; scroll 722 488 -40; wait 16; scroll 722 488 40; wait 16; type x; wait 16; key backspace; wait 16; repeat 0" }
    )

    # Reads the frame CSVs of a finished run, keeps copies of them, and checks the inputs.
    function Get-RunFrameStats([string]$Dir, [string]$RawRunDir, [string]$Label) {
        $raw = Join-Path $RawRunDir $Label
        New-Item -ItemType Directory -Path $raw -Force | Out-Null
        foreach ($f in @('frames.csv', 'frames-input.csv', 'stderr.txt')) {
            $src = Join-Path $Dir $f
            if (Test-Path -LiteralPath $src) { Copy-Item -LiteralPath $src -Destination $raw }
        }
        return Read-FrameStats (Join-Path $Dir 'frames.csv') (Join-Path $Dir 'frames-input.csv') (Join-Path $Dir 'stderr.txt')
    }

    function Measure-Scenario($Def, [int]$Run, [string]$RawRunDir) {
        $exe = Copy-App $demoApp "scenario-$($Def.name)-$Run"
        $dir = Split-Path -Parent $exe
        $frames = Join-Path $dir 'frames.csv'
        $launch = Open-App $demoApp $exe (New-AppEnv @{ JAMB_SCRIPT = $Def.script; JAMB_FRAMESTATS = $frames }) (Join-Path $dir 'stderr.txt')
        $cpu = Measure-Cpu $launch.proc { [Threading.Thread]::Sleep($ScenarioSeconds * 1000) }
        $forced = Close-App $launch
        $stats = Get-RunFrameStats $dir $RawRunDir "scenario-$($Def.name)"
        $inputEvents = 0
        foreach ($k in $stats.inputs.Keys) { $inputEvents += $stats.inputs[$k].n }
        if ($inputEvents -eq 0) {
            Warn "scenario $($Def.name), run ${Run}: zero input events were recorded. The scripted coordinates may not match the layout; check them against the comments above Measure-Scenario."
        }
        return [ordered]@{
            scenario = $Def.name; run = $Run; cpuPct = $cpu.cpuPct; ctxPerSec = $cpu.ctxPerSec
            closedForced = $forced; frameStats = $stats
        }
    }

    # Resize: the window's size changes for ScenarioSeconds, with SWP_NOACTIVATE and no
    # cursor movement. The base size is the default window; the amplitudes keep the window
    # within 1000-1400 by 640-880 logical pixels, left of the primary monitor at -1600.
    function Measure-Resize([int]$Run, [string]$RawRunDir) {
        $exe = Copy-App $demoApp "resize-$Run"
        $dir = Split-Path -Parent $exe
        $frames = Join-Path $dir 'frames.csv'
        $launch = Open-App $demoApp $exe (New-AppEnv @{ JAMB_FRAMESTATS = $frames }) (Join-Path $dir 'stderr.txt')
        [Threading.Thread]::Sleep(2000)
        $cpu = Measure-Cpu $launch.proc { Resize-Window $launch.hwnd $ScenarioSeconds 1200 760 200 120 }
        $windows = @(Assert-Windows $launch.proc.Id 'resize')
        $forced = Close-App $launch
        $stats = Get-RunFrameStats $dir $RawRunDir 'resize'
        return [ordered]@{ run = $Run; cpuPct = $cpu.cpuPct; ctxPerSec = $cpu.ctxPerSec; windows = $windows.Count; closedForced = $forced; frameStats = $stats }
    }

    # Stress: the app renders for 10 seconds and exits. The window is checked while it runs.
    function Measure-Stress([string]$Mode, [bool]$Vsync, [int]$Run, [string]$RawRunDir) {
        $label = "$Mode-$(if ($Vsync) { 'vsync' } else { 'novsync' })"
        $exe = Copy-App $demoApp "stress-$label-$Run"
        $dir = Split-Path -Parent $exe
        $err = Join-Path $dir 'stderr.txt'
        $e = New-AppEnv @{ JAMB_STRESS = $Mode; JAMB_STRESS_SECONDS = '10' }
        if (-not $Vsync) { $e['JAMB_VSYNC'] = '0' }
        $p = Start-Recorded $exe $e '' $err
        [void]$started.Add($p)
        [Threading.Thread]::Sleep(2000)
        [void](Assert-Windows $p.Id "stress $label")
        if (-not $p.WaitForExit(60000)) {
            Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
            throw "stress $label did not finish within 60 s"
        }
        $text = Read-SharedText $err
        $r = [ordered]@{ mode = $Mode; vsync = $Vsync; run = $Run }
        if ($text -match 'fps=([0-9.]+)') { $r.fps = [double]$Matches[1] }
        if ($text -match 'frames=(\d+)') { $r.frames = [int]$Matches[1] }
        if ($text -match 'cpu per frame[^:]*: avg=(\S+) p50=(\S+) p99=(\S+) max=(\S+)') {
            $r.cpuAvgMs = Convert-DurationMs $Matches[1]
            $r.cpuP50Ms = Convert-DurationMs $Matches[2]
            $r.cpuP99Ms = Convert-DurationMs $Matches[3]
            $r.cpuMaxMs = Convert-DurationMs $Matches[4]
        }
        if ($text -match 'alloc: ([0-9.]+) KB/frame, ([0-9.]+) objects/frame') {
            $r.allocKBPerFrame = [double]$Matches[1]
            $r.objectsPerFrame = [double]$Matches[2]
        }
        $rawLabel = Join-Path $RawRunDir "stress-$label"
        New-Item -ItemType Directory -Path $rawLabel -Force | Out-Null
        Copy-Item -LiteralPath $err -Destination $rawLabel
        return $r
    }

    # Two windows: the main window and a project window opened by a click on the Projects
    # page. The idle CPU is sampled with both open.
    function Measure-MultiWindow([int]$Run) {
        $exe = Copy-App $demoApp "multi-$Run"
        $dir = Split-Path -Parent $exe
        $script = "click $($nav.Projects); wait 1500; click 1100 238; wait 2000"
        $launch = Open-App $demoApp $exe (New-AppEnv @{ JAMB_SCRIPT = $script }) (Join-Path $dir 'stderr.txt')
        [Threading.Thread]::Sleep(5000)
        $windows = @(Assert-Windows $launch.proc.Id 'multi-window')
        if ($windows.Count -lt 2) { Warn "multi-window, run ${Run}: the project window did not open ($($windows.Count) window(s) visible)" }
        $cpu = Measure-Cpu $launch.proc { [Threading.Thread]::Sleep($IdleSeconds * 1000) }
        $forced = Close-App $launch
        return [ordered]@{
            run = $Run; windowCount = $windows.Count; cpuPct = $cpu.cpuPct; ctxPerSec = $cpu.ctxPerSec
            windows = @($windows | ForEach-Object { "$($_.title) at $($_.rect.X),$($_.rect.Y) $($_.rect.Width)x$($_.rect.Height)" })
            closedForced = $forced
        }
    }

    # Soak: a loop that scrolls the Activity list, switches pages and types, for SoakMinutes.
    # The loop's coordinates are the scenario coordinates above.
    function Measure-Soak([string]$RawRunDir) {
        $exe = Copy-App $demoApp 'soak'
        $dir = Split-Path -Parent $exe
        $heap = Join-Path $dir 'heaplog.csv'
        $script = "click $($nav.Activity); wait 800; mark; click 407 215; wait 200; type e; wait 100; key backspace; wait 100; scroll 700 480 -200; wait 100; scroll 700 480 200; wait 100; click $($nav.Components); wait 500; click $($nav.Dashboard); wait 500; click $($nav.Activity); wait 300; repeat 0"
        $launch = Open-App $demoApp $exe (New-AppEnv @{ JAMB_SCRIPT = $script; JAMB_HEAPLOG = $heap; JAMB_HEAPLOG_SECONDS = '10' }) (Join-Path $dir 'stderr.txt')
        $end = (Get-Date).AddMinutes($SoakMinutes)
        while ((Get-Date) -lt $end -and -not $launch.proc.HasExited) { [Threading.Thread]::Sleep(1000) }
        if ($launch.proc.HasExited) { Warn 'soak: the app exited before the soak duration ended' }
        $forced = Close-App $launch
        New-Item -ItemType Directory -Path (Join-Path $RawRunDir 'soak') -Force | Out-Null
        if (Test-Path -LiteralPath $heap) { Copy-Item -LiteralPath $heap -Destination (Join-Path $RawRunDir 'soak') }
        if (-not (Test-Path -LiteralPath $heap)) { Warn 'soak: no heap log was written'; return $null }
        $rows = @(Import-Csv -LiteralPath $heap)
        if ($rows.Count -lt 2) { Warn 'soak: the heap log has fewer than two rows'; return $null }
        $hours = [double[]]@($rows | ForEach-Object { [double]$_.elapsed_s / 3600 })
        function Get-Series([string]$Column, [double]$Scale) {
            $y = [double[]]@($rows | ForEach-Object { [double]$_.$Column / $Scale })
            $max = ($y | Measure-Object -Maximum).Maximum
            return [ordered]@{ start = [math]::Round($y[0], 2); end = [math]::Round($y[$y.Count - 1], 2); max = [math]::Round($max, 2); slopePerHour = [math]::Round((Get-Slope $hours $y), 3) }
        }
        return [ordered]@{
            samples = $rows.Count
            seconds = [math]::Round([double]$rows[-1].elapsed_s, 0)
            forcedClose = $forced
            heapInuseMB = Get-Series 'heap_inuse_bytes' 1MB
            heapObjects = Get-Series 'heap_objects' 1
            privateMB = Get-Series 'private_bytes' 1MB
            workingSetMB = Get-Series 'working_set_bytes' 1MB
            goroutines = Get-Series 'goroutines' 1
        }
    }

    # The measurements, run Runs times. Each run's results are written to raw\run<N>.
    $startup = @(); $idle = @(); $scenarios = @(); $resize = @(); $stress = @(); $multi = @()
    $stressModes = @(@{ mode = 'rebuild'; vsync = $true }, @{ mode = 'rebuild'; vsync = $false },
        @{ mode = 'paint'; vsync = $true }, @{ mode = 'paint'; vsync = $false })
    for ($run = 1; $run -le $Runs; $run++) {
        $rawRun = Join-Path $rawDir "run$run"
        New-Item -ItemType Directory -Path $rawRun -Force | Out-Null
        if (Wanted 'startup') {
            foreach ($a in $apps) {
                Write-Host "startup: $($a.key) run $run"
                $s = Measure-Startup $a $run
                $startup += $s
                Save-Json (Join-Path $rawRun "startup-$($a.key).json") $s
            }
        }
        if (Wanted 'idle') {
            foreach ($a in $apps) {
                Write-Host "idle: $($a.key) run $run"
                $i = Measure-Idle $a $run
                $idle += $i
                Save-Json (Join-Path $rawRun "idle-$($a.key).json") $i
            }
        }
        if (Wanted 'scenarios') {
            foreach ($def in $scenarioDefs) {
                Write-Host "scenario: $($def.name) run $run"
                $sc = Measure-Scenario $def $run $rawRun
                $scenarios += $sc
                Save-Json (Join-Path $rawRun "scenario-$($def.name).json") $sc
            }
        }
        if (Wanted 'resize') {
            Write-Host "resize: run $run"
            $rz = Measure-Resize $run $rawRun
            $resize += $rz
            Save-Json (Join-Path $rawRun 'resize.json') $rz
        }
        if (Wanted 'stress') {
            foreach ($sm in $stressModes) {
                Write-Host "stress: $($sm.mode) vsync=$($sm.vsync) run $run"
                $st = Measure-Stress $sm.mode $sm.vsync $run $rawRun
                $stress += $st
            }
            Save-Json (Join-Path $rawRun 'stress.json') @($stress | Where-Object { $_.run -eq $run })
        }
        if (Wanted 'multiwindow') {
            Write-Host "multi-window: run $run"
            $mw = Measure-MultiWindow $run
            $multi += $mw
            Save-Json (Join-Path $rawRun 'multiwindow.json') $mw
        }
    }
    $soak = $null
    if ((Wanted 'soak') -and $SoakMinutes -gt 0) {
        Write-Host "soak: $SoakMinutes minutes"
        $soak = Measure-Soak $rawDir
    }

    # Summaries across the runs. A null statistic means nothing was measured.
    function Pick-Stat($Rows, [scriptblock]$Pick) {
        $vals = @()
        foreach ($r in @($Rows)) {
            $v = & $Pick $r
            if ($null -ne $v) { $vals += [double]$v }
        }
        if ($vals.Count -eq 0) { return $null }
        return Get-Summary ([double[]]$vals)
    }

    $R.startup = [ordered]@{}
    foreach ($a in $apps) {
        $rows = @($startup | Where-Object { $_.app -eq $a.key })
        if ($rows.Count -eq 0) { continue }
        $warmVis = @(); $warmPaint = @(); $warmTrace = @()
        foreach ($r in $rows) {
            foreach ($w in $r.warm) {
                $warmVis += $w.visibleMs; $warmPaint += $w.paintedMs
                if ($null -ne $w.traceMs) { $warmTrace += $w.traceMs }
            }
        }
        $R.startup[$a.key] = [ordered]@{
            firstLaunchVisibleMs = Pick-Stat $rows { param($r) $r.firstVisibleMs }
            firstLaunchPaintedMs = Pick-Stat $rows { param($r) $r.firstPaintedMs }
            firstLaunchTraceMs = Pick-Stat $rows { param($r) $r.firstTraceMs }
            warmVisibleMs = $(if ($warmVis.Count) { Get-Summary ([double[]]$warmVis) })
            warmPaintedMs = $(if ($warmPaint.Count) { Get-Summary ([double[]]$warmPaint) })
            warmTraceMs = $(if ($warmTrace.Count) { Get-Summary ([double[]]$warmTrace) })
        }
    }

    $R.idle = [ordered]@{}
    foreach ($a in $apps) {
        $rows = @($idle | Where-Object { $_.app -eq $a.key })
        if ($rows.Count -eq 0) { continue }
        $R.idle[$a.key] = [ordered]@{
            cpuPct = Pick-Stat $rows { param($r) $r.idle.cpuPct }
            ctxPerSec = Pick-Stat $rows { param($r) $r.idle.ctxPerSec }
            privateMB = Pick-Stat $rows { param($r) $r.memory.privateMB }
            workingSetMB = Pick-Stat $rows { param($r) $r.memory.workingSetMB }
            threads = Pick-Stat $rows { param($r) $r.memory.threads }
            handles = Pick-Stat $rows { param($r) $r.memory.handles }
            gpuDedicatedMB = Pick-Stat $rows { param($r) $r.memory.gpuDedicatedMB }
            gpuSharedMB = Pick-Stat $rows { param($r) $r.memory.gpuSharedMB }
            appActivatedRuns = @($rows | Where-Object { $_.appActivated }).Count
        }
    }

    # The monitor refresh rate of the secondary monitor, next to the app's own rate.
    $secondary = @($monitors | Where-Object { -not $_.primary })[0]
    $R.scenarios = [ordered]@{}
    foreach ($def in $scenarioDefs) {
        $rows = @($scenarios | Where-Object { $_.scenario -eq $def.name })
        if ($rows.Count -eq 0) { continue }
        $inputs = [ordered]@{}
        foreach ($k in @('key', 'move', 'click', 'scroll')) {
            $n = Pick-Stat $rows { param($r) if ($r.frameStats.inputs.Contains($k)) { $r.frameStats.inputs[$k].n } }
            if ($null -ne $n) {
                $inputs[$k] = [ordered]@{
                    events = $n
                    toPresentP50Ms = Pick-Stat $rows { param($r) if ($r.frameStats.inputs.Contains($k)) { $r.frameStats.inputs[$k].toPresentMs.p50 } }
                    toPresentP95Ms = Pick-Stat $rows { param($r) if ($r.frameStats.inputs.Contains($k)) { $r.frameStats.inputs[$k].toPresentMs.p95 } }
                }
            }
        }
        $R.scenarios[$def.name] = [ordered]@{
            runs = $rows.Count
            cpuPct = Pick-Stat $rows { param($r) $r.cpuPct }
            ctxPerSec = Pick-Stat $rows { param($r) $r.ctxPerSec }
            frames = Pick-Stat $rows { param($r) $r.frameStats.frames }
            cpuMsP50 = Pick-Stat $rows { param($r) $r.frameStats.cpuMs.p50 }
            cpuMsP95 = Pick-Stat $rows { param($r) $r.frameStats.cpuMs.p95 }
            cpuMsP99 = Pick-Stat $rows { param($r) $r.frameStats.cpuMs.p99 }
            intervalMsMedian = Pick-Stat $rows { param($r) $r.frameStats.intervalMs.p50 }
            intervalMsP95 = Pick-Stat $rows { param($r) $r.frameStats.intervalMs.p95 }
            continuousIntervals = Pick-Stat $rows { param($r) $r.frameStats.intervalMs.n }
            overOneAndHalfPeriod = Pick-Stat $rows { param($r) $r.frameStats.overOneAndHalfPeriod }
            hookRefreshHz = Pick-Stat $rows { param($r) $r.frameStats.hookRefreshHz }
            monitorRefreshHz = $secondary.refreshHz
            inputs = $inputs
        }
    }

    $R.resize = $null
    if ($resize.Count -gt 0) {
        $R.resize = [ordered]@{
            runs = $resize.Count
            cpuPct = Pick-Stat $resize { param($r) $r.cpuPct }
            ctxPerSec = Pick-Stat $resize { param($r) $r.ctxPerSec }
            frames = Pick-Stat $resize { param($r) $r.frameStats.frames }
            cpuMsP50 = Pick-Stat $resize { param($r) $r.frameStats.cpuMs.p50 }
            cpuMsP95 = Pick-Stat $resize { param($r) $r.frameStats.cpuMs.p95 }
            intervalMsMedian = Pick-Stat $resize { param($r) $r.frameStats.intervalMs.p50 }
            intervalMsP95 = Pick-Stat $resize { param($r) $r.frameStats.intervalMs.p95 }
            overOneAndHalfPeriod = Pick-Stat $resize { param($r) $r.frameStats.overOneAndHalfPeriod }
            note = 'the frames include the first two seconds, before the resize began'
        }
    }

    $R.stress = [ordered]@{}
    foreach ($sm in $stressModes) {
        $label = "$($sm.mode)-$(if ($sm.vsync) { 'vsync' } else { 'novsync' })"
        $rows = @($stress | Where-Object { $_.mode -eq $sm.mode -and $_.vsync -eq $sm.vsync })
        if ($rows.Count -eq 0) { continue }
        $R.stress[$label] = [ordered]@{
            runs = $rows.Count
            fps = Pick-Stat $rows { param($r) $r.fps }
            cpuAvgMs = Pick-Stat $rows { param($r) $r.cpuAvgMs }
            cpuP50Ms = Pick-Stat $rows { param($r) $r.cpuP50Ms }
            cpuP99Ms = Pick-Stat $rows { param($r) $r.cpuP99Ms }
            allocKBPerFrame = Pick-Stat $rows { param($r) $r.allocKBPerFrame }
            objectsPerFrame = Pick-Stat $rows { param($r) $r.objectsPerFrame }
        }
    }

    $R.multiWindow = $null
    if ($multi.Count -gt 0) {
        $R.multiWindow = [ordered]@{
            runs = $multi.Count
            windowCount = Pick-Stat $multi { param($r) $r.windowCount }
            cpuPct = Pick-Stat $multi { param($r) $r.cpuPct }
            ctxPerSec = Pick-Stat $multi { param($r) $r.ctxPerSec }
        }
    }
    $R.soak = $soak
    $R.warnings = @($warnings)
    $R.totalSeconds = [math]::Round($total.Elapsed.TotalSeconds, 0)
    $R.runs = [ordered]@{ startup = $startup; idle = $idle; scenarios = $scenarios; resize = $resize; stress = $stress; multiwindow = $multi }

    Save-Json (Join-Path $OutDir 'summary.json') $R

    # summary.md: the same figures as tables, medians with the range across the runs.
    $md = New-Object System.Collections.ArrayList
    function Add-Md([string]$Line = '') { [void]$md.Add($Line) }
    $info = $R.environment
    Add-Md '# jamb live benchmarks'
    Add-Md ''
    Add-Md "Date $($info.date); commit $($info.commit); $($info.runs) runs. $($info.note)"
    Add-Md ''
    Add-Md '## Machine'
    Add-Md ''
    Add-Md '| Item | Value |'
    Add-Md '| --- | --- |'
    Add-Md "| CPU | $($info.cpu) |"
    foreach ($g in $info.gpus) { Add-Md "| GPU | $($g.name), driver $($g.driver) |" }
    Add-Md "| RAM | $($info.ramGB) GB |"
    Add-Md "| OS | $($info.os) |"
    Add-Md "| Go | $($info.goVersion) |"
    Add-Md "| Power plan | $($info.powerPlan) |"
    foreach ($m in $info.monitors) {
        $role = if ($m.primary) { 'primary' } else { 'secondary' }
        Add-Md "| Monitor $($m.device) ($role) | $($m.width)x$($m.height) at $($m.x),$($m.y), $($m.refreshHz) Hz |"
    }
    Add-Md "| Position | $($info.position) (logical), JAMB_NOACTIVATE=1 |"
    Add-Md ''
    Add-Md '## Binary sizes'
    Add-Md ''
    Add-Md '| Build | MB | Build seconds |'
    Add-Md '| --- | --- | --- |'
    foreach ($k in $R.binaries.Keys) { Add-Md "| $k | $($R.binaries[$k].MB) | $($R.binaries[$k].buildSeconds) |" }
    Add-Md ''
    Add-Md '## Startup (ms)'
    Add-Md ''
    Add-Md '| App | First launch, visible | First launch, painted | First launch, JAMB_TRACE | Warm, visible | Warm, painted | Warm, JAMB_TRACE |'
    Add-Md '| --- | --- | --- | --- | --- | --- | --- |'
    foreach ($k in $R.startup.Keys) {
        $s = $R.startup[$k]
        Add-Md "| $k | $(Format-Stat $s.firstLaunchVisibleMs) | $(Format-Stat $s.firstLaunchPaintedMs) | $(Format-Stat $s.firstLaunchTraceMs) | $(Format-Stat $s.warmVisibleMs) | $(Format-Stat $s.warmPaintedMs) | $(Format-Stat $s.warmTraceMs) |"
    }
    Add-Md ''
    Add-Md "## Idle, unfocused ($IdleSeconds s)"
    Add-Md ''
    Add-Md '| App | CPU % of one core | Context switches/s | Private MB | Working set MB | Threads | Handles | GPU dedicated MB | GPU shared MB |'
    Add-Md '| --- | --- | --- | --- | --- | --- | --- | --- | --- |'
    foreach ($k in $R.idle.Keys) {
        $i = $R.idle[$k]
        Add-Md "| $k | $(Format-Stat $i.cpuPct 3) | $(Format-Stat $i.ctxPerSec) | $(Format-Stat $i.privateMB) | $(Format-Stat $i.workingSetMB) | $(Format-Stat $i.threads 0) | $(Format-Stat $i.handles 0) | $(Format-Stat $i.gpuDedicatedMB) | $(Format-Stat $i.gpuSharedMB) |"
    }
    Add-Md ''
    Add-Md "## Scenarios (demo, about $ScenarioSeconds s each, with the setup)"
    Add-Md ''
    Add-Md "Monitor refresh of the secondary monitor: $($secondary.refreshHz) Hz. The hook's refresh is the app's own figure; the measured median is the continuous present interval."
    Add-Md ''
    Add-Md '| Scenario | CPU % | Frames | CPU per frame p50 / p95 / p99 (ms) | Present interval median (ms) | Interval p95 (ms) | Over 1.5x (app) | Hook refresh Hz |'
    Add-Md '| --- | --- | --- | --- | --- | --- | --- | --- |'
    foreach ($k in $R.scenarios.Keys) {
        $s = $R.scenarios[$k]
        Add-Md "| $k | $(Format-Stat $s.cpuPct 3) | $(Format-Stat $s.frames 0) | $(Format-Stat $s.cpuMsP50 3) / $(Format-Stat $s.cpuMsP95 3) / $(Format-Stat $s.cpuMsP99 3) | $(Format-Stat $s.intervalMsMedian 2) | $(Format-Stat $s.intervalMsP95 2) | $(Format-Stat $s.overOneAndHalfPeriod 0) | $(Format-Stat $s.hookRefreshHz 0) |"
    }
    Add-Md ''
    Add-Md '| Scenario | Input | Events | To Present p50 (ms) | To Present p95 (ms) |'
    Add-Md '| --- | --- | --- | --- | --- |'
    foreach ($k in $R.scenarios.Keys) {
        foreach ($ik in $R.scenarios[$k].inputs.Keys) {
            $in = $R.scenarios[$k].inputs[$ik]
            Add-Md "| $k | $ik | $(Format-Stat $in.events 0) | $(Format-Stat $in.toPresentP50Ms 3) | $(Format-Stat $in.toPresentP95Ms 3) |"
        }
    }
    Add-Md ''
    if ($R.resize) {
        Add-Md '## Resize (demo, SetWindowPos without activation)'
        Add-Md ''
        Add-Md "| CPU % | Context switches/s | Frames | CPU per frame p50 / p95 (ms) | Present interval median (ms) | Interval p95 (ms) | Over 1.5x (app) |"
        Add-Md '| --- | --- | --- | --- | --- | --- | --- |'
        $z = $R.resize
        Add-Md "| $(Format-Stat $z.cpuPct 3) | $(Format-Stat $z.ctxPerSec) | $(Format-Stat $z.frames 0) | $(Format-Stat $z.cpuMsP50 3) / $(Format-Stat $z.cpuMsP95 3) | $(Format-Stat $z.intervalMsMedian 2) | $(Format-Stat $z.intervalMsP95 2) | $(Format-Stat $z.overOneAndHalfPeriod 0) |"
        Add-Md ''
    }
    Add-Md '## Stress (demo, 10 s)'
    Add-Md ''
    Add-Md '| Mode | Frames/s | CPU per frame avg / p50 / p99 (ms) | Alloc KB/frame | Objects/frame |'
    Add-Md '| --- | --- | --- | --- | --- |'
    foreach ($k in $R.stress.Keys) {
        $s = $R.stress[$k]
        Add-Md "| $k | $(Format-Stat $s.fps) | $(Format-Stat $s.cpuAvgMs 3) / $(Format-Stat $s.cpuP50Ms 3) / $(Format-Stat $s.cpuP99Ms 3) | $(Format-Stat $s.allocKBPerFrame) | $(Format-Stat $s.objectsPerFrame 0) |"
    }
    Add-Md ''
    if ($R.multiWindow) {
        $m = $R.multiWindow
        Add-Md "## Two windows (main and a project window), idle $IdleSeconds s"
        Add-Md ''
        Add-Md "Windows visible: $(Format-Stat $m.windowCount 0). CPU % $(Format-Stat $m.cpuPct 3); context switches/s $(Format-Stat $m.ctxPerSec)."
        Add-Md ''
    }
    if ($R.soak) {
        $s = $R.soak
        Add-Md "## Soak ($SoakMinutes min)"
        Add-Md ''
        Add-Md '| Counter | Start | End | Max | Slope per hour |'
        Add-Md '| --- | --- | --- | --- | --- |'
        foreach ($k in @('heapInuseMB', 'heapObjects', 'privateMB', 'workingSetMB', 'goroutines')) {
            $c = $s[$k]
            Add-Md "| $k | $($c.start) | $($c.end) | $($c.max) | $($c.slopePerHour) |"
        }
        Add-Md ''
    }
    if ($warnings.Count -gt 0) {
        Add-Md '## Warnings'
        Add-Md ''
        foreach ($w in $warnings) { Add-Md "- $w" }
        Add-Md ''
    }
    [IO.File]::WriteAllText((Join-Path $OutDir 'summary.md'), (($md -join "`r`n") + "`r`n"), (New-Object Text.UTF8Encoding($false)))

    Write-Host "Results: $OutDir"
} finally {
    foreach ($p in $started) {
        if (-not $p.HasExited) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
    }
    if (Test-Path -LiteralPath $work) { Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue }
}
