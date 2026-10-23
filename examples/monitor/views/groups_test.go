package views

import (
	"testing"

	"github.com/tyler-builds/jamb/examples/monitor/sample"
)

func TestGroupRowsGroupsByProgramAndKeepsWindowsApart(t *testing.T) {
	procs := []sample.Process{
		{Key: "1", PID: 1, Name: "chrome.exe", CPU: 2, Private: 100},
		{Key: "2", PID: 2, Name: "chrome.exe", CPU: 3, Private: 50},
		{Key: "3", PID: 3, Name: "editor.exe", CPU: 1, Private: 10},
		{Key: "4", PID: 4, Name: "Memory Compression", Private: 1000, Program: sample.Program{Windows: true}},
		{Key: "5", PID: 5, Name: "svchost.exe", Private: 5, Program: sample.Program{Windows: true}},
		{Key: "6", PID: 6, Name: "svchost.exe", Private: 7, Program: sample.Program{Windows: true}},
	}
	rows := groupRows(procs)
	if len(rows) != 3 {
		t.Fatalf("%d top rows, want chrome, editor and Windows", len(rows))
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
	win, ok := findRow(rows, groupWindows)
	if !ok || win.count != 3 || win.Private != 1012 {
		t.Errorf("Windows group = %+v, want 3 processes and 1012 bytes", win)
	}
	if _, ok := findRow(win.children, "group:svchost.exe"); !ok {
		t.Errorf("the svchost processes are not grouped inside the Windows group")
	}
	if got := len(groupKeys(rows)); got != 3 {
		t.Errorf("%d group keys, want 3", got)
	}
}
