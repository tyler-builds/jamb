package sample

import (
	"testing"
	"time"
)

func TestRatesFromTwoReadings(t *testing.T) {
	at := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	prev := Raw{Time: at,
		Cores: []CoreTimes{{Idle: 100, Kernel: 200, User: 100}, {Idle: 0, Kernel: 0, User: 0}},
		NetIn: 1000, NetOut: 500,
		Procs: []RawProc{{PID: 7, Name: "a.exe", Created: 1, CPU: 10, ReadBytes: 100}}}
	cur := Raw{Time: at.Add(2 * time.Second),
		// Core 0: 100 of 200 units idle; core 1: 50 of 200. Kernel includes idle.
		Cores: []CoreTimes{{Idle: 200, Kernel: 350, User: 150}, {Idle: 50, Kernel: 100, User: 100}},
		NetIn: 3000, NetOut: 400, // the sent counter went back, as when an interface goes
		Procs: []RawProc{
			{PID: 7, Name: "a.exe", Created: 1, CPU: 110, ReadBytes: 2100},
			{PID: 7, Name: "b.exe", Created: 2, CPU: 500}, // a later process with the same PID
		}}
	s := Rates(&prev, cur)
	if s.Cores[0] != 50 || s.Cores[1] != 75 {
		t.Errorf("cores = %v, want [50 75]", s.Cores)
	}
	if s.CPU != 62.5 {
		t.Errorf("CPU = %v, want 62.5", s.CPU)
	}
	if s.NetIn != 1000 || s.NetOut != 0 {
		t.Errorf("network = %v in, %v out; want 1000 and 0", s.NetIn, s.NetOut)
	}
	if len(s.Procs) != 2 {
		t.Fatalf("%d processes, want 2", len(s.Procs))
	}
	for _, p := range s.Procs {
		switch p.Name {
		case "a.exe":
			if p.CPU != 25 || p.IORead != 1000 {
				t.Errorf("a.exe: CPU %v, read %v; want 25 and 1000", p.CPU, p.IORead)
			}
		case "b.exe":
			if p.CPU != 0 {
				t.Errorf("b.exe has CPU %v, but it is new: a process that reuses a PID starts from zero", p.CPU)
			}
		}
	}
}

func TestFirstSnapshotHasNoRates(t *testing.T) {
	s := NewSampler(NewSynthetic(time.Unix(0, 0), time.Second))
	snap, err := s.Next()
	if err != nil {
		t.Fatal(err)
	}
	if snap.CPU != 0 || snap.NetIn != 0 || len(snap.Procs) == 0 {
		t.Errorf("first snapshot: CPU %v, in %v, %d processes", snap.CPU, snap.NetIn, len(snap.Procs))
	}
	snap, _ = s.Next()
	if snap.CPU <= 0 || snap.CPU > 100 {
		t.Errorf("second snapshot CPU = %v, want a load", snap.CPU)
	}
}

func TestSyntheticEndChecksTheStartTime(t *testing.T) {
	src := NewSynthetic(time.Unix(0, 0), time.Second)
	raw, _ := src.Read()
	p := raw.Procs[len(raw.Procs)-1]
	if err := src.End(p.PID, p.Created+1); err != ErrGone {
		t.Errorf("End with another start time = %v, want ErrGone", err)
	}
	if err := src.End(p.PID, p.Created); err != nil {
		t.Fatalf("End = %v", err)
	}
	raw, _ = src.Read()
	for _, q := range raw.Procs {
		if q.PID == p.PID {
			t.Errorf("process %d is still running", p.PID)
		}
	}
}

func TestUtilityReplacesTheBusyTime(t *testing.T) {
	at := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	prev := Raw{Time: at, Cores: []CoreTimes{{}},
		Procs: []RawProc{{PID: 7, Name: "a.exe", Created: 1}}}
	// The core was busy for half of 200 units, and the process for 50 of them.
	cur := Raw{Time: at.Add(time.Second), Cores: []CoreTimes{{Idle: 100, Kernel: 150, User: 50}},
		HasUtility: true, Utility: 60, CoreUtility: []float64{60},
		Procs: []RawProc{{PID: 7, Name: "a.exe", Created: 1, CPU: 50}}}
	s := Rates(&prev, cur)
	if s.CPU != 60 || s.Cores[0] != 60 {
		t.Errorf("CPU %v, core %v; want the utility, 60", s.CPU, s.Cores[0])
	}
	// The process had half of the busy time, so half of the utility.
	if got := s.Procs[0].CPU; got != 30 {
		t.Errorf("process CPU = %v, want 30", got)
	}
}
