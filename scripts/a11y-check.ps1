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

# A Notification event is raised for each announcement (Announce). The event's
# arguments do not carry the text, so the check counts the events and reads the
# text from the toast that the announcement belongs to.
Add-Type -ReferencedAssemblies UIAutomationClient, UIAutomationTypes -TypeDefinition @'
using System.Threading;
using System.Windows.Automation;

public static class NotificationCounter
{
    public static int Count;

    public static void Handler(object sender, AutomationEventArgs e)
    {
        Interlocked.Increment(ref Count);
    }

    public static void Start(AutomationElement window)
    {
        Automation.AddAutomationEventHandler(AutomationElement.NotificationEvent, window, TreeScope.Subtree,
            new AutomationEventHandler(Handler));
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
    ProgressBar = [System.Windows.Automation.ControlType]::ProgressBar
    Spinner  = [System.Windows.Automation.ControlType]::Spinner
    Text     = [System.Windows.Automation.ControlType]::Text
    Tab      = [System.Windows.Automation.ControlType]::Tab
    TabItem  = [System.Windows.Automation.ControlType]::TabItem
    Tree     = [System.Windows.Automation.ControlType]::Tree
    TreeItem = [System.Windows.Automation.ControlType]::TreeItem
    Pane     = [System.Windows.Automation.ControlType]::Pane
    DataGrid = [System.Windows.Automation.ControlType]::DataGrid
    DataItem = [System.Windows.Automation.ControlType]::DataItem
    HeaderItem = [System.Windows.Automation.ControlType]::HeaderItem
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

# Get-Prop reads a property by its UI Automation ID. The .NET client accepts only the
# properties it names, so the call goes to the client's own raw reader with the
# element's native handle (UiaGetPropertyValue, which the client calls itself).
function Get-Prop($El, [int]$Id) {
    $api = [System.Windows.Automation.AutomationElement].Assembly.GetType('MS.Internal.Automation.UiaCoreApi')
    $read = $api.GetMethod('UiaGetPropertyValue', [Reflection.BindingFlags]'Static,NonPublic,Public')
    $handle = [System.Windows.Automation.AutomationElement].GetField('_hnode', [Reflection.BindingFlags]'Instance,NonPublic').GetValue($El)
    $callArgs = [object[]]@($handle, [int]$Id, $null)
    [void]$read.Invoke($null, $callArgs)
    return $callArgs[2]
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
# exits 0 with no errors and 1 with errors; anything else is a failure to scan,
# which happens now and then when the tree changes during the scan, so such a
# scan is tried up to three times before the check fails.
function Invoke-AxeScan($Proc, [string]$Name) {
    $out = Join-Path $Work "axe-$Name"
    New-Item -ItemType Directory -Force -Path $out | Out-Null
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        # The scanner writes its report to stderr, which is captured here, so the
        # native-error handling of the session must not turn those lines into errors.
        $oldPreference = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try {
            $log = & $axe --processid $Proc.Id --outputdirectory $out --scanid $Name --verbosity verbose --delayinseconds 2 2>&1 | ForEach-Object { "$_" }
            $code = $LASTEXITCODE
        } finally {
            $ErrorActionPreference = $oldPreference
        }
        $text = $log -join "`n"
        if ($code -eq 0) { return 0 }
        if ($code -eq 1 -and $text -match '(\d+) errors? (were|was) found') {
            # Show what the scanner found, since its report folder is removed on exit.
            Write-Host -Separator "`n" ($log | Where-Object { $_ -ne '' } | ForEach-Object { "  axe: $_" })
            return [int]$matches[1]
        }
        if ($attempt -lt 3) {
            Write-Host "  (AxeWindowsCLI exited $code without a count; scanning again)"
            Start-Sleep -Seconds 2
        }
    }
    throw "AxeWindowsCLI exited $code without a count, three times: $text"
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
        Assert (@(Find-Named $win 'Projects, 6 items' $CT.Button).Count -eq 1) "'Projects, 6 items' is not one button"
    }

    Check 'demo: Invoke opens the Activity page' {
        Invoke-Named $win 'Activity' $CT.Button
        Wait-For { $null -ne (Find-Named $win '10,000 events from 5 people' | Select-Object -First 1) } 'the Activity page' | Out-Null
    }

    Check 'demo: a star toggles with Toggle, and has no Invoke pattern' {
        Invoke-Named $win 'Projects, 6 items' $CT.Button
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

    Check 'demo: the Done tab is selected in the status filter' {
        Invoke-Named $win 'Projects, 6 items' $CT.Button
        Wait-For { $null -ne (Find-Named $win 'Status filter' $CT.Tab | Select-Object -First 1) } 'the status filter' | Out-Null
        $done = Find-One $win 'Done' $CT.TabItem
        $item = Require-Pattern $done ([System.Windows.Automation.SelectionItemPattern]::Pattern) 'SelectionItem'
        $item.Select()
        Wait-For { if ($item.Current.IsSelected) { $true } else { $null } } 'Done to be selected' | Out-Null
        $group = Find-One $win 'Status filter' $CT.Tab
        $sel = (Require-Pattern $group ([System.Windows.Automation.SelectionPattern]::Pattern) 'Selection').Current.GetSelection()
        Assert ($sel.Count -eq 1 -and $sel[0].Current.Name -eq 'Done') 'the status filter does not report Done as its one selection'
        (Require-Pattern (Find-One $win 'All' $CT.TabItem) ([System.Windows.Automation.SelectionItemPattern]::Pattern) 'SelectionItem').Select()
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

    Check 'demo: the header combobox expands, highlights and picks through UI Automation' {
        $search = Find-One $win 'Search projects' $CT.Edit
        $box = Find-One $win 'Search projects' $CT.ComboBox
        $ec = Require-Pattern $box ([System.Windows.Automation.ExpandCollapsePatternIdentifiers]::Pattern) 'ExpandCollapse'
        $value = Require-Pattern $search ([System.Windows.Automation.ValuePattern]::Pattern) 'Value'
        [ForegroundWindow]::SetForegroundWindow([IntPtr]$win.Current.NativeWindowHandle) | Out-Null
        $search.SetFocus()
        Wait-For { if (([System.Windows.Automation.AutomationElement]::FocusedElement).Current.Name -eq 'Search projects') { $true } } 'focus on the search field' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('alp')
        Wait-For { if ($ec.Current.ExpandCollapseState -eq [System.Windows.Automation.ExpandCollapseState]::Expanded) { $true } } 'the combo box to expand' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{DOWN}')
        # The text keeps the focus; the highlighted suggestion is the focused element.
        Wait-For {
            $f = [System.Windows.Automation.AutomationElement]::FocusedElement
            if ($f.Current.ControlType -eq $CT.ListItem -and $f.Current.Name -eq 'Project Alpha') { $true }
        } 'Project Alpha to be the focused suggestion' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{ENTER}')
        Wait-For { if ($ec.Current.ExpandCollapseState -eq [System.Windows.Automation.ExpandCollapseState]::Collapsed) { $true } } 'the combo box to collapse' | Out-Null
        Wait-For { if ($value.Current.Value -eq 'Project Alpha') { $true } } 'the value Project Alpha' | Out-Null
        $value.SetValue('')
        Wait-For { if ($value.Current.Value -eq '') { $true } } 'the value to clear' | Out-Null
    }

    Check 'demo: Escape closes the header combobox list, and a second Escape clears its text' {
        $search = Find-One $win 'Search projects' $CT.Edit
        $box = Find-One $win 'Search projects' $CT.ComboBox
        $ec = Require-Pattern $box ([System.Windows.Automation.ExpandCollapsePatternIdentifiers]::Pattern) 'ExpandCollapse'
        $value = Require-Pattern $search ([System.Windows.Automation.ValuePattern]::Pattern) 'Value'
        [ForegroundWindow]::SetForegroundWindow([IntPtr]$win.Current.NativeWindowHandle) | Out-Null
        $search.SetFocus()
        Wait-For { if (([System.Windows.Automation.AutomationElement]::FocusedElement).Current.Name -eq 'Search projects') { $true } } 'focus on the search field' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('alp')
        Wait-For { if ($ec.Current.ExpandCollapseState -eq [System.Windows.Automation.ExpandCollapseState]::Expanded) { $true } } 'the combo box to expand' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{ESC}')
        Wait-For { if ($ec.Current.ExpandCollapseState -eq [System.Windows.Automation.ExpandCollapseState]::Collapsed) { $true } } 'the combo box to collapse on Escape' | Out-Null
        Assert ($value.Current.Value -eq 'alp') 'the first Escape changed the text'
        [System.Windows.Forms.SendKeys]::SendWait('{ESC}')
        Wait-For { if ($value.Current.Value -eq '') { $true } } 'the second Escape to clear the text' | Out-Null
    }

    if ($axe) {
        Check 'demo: AxeWindowsCLI scan of the header combobox with suggestions open' {
            $search = Find-One $win 'Search projects' $CT.Edit
            $box = Find-One $win 'Search projects' $CT.ComboBox
            $ec = Require-Pattern $box ([System.Windows.Automation.ExpandCollapsePatternIdentifiers]::Pattern) 'ExpandCollapse'
            $value = Require-Pattern $search ([System.Windows.Automation.ValuePattern]::Pattern) 'Value'
            [ForegroundWindow]::SetForegroundWindow([IntPtr]$win.Current.NativeWindowHandle) | Out-Null
            $search.SetFocus()
            Wait-For { if (([System.Windows.Automation.AutomationElement]::FocusedElement).Current.Name -eq 'Search projects') { $true } } 'focus on the search field' | Out-Null
            [System.Windows.Forms.SendKeys]::SendWait('alp')
            Wait-For { if ($ec.Current.ExpandCollapseState -eq [System.Windows.Automation.ExpandCollapseState]::Expanded) { $true } } 'the combo box to expand' | Out-Null
            try {
                $errors = Invoke-AxeScan $demo 'demo-combobox-open'
            } finally {
                # Cleanup only: the list is closed and the text cleared, so the other checks see the page as it was.
                if ($ec.Current.ExpandCollapseState -eq [System.Windows.Automation.ExpandCollapseState]::Expanded) { $ec.Collapse() }
                $value.SetValue('')
            }
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI with the suggestions open"
            "$errors errors"
        }
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
        Invoke-Named $win 'Projects, 6 items' $CT.Button
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

    Check 'demo: New Project opens a dialog; Create is enabled and keeps Invoke, with no error shown yet; Cancel closes it' {
        Invoke-Named $win 'New Project' $CT.Button
        Wait-For { $null -ne (Find-Named $win 'New project' | Select-Object -First 1) } 'the New project dialog' | Out-Null
        $create = Find-One $win 'Create project' $CT.Button
        Assert ($create.Current.IsEnabled) 'Create project is disabled; a submit with no name should show the error'
        Assert ($null -eq (Find-Named $win '1 field needs attention.' | Select-Object -First 1)) 'an error is shown before Create was pressed'
        Require-Pattern $create ([System.Windows.Automation.InvokePattern]::Pattern) 'Invoke' | Out-Null
        Invoke-Named $win 'Cancel' $CT.Button
        Wait-Gone { Find-Named $win 'New project' | Select-Object -First 1 } 'the dialog to close' | Out-Null
    }

    Check 'demo: New Project with an empty name flags the name, moves focus to it, and announces the error' {
        [NotificationCounter]::Start($win)
        $before = [NotificationCounter]::Count
        Invoke-Named $win 'New Project' $CT.Button
        Wait-For { $null -ne (Find-Named $win 'New project' | Select-Object -First 1) } 'the New project dialog' | Out-Null
        Invoke-Named $win 'Create project' $CT.Button
        $name = Wait-For { Find-Named $win 'Name' $CT.Edit | Select-Object -First 1 } 'the Name field of the dialog'
        Wait-For { $f = [System.Windows.Automation.AutomationElement]::FocusedElement; if ($f -and $f.Current.Name -eq 'Name' -and $f.Current.ControlType -eq $CT.Edit) { $true } else { $null } } 'focus on the Name field' | Out-Null
        Assert ((Get-Prop $name 30025) -eq $true) 'the Name field does not report IsRequiredForForm'
        Assert ((Get-Prop $name 30103) -eq $false) 'the Name field reports IsDataValidForForm true while it has an error'
        Assert ($name.Current.HelpText -eq 'This field is required.') "the Name field's description is '$($name.Current.HelpText)'"
        Wait-For { if ([NotificationCounter]::Count -gt $before) { $true } else { $null } } 'the error announcement' | Out-Null
        Wait-For { $null -ne (Find-Named $win '1 field needs attention.' | Select-Object -First 1) } 'the error summary' | Out-Null
        if ($axe) {
            $errors = Invoke-AxeScan $demo 'demo-new-project-errors'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
        }
        # The first character clears the required error. The field keeps focus, so the
        # rest of the text goes into it.
        [System.Windows.Forms.SendKeys]::SendWait('L')
        Wait-For {
            $f = [System.Windows.Automation.AutomationElement]::FocusedElement
            if ($f -and $f.Current.Name -eq 'Name' -and $f.Current.ControlType -eq $CT.Edit) { $true } else { $null }
        } 'focus on the Name field after its error cleared' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('ab')
        $typed = Find-Named $win 'Name' $CT.Edit | Select-Object -First 1
        $typedValue = (Require-Pattern $typed ([System.Windows.Automation.ValuePatternIdentifiers]::Pattern) 'Value').Current.Value
        Assert ($typedValue -eq 'Lab') "the Name field holds '$typedValue' after typing Lab, want Lab"
        (Require-Pattern $typed ([System.Windows.Automation.ValuePatternIdentifiers]::Pattern) 'Value').SetValue('Lab project')
        Invoke-Named $win 'Create project' $CT.Button
        Wait-Gone { Find-Named $win 'New project' | Select-Object -First 1 } 'the dialog to close after Create' | Out-Null
        # Create shows the project with an Undo action; Undo keeps the projects the later checks count.
        Invoke-Named $win 'Undo' $CT.Button
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

    # ---- Demo: forms on the Components page ----------------------------------
    Check 'demo: the Forms section: an empty save shows its summary, and the summary link moves focus to Name' {
        Invoke-Named $win 'Save' $CT.Button
        Wait-For { $null -ne (Find-Named $win '1 field needs attention.' | Select-Object -First 1) } 'the Forms summary' | Out-Null
        $link = Find-One $win 'Name: This field is required.'
        (Require-Pattern $link ([System.Windows.Automation.InvokePattern]::Pattern) 'Invoke').Invoke()
        Wait-For { $f = [System.Windows.Automation.AutomationElement]::FocusedElement; if ($f -and $f.Current.Name -eq 'Name' -and $f.Current.ControlType -eq $CT.Edit) { $true } else { $null } } 'the summary link to move focus to the Name field' | Out-Null
        $name = Find-One $win 'Name' $CT.Edit
        Assert ((Get-Prop $name 30025) -eq $true) 'the Forms name field does not report IsRequiredForForm'
        Assert ((Get-Prop $name 30103) -eq $false) 'the Forms name field reports IsDataValidForForm true while it has an error'
    }

    if ($axe) {
        Check 'demo: AxeWindowsCLI scan of the Forms section with its summary shown' {
            $save = Find-One $win 'Save' $CT.Button
            $item = Require-Pattern $save ([System.Windows.Automation.ScrollItemPatternIdentifiers]::Pattern) 'ScrollItem'
            $item.ScrollIntoView()
            Wait-For { if ($save.Current.BoundingRectangle.Top -lt $win.Current.BoundingRectangle.Bottom) { $true } else { $null } } 'the Save button to be in view' | Out-Null
            $errors = Invoke-AxeScan $demo 'demo-forms-summary'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }
    }

    Check 'demo: the Forms save runs in the background with Save disabled, and the server refusal is shown' {
        $name = Find-One $win 'Name' $CT.Edit
        (Require-Pattern $name ([System.Windows.Automation.ValuePatternIdentifiers]::Pattern) 'Value').SetValue('taken')
        Invoke-Named $win 'Save' $CT.Button
        Wait-For { $s = @(Find-Named $win 'Save' $CT.Button)[0]; if ($s -and -not $s.Current.IsEnabled) { $true } else { $null } } 'Save to be disabled while the save runs' | Out-Null
        Wait-For { $null -ne (Find-Named $win 'That name is already in use. Choose another.' | Select-Object -First 1) } 'the server refusal to be shown' | Out-Null
        Wait-For { $s = @(Find-Named $win 'Save' $CT.Button)[0]; if ($s -and $s.Current.IsEnabled) { $true } else { $null } } 'Save to be enabled again' | Out-Null
    }

    # ---- Demo: navigation and layout on the Components page ------------------
    # The section is above the form controls, and its controls are built with it.
    Check 'demo: a tab list is a Tab, its tabs are TabItems with SelectionItem, and Select shows the panel' {
        $list = Find-One $win 'Plain sections' $CT.Tab
        $alpha = Find-One $list 'Alpha' $CT.TabItem
        $gamma = Find-One $list 'Gamma' $CT.TabItem
        $ai = Require-Pattern $alpha ([System.Windows.Automation.SelectionItemPatternIdentifiers]::Pattern) 'SelectionItem'
        $gi = Require-Pattern $gamma ([System.Windows.Automation.SelectionItemPatternIdentifiers]::Pattern) 'SelectionItem'
        Require-Pattern $list ([System.Windows.Automation.SelectionPatternIdentifiers]::Pattern) 'Selection' | Out-Null
        Assert ($ai.Current.IsSelected) 'Alpha is not selected at the start'
        $gi.Select()
        Wait-For { if ($gi.Current.IsSelected -and -not $ai.Current.IsSelected) { $true } else { $null } } 'Gamma to be selected' | Out-Null
        Wait-For { $null -ne (Find-Named $win 'Gamma panel' $CT.Text | Select-Object -First 1) } 'the Gamma panel to be shown' | Out-Null
        Assert ($null -eq (Find-Named $win 'Alpha panel' $CT.Text | Select-Object -First 1)) 'the Alpha panel is still shown'
        Find-One $win 'Gamma' $CT.Pane | Out-Null
        $ai.Select()
        Wait-For { if ($ai.Current.IsSelected) { $true } else { $null } } 'Alpha to be selected again' | Out-Null
    }

    Check 'demo: a kept tab panel keeps its field text, and the hidden field is not in the tree' {
        $list = Find-One $win 'Plain sections' $CT.Tab
        $alpha = Find-One $list 'Alpha' $CT.TabItem
        $gamma = Find-One $list 'Gamma' $CT.TabItem
        $ai = Require-Pattern $alpha ([System.Windows.Automation.SelectionItemPatternIdentifiers]::Pattern) 'SelectionItem'
        $gi = Require-Pattern $gamma ([System.Windows.Automation.SelectionItemPatternIdentifiers]::Pattern) 'SelectionItem'
        $ai.Select()
        Wait-For { if ($ai.Current.IsSelected) { $true } else { $null } } 'Alpha to be selected' | Out-Null
        $field = Find-One $win 'Note' $CT.Edit
        $vp = Require-Pattern $field ([System.Windows.Automation.ValuePatternIdentifiers]::Pattern) 'Value'
        $vp.SetValue('kept text')
        Wait-For { if ($vp.Current.Value -eq 'kept text') { $true } else { $null } } 'the field to hold the text' | Out-Null
        $gi.Select()
        Wait-For { if ($gi.Current.IsSelected) { $true } else { $null } } 'Gamma to be selected' | Out-Null
        Wait-Gone { Find-Named $win 'Note' $CT.Edit | Select-Object -First 1 } 'the hidden Note field to leave the tree' | Out-Null
        $ai.Select()
        $field = Wait-For { Find-Named $win 'Note' $CT.Edit | Select-Object -First 1 } 'the Note field to be shown again'
        $vp = Require-Pattern $field ([System.Windows.Automation.ValuePatternIdentifiers]::Pattern) 'Value'
        Assert ($vp.Current.Value -eq 'kept text') "the field holds '$($vp.Current.Value)' after switching back, want 'kept text'"
        $vp.SetValue('')
    }

    Check 'demo: a collapsible header is a Button with ExpandCollapse, and its content opens and closes' {
        $header = Find-One $win 'Advanced options' $CT.Button
        $ec = Require-Pattern $header ([System.Windows.Automation.ExpandCollapsePatternIdentifiers]::Pattern) 'ExpandCollapse'
        Assert ($ec.Current.ExpandCollapseState -eq [System.Windows.Automation.ExpandCollapseState]::Collapsed) 'the header is not collapsed at the start'
        $ec.Expand()
        Wait-For { $null -ne (Find-Named $win 'The options are shown while the section is open.' $CT.Text | Select-Object -First 1) } 'the content to open' | Out-Null
        $ec.Collapse()
        Wait-Gone { Find-Named $win 'The options are shown while the section is open.' $CT.Text | Select-Object -First 1 } 'the content to close' | Out-Null
    }

    Check 'demo: a tree is a Tree whose items expand, with levels and positions in their sets' {
        $tree = Find-One $win 'Project files' $CT.Tree
        $src = Find-One $tree 'src' $CT.TreeItem
        $ec = Require-Pattern $src ([System.Windows.Automation.ExpandCollapsePatternIdentifiers]::Pattern) 'ExpandCollapse'
        # The tree is virtualized: its rows are built once it has been drawn, so the tree
        # is scrolled into view of the page before its items are expanded.
        $item = Require-Pattern $tree ([System.Windows.Automation.ScrollItemPatternIdentifiers]::Pattern) 'ScrollItem'
        $item.ScrollIntoView()
        Wait-For { if ($src.Current.BoundingRectangle.Height -gt 0) { $true } else { $null } } 'src to be drawn' | Out-Null
        $pos = Get-Prop $src 30152
        $size = Get-Prop $src 30153
        $level = Get-Prop $src 30154
        Assert ($pos -eq 1 -and $size -eq 3 -and $level -eq 1) "src is at $pos of $size, level $level; want 1 of 3, level 1"
        $ec.Expand()
        Wait-For { $null -ne (Find-Named $win 'main.go' $CT.TreeItem | Select-Object -First 1) } 'the children of src' | Out-Null
        $main = Find-One $win 'main.go' $CT.TreeItem
        $level = Get-Prop $main 30154
        Assert ($level -eq 2) "main.go is at level $level, want 2"
        $ec.Collapse()
        Wait-Gone { Find-Named $win 'main.go' $CT.TreeItem | Select-Object -First 1 } 'the children of src to close' | Out-Null
    }

    Check 'demo: Down and Right move through the tree with the keys, and focus follows the selection' {
        $tree = Find-One $win 'Project files' $CT.Tree
        $focusIs = {
            param([string]$Name, $Type)
            $f = [System.Windows.Automation.AutomationElement]::FocusedElement
            $null -ne $f -and $f.Current.Name -eq $Name -and $f.Current.ControlType -eq $Type
        }
        $tree.SetFocus()
        [ForegroundWindow]::SetForegroundWindow([IntPtr]$win.Current.NativeWindowHandle) | Out-Null
        Start-Sleep -Milliseconds 300
        Wait-For { & $focusIs 'src' $CT.TreeItem } 'the selected tree item to take focus' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{DOWN}')
        Wait-For { & $focusIs 'docs' $CT.TreeItem } 'Down to move the focus to docs' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{RIGHT}')
        Wait-For { $null -ne (Find-Named $win 'readme.md' $CT.TreeItem | Select-Object -First 1) } 'Right to open docs' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{RIGHT}')
        Wait-For { & $focusIs 'readme.md' $CT.TreeItem } 'Right on an open item to move the focus to its first child' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{LEFT}')
        Wait-For { & $focusIs 'docs' $CT.TreeItem } 'Left to move the focus to the parent' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{LEFT}')
        Wait-Gone { Find-Named $win 'readme.md' $CT.TreeItem | Select-Object -First 1 } 'Left on an open item to close docs' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{UP}')
        Wait-For { & $focusIs 'src' $CT.TreeItem } 'Up to move the focus back to src' | Out-Null
    }

    if ($axe) {
        Check 'demo: AxeWindowsCLI scan of the Components page, scrolled to the navigation section' {
            $header = Find-One $win 'Advanced options' $CT.Button
            $item = Require-Pattern $header ([System.Windows.Automation.ScrollItemPatternIdentifiers]::Pattern) 'ScrollItem'
            $item.ScrollIntoView()
            Wait-For { if ($header.Current.BoundingRectangle.Top -lt $win.Current.BoundingRectangle.Bottom) { $true } else { $null } } 'the navigation section to be in view' | Out-Null
            $errors = Invoke-AxeScan $demo 'demo-navigation'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }
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

        Check 'demo: AxeWindowsCLI scan of the Components page, scrolled to the Customizing section' {
            $spin = Find-One $win 'Hour, stacked' $CT.Spinner
            $item = Require-Pattern $spin ([System.Windows.Automation.ScrollItemPatternIdentifiers]::Pattern) 'ScrollItem'
            $item.ScrollIntoView()
            Wait-For { if ($spin.Current.BoundingRectangle.Top -lt $win.Current.BoundingRectangle.Bottom) { $true } else { $null } } 'the Customizing section to be in view' | Out-Null
            $errors = Invoke-AxeScan $demo 'demo-customizing'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }
    }

    if ($axe) {
        Check 'demo: AxeWindowsCLI scan of the Projects page with its status filter' {
            Invoke-Named $win 'Projects, 6 items' $CT.Button
            Wait-For { $null -ne (Find-Named $win 'Status filter' $CT.Tab | Select-Object -First 1) } 'the status filter' | Out-Null
            $errors = Invoke-AxeScan $demo 'demo-projects'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }
    }

    # ---- Demo: data grids (the Projects table and the Components data grid) ----

    $gridNamed = {
        param([string]$Name)
        Find-Named $win $Name $CT.DataGrid | Select-Object -First 1
    }
    # Grid helpers, shared by the Projects and Components checks: focus the grid and
    # send keys (each key waits a little, so the grid handles it before the next), and
    # find the focused edit field by its name.
    $focusGrid = {
        param($g)
        $g.SetFocus()
        [ForegroundWindow]::SetForegroundWindow([IntPtr]$win.Current.NativeWindowHandle) | Out-Null
        Start-Sleep -Milliseconds 300
    }
    $sendKeys = {
        param([string]$Seq)
        [System.Windows.Forms.SendKeys]::SendWait($Seq)
        Start-Sleep -Milliseconds 120
    }
    # The built data item at the 0-based row and column of a grid, or nothing.
    $cellAt = {
        param($Grid, [int]$Row, [int]$Col)
        $cond = New-Object System.Windows.Automation.PropertyCondition -ArgumentList ([System.Windows.Automation.AutomationElement]::ControlTypeProperty), $CT.DataItem
        foreach ($el in $Grid.FindAll($Descendants, $cond)) {
            $gi = $el.GetCurrentPattern([System.Windows.Automation.GridItemPattern]::Pattern)
            if ([int]$gi.Current.Row -eq $Row -and [int]$gi.Current.Column -eq $Col) { return $el }
        }
        return $null
    }
    $focusedEdit = {
        param([string]$Name)
        $f = [System.Windows.Automation.AutomationElement]::FocusedElement
        if ($f -and $f.Current.ControlType -eq $CT.Edit -and $f.Current.Name -eq $Name) { return $f }
        return $null
    }
    # The view tabs are selection items, so a tab is chosen with Select.
    $selectTab = {
        param([string]$Name)
        $tab = Find-One $win $Name $CT.TabItem
        (Require-Pattern $tab ([System.Windows.Automation.SelectionItemPattern]::Pattern) 'SelectionItem').Select()
    }
    $cellAtFocus = {
        $f = [System.Windows.Automation.AutomationElement]::FocusedElement
        if ($null -eq $f -or $f.Current.ControlType -ne $CT.DataItem) { return $null }
        $gi = $f.GetCurrentPattern([System.Windows.Automation.GridItemPattern]::Pattern)
        return ,@([int]$gi.Current.Row, [int]$gi.Current.Column)
    }

    Check 'demo: the Projects table is a DataGrid with the Grid and Table patterns' {
        Invoke-Named $win 'Projects, 6 items' $CT.Button
        Wait-For { $null -ne (Find-Named $win 'Table' $CT.TabItem | Select-Object -First 1) } 'the Projects view tabs' | Out-Null
        & $selectTab 'Table'
        $grid = Wait-For { & $gridNamed 'Projects' } 'the Projects table'
        $g = Require-Pattern $grid ([System.Windows.Automation.GridPattern]::Pattern) 'Grid'
        $t = Require-Pattern $grid ([System.Windows.Automation.TablePattern]::Pattern) 'Table'
        Assert ($g.Current.RowCount -eq 6) "RowCount is $($g.Current.RowCount), want the 6 projects"
        Assert ($g.Current.ColumnCount -eq 6) "ColumnCount is $($g.Current.ColumnCount), want 6 (with the Open column)"
        Assert (@($t.Current.GetColumnHeaders()).Count -eq 6) 'GetColumnHeaders does not give the 6 header items'
        $cell = $g.GetItem(0, 0)
        Assert ($null -ne $cell) 'GetItem(0, 0) is null for a visible cell'
        $gi = Require-Pattern $cell ([System.Windows.Automation.GridItemPattern]::Pattern) 'GridItem'
        Assert ($gi.Current.Row -eq 0 -and $gi.Current.Column -eq 0) 'the first cell is not at row 0, column 0'
        Require-Pattern $cell ([System.Windows.Automation.TableItemPattern]::Pattern) 'TableItem' | Out-Null
        "RowCount 6, ColumnCount 6"
    }

    Check 'demo: a Projects header sorts with Invoke, and its name and ItemStatus say so' {
        $grid = & $gridNamed 'Projects'
        $g = Require-Pattern $grid ([System.Windows.Automation.GridPattern]::Pattern) 'Grid'
        $hdr = Find-One $win 'Name' $CT.HeaderItem
        $inv = Require-Pattern $hdr ([System.Windows.Automation.InvokePattern]::Pattern) 'Invoke'
        $inv.Invoke()
        $sorted = Wait-For { Find-Named $win 'Name, sorted ascending' $CT.HeaderItem | Select-Object -First 1 } 'the Name header to say it is sorted ascending'
        Assert ((Get-Prop $sorted 30026) -eq 'sorted ascending') 'ItemStatus of the sorted header is not "sorted ascending"'
        $first = Wait-For { $n = $g.GetItem(0, 0).Current.Name; if ($n -eq 'Dev Runner') { $n } else { $null } } 'the first row to be Dev Runner after a sort by name'
        Assert ($first -eq 'Dev Runner') "the first row after a sort by name is '$first', want 'Dev Runner'"
    }

    Check 'demo: rows of the Projects table select with SelectionItem, and the table takes many' {
        $grid = & $gridNamed 'Projects'
        $sel = Require-Pattern $grid ([System.Windows.Automation.SelectionPattern]::Pattern) 'Selection'
        Assert $sel.Current.CanSelectMultiple 'the Projects table does not take many selections'
        $rows = @($grid.FindAll($Children, [System.Windows.Automation.Condition]::TrueCondition) | Where-Object { $_.Current.ControlType -eq $CT.ListItem })
        Assert ($rows.Count -ge 2) "the table has $($rows.Count) rows, want two"
        $a = Require-Pattern $rows[0] ([System.Windows.Automation.SelectionItemPattern]::Pattern) 'SelectionItem'
        $b = Require-Pattern $rows[1] ([System.Windows.Automation.SelectionItemPattern]::Pattern) 'SelectionItem'
        $a.Select()
        Wait-For { if (@($sel.Current.GetSelection()).Count -eq 1) { $true } else { $null } } 'one row selected' | Out-Null
        $b.AddToSelection()
        Wait-For { if (@($sel.Current.GetSelection()).Count -eq 2) { $true } else { $null } } 'two rows selected' | Out-Null
        $a.RemoveFromSelection()
        $b.RemoveFromSelection()
    }

    Check 'demo: Down and Right move the focused cell of the Projects table, with focus events' {
        $grid = & $gridNamed 'Projects'
        $grid.SetFocus()
        [ForegroundWindow]::SetForegroundWindow([IntPtr]$win.Current.NativeWindowHandle) | Out-Null
        Start-Sleep -Milliseconds 300
        $before = [FocusCounter]::Count
        [System.Windows.Forms.SendKeys]::SendWait('^{HOME}')
        Wait-For { $p = & $cellAtFocus; if ($p -and $p[0] -eq 0 -and $p[1] -eq 0) { $true } else { $null } } 'the first cell to take focus' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{DOWN}')
        Wait-For { $p = & $cellAtFocus; if ($p -and $p[0] -eq 1 -and $p[1] -eq 0) { $true } else { $null } } 'Down to move the focus to row 1' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{RIGHT}')
        Wait-For { $p = & $cellAtFocus; if ($p -and $p[0] -eq 1 -and $p[1] -eq 1) { $true } else { $null } } 'Right to move the focus to column 1' | Out-Null
        Assert ([FocusCounter]::Count -gt $before) 'the focus moves fired no focus events'
    }

    Check 'demo: F2 edits Open tasks in the Projects table, and a value outside 0 to 999 keeps the editor open' {
        $grid = & $gridNamed 'Projects'
        & $focusGrid $grid
        & $sendKeys '^{HOME}'
        1..3 | ForEach-Object { & $sendKeys '{RIGHT}' }
        & $sendKeys '{F2}'
        Wait-For { & $focusedEdit 'Open tasks' } 'the Open tasks editor to take focus' | Out-Null
        & $sendKeys '^a'
        & $sendKeys '42'
        & $sendKeys '{ENTER}'
        Wait-For { Find-Named $win '42' $CT.DataItem | Select-Object -First 1 } 'the saved open tasks' | Out-Null
        & $sendKeys '{F2}'
        Wait-For { & $focusedEdit 'Open tasks' } 'the Open tasks editor to take focus again' | Out-Null
        & $sendKeys '^a'
        & $sendKeys '1000'
        & $sendKeys '{ENTER}'
        Wait-For { $e = & $focusedEdit 'Open tasks'; if ($e -and ((Get-Prop $e 30013) -like '*whole number from 0 to 999*')) { $e } else { $null } } 'the open tasks error in the editor help text' | Out-Null
        Assert ($null -ne (& $focusedEdit 'Open tasks')) 'the editor lost focus after an invalid value'
        & $sendKeys '{ESC}'
        Wait-Gone { & $focusedEdit 'Open tasks' } 'the editor to close on Escape' | Out-Null
    }

    if ($axe) {
        Check 'demo: AxeWindowsCLI scan of the Projects table' {
            $errors = Invoke-AxeScan $demo 'demo-projects-table'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }
    }
    & $selectTab 'Cards'

    Check 'demo: the Components data grid has 100,000 rows, and sorts and selects with UI Automation' {
        Invoke-Named $win 'Components' $CT.Button
        $grid = Wait-For { & $gridNamed 'Deploy runs' } 'the data grid on the Components page'
        $g = Require-Pattern $grid ([System.Windows.Automation.GridPattern]::Pattern) 'Grid'
        Assert ($g.Current.RowCount -eq 100000) "RowCount is $($g.Current.RowCount), want 100,000"
        # The grid is far down the page, and a grid that is not drawn builds only its first row.
        # The grid is scrolled into view through its own ScrollItem, and waited for in the window.
        $gsi = Require-Pattern $grid ([System.Windows.Automation.ScrollItemPattern]::Pattern) 'ScrollItem'
        $gsi.ScrollIntoView()
        Wait-For { if ($grid.Current.BoundingRectangle.Top -lt $win.Current.BoundingRectangle.Bottom -and $grid.Current.BoundingRectangle.Bottom -gt $win.Current.BoundingRectangle.Top) { $true } else { $null } } 'the data grid to be in the window' | Out-Null
        Assert ($g.Current.ColumnCount -eq 8) "ColumnCount is $($g.Current.ColumnCount), want 8"
        $hdr = Wait-For { Find-Named $win 'Duration' $CT.HeaderItem | Select-Object -First 1 } 'the Duration header'
        $hsi = Get-Pattern $hdr ([System.Windows.Automation.ScrollItemPattern]::Pattern)
        if ($hsi) { $hsi.ScrollIntoView() }
        $inv = Require-Pattern $hdr ([System.Windows.Automation.InvokePattern]::Pattern) 'Invoke'
        $inv.Invoke()
        $sorted = Wait-For { Find-Named $win 'Duration, sorted ascending' $CT.HeaderItem | Select-Object -First 1 } 'the Duration header to be sorted ascending'
        Assert ((Get-Prop $sorted 30026) -eq 'sorted ascending') 'ItemStatus of the sorted header is not "sorted ascending"'
        $sel = Require-Pattern $grid ([System.Windows.Automation.SelectionPattern]::Pattern) 'Selection'
        Assert $sel.Current.CanSelectMultiple 'the data grid does not take many selections'
        Wait-For { if (@($grid.FindAll($Children, [System.Windows.Automation.Condition]::TrueCondition) | Where-Object { $_.Current.ControlType -eq $CT.ListItem }).Count -ge 2) { $true } else { $null } } 'two rows of the data grid in view' | Out-Null
        $rows = @($grid.FindAll($Children, [System.Windows.Automation.Condition]::TrueCondition) | Where-Object { $_.Current.ControlType -eq $CT.ListItem })
        Assert ($rows.Count -ge 2) "the data grid has $($rows.Count) rows in view, want two"
        $a = Require-Pattern $rows[0] ([System.Windows.Automation.SelectionItemPattern]::Pattern) 'SelectionItem'
        $b = Require-Pattern $rows[1] ([System.Windows.Automation.SelectionItemPattern]::Pattern) 'SelectionItem'
        $a.Select()
        $b.AddToSelection()
        Wait-For { if (@($sel.Current.GetSelection()).Count -eq 2) { $true } else { $null } } 'two runs selected' | Out-Null
        $a.RemoveFromSelection()
        $b.RemoveFromSelection()
    }

    # Editing and the Actions column, from the keyboard and through UI Automation (the
    # helpers are defined with the grid helpers above). Each step waits for the one before.
    # RUN-000004 is the first failed run, which is the fourth row by ID. From the first
    # cell, seven Rights reach the Actions column and three Downs reach the row.
    $retry = 'Retry RUN-000004'
    Check 'demo: Enter on a failed run moves focus to its Retry button' {
        # The sort check above left the runs sorted by Duration. Sorting by Run puts the
        # runs back in ID order, so that RUN-000004 is the fourth row again.
        $runHdr = Wait-For { Find-Named $win 'Run' $CT.HeaderItem | Select-Object -First 1 } 'the Run header'
        (Require-Pattern $runHdr ([System.Windows.Automation.InvokePattern]::Pattern) 'Invoke').Invoke()
        Wait-For { Find-Named $win 'Run, sorted ascending' $CT.HeaderItem | Select-Object -First 1 } 'the runs sorted by Run' | Out-Null
        $grid = & $gridNamed 'Deploy runs'
        & $focusGrid $grid
        & $sendKeys '^{HOME}'
        1..7 | ForEach-Object { & $sendKeys '{RIGHT}' }
        1..3 | ForEach-Object { & $sendKeys '{DOWN}' }
        & $sendKeys '{ENTER}'
        Wait-For {
            $f = [System.Windows.Automation.AutomationElement]::FocusedElement
            if ($f -and $f.Current.ControlType -eq $CT.Button -and $f.Current.Name -eq $retry) { $true } else { $null }
        } "focus on $retry" | Out-Null
    }
    if ($axe) {
        Check 'demo: AxeWindowsCLI scan of the Components grid with the Retry button focused' {
            $errors = Invoke-AxeScan $demo 'demo-cell-focus'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }
    }
    Check 'demo: Enter on the focused Retry queues the run, and says so' {
        & $sendKeys '{ENTER}'
        Wait-For { Find-Named $win 'Run RUN-000004 is queued again' | Select-Object -First 1 } 'the retry toast' | Out-Null
    }
    Check 'demo: F2 edits a label from the keyboard, and Enter saves it' {
        $grid = & $gridNamed 'Deploy runs'
        & $focusGrid $grid
        & $sendKeys '^{HOME}'
        1..6 | ForEach-Object { & $sendKeys '{RIGHT}' }
        & $sendKeys '{F2}'
        $ed = Wait-For { & $focusedEdit 'Label' } 'the Label editor to take focus'
        & $sendKeys '^a'
        & $sendKeys 'Nightly'
        if ($axe) {
            $errors = Invoke-AxeScan $demo 'demo-editor-open'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI with the editor open"
        }
        & $sendKeys '{ENTER}'
        Wait-For { Find-Named $win 'Nightly' $CT.DataItem | Select-Object -First 1 } 'the saved label' | Out-Null
        Assert ($null -eq (& $focusedEdit 'Label')) 'the editor still has focus after Enter saved the label'
        Assert ($null -ne $ed) 'the editor was not found'
    }
    Check 'demo: a label edited through the UI Automation Value pattern of its editor' {
        $grid = & $gridNamed 'Deploy runs'
        & $focusGrid $grid
        & $sendKeys '^{HOME}'
        1..6 | ForEach-Object { & $sendKeys '{RIGHT}' }
        & $sendKeys '{F2}'
        $ed = Wait-For { & $focusedEdit 'Label' } 'the Label editor to take focus'
        $vp = Require-Pattern $ed ([System.Windows.Automation.ValuePattern]::Pattern) 'Value'
        $vp.SetValue('Via UIA')
        # SetValue is applied on the UI thread, so the editor is waited for before Enter.
        Wait-For { if ($ed.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern).Current.Value -eq 'Via UIA') { $true } else { $null } } 'the editor to take the value set through UI Automation' | Out-Null
        & $sendKeys '{ENTER}'
        Wait-For { Find-Named $win 'Via UIA' $CT.DataItem | Select-Object -First 1 } 'the label set through the Value pattern' | Out-Null
    }
    Check 'demo: a label over 40 characters shows its error and keeps the editor open' {
        $grid = & $gridNamed 'Deploy runs'
        & $focusGrid $grid
        & $sendKeys '^{HOME}'
        1..6 | ForEach-Object { & $sendKeys '{RIGHT}' }
        & $sendKeys '{F2}'
        Wait-For { & $focusedEdit 'Label' } 'the Label editor to take focus' | Out-Null
        & $sendKeys '^a'
        & $sendKeys ('a' * 41)
        & $sendKeys '{ENTER}'
        # The error is the editor's help text, which the editor reads after its name.
        Wait-For { $e = & $focusedEdit 'Label'; if ($e -and ((Get-Prop $e 30013) -like '*at most 40 characters*')) { $e } else { $null } } 'the label error in the editor help text' | Out-Null
        Assert ($null -ne (& $focusedEdit 'Label')) 'the editor lost focus after an invalid label'
        if ($axe) {
            $errors = Invoke-AxeScan $demo 'demo-editor-error'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI with the error shown"
        }
        & $sendKeys '{ESC}'
        Wait-Gone { & $focusedEdit 'Label' } 'the Label editor to close on Escape' | Out-Null
    }

    Check 'demo: a label is set through its Value pattern without entering an edit' {
        $grid = & $gridNamed 'Deploy runs'
        & $focusGrid $grid
        & $sendKeys '^{HOME}'
        $cell = Wait-For { $c = & $cellAt $grid 0 6; if ($c) { $c } else { $null } } 'the label cell of the first run'
        $vp = Require-Pattern $cell ([System.Windows.Automation.ValuePattern]::Pattern) 'Value'
        Assert (-not $vp.Current.IsReadOnly) 'the editable label cell reports itself read-only'
        $vp.SetValue('Set by UIA')
        Wait-For { if (Find-Named $win 'Set by UIA' $CT.DataItem | Select-Object -First 1) { $true } else { $null } } 'the label set through its Value pattern' | Out-Null
        $f = [System.Windows.Automation.AutomationElement]::FocusedElement
        Assert ($f.Current.ControlType -ne $CT.Edit) 'SetValue moved focus into an editor'
    }
    Check 'demo: an invalid value set through the Value pattern is refused, and the label keeps its value' {
        $grid = & $gridNamed 'Deploy runs'
        $cell = & $cellAt $grid 0 6
        Assert ($null -ne $cell) 'the label cell of the first run is not built'
        $vp = Require-Pattern $cell ([System.Windows.Automation.ValuePattern]::Pattern) 'Value'
        $vp.SetValue(('b' * 41))
        Start-Sleep -Milliseconds 800
        Assert ($null -eq (Find-Named $win ('b' * 41) $CT.DataItem | Select-Object -First 1)) 'the refused value was saved'
        Assert ($null -ne (Find-Named $win 'Set by UIA' $CT.DataItem | Select-Object -First 1)) 'the label changed on a refused value'
    }
    Check 'demo: a read-only name cell has no Value pattern' {
        $grid = & $gridNamed 'Deploy runs'
        $cell = & $cellAt $grid 0 0
        Assert ($null -ne $cell) 'the name cell of the first run is not built'
        Assert ($null -eq (Get-Pattern $cell ([System.Windows.Automation.ValuePattern]::Pattern))) 'a read-only cell offers the Value pattern'
    }

    if ($axe) {
        Check 'demo: AxeWindowsCLI scan of the Components data grid' {
            $errors = Invoke-AxeScan $demo 'demo-data-grid'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }
    }

    # ---- Demo: feedback (progress, busy rings, toasts, badges) ---------------
    # The Feedback section is the last section of the Components page. Each
    # check scrolls the Info button into view, which is in the section.
    [NotificationCounter]::Start($win)
    Invoke-Named $win 'Components' $CT.Button
    Wait-For { $null -ne (Find-Named $win 'Info toast' $CT.Button | Select-Object -First 1) } 'the Feedback section' | Out-Null
    $infoItem = Require-Pattern (Find-One $win 'Info toast' $CT.Button) ([System.Windows.Automation.ScrollItemPatternIdentifiers]::Pattern) 'ScrollItem'
    $infoItem.ScrollIntoView()
    $focusIs = {
        param([string]$Name, $Type)
        $f = [System.Windows.Automation.AutomationElement]::FocusedElement
        $null -ne $f -and $f.Current.Name -eq $Name -and $f.Current.ControlType -eq $Type
    }

    Check 'demo: a determinate progress bar is a ProgressBar with a read-only RangeValue' {
        $bar = Find-One $win 'Determinate progress' $CT.ProgressBar
        $rv = Require-Pattern $bar ([System.Windows.Automation.RangeValuePatternIdentifiers]::Pattern) 'RangeValue'
        Assert ([Math]::Abs($rv.Current.Value - 0.6) -lt 0.001) "the value is $($rv.Current.Value), want 0.6"
        Assert ($rv.Current.Minimum -eq 0 -and $rv.Current.Maximum -eq 1) "the range is $($rv.Current.Minimum) to $($rv.Current.Maximum)"
        Assert $rv.Current.IsReadOnly 'the progress bar is not read-only'
    }

    Check 'demo: an indeterminate bar and a busy ring are ProgressBars with no RangeValue' {
        foreach ($n in 'Searching', 'Small busy indicator') {
            $el = Find-One $win $n $CT.ProgressBar
            Assert ($null -eq (Get-Pattern $el ([System.Windows.Automation.RangeValuePatternIdentifiers]::Pattern))) "'$n' has a RangeValue; it must not"
        }
    }

    Check 'demo: a toast is a group named by its text, and its announcement raises a Notification event' {
        $before = [NotificationCounter]::Count
        Invoke-Named $win 'Info toast' $CT.Button
        Wait-For { $null -ne (Find-Named $win 'Sync started' $CT.Group | Select-Object -First 1) } 'the toast' | Out-Null
        Wait-For { if ([NotificationCounter]::Count -gt $before) { $true } else { $null } } 'a Notification event' | Out-Null
        Find-One $win 'Notifications' $CT.Group | Out-Null
    }

    Check 'demo: a toast action runs from its button, and the toast leaves' {
        Invoke-Named $win 'Increment' $CT.Button
        Wait-For { $null -ne (Find-Named $win 'Count is 1' $CT.Group | Select-Object -First 1) } 'the Count toast' | Out-Null
        Invoke-Named $win 'Undo' $CT.Button
        Wait-Gone { Find-Named $win 'Count is 1' $CT.Group | Select-Object -First 1 } 'the Count toast to leave' | Out-Null
        Find-One $win 'Toast with Undo: count 0' | Out-Null
    }

    Check 'demo: F6 moves the focus into the notifications and back, and Escape on a toast dismisses it' {
        # A sticky error toast stays up, so that the Escape check has one to close.
        Invoke-Named $win 'Error toast' $CT.Button
        Wait-For { $null -ne (Find-Named $win 'Upload failed. It stays until you dismiss it.' $CT.Group | Select-Object -First 1) } 'the error toast' | Out-Null
        $inc = Find-One $win 'Increment' $CT.Button
        $inc.SetFocus()
        [ForegroundWindow]::SetForegroundWindow([IntPtr]$win.Current.NativeWindowHandle) | Out-Null
        Start-Sleep -Milliseconds 300
        [System.Windows.Forms.SendKeys]::SendWait('{F6}')
        Wait-For { & $focusIs 'Notifications' $CT.Group } 'F6 to move the focus to the notifications' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{F6}')
        Wait-For { & $focusIs 'Increment' $CT.Button } 'F6 again to return the focus' | Out-Null

        [System.Windows.Forms.SendKeys]::SendWait('{F6}')
        Wait-For { & $focusIs 'Notifications' $CT.Group } 'F6 to enter the notifications again' | Out-Null
        [System.Windows.Forms.SendKeys]::SendWait('{TAB}')
        Wait-For { & $focusIs 'Dismiss notification' $CT.Button } 'Tab to reach the first toast' | Out-Null
        $before = @(Find-Named $win 'Dismiss notification' $CT.Button).Count
        [System.Windows.Forms.SendKeys]::SendWait('{ESC}')
        Wait-For { (@(Find-Named $win 'Dismiss notification' $CT.Button).Count) -eq ($before - 1) } 'Escape to dismiss the toast' | Out-Null
    }

    if ($axe) {
        Check 'demo: AxeWindowsCLI scan of the Feedback section with a toast visible' {
            Invoke-Named $win 'Warning toast' $CT.Button
            Wait-For { $null -ne (Find-Named $win 'Disk nearly full' $CT.Group | Select-Object -First 1) } 'the warning toast' | Out-Null
            $errors = Invoke-AxeScan $demo 'demo-feedback'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }

        Check 'demo: AxeWindowsCLI scan of the notifications list open from the bell' {
            # Scan a settled screen: close every toast first, so none is leaving
            # (an auto-dismiss) while the scanner reads the tree. A scanner that
            # reads an element before and after its removal reports it as an
            # on-screen element without bounds.
            foreach ($close in @(Find-Named $win 'Dismiss notification' $CT.Button)) {
                try { (Require-Pattern $close ([System.Windows.Automation.InvokePattern]::Pattern) 'Invoke').Invoke() } catch { }
            }
            Wait-For { @(Find-Named $win 'Dismiss notification' $CT.Button).Count -eq 0 } 'every toast to close' | Out-Null
            $bellCond = New-Object System.Windows.Automation.PropertyCondition -ArgumentList ([System.Windows.Automation.AutomationElement]::ControlTypeProperty), $CT.Button
            $bell = @($win.FindAll($Descendants, $bellCond) | Where-Object { $_.Current.Name -match '^Notifications(, \d+ unread)?$' })[0]
            (Require-Pattern $bell ([System.Windows.Automation.InvokePattern]::Pattern) 'Invoke').Invoke()
            Wait-For { $null -ne (Find-Named $win 'Mark all read' $CT.Button | Select-Object -First 1) } 'the notifications list' | Out-Null
            try {
                $errors = Invoke-AxeScan $demo 'demo-notifications'
            } finally {
                (Require-Pattern $bell ([System.Windows.Automation.InvokePattern]::Pattern) 'Invoke').Invoke()
            }
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }

        Check 'demo: AxeWindowsCLI scan of the Projects page, with its progress bars and badges' {
            Invoke-Named $win 'Projects, 6 items' $CT.Button
            Wait-For { $null -ne (Find-Named $win 'Project Alpha progress' $CT.ProgressBar | Select-Object -First 1) } 'the Projects page' | Out-Null
            $errors = Invoke-AxeScan $demo 'demo-projects'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }

        Check 'demo: AxeWindowsCLI scan of the Dashboard page' {
            Invoke-Named $win 'Dashboard' $CT.Button
            Wait-For { $null -ne (Find-Named $win '+18%' | Select-Object -First 1) } 'the Dashboard page' | Out-Null
            $errors = Invoke-AxeScan $demo 'demo-dashboard'
            Assert ($errors -eq 0) "$errors errors found by AxeWindowsCLI"
            "$errors errors"
        }
    }

    Check 'demo: the bell names its unread count, and its badge is not a separate element' {
        $bellCond = New-Object System.Windows.Automation.PropertyCondition -ArgumentList ([System.Windows.Automation.AutomationElement]::ControlTypeProperty), $CT.Button
        $names = @($win.FindAll($Descendants, $bellCond) | ForEach-Object { $_.Current.Name } | Where-Object { $_ -match '^Notifications(, \d+ unread)?$' })
        Assert ($names.Count -eq 1) "found $($names.Count) bells named Notifications"
        Assert ($names[0] -match ', \d+ unread$') "the bell is named '$($names[0])'; it must name the unread count"
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
    # The work folder holds the AxeWindowsCLI results. It is kept when a check fails, so
    # that the findings can be read; otherwise it is removed.
    if ($Failed -eq 0 -and (Test-Path $Work)) { Remove-Item -Recurse -Force $Work -ErrorAction SilentlyContinue }
}

Write-Output ''
if ($Failed -gt 0 -and (Test-Path $Work)) { Write-Output "Results kept in $Work" }
Write-Output "Summary: $Passed passed, $Failed failed"
if ($Failed -gt 0) { exit 1 }
exit 0
