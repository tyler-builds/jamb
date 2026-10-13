package views

import (
	"fmt"
	"slices"
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
				ID:       fmt.Sprintf("RUN-%06d", i+1),
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
		Child(grid)
}
