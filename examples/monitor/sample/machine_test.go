package sample

import (
	"runtime"
	"testing"
	"time"
)

func TestMachineReads(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("reads Windows counters")
	}
	s := NewSampler(Machine())
	if _, err := s.Next(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	snap, err := s.Next()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("cpu %.1f cores %d mem %.1f%% net %.0f/%.0f io %.0f/%.0f procs %d threads %d",
		snap.CPU, len(snap.Cores), snap.Mem.Percent(), snap.NetIn, snap.NetOut, snap.IORead, snap.IOWrite, len(snap.Procs), snap.Threads())
	if len(snap.Procs) < 10 || len(snap.Cores) == 0 || snap.Mem.Total == 0 {
		t.Fatalf("implausible snapshot")
	}
	for _, p := range snap.Procs[:5] {
		t.Logf("%d %s cpu %.1f priv %d started %v", p.PID, p.Name, p.CPU, p.Private, p.Started)
	}
}
