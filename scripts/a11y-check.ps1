<#
.SYNOPSIS
    Live checks of jamb's accessibility layer on the demo and notes examples.

.DESCRIPTION
    Builds the two examples into a temporary folder, starts them, and reads their
    UI Automation trees with the .NET UIAutomationClient that ships with Windows,
    the way a screen reader does. Each check prints PASS or FAIL. The notes app
    runs with a temporary notes file, never the user's own notes.

    When AxeWindowsCLI is installed (%LOCALAPPDATA%\Programs\AxeWindowsCLI, or on
    PATH), each app is also scanned with it, and a scan that finds errors is a
    FAIL. The scanner needs the app to run non-elevated, as the apps do here.

    The script starts and writes only temporary files and processes. It removes
    them on exit, after a failure too.

.PARAMETER TimeoutSeconds
    How long to wait for a window or an element to appear. Default 30.

.PARAMETER SkipAxe
    Do not run the AxeWindowsCLI scans.

.PARAMETER HighContrast
    Start the apps with JAMB_HIGH_CONTRAST=1, which turns on high contrast with
    the representative black scheme. The system setting is not changed.

.PARAMETER TextScale
    Start the apps with JAMB_TEXT_SCALE set to this Text size in percent, 100 to
    225. The default, 100, is no override. The system setting is not changed.

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File scripts\a11y-check.ps1

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File scripts\a11y-check.ps1 -HighContrast -TextScale 225

.NOTES
    Exit code 0 when every check passes, 1 when any check fails.
#>
[CmdletBinding()]
param(
    [int]$TimeoutSeconds = 30,
    [switch]$SkipAxe,
    [switch]$HighContrast,
    [ValidateRange(100, 225)]
    [int]$TextScale = 100
)

Set-StrictMode -Version 3
$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Windows.Forms

# A compiled handler is used for the focus events: a script block is not run on
# the thread that UI Automation calls back on. The script pumps messages with
# Application.DoEvents while it waits.
Add-Type -ReferencedAssemblies UIAutomationClient, UIAutomationTypes -TypeDefinition @'
using System.Threading;
using System.Windows.Automation;

public static class FocusCounter
{
    public static int Count;

    public static void Handler(object sender, AutomationFocusChangedEventArgs e)
    {
        Interlocked.Increment(ref Count);
    }

    public static void Start()
    {
        Automation.AddAutomationFocusChangedEventHandler(new AutomationFocusChangedEventHandler(Handler));
    }

    public static void Stop()
    {
        Automation.RemoveAllEventHandlers();
    }
}
'@

# Brings a window to the foreground, so that keys sent with SendKeys reach it.
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;

public static class ForegroundWindow
{
    [DllImport("user32.dll")]
    public static extern bool SetForegroundWindow(IntPtr hWnd);
}
'@

$repo = Split-Path -Parent $PSScriptRoot
$axe = Join-Path $env:LOCALAPPDATA 'Programs\AxeWindowsCLI\AxeWindowsCLI.exe'
if (-not (Test-Path $axe)) {
    $onPath = Get-Command AxeWindowsCLI.exe -ErrorAction SilentlyContinue
    if ($onPath) { $axe = $onPath.Source } else { $axe = $null }
}
if ($SkipAxe) { $axe = $null }

$NameProp = [System.Windows.Automation.AutomationElement]::NameProperty
$ProcProp = [System.Windows.Automation.AutomationElement]::ProcessIdProperty
$RootEl = [System.Windows.Automation.AutomationElement]::RootElement
$Descendants = [System.Windows.Automation.TreeScope]::Descendants
$Children = [System.Windows.Automation.TreeScope]::Children
$CT = @{
    Button   = [System.Windows.Automation.ControlType]::Button
    CheckBox = [System.Windows.Automation.ControlType]::CheckBox
    Edit     = [System.Windows.Automation.ControlType]::Edit
    Menu     = [System.Windows.Automation.ControlType]::Menu
    MenuItem = [System.Windows.Automation.ControlType]::MenuItem
    ListItem = [System.Windows.Automation.ControlType]::ListItem
    ComboBox = [System.Windows.Automation.ControlType]::ComboBox
    List     = [System.Windows.Automation.ControlType]::List
    Group    = [System.Windows.Automation.ControlType]::Group
    RadioButton = [System.Windows.Automation.ControlType]::RadioButton
    Slider   = [System.Windows.Automation.ControlType]::Slider
    Spinner  = [System.Windows.Automation.ControlType]::Spinner
    Text     = [System.Windows.Automation.ControlType]::Text
}
$Passed = 0
$Failed = 0
$Work = Join-Path ([IO.Path]::GetTempPath()) ('jamb-a11y-' + [Guid]::NewGuid().ToString('N'))
$Procs = @()

function Assert([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
}

# Check runs one check. A body may end with a string, which is shown after the
# name, as the count of an AxeWindowsCLI scan is.
function Check([string]$Name, [scriptblock]$Body) {
    try {
        $out = @(& $Body)
        $detail = $out | Where-Object { $_ -is [string] } | Select-Object -Last 1
        $script:Passed++
        if ($detail) { Write-Output "PASS $Name ($detail)" } else { Write-Output "PASS $Name" }
    } catch {
        $script:Failed++
        Write-Output "FAIL $Name - $($_.Exception.Message)"
    }
}

# Wait-For returns the first truthy result of the probe, or throws at the timeout.
# A probe answers with the element it found, or $true, or nothing.
function Wait-For([scriptblock]$Probe, [string]$What) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $last = ''
    do {
        # A COM call can fail while the UI changes under it; the probe is tried again.
        try {
            $found = & $Probe
            if ($found) { return $found }
        } catch {
            $last = $_.Exception.Message
        }
        Start-Sleep -Milliseconds 200
    } while ((Get-Date) -lt $deadline)
    throw "timed out waiting for $What $(if ($last) { "(last error: $last)" })"
}

# Wait-Gone returns once the probe finds nothing.
function Wait-Gone([scriptblock]$Probe, [string]$What) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            if (-not (& $Probe)) { return }
        } catch {
            # A COM call can fail while the UI changes under it; the probe is tried again.
        }
        Start-Sleep -Milliseconds 200
    } while ((Get-Date) -lt $deadline)
    throw "timed out waiting for $What"
}

# Find-Named writes the elements under root with the exact name and, when given,
# the control type. Callers that count them wrap the call in @( ).
function Find-Named($Root, [string]$Name, $Type = $null) {
    $cond = New-Object System.Windows.Automation.PropertyCondition -ArgumentList $NameProp, $Name
    $all = $Root.FindAll($Descendants, $cond)
    foreach ($el in $all) {
        if ($null -eq $Type -or $el.Current.ControlType -eq $Type) { $el }
    }
}

function Find-One($Root, [string]$Name, $Type = $null) {
    $hits = @(Find-Named $Root $Name $Type)
    Assert ($hits.Count -ge 1) "no element named '$Name'"
    return $hits[0]
}

function Get-Pattern($El, $Pattern) {
    $p = $null
    if ($El.TryGetCurrentPattern($Pattern, [ref]$p)) { return $p }
    return $null
}

function Require-Pattern($El, $Pattern, [string]$Label) {
    $p = Get-Pattern $El $Pattern
    Assert ($null -ne $p) "'$($El.Current.Name)' has no $Label pattern"
    return $p
}

function Start-App([string]$Exe, [string]$NotesFile) {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $Exe
    $psi.WorkingDirectory = Split-Path $Exe
    $psi.UseShellExecute = $false
    if ($NotesFile) { $psi.EnvironmentVariables['JAMB_NOTES_FILE'] = $NotesFile }
    if ($HighContrast) { $psi.EnvironmentVariables['JAMB_HIGH_CONTRAST'] = '1' }
    if ($TextScale -ne 100) { $psi.EnvironmentVariables['JAMB_TEXT_SCALE'] = "$TextScale" }
    $proc = [System.Diagnostics.Process]::Start($psi)
    $script:Procs += $proc
    return $proc
}

# Window returns the top-level window of the process, once it exists.
function Get-Window($Proc) {
    $cond = New-Object System.Windows.Automation.PropertyCondition -ArgumentList $ProcProp, $Proc.Id
    return Wait-For { $RootEl.FindFirst($Children, $cond) } "the window of process $($Proc.Id)"
}

function Invoke-Named($Root, [string]$Name, $Type = $null) {
    $el = Find-One $Root $Name $Type
    (Require-Pattern $el ([System.Windows.Automation.InvokePattern]::Pattern) 'Invoke').Invoke()
}

# Scan runs AxeWindowsCLI on the process and returns its error count. The scanner
# exits 0 with no errors and 1 with errors; anything else is a failure to scan.
function Invoke-AxeScan($Proc, [string]$Name) {
    $out = Join-Path $Work "axe-$Name"
    New-Item -ItemType Directory -Force -Path $out | Out-Null
    # The scanner writes its report to stderr, which is captured here, so the
    # native-error handling of the session must not turn those lines into errors.
    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $log = & $axe --processid $Proc.Id --outputdirectory $out --scanid $Name --verbosity default --delayinseconds 2 2>&1 | ForEach-Object { "$_" }
        $code = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldPreference
    }
    $text = $log -join "`n"
    if ($code -eq 0) { return 0 }
    if ($code -eq 1 -and $text -match '(\d+) errors? (were|was) found') { return [int]$matches[1] }
    throw "AxeWindowsCLI exited $code without a count: $text"
}

try {
    # Build both examples into the temporary folder, so that the check runs the
    # code in this checkout.
    $demoExe = Join-Path $Work 'demo.exe'
    $notesExe = Join-Path $Work 'notes.exe'
    New-Item -ItemType Directory -Force -Path $Work | Out-Null
    Push-Location $repo
    try {
        & go build -o $demoExe ./examples/demo
        if ($LASTEXITCODE -ne 0) { throw "go build of the demo failed" }
        & go build -o $notesExe ./examples/notes
        if ($LASTEXITCODE -ne 0) { throw "go build of the notes app failed" }
    } finally {
        Pop-Location
    }
    $notesFile = Join-Path $Work 'notes.json'

    $overrides = @()
    if ($HighContrast) { $overrides += 'high contrast' }
    if ($TextScale -ne 100) { $overrides += "text size $TextScale percent" }
    Write-Output "Overrides: $(if ($overrides) { $overrides -join ', ' } else { 'none' })"
    Write-Output "AxeWindowsCLI: $(if ($axe) { $axe } else { 'not found, scans skipped' })"
    [FocusCounter]::Start()

    # ---- Demo ----------------------------------------------------------------
    $demo = Start-App $demoExe $null
    $win = Get-Window $demo
    Wait-For { $null -ne (Find-Named $win 'New Project' $CT.Button | Select-Object -First 1) } 'the demo to show its toolbar' | Out-Null

    Check 'demo: navigation and toolbar controls have Button role' {
        foreach ($n in 'Dashboard', 'Settings', 'Components', 'Activity', 'New Project', 'Notifications') {
            Assert (@(Find-Named $win $n $CT.Button).Count -eq 1) "'$n' is not one button"
        }
        Assert (@(Find-Named $win 'Projects 6' $CT.Button).Count -eq 1) "'Projects 6' is not one button"
    }

    Check 'demo: Invoke opens the Activity page' {
        Invoke-Named $win 'Activity' $CT.Button
        Wait-For { $null -ne (Find-Named $win '10,000 events from 5 people' | Select-Object -First 1) } 'the Activity page' | Out-Null
    }

    Check 'demo: a star toggles with Toggle, and has no Invoke pattern' {
        Invoke-Named $win 'Projects 6' $CT.Button
        Wait-For { $null -ne (Find-Named $win 'Star project' $CT.Button | Select-Object -First 1) } 'the project list' | Out-Null
        $star = Find-Named $win 'Star project' $CT.Button | Where-Object { -not $_.Current.IsOffscreen } | Select-Object -First 1
        Assert ($null -ne $star) 'no star on screen'
        Assert ($null -eq (Get-Pattern $star ([System.Windows.Automation.InvokePattern]::Pattern))) 'a toggle button also has Invoke'
        $toggle = Require-Pattern $star ([System.Windows.Automation.TogglePattern]::Pattern) 'Toggle'
        $before = $toggle.Current.ToggleState
        $toggle.Toggle()
        Wait-For { if ($toggle.Current.ToggleState -ne $before) { $true } else { $null } } 'the star to change' | Out-Null
        $toggle.Toggle()
        Wait-For { if ($toggle.Current.ToggleState -eq $before) { $true } else { $null } } 'the star to restore' | Out-Null
    }

    Check 'demo: a build row expands and collapses' {
        Invoke-Named $win 'Activity' $CT.Button
        Wait-For { $null -ne (Find-Named $win '10,000 events from 5 people' | Select-Object -First 1) } 'the Activity page' | Out-Null
        $rows = @($win.FindAll($Descendants, [System.Windows.Automation.Condition]::TrueCondition) |
            Where-Object { $_.Current.ControlType -eq $CT.Button -and $_.Current.Name -like '*finished a build of Dev Runner*' })
        Assert ($rows.Count -ge 1) 'no build row'
        $row = $rows[0]
        $ec = Require-Pattern $row ([System.Windows.Automation.ExpandCollapsePatternIdentifiers]::Pattern) 'ExpandCollapse'
        $ec.Expand()
        Wait-For { if ($ec.Current.ExpandCollapseState -eq 'Expanded') { $true } else { $null } } 'the row to expand' | Out-Null
        $ec.Collapse()
        Wait-For { if ($ec.Current.ExpandCollapseState -eq 'Collapsed') { $true } else { $null } } 'the row to collapse' | Out-Null
    }

    Check 'demo: the Done filter is selected in its group' {
        Invoke-Named $win 'Projects 6' $CT.Button
        Wait-For { $null -ne (Find-Named $win 'Status filter' | Select-Object -First 1) } 'the filter group' | Out-Null
        $done = Find-One $win 'Done' $CT.Button
        $item = Require-Pattern $done ([System.Windows.Automation.SelectionItemPattern]::Pattern) 'SelectionItem'
        $item.Select()
        Wait-For { if ($item.Current.IsSelected) { $true } else { $null } } 'Done to be selected' | Out-Null
        $group = Find-One $win 'Status filter'
        $sel = (Require-Pattern $group ([System.Windows.Automation.SelectionPattern]::Pattern) 'Selection').Current.GetSelection()
        Assert ($sel.Count -eq 1 -and $sel[0].Current.Name -eq 'Done') 'the group does not report Done as its one selection'
        (Require-Pattern (Find-One $win 'All' $CT.Button) ([System.Windows.Automation.SelectionItemPattern]::Pattern) 'SelectionItem').Select()
    }

    Check 'demo: the search field takes a value through SetValue' {
        $search = Find-One $win 'Search projects' $CT.Edit
        $value = Require-Pattern $search ([System.Windows.Automation.ValuePattern]::Pattern) 'Value'
        $value.SetValue('Alpha')
        Wait-For { if ($value.Current.Value -eq 'Alpha') { $true } } 'the value Alpha' | Out-Null
        $value.SetValue('')
        Wait-For { if ($value.Current.Value -eq '') { $true } } 'the value to clear' | Out-Null
    }

    Check 'demo: keyboard focus moves to the search field and raises a focus event' {
        $search = Find-One $win 'Search projects' $CT.Edit
        (Find-One $win 'Dashboard' $CT.Button).SetFocus()
        Wait-For { if (([System.Windows.Automation.AutomationElement]::FocusedElement).Current.Name -eq 'Dashboard') { $true } } 'focus on Dashboard' | Out-Null
        $before = [FocusCounter]::Count
        $search.SetFocus()
        Wait-For {
            [System.Windows.Forms.Application]::DoEvents()
            if (([System.Windows.Automation.AutomationElement]::FocusedElement).Current.Name -eq 'Search projects' -and [FocusCounter]::Count -gt $before) { $true }
        } 'the focus and its event' | Out-Null
    }

    Check 'demo: the Components page scrolls through the Scroll pattern' {
        Invoke-Named $win 'Components' $CT.Button
        Wait-For { $null -ne (Find-Named $win 'Buttons' | Select-Object -First 1) } 'the Components page' | Out-Null
        $scroll = $null
        foreach ($el in $win.FindAll($Descendants, [System.Windows.Automation.Condition]::TrueCondition)) {
            $p = Get-Pattern $el ([System.Windows.Automation.ScrollPattern]::Pattern)
            if ($null -ne $p -and $p.Current.VerticallyScrollable) { $scroll = $p; break }
        }
        Assert ($null -ne $scroll) 'no vertically scrolling element'
        $scroll.SetScrollPercent([System.Windows.Automation.ScrollPattern]::NoScroll, 50)
        Wait-For { if ([Math]::Abs($scroll.Current.VerticalScrollPercent - 50) -lt 5) { $true } else { $null } } 'the scroll to 50 percent' | Out-Null
        $scroll.SetScrollPercent([System.Windows.Automation.ScrollPattern]::NoScroll, 0)
    }

    Check 'demo: the dropdown trigger expands to a menu named after it' {
        $trigger = Find-One $win 'Project actions' $CT.Button
        $ec = Require-Pattern $trigger ([System.Windows.Automation.ExpandCollapsePatternIdentifiers]::Pattern) 'ExpandCollapse'
        $ec.Expand()
        Wait-For { $null -ne (Find-Named $win 'Project actions' $CT.Menu | Select-Object -First 1) } 'the menu named Project actions' | Out-Null
        $ec.Collapse()
        Wait-Gone { Find-Named $win 'Project actions' $CT.Menu | Select-Object -First 1 } 'the menu to close' | Out-Null
    }

    Check 'demo: Shift+F10 on a project card opens its menu, and Down moves the focus through the items' {
        Invoke-Named $win 'Projects 6' $CT.Button
        Wait-For { $null -ne (Find-Named $win 'Star project' $CT.Button | Select-Object -First 1) } 'the project list' | Out-Null
        $card = Wait-For {
            $all = $win.FindAll($Descendants, [System.Windows.Automation.Condition]::TrueCondition)
            $all | Where-Object { $_.Current.ControlType -eq $CT.ListItem -and $_.Current.Name -like 'Project Alpha*' } | Select-Object -First 1
        } 'the first project card'
        $card.SetFocus()
        # The keys go to the window that has the focus, so it is brought forward first.
        [ForegroundWindow]::SetForegroundWindow([IntPtr]$win.Current.NativeWindowHandle) | Out-Null
        Start-Sleep -Milliseconds 300
        [System.Windows.Forms.SendKeys]::SendWait('+{F10}')
        $focusIs = {
            param([string]$Name, $Type)
            $f = [System.Windows.Automation.AutomationElement]::FocusedElement
            $null -ne $f -and $f.Current.Name -eq $Name -and $f.Current.ControlType -eq $Type
        }
        Wait-For { & $focusIs 'Open' $CT.MenuItem } 'Open, the first item of the card menu, to take focus' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{DOWN}')
        Wait-For { & $focusIs 'Open in window' $CT.MenuItem } 'Down to move the focus to Open in window' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{DOWN}')
        Wait-For { & $focusIs 'Star' $CT.MenuItem } 'Down to move the focus to Star' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{ESC}')
        Wait-For {
            $f = [System.Windows.Automation.AutomationElement]::FocusedElement
            $null -ne $f -and $f.Current.Name -like 'Project Alpha*' -and $f.Current.ControlType -eq $CT.ListItem
        } 'the focus back on the card after Escape' | Out-Null
    }

    Check 'demo: New Project opens a dialog; its disabled Create keeps Invoke; Cancel closes it' {
        Invoke-Named $win 'New Project' $CT.Button
        Wait-For { $null -ne (Find-Named $win 'New project' | Select-Object -First 1) } 'the New project dialog' | Out-Null
        $create = Find-One $win 'Create project' $CT.Button
        Assert (-not $create.Current.IsEnabled) 'Create project is enabled with no name'
        Require-Pattern $create ([System.Windows.Automation.InvokePattern]::Pattern) 'Invoke' | Out-Null
        Invoke-Named $win 'Cancel' $CT.Button
        Wait-Gone { Find-Named $win 'New project' | Select-Object -First 1 } 'the dialog to close' | Out-Null
    }

    Check 'demo: the Settings switches toggle with Toggle' {
        Invoke-Named $win 'Settings' $CT.Button
        Wait-For { $null -ne (Find-Named $win 'Compact activity' $CT.CheckBox | Select-Object -First 1) } 'the Settings page' | Out-Null
        $switch = Find-One $win 'Compact activity' $CT.CheckBox
        $t = Require-Pattern $switch ([System.Windows.Automation.TogglePattern]::Pattern) 'Toggle'
        $before = $t.Current.ToggleState
        $t.Toggle()
        Wait-For { if ($t.Current.ToggleState -ne $before) { $true } else { $null } } 'the switch to change' | Out-Null
        $t.Toggle()
    }

    if ($axe) {
        Check 'demo: AxeWindowsCLI scan of the Settings page' {
            $errors = Invoke-AxeScan $demo 'demo'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }
    }

    # ---- Demo: form controls on the Components page ---------------------------
    Invoke-Named $win 'Components' $CT.Button
    Wait-For { $null -ne (Find-Named $win 'Email alerts' $CT.CheckBox | Select-Object -First 1) } 'the form controls on the Components page' | Out-Null

    Check 'demo: a check box has Toggle, and the mixed state is indeterminate' {
        $box = Find-One $win 'Email alerts' $CT.CheckBox
        $t = Require-Pattern $box ([System.Windows.Automation.TogglePattern]::Pattern) 'Toggle'
        Assert ($t.Current.ToggleState -eq [System.Windows.Automation.ToggleState]::On) "Email alerts is $($t.Current.ToggleState), want On"
        $mixed = Find-One $win 'Some channels' $CT.CheckBox
        $mt = Require-Pattern $mixed ([System.Windows.Automation.TogglePattern]::Pattern) 'Toggle'
        Assert ($mt.Current.ToggleState -eq [System.Windows.Automation.ToggleState]::Indeterminate) "Some channels is $($mt.Current.ToggleState), want Indeterminate"
        $mt.Toggle()
        Wait-For { if ($mt.Current.ToggleState -eq [System.Windows.Automation.ToggleState]::On) { $true } else { $null } } 'the mixed check box to turn on' | Out-Null
        $mt.Toggle()
        Wait-For { if ($mt.Current.ToggleState -eq [System.Windows.Automation.ToggleState]::Off) { $true } else { $null } } 'the check box to turn off' | Out-Null
    }

    Check 'demo: the radio group is a Selection container, and its buttons are SelectionItems' {
        $group = Find-One $win 'Frequency' $CT.Group
        Require-Pattern $group ([System.Windows.Automation.SelectionPatternIdentifiers]::Pattern) 'Selection' | Out-Null
        $daily = Find-One $group 'Daily digest' $CT.RadioButton
        $ds = Require-Pattern $daily ([System.Windows.Automation.SelectionItemPatternIdentifiers]::Pattern) 'SelectionItem'
        $immediate = Find-One $group 'As they happen' $CT.RadioButton
        $is = Require-Pattern $immediate ([System.Windows.Automation.SelectionItemPatternIdentifiers]::Pattern) 'SelectionItem'
        Assert ($ds.Current.IsSelected) 'Daily digest is not selected at the start'
        $is.Select()
        Wait-For { if ($is.Current.IsSelected -and -not $ds.Current.IsSelected) { $true } else { $null } } 'As they happen to be selected' | Out-Null
        $ds.Select()
        Wait-For { if ($ds.Current.IsSelected) { $true } else { $null } } 'Daily digest to be selected again' | Out-Null
    }

    Check 'demo: a select is a ComboBox with ExpandCollapse and Value, and its list has items' {
        $combo = Find-One $win 'Channel' $CT.ComboBox
        $ec = Require-Pattern $combo ([System.Windows.Automation.ExpandCollapsePatternIdentifiers]::Pattern) 'ExpandCollapse'
        $vp = Require-Pattern $combo ([System.Windows.Automation.ValuePatternIdentifiers]::Pattern) 'Value'
        Assert ($vp.Current.Value -eq 'Email') "the value is '$($vp.Current.Value)', want Email"
        Assert ($vp.Current.IsReadOnly) 'the select value is not read-only'
        $ec.Expand()
        Wait-For { $null -ne (Find-Named $win 'Channel' $CT.List | Select-Object -First 1) } 'the list named Channel' | Out-Null
        Find-One $win 'Text message' $CT.ListItem | Out-Null
        $ec.Collapse()
        Wait-Gone { Find-Named $win 'Channel' $CT.List | Select-Object -First 1 } 'the list to close' | Out-Null
    }

    Check 'demo: a slider has RangeValue, and SetValue moves it' {
        $slider = Find-One $win 'Volume' $CT.Slider
        $rv = Require-Pattern $slider ([System.Windows.Automation.RangeValuePatternIdentifiers]::Pattern) 'RangeValue'
        Assert ($rv.Current.Minimum -eq 0 -and $rv.Current.Maximum -eq 100) "the range is $($rv.Current.Minimum) to $($rv.Current.Maximum)"
        $before = $rv.Current.Value
        $rv.SetValue(50)
        Wait-For { if ([Math]::Abs($rv.Current.Value - 50) -lt 0.01) { $true } else { $null } } 'the slider to move to 50' | Out-Null
        $rv.SetValue($before)
    }

    Check 'demo: a number field is a Spinner with RangeValue, and its text is an Edit' {
        $spin = Find-One $win 'Sample quiet hours' $CT.Spinner
        $rv = Require-Pattern $spin ([System.Windows.Automation.RangeValuePatternIdentifiers]::Pattern) 'RangeValue'
        $edit = Find-One $spin 'Sample quiet hours' $CT.Edit
        $vp = Require-Pattern $edit ([System.Windows.Automation.ValuePatternIdentifiers]::Pattern) 'Value'
        $before = $rv.Current.Value
        $rv.SetValue(10)
        Wait-For { if ($vp.Current.Value -eq '10') { $true } else { $null } } 'the text to show 10' | Out-Null
        $rv.SetValue($before)
    }

    if ($axe) {
        Check 'demo: AxeWindowsCLI scan of the Components page, scrolled to the Form controls section' {
            $heading = Find-One $win 'Email alerts' $CT.CheckBox
            $item = Require-Pattern $heading ([System.Windows.Automation.ScrollItemPatternIdentifiers]::Pattern) 'ScrollItem'
            $item.ScrollIntoView()
            Wait-For { if ($heading.Current.BoundingRectangle.Top -lt $win.Current.BoundingRectangle.Bottom) { $true } else { $null } } 'the Form controls section to be in view' | Out-Null
            $errors = Invoke-AxeScan $demo 'demo-components'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }

        Check 'demo: AxeWindowsCLI scan of the Components page with a select open' {
            $combo = Find-One $win 'Channel' $CT.ComboBox
            $ec = Require-Pattern $combo ([System.Windows.Automation.ExpandCollapsePatternIdentifiers]::Pattern) 'ExpandCollapse'
            $ec.Expand()
            Wait-For { $null -ne (Find-Named $win 'Channel' $CT.List | Select-Object -First 1) } 'the list to open' | Out-Null
            try {
                $errors = Invoke-AxeScan $demo 'demo-select-open'
            } finally {
                $ec.Collapse()
            }
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }
    }

    # ---- Notes ---------------------------------------------------------------
    $notes = Start-App $notesExe $notesFile
    $nwin = Get-Window $notes
    Wait-For { $null -ne (Find-Named $nwin 'Body' $CT.Edit | Select-Object -First 1) } 'the notes editor' | Out-Null

    Check 'notes: the menu bar and editor controls have their roles' {
        foreach ($n in 'File', 'Edit', 'View') {
            Assert (@(Find-Named $nwin $n $CT.MenuItem).Count -ge 1) "menu '$n' is not a menu item"
        }
        Find-One $nwin 'Search notes' $CT.Edit | Out-Null
        Find-One $nwin 'Title' $CT.Edit | Out-Null
    }

    Check 'notes: the File menu expands to a menu named File' {
        $file = Find-One $nwin 'File' $CT.MenuItem
        $ec = Require-Pattern $file ([System.Windows.Automation.ExpandCollapsePatternIdentifiers]::Pattern) 'ExpandCollapse'
        $ec.Expand()
        Wait-For { $null -ne (Find-Named $nwin 'File' $CT.Menu | Select-Object -First 1) } 'the File menu' | Out-Null
        $ec.Collapse()
        Wait-Gone { Find-Named $nwin 'File' $CT.Menu | Select-Object -First 1 } 'the File menu to close' | Out-Null
    }

    Check 'notes: F10 focuses the menu bar, and Down opens File on its first item' {
        $search = Find-One $nwin 'Search notes' $CT.Edit
        $search.SetFocus()
        [ForegroundWindow]::SetForegroundWindow([IntPtr]$nwin.Current.NativeWindowHandle) | Out-Null
        Start-Sleep -Milliseconds 300
        [System.Windows.Forms.SendKeys]::SendWait('{F10}')
        Wait-For { $f = [System.Windows.Automation.AutomationElement]::FocusedElement; $null -ne $f -and $f.Current.Name -eq 'File' -and $f.Current.ControlType -eq $CT.MenuItem } 'F10 to focus the File label' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{DOWN}')
        Wait-For { $null -ne (Find-Named $nwin 'File' $CT.Menu | Select-Object -First 1) } 'Down to open the File menu' | Out-Null
        $menu = Find-One $nwin 'File' $CT.Menu
        $first = $menu.FindFirst([System.Windows.Automation.TreeScope]::Children, [System.Windows.Automation.Condition]::TrueCondition)
        Assert ($null -ne $first) 'the File menu has no items'
        $expected = $first.Current.Name
        Wait-For { $f = [System.Windows.Automation.AutomationElement]::FocusedElement; $null -ne $f -and $f.Current.Name -eq $expected -and $f.Current.ControlType -eq $CT.MenuItem } "the first File item '$expected' to take focus" | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{ESC}')
        Wait-Gone { Find-Named $nwin 'File' $CT.Menu | Select-Object -First 1 } 'the File menu to close' | Out-Null
    }

    Check 'notes: Pin note toggles with Toggle, and has no Invoke pattern' {
        $pin = Find-One $nwin 'Pin note' $CT.Button
        Assert ($null -eq (Get-Pattern $pin ([System.Windows.Automation.InvokePattern]::Pattern))) 'a toggle button also has Invoke'
        $t = Require-Pattern $pin ([System.Windows.Automation.TogglePattern]::Pattern) 'Toggle'
        $before = $t.Current.ToggleState
        $t.Toggle()
        Wait-For { if ($t.Current.ToggleState -ne $before) { $true } else { $null } } 'the pin to change' | Out-Null
        $t.Toggle()
    }

    Check 'notes: the title takes a value through SetValue' {
        $title = Find-One $nwin 'Title' $CT.Edit
        $v = Require-Pattern $title ([System.Windows.Automation.ValuePattern]::Pattern) 'Value'
        $v.SetValue('Accessibility check')
        Wait-For { if ($v.Current.Value -eq 'Accessibility check') { $true } } 'the title to take the value' | Out-Null
    }

    Check 'notes: the body reads and selects the word "small" through the Text pattern' {
        $body = Find-One $nwin 'Body' $CT.Edit
        # A text field returns no selection until it has keyboard focus.
        $body.SetFocus()
        Wait-For { if (([System.Windows.Automation.AutomationElement]::FocusedElement).Current.Name -eq 'Body') { $true } } 'focus on the body' | Out-Null
        $tp = Require-Pattern $body ([System.Windows.Automation.TextPattern]::Pattern) 'Text'
        $all = $tp.DocumentRange
        Assert ($all.GetText(-1) -match 'small note-taking') 'the body text does not hold the welcome sentence'
        $found = $all.FindText('small', $false, $false)
        Assert ($null -ne $found) 'FindText did not find "small"'
        $found.Select()
        # Select runs on the UI thread, and the next frame publishes the new selection,
        # so a read right after it can still see the old one. Poll until it is there.
        $sel = @(Wait-For {
            $s = $tp.GetSelection()
            if ($s.Length -eq 1 -and $s[0].GetText(-1) -eq 'small') { , $s }
        } 'the selection of "small" to be published')
        Assert ($sel.Length -eq 1 -and $sel[0].GetText(-1) -eq 'small') "the selection is '$($sel[0].GetText(-1))'"
        $word = $found.Clone()
        $word.ExpandToEnclosingUnit([System.Windows.Automation.Text.TextUnit]::Word)
        Assert ($word.GetText(-1).Trim() -eq 'small') "the word unit is '$($word.GetText(-1))'"
        $rects = @($found.GetBoundingRectangles())
        Assert ($rects.Count -eq 1 -and $rects[0].Width -gt 0 -and $rects[0].Height -gt 0) 'the word does not have one bounding rectangle with an area'
    }

    if ($axe) {
        Check 'notes: AxeWindowsCLI scan of the editor' {
            $errors = Invoke-AxeScan $notes 'notes'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }
    }
} finally {
    [FocusCounter]::Stop()
    foreach ($p in $Procs) {
        if ($p -and -not $p.HasExited) { $p.Kill() }
    }
    foreach ($p in $Procs) {
        if ($p) { $p.WaitForExit(5000) | Out-Null }
    }
    if (Test-Path $Work) { Remove-Item -Recurse -Force $Work -ErrorAction SilentlyContinue }
}

Write-Output ''
Write-Output "Summary: $Passed passed, $Failed failed"
if ($Failed -gt 0) { exit 1 }
exit 0
