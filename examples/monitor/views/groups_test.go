package views

import (
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/examples/monitor/sample"
)

func TestGroupRowsGroupsByProgramAndKeepsWindowsApart(t *testing.T) {
	procs := []sample.Process{
		{Key: "1", PID: 1, Name: "chrome.exe", CPU: 2, Private: 100, HasWindow: true},
		{Key: "2", PID: 2, Name: "chrome.exe", CPU: 3, Private: 50},
		{Key: "3", PID: 3, Name: "editor.exe", CPU: 1, Private: 10},
		{Key: "4", PID: 4, Name: "Memory Compression", Private: 1000, Program: sample.Program{Windows: true}},
		{Key: "5", PID: 5, Name: "svchost.exe", Private: 5, Program: sample.Program{Windows: true}},
		{Key: "6", PID: 6, Name: "svchost.exe", Private: 7, Program: sample.Program{Windows: true}},
	}
	rows := groupRows(procs)
	var keys []string
	for _, r := range rows {
		keys = append(keys, r.key)
	}
	if want := []string{sectionApps, sectionBackground, groupWindows}; !slices.Equal(keys, want) {
		t.Fatalf("sections %v, want %v", keys, want)
	}
	apps, _ := findRow(rows, sectionApps)
	if len(apps.children) != 1 || apps.children[0].key != "group:chrome.exe" {
		t.Errorf("Apps holds %+v, want the chrome group, one of whose processes has a window", apps.children)
	}
	chrome, ok := findRow(rows, "group:chrome.exe")
	if !ok || chrome.count != 2 || chrome.CPU != 5 || chrome.Private != 150 {
		t.Errorf("chrome group = %+v, want 2 processes, CPU 5, memory 150", chrome)
	}
	if got := groupName(chrome); got != "chrome.exe (2)" {
		t.Errorf("group name = %q", got)
	}
	if editor, ok := findRow(rows, "3"); !ok || editor.isGroup() {
		t.Errorf("a program with one process is its own row, not a group")
	}
	win, _ := findRow(rows, groupWindows)
	if win.count != 3 || win.Private != 1012 || !win.section {
		t.Errorf("Windows section = %+v, want 3 processes and 1012 bytes", win)
	}
	if got := groupName(win); got != "Windows processes (2)" {
		t.Errorf("section name = %q, want its two entries counted", got)
	}
	if _, ok := findRow(win.children, "group:svchost.exe"); !ok {
		t.Errorf("the svchost processes are not grouped inside the Windows section")
	}
}
