package views

import (
	"slices"
	"strconv"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/monitor/sample"
)

// The keys of the process table's columns.
const (
	colName    = "name"
	colPID     = "pid"
	colCPU     = "cpu"
	colMemory  = "memory"
	colThreads = "threads"
	colHandles = "handles"
	colIO      = "io"
	colStarted = "started"
)

// rows returns the rows of the process table: grouped by program, or one a
// process. A filter lists the matching processes alone, since a group that
// matched only by some of its processes would hide the rest.
func (a *App) rows() []row {
	procs := a.s.Latest.Get().Procs
	q := a.filter.Get()
	if q != "" {
		var match []sample.Process
		for _, p := range procs {
			if matches(q, p) {
				match = append(match, p)
			}
		}
		return flatRows(match)
	}
	if a.layout.Grouped.Get() {
		return groupRows(procs)
	}
	return flatRows(procs)
}

// processPanel is the table of the running processes, grouped by program or
// filtered by the title bar's field. It is sampled again every interval, and the
// focus, the selection, the open groups and the sort stay on the same rows as
// they move.
func (a *App) processPanel() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		hc := ctx.HighContrast()
		q := a.filter.Get()
		rows := a.rows()
		empty := ui.Element(ui.Empty("Sampling the processes").Icon(icons.Activity))
		if q != "" {
			empty = ui.Empty("No process matches “" + q + "”").Description("Try fewer words, or clear the filter.")
		}
		grid := ui.DataGrid(rows, func(r row) string { return r.key },
			ui.DataColumn[row]{Key: colName, Header: "Name", Text: groupName,
				Cell:    func(r row) ui.Element { return a.nameCell(t, r) },
				Compare: ui.SortByText(func(r row) string { return r.Label() }),
				Width:   240, Grow: 1, Resizable: true, Movable: true, Pin: ui.PinStart},
			ui.DataColumn[row]{Key: colPID, Header: "PID", Text: pidText,
				Compare: ui.SortBy(func(r row) uint32 { return r.PID }),
				Width:   80, Align: ui.End, Resizable: true, Movable: true},
			ui.DataColumn[row]{Key: colCPU, Header: "CPU", Text: func(r row) string { return percent(r.CPU) },
				Cell:      func(r row) ui.Element { return heat(t, hc, r.CPU/100, percent(r.CPU)) },
				CellStyle: ui.Style{Clear: ui.ClearPadding},
				Compare:   ui.SortBy(func(r row) float64 { return r.CPU }),
				Width:     80, Align: ui.End, Resizable: true, Movable: true},
			ui.DataColumn[row]{Key: colMemory, Header: "Memory", Text: func(r row) string { return ui.FormatBytes(r.Private) },
				Compare: ui.SortBy(func(r row) uint64 { return r.Private }),
				Width:   100, Align: ui.End, Resizable: true, Movable: true},
			ui.DataColumn[row]{Key: colIO, Header: "I/O", Text: func(r row) string { return rate(r.IORead + r.IOWrite) },
				Compare: ui.SortBy(func(r row) float64 { return r.IORead + r.IOWrite }),
				Width:   100, Align: ui.End, Resizable: true, Movable: true},
			ui.DataColumn[row]{Key: colThreads, Header: "Threads", Text: func(r row) string { return count(int(r.Threads)) },
				Compare: ui.SortBy(func(r row) uint32 { return r.Threads }),
				Width:   80, Align: ui.End, Resizable: true, Movable: true},
			ui.DataColumn[row]{Key: colHandles, Header: "Handles", Text: func(r row) string { return count(int(r.Handles)) },
				Compare: ui.SortBy(func(r row) uint32 { return r.Handles }),
				Width:   90, Align: ui.End, Resizable: true, Movable: true},
			ui.DataColumn[row]{Key: colStarted, Header: "Started", Text: startedText,
				Compare: func(x, y row) int { return x.Started.Compare(y.Started) },
				Width:   150, Resizable: true, Movable: true},
		).Key("processes").Label("Processes").
			Children(func(r row) []row { return r.children }).
			Expanded(a.expanded).
			Selection(ui.SelectionSingle, a.selected).
			Sort(a.layout.Sort).
			ColumnOrder(a.layout.Order).ColumnWidths(a.layout.Widths).
			OnColumnsChange(func(ui.ColumnState) { a.changed() }).
			Density(ui.DataGridCompact).
			Handle(a.grid).
			Empty(empty).
			OnActivate(func(r row) {
				if r.isGroup() {
					a.toggleGroup(r.key)
					return
				}
				a.selectProcess(r.key)
				a.showPanel(panelDetails)
			}).
			ContextMenu(a.processMenu(ctx))
		return ui.Div().Column().Size(ui.Full, ui.Full).KeyContext("Processes").
			Child(a.processToolbar(t, rows)).
			Child(ui.Div().Grow(1).MinHeight(ui.Px(0)).Child(grid))
	})
}

// processToolbar switches the table between groups and a plain list, and says
// how many processes it lists.
func (a *App) processToolbar(t *ui.Theme, rows []row) ui.Element {
	n := 0
	for _, r := range rows {
		n += max(r.count, 1)
	}
	return ui.Div().Row().Align(ui.Center).Gap(10).PaddingX(8).PaddingY(6).
		Child(ui.ToggleGroup(a.layout.Grouped,
			ui.Option[bool]{Value: true, Label: "By program", Icon: icons.Layers},
			ui.Option[bool]{Value: false, Label: "All processes", Icon: icons.List},
		).Key("grouping").Label("Group the processes").OnChange(func(bool) { a.changed() })).
		Child(ui.Spacer()).
		Child(ui.Text(count(n) + " processes").FontSize(12).TextColor(t.TextMuted))
}

// toggleGroup opens or closes the group with key.
func (a *App) toggleGroup(key string) {
	open := a.expanded.Peek()
	if slices.Contains(open, key) {
		a.expanded.Set(slices.DeleteFunc(slices.Clone(open), func(k string) bool { return k == key }))
		return
	}
	a.expanded.Set(append(slices.Clone(open), key))
}

func pidText(r row) string {
	if r.isGroup() {
		return ""
	}
	return strconv.FormatUint(uint64(r.PID), 10)
}

// startedText is when the process started, or the first of a group's did, or
// nothing for the system's processes, which Windows gives no start time.
func startedText(r row) string {
	if r.Started.IsZero() {
		return ""
	}
	return ui.FormatDate(r.Started) + " " + ui.FormatTime(r.Started)
}

// heat is a cell whose fill deepens with level, from 0 to 1, as Task Manager
// shades its busiest processes. Under high contrast the cell is plain text.
func heat(t *ui.Theme, highContrast bool, level float64, text string) ui.Element {
	cell := ui.Div().Row().Size(ui.Full, ui.Full).Align(ui.Center).Justify(ui.End).PaddingX(8).
		Child(ui.Text(text))
	if !highContrast && level > 0.005 {
		cell.Background(t.Accent.Alpha(float32(0.08 + 0.5*min(level*4, 1))))
	}
	return cell
}

// processMenu is the menu of a row of the table.
func (a *App) processMenu(ctx *ui.Context) func(r row, column string) []ui.MenuItem {
	return func(r row, column string) []ui.MenuItem {
		key, name := r.key, r.Label()
		if r.isGroup() {
			label := "Expand"
			if slices.Contains(a.expanded.Peek(), key) {
				label = "Collapse"
			}
			return []ui.MenuItem{
				{Label: label, OnClick: func() { a.toggleGroup(key) }},
				{Label: "Filter to " + name, Icon: icons.Search, OnClick: func() { a.filter.Set(name) }},
				ui.MenuSeparator(),
				{Action: CopyName, Icon: icons.Copy},
			}
		}
		pid := r.PID
		items := []ui.MenuItem{
			{Label: "Show details", Icon: icons.Info, OnClick: func() { a.selectProcess(key); a.showPanel(panelDetails) }},
			{Label: "Filter to " + name, Icon: icons.Search, OnClick: func() { a.filter.Set(name) }},
			ui.MenuSeparator(),
			{Action: CopyName, Icon: icons.Copy},
			{Label: "Copy PID", OnClick: func() { ctx.SetClipboard(strconv.FormatUint(uint64(pid), 10)) }},
		}
		if parent, ok := a.parentOf(r.Process); ok {
			items = append(items, ui.MenuItem{Label: "Go to parent, " + parent.Label(),
				OnClick: func() { a.selectProcess(parent.Key) }})
		}
		return append(items, ui.MenuSeparator(), ui.MenuItem{Action: EndProcess, Icon: icons.CircleX})
	}
}

// parentOf returns the running process that started p. A parent that has exited,
// and a later process that took over its PID, are not p's parent: a parent starts
// before its child.
func (a *App) parentOf(p sample.Process) (sample.Process, bool) {
	if p.PPID == 0 {
		return sample.Process{}, false
	}
	for _, q := range a.s.Latest.Peek().Procs {
		if q.PID == p.PPID && q.Created <= p.Created {
			return q, true
		}
	}
	return sample.Process{}, false
}
