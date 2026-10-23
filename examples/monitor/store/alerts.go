package store

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/tyler-builds/jamb/examples/monitor/sample"
)

// AlertKind names a rule.
type AlertKind string

const (
	AlertCPU    AlertKind = "cpu"
	AlertMemory AlertKind = "memory"
)

// Rule raises an alert when a value stays above Threshold for For.
type Rule struct {
	On bool `json:"on"`
	// Threshold is a percentage.
	Threshold float64 `json:"threshold"`
	// For is how long the value must stay above the threshold, in seconds.
	For int `json:"for"`
}

// Rules are the alert rules.
type Rules struct {
	CPU    Rule `json:"cpu"`
	Memory Rule `json:"memory"`
}

// DefaultRules alert on a processor load above 90 percent for 10 seconds, and on
// memory above 90 percent for 5.
var DefaultRules = Rules{
	CPU:    Rule{On: true, Threshold: 90, For: 10},
	Memory: Rule{On: true, Threshold: 90, For: 5},
}

// Alert is a rule's condition that held for its time.
type Alert struct {
	Time time.Time
	Kind AlertKind
	// Value is the percentage that crossed the threshold, and Message says so in a
	// sentence.
	Value   float64
	Message string
	// Top is the name of the process that used the most of the resource then.
	Top string
}

// maxAlerts is how many alerts the store keeps.
const maxAlerts = 200

// check runs the rules on snap, and returns the alerts it raises. A rule raises
// one alert when its condition has held for its time, and another only after the
// condition has stopped holding. Called with s.mu held.
func (s *Store) check(snap sample.Snapshot) []Alert {
	rules := s.Rules.Peek()
	var alerts []Alert
	var firing []AlertKind
	for _, r := range []struct {
		kind  AlertKind
		rule  Rule
		value float64
		what  string
		top   func(a, b sample.Process) int
	}{
		{AlertCPU, rules.CPU, snap.CPU, "The processor load",
			func(a, b sample.Process) int { return cmp.Compare(a.CPU, b.CPU) }},
		{AlertMemory, rules.Memory, snap.Mem.Percent(), "Memory use",
			func(a, b sample.Process) int { return cmp.Compare(a.Private, b.Private) }},
	} {
		if !r.rule.On || r.value <= r.rule.Threshold {
			delete(s.over, r.kind)
			delete(s.raised, r.kind)
			continue
		}
		start, ok := s.over[r.kind]
		if !ok {
			start = snap.Time
			s.over[r.kind] = start
		}
		held := snap.Time.Sub(start)
		if held < time.Duration(r.rule.For)*time.Second {
			continue
		}
		firing = append(firing, r.kind)
		if s.raised[r.kind] {
			continue
		}
		s.raised[r.kind] = true
		a := Alert{Time: snap.Time, Kind: r.kind, Value: r.value,
			Message: fmt.Sprintf("%s has been above %.0f%% for %d seconds.", r.what, r.rule.Threshold, max(r.rule.For, int(held.Seconds())))}
		if len(snap.Procs) > 0 {
			a.Top = slices.MaxFunc(snap.Procs, r.top).Label()
		}
		alerts = append(alerts, a)
	}
	if !slices.Equal(firing, s.Firing.Peek()) {
		s.Firing.Set(firing)
	}
	return alerts
}
