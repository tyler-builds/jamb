package views

import (
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// The panels of the dock, by ID.
const (
	panelCPU     = "cpu"
	panelMemory  = "memory"
	panelNetwork = "network"
	panelIO      = "io"
	panelProcs   = "processes"
	panelDetails = "details"
	panelAlerts  = "alerts"
)

// DefaultDock is the layout the window starts with, and the one Reset panel layout
// returns to: the charts above the process table, and the selected process's
// details above the alerts beside them.
var DefaultDock = ui.DockLayout{Root: ui.DockNode{
	Sizes: []float64{0.62, 0.38},
	Children: []ui.DockNode{
		{Vertical: true, Sizes: []float64{0.42, 0.58}, Children: []ui.DockNode{
			{Panels: []string{panelCPU, panelMemory, panelNetwork, panelIO}, Active: panelCPU},
			{Panels: []string{panelProcs}, Active: panelProcs},
		}},
		{Vertical: true, Sizes: []float64{0.58, 0.42}, Children: []ui.DockNode{
			{Panels: []string{panelDetails}, Active: panelDetails},
			{Panels: []string{panelAlerts}, Active: panelAlerts},
		}},
	},
}}

// DefaultSort is the process table's sort until the user picks another: the
// busiest first.
var DefaultSort = ui.SortState{Key: colCPU, Desc: true}

// dock shows the panels. The layout is the app's, so it is kept with the settings,
// and a panel dragged out of the window floats in a window of its own.
func (a *App) dock() ui.Element {
	panels := map[string]ui.DockPanel{
		panelCPU:     {Title: "Processor", Icon: icons.Cpu, Closable: true, Content: func() ui.Element { return ui.Component(a.cpuPanel) }},
		panelMemory:  {Title: "Memory", Icon: icons.MemoryStick, Closable: true, Content: func() ui.Element { return ui.Component(a.memoryPanel) }},
		panelNetwork: {Title: "Network", Icon: icons.Network, Closable: true, Content: func() ui.Element { return ui.Component(a.networkPanel) }},
		panelIO:      {Title: "Process I/O", Icon: icons.HardDrive, Closable: true, Content: func() ui.Element { return ui.Component(a.ioPanel) }},
		panelProcs:   {Title: "Processes", Icon: icons.List, Content: a.processPanel},
		panelDetails: {Title: "Details", Icon: icons.Info, Closable: true, Content: func() ui.Element { return ui.Component(a.detailPanel) }},
		panelAlerts:  {Title: "Alerts", Icon: icons.Bell, Closable: true, Content: func() ui.Element { return ui.Component(a.alertPanel) }},
	}
	d := ui.Dock(a.layout.Dock, panels).Key("dock").OnChange(func(ui.DockLayout) { a.changed() })
	if a.layout.Floating != nil {
		d.Floating(a.layout.Floating)
	}
	return d
}
