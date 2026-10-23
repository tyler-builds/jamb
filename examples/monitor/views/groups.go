package views

import (
	"strings"

	"github.com/tyler-builds/jamb/examples/monitor/sample"
)

// row is a row of the process table: a process, or a group of processes with
// their sums. A group has no PID.
type row struct {
	sample.Process
	// key is the process's key, or "group:" and the group's name.
	key string
	// count is how many processes a group holds, 0 for a process.
	count    int
	children []row
	// section is set for the rows of the sections: Apps, Background processes and
	// Windows processes. They keep their order as the table sorts, and show only
	// their names.
	section bool
}

func (r row) isGroup() bool { return r.count > 0 }

// The keys of the sections of the grouped table, in their order.
const (
	sectionApps       = "group:apps"
	sectionBackground = "group:background"
	groupWindows      = "group:windows"
)

// sections lists the sections' keys, which are open until the user closes them.
var sections = []string{sectionApps, sectionBackground, groupWindows}

// groupRows groups procs as Task Manager does, in three sections: Apps, the
// programs with a window on the taskbar; Windows processes, the parts of Windows
// (see sample.Program.Windows); and Background processes, the rest. In each, the
// processes of a program, by its executable, are a group of their own when it runs
// more than one. A group is named for the program, as its version information
// names it ("Helium" for a chrome.exe that names itself so).
func groupRows(procs []sample.Process) []row {
	var apps, background, system []row
	for _, r := range byProgram(procs) {
		switch {
		case r.hasWindow():
			apps = append(apps, r)
		case r.Windows:
			system = append(system, r)
		default:
			background = append(background, r)
		}
	}
	var rows []row
	for _, s := range []struct {
		key, name string
		rows      []row
	}{
		{sectionApps, "Apps", apps},
		{sectionBackground, "Background processes", background},
		{groupWindows, "Windows processes", system},
	} {
		if len(s.rows) > 0 {
			sec := group(s.key, s.name, s.rows)
			sec.section = true
			rows = append(rows, sec)
		}
	}
	return rows
}

// hasWindow reports whether the process, or a process of the group, has a window
// on the taskbar.
func (r row) hasWindow() bool {
	if r.HasWindow {
		return true
	}
	for _, c := range r.children {
		if c.hasWindow() {
			return true
		}
	}
	return false
}

// byProgram is the rows of procs, one for each executable: the process itself
// when it runs alone, or a group of them.
func byProgram(procs []sample.Process) []row {
	byExe := map[string][]sample.Process{}
	var exes []string
	for _, p := range procs {
		exe := programKey(p)
		if _, ok := byExe[exe]; !ok {
			exes = append(exes, exe)
		}
		byExe[exe] = append(byExe[exe], p)
	}
	rows := make([]row, 0, len(exes))
	for _, exe := range exes {
		members := byExe[exe]
		if len(members) == 1 {
			rows = append(rows, processRow(members[0]))
			continue
		}
		children := make([]row, len(members))
		for i, p := range members {
			children[i] = processRow(p)
		}
		g := group("group:"+exe, members[0].Label(), children)
		g.Path = members[0].Path       // for the program's icon
		g.Windows = members[0].Windows // the processes of one executable are all parts of Windows, or none
		rows = append(rows, g)
	}
	return rows
}

// programKey is what groups a process with others: its executable's path, or its
// name when Windows gives no path.
func programKey(p sample.Process) string {
	if p.Path != "" {
		return strings.ToLower(p.Path)
	}
	return strings.ToLower(p.Name)
}

func processRow(p sample.Process) row { return row{Process: p, key: p.Key} }

// group returns a group of children named name, with their sums: the processor,
// memory and I/O, the threads and handles, and the earliest start.
func group(key, name string, children []row) row {
	g := row{key: key, children: children}
	g.Name, g.Description = name, name
	for _, c := range children {
		g.count += max(c.count, 1)
		g.CPU += c.CPU
		g.Private += c.Private
		g.WorkingSet += c.WorkingSet
		g.Threads += c.Threads
		g.Handles += c.Handles
		g.IORead += c.IORead
		g.IOWrite += c.IOWrite
		if !c.Started.IsZero() && (g.Started.IsZero() || c.Started.Before(g.Started)) {
			g.Started = c.Started
		}
	}
	g.CPU = min(g.CPU, 100)
	return g
}

// flatRows is the table without groups: one row a process.
func flatRows(procs []sample.Process) []row {
	rows := make([]row, len(procs))
	for i, p := range procs {
		rows[i] = processRow(p)
	}
	return rows
}

// groupKeys returns the keys of every group in rows, for expanding them all.
func groupKeys(rows []row) []string {
	var keys []string
	for _, r := range rows {
		if r.isGroup() {
			keys = append(keys, r.key)
			keys = append(keys, groupKeys(r.children)...)
		}
	}
	return keys
}

// findRow returns the row with key, at any depth.
func findRow(rows []row, key string) (row, bool) {
	for _, r := range rows {
		if r.key == key {
			return r, true
		}
		if f, ok := findRow(r.children, key); ok {
			return f, true
		}
	}
	return row{}, false
}

// groupName is a group's name with its count, as "chrome.exe (16)": its processes,
// or a section's entries, as Task Manager counts them.
func groupName(r row) string {
	if !r.isGroup() {
		return r.Label()
	}
	n := r.count
	if r.section {
		n = len(r.children)
	}
	var b strings.Builder
	b.WriteString(r.Label())
	b.WriteString(" (")
	b.WriteString(count(n))
	b.WriteString(")")
	return b.String()
}
