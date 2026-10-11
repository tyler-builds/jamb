package views

import (
	"cmp"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// runRow is one deploy run of the data grid sample. The runs are generated, not
// recorded: they show the grid's behavior, and name no person.
type runRow struct {
	ID       string
	Service  string
	Status   string
	Duration time.Duration
	Started  time.Time
	Cost     float32 // in dollars
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

// runColumns are the columns of the Components page's data grid.
func runColumns() []ui.DataColumn[runRow] {
	return []ui.DataColumn[runRow]{
		{Header: "Run", Text: func(r runRow) string { return r.ID },
			Compare: func(a, b runRow) int { return strings.Compare(a.ID, b.ID) }, Width: 130, Resizable: true},
		{Header: "Service", Text: func(r runRow) string { return r.Service },
			Compare: func(a, b runRow) int { return strings.Compare(a.Service, b.Service) }, Grow: 1, Resizable: true},
		{Header: "Status", Text: func(r runRow) string { return r.Status },
			Compare: func(a, b runRow) int { return strings.Compare(a.Status, b.Status) },
			Cell:    func(r runRow) ui.Element { return ui.BadgeText(r.Status).Variant(runBadge(r.Status)) },
			Width:   130, Resizable: true},
		{Header: "Duration", Text: func(r runRow) string { return durationText(r.Duration) },
			Compare: func(a, b runRow) int { return cmp.Compare(a.Duration, b.Duration) },
			Align:   ui.End, Width: 110, Resizable: true},
		{Header: "Started", Text: func(r runRow) string { return r.Started.Format("2006-01-02 15:04") },
			Compare: func(a, b runRow) int { return a.Started.Compare(b.Started) }, Width: 170, Resizable: true},
		{Header: "Cost", Text: func(r runRow) string { return fmt.Sprintf("$%.2f", r.Cost) },
			Compare: func(a, b runRow) int { return cmp.Compare(a.Cost, b.Cost) },
			Align:   ui.End, Width: 100, Resizable: true},
	}
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

	rows := deployRuns()
	if none.Get() {
		rows = nil
	}
	d := ui.DataGridComfortable
	if density.Get() == 1 {
		d = ui.DataGridCompact
	}
	grid := ui.DataGrid(rows, func(r runRow) string { return r.ID }, runColumns()...).
		Label("Deploy runs").Density(d).Selection(ui.SelectionMultiple, picked).Sort(order).
		Empty(ui.Text("No deploy runs match")).Style(ui.Style{Height: ui.Px(420)})
	if striped.Get() {
		grid.Striped()
	}
	return Card(t).Gap(14).
		Child(SectionTitle(t, "Data grid")).
		Child(ui.Text("100,000 deploy runs. Click a header to sort, drag its edge to resize, and select rows with Ctrl or Shift. " +
			"Enter opens the focused row.").Size(13).Color(t.TextMuted)).
		Child(ui.Div().Row().Wrap().Align(ui.Center).Gap(18).
			Child(ui.Tabs(density,
				ui.Tab[int]{Value: 0, Label: "Comfortable"},
				ui.Tab[int]{Value: 1, Label: "Compact"},
			).Key("runs-density").Label("Row density").Variant(ui.TabsSegmented)).
			Child(ui.Checkbox("Striped", striped).Key("runs-striped")).
			Child(ui.Checkbox("No rows", none).Key("runs-none")).
			Child(ui.Text(fmt.Sprintf("%d selected", len(picked.Get()))).Size(13).Color(t.TextMuted))).
		Child(grid)
}
