package views

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/charts"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/monitor/store"
)

// detailPanel shows the selected process: its facts, its executable's path, its
// parent, and charts of its processor and memory use since it was first sampled.
func (a *App) detailPanel(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	key, p, ok := a.selectedProcess()
	switch {
	case key == "":
		return ui.Empty("No process selected").Icon(icons.Info).
			Description("Select a process in the table to see its details here.")
	case strings.HasPrefix(key, "group:"):
		return a.groupDetail(t, key)
	case !ok:
		return ui.Empty("The process has exited").Icon(icons.CircleX).
			Description("It is no longer running. Select another process in the table.")
	}
	hist := a.s.ProcessHistory(key)
	pid := p.PID

	pathEl := ui.Element(ui.Text("Not given by Windows").TextColor(t.TextMuted))
	if p.Path != "" {
		pathEl = ui.Div().Row().Align(ui.Center).Gap(6).MinWidth(ui.Px(0)).
			Child(ui.Div().Shrink(1).MinWidth(ui.Px(0)).Tooltip(p.Path).Child(ui.Text(p.Path).Truncate())).
			Child(ui.CopyButton(p.Path).Label("Copy path"))
	}

	parent := ui.Element(ui.Text("None"))
	if q, ok := a.parentOf(p); ok {
		pk := q.Key
		parent = ui.Button(q.Label() + " (" + strconv.FormatUint(uint64(q.PID), 10) + ")").Key("parent").Ghost().Small().
			Tooltip("Select the parent process").OnClick(func() { a.selectProcess(pk) })
	} else if p.PPID != 0 {
		parent = ui.Text("Process " + strconv.FormatUint(uint64(p.PPID), 10) + ", which has exited").TextColor(t.TextMuted)
	}

	started := ui.Element(ui.Text("Not reported").TextColor(t.TextMuted))
	if !p.Started.IsZero() {
		started = ui.TimeAgo(p.Started)
	}

	bytes := func(v float64) string { return ui.FormatBytes(uint64(max(v, 0))) }
	body := ui.Div().Column().Size(ui.Full, ui.Full).Padding(14).Gap(14).ScrollY().Key("detail")
	if about := aboutProcess[p.Name]; about != "" {
		body.Child(ui.Alert("About " + p.Label()).Key("about").Description(about))
	}
	return body.
		Child(ui.Div().Row().Align(ui.Center).Gap(10).
			Child(ui.Div().Column().Gap(2).Grow(1).MinWidth(ui.Px(0)).
				Child(ui.Text(p.Label()).Key("detail-name").FontSize(17).FontWeight(ui.FontSemibold).Truncate()).
				Child(ui.Text(p.Name + ", process ID " + strconv.FormatUint(uint64(pid), 10)).FontSize(12).TextColor(t.TextMuted))).
			Child(ui.Button("End process…").Key("end").Danger().Small().Icon(icons.CircleX).
				OnClick(func() { a.ending.Set(&p) }))).
		Child(ui.DescriptionList().Key("detail-facts").
			Item("Path", pathEl).
			Item("Parent", parent).
			Item("Started", started).
			ItemText("Processor", percent(p.CPU)).
			ItemText("Memory", ui.FormatBytes(p.Private)+" private, "+ui.FormatBytes(p.WorkingSet)+" working set").
			ItemText("I/O", "Read "+rate(p.IORead)+", write "+rate(p.IOWrite)).
			ItemText("Threads", count(int(p.Threads))).
			ItemText("Handles", count(int(p.Handles)))).
		Child(charts.Area(charts.Series[store.ProcPoint]{Name: "Processor", Data: hist, Time: procTime,
			Y: func(q store.ProcPoint) float64 { return q.CPU }}).
			Key("detail-cpu").Title("Processor").YAxis(charts.Axis{Format: percent}.Min(0)).
			Legend(charts.LegendNone).Format(percent).Height(ui.Px(160))).
		Child(charts.Area(charts.Series[store.ProcPoint]{Name: "Memory", Data: hist, Time: procTime,
			Y: func(q store.ProcPoint) float64 { return float64(q.Private) }}).
			Key("detail-memory").Title("Private memory").YAxis(charts.Axis{Format: bytes}.Min(0)).
			Legend(charts.LegendNone).Format(bytes).Height(ui.Px(160)))
}

func procTime(q store.ProcPoint) time.Time { return q.Time }

// aboutProcess explains the Windows processes whose numbers are easy to misread.
var aboutProcess = map[string]string{
	"Memory Compression": "Windows keeps the memory it has compressed here. The memory belongs to other " +
		"processes, which is why Task Manager lists this process only on its Details tab.",
	"Secure System": "Windows runs its virtualization-based security here. Its memory is set aside when Windows starts.",
}

// groupDetail sums up a group of processes, and lists those that use the most
// memory; each selects its process.
func (a *App) groupDetail(t *ui.Theme, key string) ui.Element {
	g, ok := findRow(groupRows(a.s.Latest.Get().Procs), key)
	if !ok {
		return ui.Empty("The group has no processes left").Icon(icons.CircleX)
	}
	members := slices.Clone(g.children)
	slices.SortFunc(members, func(x, y row) int { return cmp.Compare(y.Private, x.Private) })
	top := ui.Div().Column().Align(ui.Start).Key("detail-members")
	for _, c := range members[:min(len(members), 8)] {
		ck := c.key
		top.Child(ui.Button(groupName(c) + ", " + ui.FormatBytes(c.Private)).Ghost().Small().
			OnClick(func() { a.selectProcess(ck) }))
	}
	return ui.Div().Column().Size(ui.Full, ui.Full).Padding(14).Gap(14).ScrollY().Key("detail").
		Child(ui.Text(groupName(g)).Key("detail-name").FontSize(17).FontWeight(ui.FontSemibold)).
		Child(ui.DescriptionList().Key("detail-facts").
			ItemText("Processes", count(g.count)).
			ItemText("Processor", percent(g.CPU)).
			ItemText("Memory", ui.FormatBytes(g.Private)+" private, "+ui.FormatBytes(g.WorkingSet)+" working set").
			ItemText("I/O", "Read "+rate(g.IORead)+", write "+rate(g.IOWrite)).
			ItemText("Threads", count(int(g.Threads))).
			ItemText("Handles", count(int(g.Handles)))).
		Child(ui.Text("Most memory").FontSize(12).FontWeight(ui.FontMedium).TextColor(t.TextMuted)).
		Child(top)
}
