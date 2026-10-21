#Requires -Version 5.1
<#
.SYNOPSIS
    Helpers shared by scripts/compare-gpui.ps1 and scripts/bench-live.ps1.

.DESCRIPTION
    Dot-source this file; it defines no output of its own. It holds the Win32 calls
    (window enumeration and capture, cursor-free message posting, window placement,
    display modes), the process measurements (CPU, context switches, GPU memory) and
    the summaries (medians and percentiles). None of the helpers moves the mouse
    cursor, activates a window or injects input, except ForceForeground, which only
    the opt-in focused measurements call.
#>

Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms

if (-not ('Cmp.Win32' -as [type])) {
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
    [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr hWnd, IntPtr after, int x, int y, int cx, int cy, uint flags);
    [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr hWnd, uint msg, IntPtr wParam, IntPtr lParam);
    [DllImport("user32.dll")] public static extern uint GetDpiForWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern void keybd_event(byte vk, byte scan, uint flags, UIntPtr extra);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern IntPtr FindWindow(string className, string title);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] public static extern bool EnumDisplaySettings(string deviceName, int modeNum, ref DEVMODE devMode);

    // Only the focused measurements call this. Windows only lets the foreground
    // process move focus. A quick Alt press, which the app ignores, makes the system
    // accept the request.
    public static bool ForceForeground(IntPtr hWnd) {
        keybd_event(0x12, 0, 0, UIntPtr.Zero);
        keybd_event(0x12, 0, 2, UIntPtr.Zero);
        return SetForegroundWindow(hWnd);
    }

    [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
    [StructLayout(LayoutKind.Sequential)] public struct POINT { public int X, Y; }

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    public struct DEVMODE {
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 32)] public string dmDeviceName;
        public short dmSpecVersion, dmDriverVersion, dmSize, dmDriverExtra;
        public int dmFields;
        public int dmPositionX, dmPositionY, dmDisplayOrientation, dmDisplayFixedOutput;
        public short dmColor, dmDuplex, dmYResolution, dmTTOption, dmCollate;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 32)] public string dmFormName;
        public short dmLogPixels;
        public int dmBitsPerPel, dmPelsWidth, dmPelsHeight, dmDisplayFlags, dmDisplayFrequency;
        public int dmICMMethod, dmICMIntent, dmMediaType, dmDitherType, dmReserved1, dmReserved2, dmPanningWidth, dmPanningHeight;
    }

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

    // Outer rectangle of the window in screen coordinates.
    public static Rectangle WindowScreenRect(IntPtr hWnd) {
        RECT wr;
        GetWindowRect(hWnd, out wr);
        return new Rectangle(wr.Left, wr.Top, wr.Right - wr.Left, wr.Bottom - wr.Top);
    }

    // Renders the window with PW_RENDERFULLCONTENT and returns only its client area.
    public static Bitmap CaptureClient(IntPtr hWnd) {
        Rectangle wr = WindowScreenRect(hWnd);
        var full = new Bitmap(wr.Width, wr.Height, PixelFormat.Format32bppArgb);
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

# The value at pct percent of the sorted values, the way jamb's frame statistics
# compute their percentiles (index n*pct/100, capped at the last value).
function Get-Percentile([double[]]$Values, [int]$Pct) {
    $s = @($Values | Sort-Object)
    $n = $s.Count
    if ($n -eq 0) { return $null }
    $i = [math]::Min([math]::Floor($n * $Pct / 100), $n - 1)
    return [math]::Round($s[[int]$i], 3)
}

function MB([double]$Bytes) { return [math]::Round($Bytes / 1MB, 2) }

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

# Every visible top-level window of a process, with its screen rectangle.
function Get-VisibleWindowRects([int]$ProcessId) {
    $out = @()
    foreach ($w in [Cmp.Win32]::WindowsOf([uint32]$ProcessId)) {
        if (-not [Cmp.Win32]::IsWindowVisible($w)) { continue }
        $r = [Cmp.Win32]::WindowScreenRect($w)
        $out += [pscustomobject]@{ hwnd = $w; title = [Cmp.Win32]::TitleOf($w); rect = $r }
    }
    return $out
}

# Monitors with their bounds, primary flag and refresh rate (the current mode).
function Get-MonitorInfo {
    $list = @()
    foreach ($s in [Windows.Forms.Screen]::AllScreens) {
        $dm = New-Object Cmp.Win32+DEVMODE
        $dm.dmSize = [int16][Runtime.InteropServices.Marshal]::SizeOf([Type]'Cmp.Win32+DEVMODE')
        $hz = $null
        if ([Cmp.Win32]::EnumDisplaySettings($s.DeviceName, -1, [ref]$dm)) { $hz = $dm.dmDisplayFrequency }
        $list += [pscustomobject]@{
            device = $s.DeviceName
            primary = [bool]$s.Primary
            x = $s.Bounds.X; y = $s.Bounds.Y; width = $s.Bounds.Width; height = $s.Bounds.Height
            refreshHz = $hz
        }
    }
    return $list
}

# Parses "X,Y" into two integers and checks that the point lies on a non-primary
# monitor. Throws otherwise, so that no run starts a window on the primary monitor.
function Get-SecondaryPoint([string]$Text, [object[]]$Monitors) {
    $parts = $Text -split ','
    if ($parts.Count -ne 2) { throw "Position must be X,Y in logical pixels, got '$Text'" }
    $x = [int]$parts[0]; $y = [int]$parts[1]
    $hit = @($Monitors | Where-Object { -not $_.primary -and $x -ge $_.x -and $x -lt ($_.x + $_.width) -and $y -ge $_.y -and $y -lt ($_.y + $_.height) })
    if ($hit.Count -eq 0) { throw "Position $x,$y is not on a non-primary monitor" }
    return [pscustomobject]@{ x = $x; y = $y }
}

# Throws when a window's rectangle is not wholly on a non-primary monitor. The
# primary monitor's bounds come from Windows.Forms, at 100 percent scale.
function Assert-OffPrimary([Drawing.Rectangle]$Rect, [object[]]$Monitors, [string]$What) {
    foreach ($m in $Monitors | Where-Object { $_.primary }) {
        $overlap = [Drawing.Rectangle]::Intersect($Rect, [Drawing.Rectangle]::new($m.x, $m.y, $m.width, $m.height))
        if (-not $overlap.IsEmpty) { throw "$What is at $($Rect.Left),$($Rect.Top) $($Rect.Width)x$($Rect.Height), over the primary monitor" }
    }
}

# Starts an executable with environment variables set for the child only. When
# StdOut or StdErr is a path, that stream is written to the file. Returns the process.
function Start-Recorded([string]$Exe, [hashtable]$EnvVars, [string]$StdOut = '', [string]$StdErr = '') {
    $old = @{}
    foreach ($k in $EnvVars.Keys) {
        $old[$k] = [Environment]::GetEnvironmentVariable($k)
        [Environment]::SetEnvironmentVariable($k, $EnvVars[$k])
    }
    $startArgs = @{ FilePath = $Exe; PassThru = $true }
    if ($StdOut) { $startArgs.RedirectStandardOutput = $StdOut }
    if ($StdErr) { $startArgs.RedirectStandardError = $StdErr }
    try {
        return Start-Process @startArgs
    } finally {
        foreach ($k in $EnvVars.Keys) { [Environment]::SetEnvironmentVariable($k, $old[$k]) }
    }
}

# Resizes a window in a sine wave around a base size, with SWP_NOMOVE | SWP_NOZORDER |
# SWP_NOACTIVATE, so the window keeps its place and is never activated. It restores the
# base size at the end.
function Resize-Window([IntPtr]$Hwnd, [int]$Seconds, [int]$BaseWidth = 800, [int]$BaseHeight = 600, [int]$AmplitudeWidth = 240, [int]$AmplitudeHeight = 180) {
    $flags = [uint32](0x0002 -bor 0x0004 -bor 0x0010)
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ($sw.Elapsed.TotalSeconds -lt $Seconds) {
        $t = $sw.Elapsed.TotalSeconds
        $w = [int]($BaseWidth + $AmplitudeWidth * [math]::Sin(2 * [math]::PI * $t / 4))
        $h = [int]($BaseHeight + $AmplitudeHeight * [math]::Sin(2 * [math]::PI * $t / 5))
        [void][Cmp.Win32]::SetWindowPos($Hwnd, [IntPtr]::Zero, 0, 0, $w, $h, $flags)
        [Threading.Thread]::Sleep(16)
    }
    [void][Cmp.Win32]::SetWindowPos($Hwnd, [IntPtr]::Zero, 0, 0, $BaseWidth, $BaseHeight, $flags)
}

# The text of a file that an app may still have open, read with shared access.
function Read-SharedText([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return '' }
    $fs = [IO.File]::Open($Path, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::ReadWrite)
    try {
        $reader = New-Object IO.StreamReader($fs)
        return $reader.ReadToEnd()
    } finally {
        $fs.Dispose()
    }
}

# Converts a Go duration as printed by time.Duration ("1.25ms", "845 us" in microseconds,
# "2s") to milliseconds, or $null when the text is not a duration.
function Convert-DurationMs([string]$Text) {
    if ($Text -notmatch '^([0-9.]+)(\S+)$') { return $null }
    $n = [double]$Matches[1]
    switch ($Matches[2]) {
        'ms' { return $n }
        's' { return $n * 1000 }
        'ns' { return $n / 1e6 }
        default { return $n / 1000 }
    }
}

# Least-squares slope of Y against X (the units of Y per unit of X), or $null.
function Get-Slope([double[]]$X, [double[]]$Y) {
    $n = $X.Count
    if ($n -lt 2) { return $null }
    $sx = 0.0; $sy = 0.0; $sxy = 0.0; $sxx = 0.0
    for ($i = 0; $i -lt $n; $i++) {
        $sx += $X[$i]; $sy += $Y[$i]; $sxy += $X[$i] * $Y[$i]; $sxx += $X[$i] * $X[$i]
    }
    $den = $n * $sxx - $sx * $sx
    if ($den -eq 0) { return $null }
    return ($n * $sxy - $sx * $sy) / $den
}

# Reads a jamb frame-statistics CSV pair and the summary the app printed to stderr. The
# present interval statistics cover the continuous frames only, as the app's own summary
# does. The over-1.5x count and the refresh rate are the app's own: the measured median
# is what the docs should report next to the monitor's refresh rate.
function Read-FrameStats([string]$FramesCsv, [string]$InputCsv, [string]$StderrPath) {
    $frames = @()
    if (Test-Path -LiteralPath $FramesCsv) { $frames = @(Import-Csv -LiteralPath $FramesCsv) }
    $inputs = @()
    if (Test-Path -LiteralPath $InputCsv) { $inputs = @(Import-Csv -LiteralPath $InputCsv) }

    $cpu = [double[]]@($frames | ForEach-Object { [double]$_.cpu_ms })
    $intervals = [double[]]@($frames | Where-Object { $_.continuous -eq '1' -and $_.present_interval_ms -ne '' } |
        ForEach-Object { [double]$_.present_interval_ms })
    $kinds = [ordered]@{}
    foreach ($g in @($frames | Group-Object kind)) { $kinds[$g.Name] = $g.Count }

    $inputOut = [ordered]@{}
    foreach ($g in @($inputs | Group-Object kind)) {
        $toCpu = [double[]]@($g.Group | ForEach-Object { [double]$_.to_cpu_end_ms })
        $toPresent = [double[]]@($g.Group | ForEach-Object { [double]$_.to_present_ms })
        $inputOut[$g.Name] = [ordered]@{
            n = $g.Count
            toCpuEndMs = [ordered]@{ p50 = Get-Percentile $toCpu 50; p95 = Get-Percentile $toCpu 95; p99 = Get-Percentile $toCpu 99 }
            toPresentMs = [ordered]@{ p50 = Get-Percentile $toPresent 50; p95 = Get-Percentile $toPresent 95; p99 = Get-Percentile $toPresent 99 }
        }
    }

    $text = Read-SharedText $StderrPath
    $hookHz = $null
    if ($text -match 'refresh=([0-9.]+)Hz') { $hookHz = [double]$Matches[1] }
    $over = $null
    if ($text -match 'over 1\.5x period: (\d+)') { $over = [int]$Matches[1] }
    $summaryLines = @($text -split "`r?`n" | Where-Object { $_ -match '^(jamb framestats|  )' })

    return [ordered]@{
        frames = $frames.Count
        cpuMs = [ordered]@{ p50 = Get-Percentile $cpu 50; p95 = Get-Percentile $cpu 95; p99 = Get-Percentile $cpu 99; max = Get-Percentile $cpu 100 }
        intervalMs = [ordered]@{ n = $intervals.Count; p50 = Get-Percentile $intervals 50; p95 = Get-Percentile $intervals 95; p99 = Get-Percentile $intervals 99; max = Get-Percentile $intervals 100 }
        overOneAndHalfPeriod = $over
        hookRefreshHz = $hookHz
        frameKinds = $kinds
        inputs = $inputOut
        summaryLines = $summaryLines
    }
}

# "median [min-max]" for a summary from Get-Summary, or n/a.
function Format-Stat($Stat, [int]$Digits = 1) {
    if ($null -eq $Stat) { return 'n/a' }
    $med = [math]::Round($Stat.median, $Digits)
    if ($Stat.n -le 1) { return "$med" }
    return "$med [$([math]::Round($Stat.min, $Digits))$([char]0x2013)$([math]::Round($Stat.max, $Digits))]"
}
