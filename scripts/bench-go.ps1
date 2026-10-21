#Requires -Version 5.1
<#
.SYNOPSIS
    Runs every Go benchmark in the jamb workspace and summarizes them with benchstat.

.DESCRIPTION
    Finds the modules of go.work and runs go test -run NONE -bench over each of their
    packages, -count times. The raw output goes to one file, named with the date and
    the short commit, and starts with a header that records the machine: CPU, cores,
    memory, GPU, OS build, power plan, Go version and the commit. The header is in
    '#' comment lines, which benchstat ignores.

    Then benchstat (pinned version, run with go run, so nothing is installed) writes a
    summary: the median of each benchmark with its spread, and, with -Baseline, a
    comparison of the baseline file against the new one.

    The script exits non-zero only when a benchmark fails or does not build. A slower
    or faster result never fails it.

    The numbers are only as steady as the machine. Close other work, keep the power
    plan on a high-performance setting and plug in a laptop. Results from different
    machines or power plans are not comparable; compare runs made on the same machine
    with the same settings. Benchmarks that open no window and touch no GPU are
    headless; the rest of the suite can be run, but do not claim the numbers are the
    reference numbers of BENCHMARKS.md.

.PARAMETER Count
    Repetitions of each benchmark (go test -count). Default 10. benchstat needs
    several samples to say whether a difference is noise.

.PARAMETER Filter
    Regular expression of the benchmarks to run (go test -bench). Default '.', all of them.

.PARAMETER Skip
    Regular expression of the benchmarks not to run (go test -skip). Default none.
    For example, '100MB' leaves out the 100 MB rope benchmarks, which are slow.

.PARAMETER Package
    Package patterns to run, relative to the repository root. Default: every module
    of go.work, with its packages (./..., ./cmd/jamb/..., ./examples/demo/..., ...).

.PARAMETER BenchTime
    Length or count of each benchmark (go test -benchtime), such as 2s or 100x.
    Default: go test's own (1s).

.PARAMETER OutDir
    Folder for the raw output and the summaries. Default: bench-results in the
    repository root, which is git-ignored.

.PARAMETER Baseline
    A raw output file from an earlier run, to compare the new run against. The
    comparison is written next to the new output and printed.

.PARAMETER CpuProfile
    Also write a CPU profile of each package to OutDir\profiles. It runs the packages
    one at a time, because go test allows one profile per run. Off by default: the
    profile itself slows the benchmarks.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\bench-go.ps1

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\bench-go.ps1 -Count 5 -Filter 'Scale|Hit' -Package ./ui

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\bench-go.ps1 -Baseline bench-results\bench-2026-10-20-1200-abc1234.txt
#>
[CmdletBinding()]
param(
    [ValidateRange(1, 1000)][int]$Count = 10,
    [string]$Filter = '.',
    [string]$Skip = '',
    [string[]]$Package = @(),
    [string]$BenchTime = '',
    [string]$OutDir = '',
    [string]$Baseline = '',
    [switch]$CpuProfile
)

# benchstat is pinned to one module version; the same version is used in CI.
$benchstat = 'golang.org/x/perf/cmd/benchstat@v0.0.0-20261009192801-be2c69fb417e'

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $root
# benchstat prints UTF-8 (for the microsecond sign); the console must decode it so.
$consoleEncoding = $null
try {
    $consoleEncoding = [Console]::OutputEncoding
    [Console]::OutputEncoding = [Text.Encoding]::UTF8
} catch { }
try {
    if (-not $OutDir) { $OutDir = Join-Path $root 'bench-results' }
    New-Item -ItemType Directory -Path $OutDir -Force | Out-Null
    $OutDir = (Resolve-Path -LiteralPath $OutDir).Path

    # Packages: the modules of go.work, each with its packages.
    if ($Package.Count -eq 0) {
        $work = (& go work edit -json) | ConvertFrom-Json
        foreach ($use in $work.Use) {
            $dir = ($use.DiskPath -replace '\\', '/').TrimStart('./').TrimEnd('/')
            if ($dir -eq '') { $Package += './...' } else { $Package += "./$dir/..." }
        }
    }

    # Commit and date name the output file.
    $sha = (& git rev-parse --short HEAD).Trim()
    $dirty = if (& git status --porcelain) { '-dirty' } else { '' }
    $stamp = Get-Date -Format 'yyyy-MM-dd-HHmm'
    $name = "bench-$stamp-$sha$dirty"
    $raw = Join-Path $OutDir "$name.txt"

    # Machine information for the header. Each query is optional: a machine
    # without the WMI class still gets a run, with the field left blank.
    function Get-Cim($class, $prop) {
        try { (Get-CimInstance -ClassName $class -ErrorAction Stop | Select-Object -First 1).$prop } catch { '' }
    }
    $cpu = Get-Cim 'Win32_Processor' 'Name'
    $cores = Get-Cim 'Win32_Processor' 'NumberOfCores'
    $threads = Get-Cim 'Win32_Processor' 'NumberOfLogicalProcessors'
    $ramGB = ''
    try {
        $ramGB = [math]::Round((Get-CimInstance -ClassName Win32_ComputerSystem -ErrorAction Stop).TotalPhysicalMemory / 1GB, 1)
    } catch { }
    $os = Get-Cim 'Win32_OperatingSystem' 'Caption'
    $build = Get-Cim 'Win32_OperatingSystem' 'BuildNumber'
    $gpu = ''
    try {
        $gpu = (Get-CimInstance -ClassName Win32_VideoController -ErrorAction Stop | ForEach-Object { $_.Name }) -join '; '
    } catch { }
    $plan = ''
    try {
        $scheme = (& powercfg /getactivescheme) -join ' '
        if ($scheme -match '\((.+)\)') { $plan = $Matches[1].Trim() }
    } catch { }
    $goVersion = (& go version).Trim()
    $battery = ''
    try {
        if (Get-CimInstance -ClassName Win32_Battery -ErrorAction Stop) { $battery = 'battery present (laptop)' }
    } catch { }

    $header = @(
        '# jamb Go benchmarks'
        "# date:      $(Get-Date -Format 'yyyy-MM-dd HH:mm zzz')"
        "# commit:    $sha$dirty"
        "# go:        $goVersion"
        "# cpu:       $cpu"
        "# cores:     $cores physical, $threads logical"
        "# memory:    $ramGB GB"
        "# gpu:       $gpu"
        "# os:        $os, build $build"
        "# power:     $plan $battery"
        "# run:       -count $Count -bench '$Filter' -skip '$Skip' -benchtime '$BenchTime'"
        "# packages:  $($Package -join ' ')"
        '#'
    )

    $goArgs = @('test', '-run', 'NONE', '-bench', $Filter, '-benchmem', "-count=$Count", '-timeout', '3h')
    if ($Skip) { $goArgs += @('-skip', $Skip) }
    if ($BenchTime) { $goArgs += "-benchtime=$BenchTime" }

    $lines = New-Object System.Collections.Generic.List[string]
    foreach ($h in $header) { $lines.Add($h) }
    $failed = $false

    # Runs go test with the given arguments, echoing its output to the console and
    # keeping it in $lines. Returns the exit code.
    function Invoke-Go([string[]]$arguments) {
        $prev = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try {
            & go @arguments 2>&1 | ForEach-Object {
                $s = "$_"
                $lines.Add($s)
                Write-Host $s
            }
            return $LASTEXITCODE
        } finally {
            $ErrorActionPreference = $prev
        }
    }

    # Runs a native command and returns its standard output, one string per line.
    # Its standard error is dropped; the exit code is left in $LASTEXITCODE.
    function Invoke-Quiet([string[]]$arguments) {
        $prev = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try {
            return @(& go @arguments 2>$null)
        } finally {
            $ErrorActionPreference = $prev
        }
    }

    if ($CpuProfile) {
        $profiles = Join-Path $OutDir 'profiles'
        New-Item -ItemType Directory -Path $profiles -Force | Out-Null
        $pkgs = @()
        foreach ($p in $Package) {
            $pkgs += @(Invoke-Quiet @('list', $p))
        }
        $i = 0
        foreach ($pkg in $pkgs) {
            $i++
            $safe = ($pkg -replace '[^A-Za-z0-9]+', '-').Trim('-')
            $prof = Join-Path $profiles ("cpu-{0:D3}-{1}.prof" -f $i, $safe)
            $code = Invoke-Go ($goArgs + @("-cpuprofile=$prof", $pkg))
            if ($code -ne 0) { $failed = $true }
        }
    } else {
        $code = Invoke-Go ($goArgs + $Package)
        if ($code -ne 0) { $failed = $true }
    }

    [IO.File]::WriteAllText($raw, (($lines -join "`n") + "`n"), (New-Object System.Text.UTF8Encoding($false)))
    Write-Host ''
    Write-Host "Raw output: $raw"

    # benchstat, from the pinned version. The summary has one column per file; the
    # comparison, with -Baseline, puts the baseline first.
    # The module is fetched on first use, with GOWORK off so the workspace does not apply.
    $benchEnv = $env:GOWORK
    $env:GOWORK = 'off'
    try {
        $summary = Join-Path $OutDir "$name.summary.txt"
        $out = Invoke-Quiet @('run', $benchstat, $raw)
        if ($LASTEXITCODE -ne 0) { throw 'benchstat failed' }
        $text = $out -join "`n"
        [IO.File]::WriteAllText($summary, $text + "`n", (New-Object System.Text.UTF8Encoding($false)))
        Write-Host ''
        Write-Host "== Summary ($summary)"
        Write-Host $text

        if ($Baseline) {
            $base = (Resolve-Path -LiteralPath $Baseline).Path
            $compare = Join-Path $OutDir "$name.compare.txt"
            $out = Invoke-Quiet @('run', $benchstat, $base, $raw)
            if ($LASTEXITCODE -ne 0) { throw 'benchstat failed on the comparison' }
            $text = $out -join "`n"
            [IO.File]::WriteAllText($compare, $text + "`n", (New-Object System.Text.UTF8Encoding($false)))
            Write-Host ''
            Write-Host "== Comparison with $base ($compare)"
            Write-Host $text
        }
    } finally {
        $env:GOWORK = $benchEnv
    }

    if ($failed) {
        Write-Host ''
        Write-Host 'A benchmark failed or did not build. See the raw output.'
        exit 1
    }
    exit 0
} finally {
    if ($consoleEncoding) { [Console]::OutputEncoding = $consoleEncoding }
    Pop-Location
}
