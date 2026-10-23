package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/monitor/sample"
)

func newStore() (*Store, *sample.Synthetic) {
	src := sample.NewSynthetic(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC), time.Second)
	return New(src, time.Second), src
}

func TestHistoryKeepsTheLastFiveMinutes(t *testing.T) {
	s, _ := newStore()
	for range HistoryLen + 10 {
		s.Step()
	}
	hist := s.History.Peek()
	if len(hist) != HistoryLen {
		t.Fatalf("%d points, want %d", len(hist), HistoryLen)
	}
	if !hist[len(hist)-1].Time.Equal(s.Latest.Peek().Time) {
		t.Errorf("the last point is not the latest sample")
	}
	key := s.Latest.Peek().Procs[0].Key
	if n := len(s.ProcessHistory(key)); n != HistoryLen {
		t.Errorf("the process history has %d points, want %d", n, HistoryLen)
	}
}

func TestAlertIsRaisedOnceWhileTheConditionHolds(t *testing.T) {
	s, src := newStore()
	s.Rules.Set(Rules{CPU: Rule{On: true, Threshold: 90, For: 2}})
	var heard []Alert
	s.OnAlert = func(a Alert) { heard = append(heard, a) }
	src.Spike = 100
	for range 2 {
		s.Step()
	}
	if len(heard) != 0 {
		t.Fatalf("an alert after %d seconds, before the rule's 2", len(heard))
	}
	for range 5 {
		s.Step()
	}
	if len(heard) != 1 {
		t.Fatalf("%d alerts while the load stayed high, want 1", len(heard))
	}
	if heard[0].Top == "" {
		t.Error("the alert does not name the busiest process")
	}
	src.Spike = 0
	s.Step()
	src.Spike = 100
	for range 4 {
		s.Step()
	}
	if len(heard) != 2 {
		t.Errorf("%d alerts after the load fell and rose again, want 2", len(heard))
	}
}

func TestSettingsKeepTheLayout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	l := ui.DockLayout{Root: ui.DockNode{Panels: []string{"cpu", "processes"}, Active: "processes"}}
	in := Settings{IntervalMS: 2000, Layout: &l, Rules: &DefaultRules,
		Columns: ui.ColumnState{Order: []string{"name", "cpu"}, Widths: map[string]float32{"name": 300}}}
	if err := SaveSettings(path, in); err != nil {
		t.Fatal(err)
	}
	out := LoadSettings(path)
	a, _ := json.Marshal(in)
	b, _ := json.Marshal(out)
	if string(a) != string(b) {
		t.Errorf("loaded %s, want %s", b, a)
	}
}
