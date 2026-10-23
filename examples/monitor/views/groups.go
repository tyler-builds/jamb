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
}

func (r row) isGroup() bool { return r.count > 0 }

// groupWindows is the key of the group of Windows' own processes.
const groupWindows = "group:windows"

// groupRows groups procs as Task Manager does: the parts of Windows in one group
// (see sample.Program.Windows), and the processes of each other program, by its executable, in a group
// of their own when it runs more than one. A group is named for the program, as
// its version information names it ("Helium" for a chrome.exe that names itself
// so).
func groupRows(procs []sample.Process) []row {
	var system []sample.Process
	var apps []sample.Process
	for _, p := range procs {
		if p.Windows {
			system = append(system, p)
		} else {
			apps = append(apps, p)
		}
	}
	rows := byProgram(apps)
	if len(system) > 0 {
		rows = append(rows, group(groupWindows, "Windows processes", byProgram(system)))
	}
	return rows
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
		g.Path = members[0].Path // for the program's icon
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

// groupName is a group's name with its count, as "chrome.exe (16)".
func groupName(r row) string {
	if !r.isGroup() {
		return r.Label()
	}
	var b strings.Builder
	b.WriteString(r.Label())
	b.WriteString(" (")
	b.WriteString(count(r.count))
	b.WriteString(")")
	return b.String()
}
