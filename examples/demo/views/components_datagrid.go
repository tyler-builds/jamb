package views

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/ui"
)

// maxRunLabel is the longest label a run can be given in the grid.
const maxRunLabel = 40

// runRow is one deploy run of the data grid sample. The runs are generated, not
// recorded: they show the grid's behavior, and name no person. Label is a note that
// the user types in the grid.
type runRow struct {
	ID       string
	Service  string
	Status   string
	Duration time.Duration
	Started  time.Time
	Cost     float32 // in dollars
	Label    string
}

var (
	runsOnce sync.Once
	runsData []runRow
)

// deployRuns returns the 100,000 sample runs. They are made once, from the index
// of each run, so that the same runs appear on every start.
func deployRuns() []runRow {
	runsOnce.Do(func() {
		statuses := []string{"Succeeded", "Succeeded", "Succeeded", "Failed", "Running", "Queued", "Cancelled"}
		services := []string{"api", "billing", "search", "worker", "web", "ingest", "auth", "reports"}
		start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
		runsData = make([]runRow, 100000)
		for i := range runsData {
			h := uint32(i+1) * 2654435761 // a multiplicative hash spreads the fields
			runsData[i] = runRow{
				ID:       runID(i + 1),
				Service:  services[h%uint32(len(services))],
				Status:   statuses[(h>>8)%uint32(len(statuses))],
				Duration: time.Duration(20+(h>>12)%900) * time.Second,
				Started:  start.Add(time.Duration(i) * 9 * time.Minute),
				Cost:     float32((h>>16)%40000) / 100,
			}
		}
	})
	return runsData
}

// runID is "RUN-" and n in six digits, as fmt.Sprintf("RUN-%06d", n) writes it,
// without fmt: the 100,000 runs are made on the first visit of the page.
func runID(n int) string {
	b := []byte("RUN-000000")
	for i := len(b) - 1; i >= 4 && n > 0; i-- {
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b)
}

// PrewarmRuns makes the runs on a goroutine of its own, so that the first visit of
// the Components page does not wait for them. deployRuns is safe to call from
// another goroutine (it is made once, and the callers wait for it).
func PrewarmRuns() { go deployRuns() }

// runColumns are the columns of the Components page's data grid. The runs are the
// signal runs: the Actions column retries a failed run, and the Label column edits
// the label of a run, and both change the runs.
func runColumns(ctx *ui.Context, runs *ui.Signal[[]runRow]) []ui.DataColumn[runRow] {
	return []ui.DataColumn[runRow]{
		{Header: "Run", Text: func(r runRow) string { return r.ID },
			Compare: ui.SortBy(func(r runRow) string { return r.ID }), Width: 130, Resizable: true},
		{Header: "Service", Text: func(r runRow) string { return r.Service },
			Compare: ui.SortByText(func(r runRow) string { return r.Service }), Grow: 1, Resizable: true},
		{Header: "Status", Text: func(r runRow) string { return r.Status },
			Compare: ui.SortBy(func(r runRow) string { return r.Status }),
			Cell:    func(r runRow) ui.Element { return ui.BadgeText(r.Status).Variant(runBadge(r.Status)) },
			// Choosing a status saves it: the Select's change is the commit.
			Edit: func(r runRow, v string) error {
				updateRun(runs, r.ID, func(x *runRow) { x.Status = v })
				return nil
			},
			Editor: func(r runRow, value string, change func(string), commit, cancel func()) ui.Element {
				return ui.Select(ui.NewSignal(value), runStatusOptions...).Label("Status").Width(ui.Px(118)).
					OnChange(func(s string) { change(s); commit() })
			},
			Width: 130, Resizable: true},
		{Header: "Duration", Text: func(r runRow) string { return durationText(r.Duration) },
			Compare: ui.SortBy(func(r runRow) time.Duration { return r.Duration }),
			Align:   ui.End, Width: 110, Resizable: true},
		{Header: "Started", Text: func(r runRow) string { return r.Started.Format("2006-01-02 15:04") },
			Compare: ui.SortBy(func(r runRow) int64 { return r.Started.UnixNano() }), Width: 170, Resizable: true},
		{Header: "Cost", Text: func(r runRow) string { return fmt.Sprintf("$%.2f", r.Cost) },
			Compare: ui.SortBy(func(r runRow) float32 { return r.Cost }),
			Align:   ui.End, Width: 100, Resizable: true},
		{Header: "Label", Text: func(r runRow) string { return r.Label },
			Compare: ui.SortByText(func(r runRow) string { return r.Label }), Width: 200, Resizable: true,
			Edit: func(r runRow, v string) error {
				if n := utf8.RuneCountInString(v); n > maxRunLabel {
					return fmt.Errorf("A label is at most %d characters; this one has %d", maxRunLabel, n)
				}
				updateRun(runs, r.ID, func(x *runRow) { x.Label = v })
				return nil
			}},
		{Header: "Actions", Interactive: true, Width: 110, Resizable: true,
			Cell: func(r runRow) ui.Element {
				if r.Status != "Failed" {
					return ui.Div()
				}
				return ui.Button("Retry").Small().Label("Retry " + r.ID).OnClick(func() {
					updateRun(runs, r.ID, func(x *runRow) { x.Status, x.Duration = "Queued", 0 })
					ctx.Toast(ui.ToastSuccess, fmt.Sprintf("Run %s is queued again", r.ID))
				})
			}},
	}
}

// runStatusOptions are the statuses a run can be given in the grid.
var runStatusOptions = []ui.Option[string]{
	{Value: "Queued", Label: "Queued"},
	{Value: "Running", Label: "Running"},
	{Value: "Succeeded", Label: "Succeeded"},
	{Value: "Failed", Label: "Failed"},
	{Value: "Cancelled", Label: "Cancelled"},
}

// updateRun sets the run with the ID id to what edit sets. The runs are copied, so
// the grid gets a new slice and sorts them again.
func updateRun(runs *ui.Signal[[]runRow], id string, edit func(*runRow)) {
	runs.Update(func(rs []runRow) []runRow {
		out := slices.Clone(rs)
		for i := range out {
			if out[i].ID == id {
				edit(&out[i])
			}
		}
		return out
	})
}

// runBadge is the badge variant of a run's status.
func runBadge(status string) ui.BadgeVariant {
	switch status {
	case "Succeeded":
		return ui.BadgeSuccess
	case "Failed":
		return ui.BadgeDanger
	case "Running":
		return ui.BadgeAccent
	case "Cancelled":
		return ui.BadgeWarning
	}
	return ui.BadgeNeutral
}

// durationText is a duration as minutes and seconds, such as "4m 12s".
func durationText(d time.Duration) string {
	m := int(d / time.Minute)
	s := int(d/time.Second) % 60
	return fmt.Sprintf("%dm %02ds", m, s)
}

// dataGridSection is the Components page's data grid: 100,000 deploy runs that
// the user sorts, selects and resizes, with the density, the striping and the
// empty state switched by the controls above it.
func dataGridSection(ctx *ui.Context, t *ui.Theme) ui.Element {
	density := ui.Local(ctx, "runs-density", 0) // 0 Comfortable, 1 Compact
	striped := ui.Local(ctx, "runs-striped", true)
	none := ui.Local(ctx, "runs-none", false)
	picked := ui.Local(ctx, "runs-picked", []string(nil))
	order := ui.Local(ctx, "runs-order", ui.SortState{})

	runs := ui.Local(ctx, "runs-data", deployRuns())
	rows := runs.Get()
	if none.Get() {
		rows = nil
	}
	d := ui.DataGridComfortable
	if density.Get() == 1 {
		d = ui.DataGridCompact
	}
	grid := ui.DataGrid(rows, func(r runRow) string { return r.ID }, runColumns(ctx, runs)...).
		Label("Deploy runs").Density(d).Selection(ui.SelectionMultiple, picked).Sort(order).
		Empty(ui.Text("No deploy runs match")).Style(ui.Style{Height: ui.Px(420)})
	if striped.Get() {
		grid.Striped()
	}
	return Card(t).Gap(14).
		Child(SectionTitle(t, "Data grid")).
		Child(ui.Text("100,000 deploy runs. Click a header to sort, drag its edge to resize, and select rows with Ctrl or Shift. " +
			"Enter opens the focused row. In the Actions column, Enter or F2 reaches the Retry of a failed run. " +
			"In the Label column, Enter, F2 or typing edits the label, which is at most 40 characters. " +
			"In the Status column, Enter or F2 opens a choice of status.").FontSize(13).TextColor(t.TextMuted)).
		Child(ui.Div().Row().Wrap().Align(ui.Center).Gap(18).
			Child(ui.Tabs(density,
				ui.Tab[int]{Value: 0, Label: "Comfortable"},
				ui.Tab[int]{Value: 1, Label: "Compact"},
			).Key("runs-density").Label("Row density").Variant(ui.TabsSegmented)).
			Child(ui.Checkbox("Striped", striped).Key("runs-striped")).
			Child(ui.Checkbox("No rows", none).Key("runs-none")).
			Child(ui.Text(fmt.Sprintf("%d selected", len(picked.Get()))).FontSize(13).TextColor(t.TextMuted))).
		Child(grid).
		Child(shipmentsPart(ctx, t))
}

// shipRow is one shipment of the columns sample. Like the deploy runs, the shipments
// are generated from their index, so they are the same on every start, and they
// name no real company.
type shipRow struct {
	Index    int
	ID       string
	Customer string
	Region   string
	Carrier  string
	Status   string
	Weight   float32 // in kilograms
}

// The values the shipments are drawn from.
var (
	shipCustomers = []string{"Harbor Supply", "Pine Works", "Blue Mesa Ltd", "Riverside Foods", "Northgate Parts"}
	shipRegions   = []string{"North", "South", "East", "West"}
	shipCarriers  = []string{"Ground", "Air", "Freight"}
	shipStatuses  = []string{"Packed", "In transit", "Delivered", "Delayed"}
)

// shipments returns n shipments, from the one at index first.
func shipments(first, n int) []shipRow {
	out := make([]shipRow, n)
	for k := range out {
		i := first + k
		h := uint32(i+1) * 2246822519
		out[k] = shipRow{
			Index:    i,
			ID:       fmt.Sprintf("SHP-%06d", i+1),
			Customer: shipCustomers[h%uint32(len(shipCustomers))],
			Region:   shipRegions[(h>>8)%uint32(len(shipRegions))],
			Carrier:  shipCarriers[(h>>12)%uint32(len(shipCarriers))],
			Status:   shipStatuses[(h>>16)%uint32(len(shipStatuses))],
			Weight:   float32((h>>4)%5000) / 10,
		}
	}
	return out
}

// shipUnits is the units a shipment sent in week w (from 0) of the year.
func shipUnits(r shipRow, w int) int {
	return int(((uint32(r.Index+1)*2654435761 + uint32(w+1)*40503) >> 20) % 500)
}

// shipColumns are the columns of the shipments grid. The order is pinned at the start
// and the actions at the end, the week columns share a group header, and every column
// can be moved. Track opens nothing: it reports the shipment in a toast.
func shipColumns(ctx *ui.Context) []ui.DataColumn[shipRow] {
	cols := []ui.DataColumn[shipRow]{
		{Header: "Order", Pin: ui.PinStart, Width: 130, Resizable: true,
			Text:    func(r shipRow) string { return r.ID },
			Compare: ui.SortBy(func(r shipRow) string { return r.ID })},
		{Header: "Customer", Group: "Customer", Width: 150, Resizable: true,
			Text:    func(r shipRow) string { return r.Customer },
			Compare: ui.SortByText(func(r shipRow) string { return r.Customer })},
		{Header: "Region", Group: "Customer", Width: 100, Resizable: true,
			Text:    func(r shipRow) string { return r.Region },
			Compare: ui.SortByText(func(r shipRow) string { return r.Region })},
		{Header: "Carrier", Group: "Shipment", Width: 110, Resizable: true,
			Text:    func(r shipRow) string { return r.Carrier },
			Compare: ui.SortByText(func(r shipRow) string { return r.Carrier })},
		{Header: "Status", Group: "Shipment", Width: 120, Resizable: true,
			Text:    func(r shipRow) string { return r.Status },
			Compare: ui.SortByText(func(r shipRow) string { return r.Status })},
		{Header: "Weight", Group: "Shipment", Width: 110, Resizable: true, Align: ui.End,
			Text:    func(r shipRow) string { return fmt.Sprintf("%.1f kg", r.Weight) },
			Compare: ui.SortBy(func(r shipRow) float32 { return r.Weight })},
	}
	for w := range 24 {
		cols = append(cols, ui.DataColumn[shipRow]{Header: fmt.Sprintf("Week %02d", w+1), Group: "Units shipped",
			Width: 84, Resizable: true, Align: ui.End,
			Text:    func(r shipRow) string { return strconv.Itoa(shipUnits(r, w)) },
			Compare: ui.SortBy(func(r shipRow) int { return shipUnits(r, w) })})
	}
	return append(cols, ui.DataColumn[shipRow]{Header: "Actions", Pin: ui.PinEnd, Interactive: true, Width: 110, Resizable: true,
		Cell: func(r shipRow) ui.Element {
			return ui.Button("Track").Small().Label("Track " + r.ID).OnClick(func() {
				ctx.Toast(ui.ToastInfo, "Tracking "+r.ID+" is not connected in this demo")
			})
		}})
}

// setShipStatus sets the status of the shipment with the ID id. Like updateRun, it
// copies the rows, so the grid sorts them again.
func setShipStatus(rows *ui.Signal[[]shipRow], id, status string) {
	rows.Update(func(rs []shipRow) []shipRow {
		out := slices.Clone(rs)
		for i := range out {
			if out[i].ID == id {
				out[i].Status = status
			}
		}
		return out
	})
}

// layoutText says what the saved layout holds: the order and the widths the user has
// set, as OnColumnsChange reports them.
func layoutText(st ui.ColumnState) string {
	if st.Order == nil {
		return "No column change yet. Move a header, drag an edge, or use the header menu; each change is saved here."
	}
	return fmt.Sprintf("Saved layout: order starts %s; %d widths set.", strings.Join(st.Order[:min(3, len(st.Order))], ", "), len(st.Widths))
}

// shipmentsPart is the second data grid of the section: 240 shipments that load 60 at
// a time, with a movable, pinned and grouped column layout that the app saves, and
// buttons that scroll and focus the grid from code. A grid of six shipments under it
// selects cells.
func shipmentsPart(ctx *ui.Context, t *ui.Theme) ui.Element {
	rows := ui.Local(ctx, "ship.rows", shipments(0, 60))
	order := ui.Local(ctx, "ship.order", []string(nil))
	widths := ui.Local(ctx, "ship.widths", map[string]float32(nil))
	saved := ui.Local(ctx, "ship.saved", ui.ColumnState{})
	loading := ui.Local(ctx, "ship.loading", false)
	handle := ui.Local(ctx, "ship.handle", ui.NewDataGridHandle()).Peek()
	cells := ui.Local(ctx, "ship.cells", []ui.CellKey(nil))

	reset := func() {
		order.Set(nil)
		widths.Set(nil)
		saved.Set(ui.ColumnState{})
	}
	grid := ui.DataGrid(rows.Get(), func(r shipRow) string { return r.ID }, shipColumns(ctx)...).
		Label("Shipments").Movable().ColumnOrder(order).ColumnWidths(widths).
		OnColumnsChange(func(st ui.ColumnState) { saved.Set(st) }).
		ContextMenu(func(r shipRow, column string) []ui.MenuItem {
			return []ui.MenuItem{
				{Label: "Mark " + r.ID + " as delivered", OnClick: func() { setShipStatus(rows, r.ID, "Delivered") }},
				{Label: "Mark " + r.ID + " as delayed", OnClick: func() { setShipStatus(rows, r.ID, "Delayed") }},
				ui.MenuSeparator(),
				{Label: "Column: " + column, Disabled: true},
			}
		}).
		HeaderContextMenu(func(column string) []ui.MenuItem {
			return []ui.MenuItem{
				{Label: "Reset column layout", OnClick: reset},
				{Label: "Say the column name", OnClick: func() { ctx.Toast(ui.ToastInfo, column) }},
			}
		}).
		LoadMore(len(rows.Get()) < 240, func() {
			loading.Set(true)
			ctx.After(900*time.Millisecond, func() {
				loading.Set(false)
				rows.Update(func(rs []shipRow) []shipRow { return append(slices.Clone(rs), shipments(len(rs), 60)...) })
			})
		}).
		LoadingMore(loading.Get()).Handle(handle).
		Empty(ui.Text("No shipments")).Style(ui.Style{Height: ui.Px(320)})
	sixRows := shipments(0, 6)
	small := ui.DataGrid(sixRows, func(r shipRow) string { return r.ID },
		ui.DataColumn[shipRow]{Header: "Order", Width: 130, Text: func(r shipRow) string { return r.ID }},
		ui.DataColumn[shipRow]{Header: "Customer", Width: 170, Text: func(r shipRow) string { return r.Customer }},
		ui.DataColumn[shipRow]{Header: "Carrier", Width: 120, Text: func(r shipRow) string { return r.Carrier }},
		ui.DataColumn[shipRow]{Header: "Weight", Width: 120, Align: ui.End, Text: func(r shipRow) string { return fmt.Sprintf("%.1f kg", r.Weight) }},
	).Label("Cell selection").CellSelection(ui.SelectionMultiple, cells).Style(ui.Style{Height: ui.Px(190)})

	return ui.Div().Column().Gap(12).
		Child(ui.Divider()).
		Child(SectionTitle(t, "Data grid: columns, groups and rows")).
		Child(ui.Text("240 shipments, 60 loaded at a time: scroll to the end to load more. Drag a header to move a column, " +
			"with Ctrl+Shift+Left and Right on a header to move it from the keyboard. Drag an edge to resize; " +
			"Order is pinned at the start and Actions at the end. Right-click a row or a header for a menu. " +
			"Week columns sit under one group header and scroll sideways.").FontSize(13).TextColor(t.TextMuted)).
		Child(ui.Text(layoutText(saved.Get())).FontSize(13).TextColor(t.TextMuted)).
		Child(ui.Div().Row().Wrap().Align(ui.Center).Gap(10).
			Child(ui.Button("Scroll to row 45").Secondary().Small().OnClick(func() { handle.ScrollToRow("SHP-000045") })).
			Child(ui.Button("Focus Status of row 12").Secondary().Small().OnClick(func() { handle.FocusCell("SHP-000012", "Status") })).
			Child(ui.Button("Scroll to Week 20").Secondary().Small().OnClick(func() { handle.ScrollToColumn("Week 20") })).
			Child(ui.Button("Reset columns").Secondary().Small().OnClick(reset))).
		Child(grid).
		Child(ui.Text("Cell selection: click a cell, Shift+click for a rectangle, Ctrl+Space for a column, Shift+Space for a row, Ctrl+A for all.").
			FontSize(13).TextColor(t.TextMuted)).
		Child(small).
		Child(ui.Text(fmt.Sprintf("%d cells selected", len(cells.Get()))).FontSize(13).TextColor(t.TextMuted))
}
