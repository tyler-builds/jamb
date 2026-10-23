// Package store samples the machine on a timer and keeps the samples as signals:
// the latest snapshot, the history of the totals and of each process, and the
// alerts the samples raised.
package store

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/monitor/sample"
)

// HistoryLen is how many samples the history keeps: five minutes at one a second.
const HistoryLen = 300

// Point is one sample of the totals, for the charts.
type Point struct {
	Time  time.Time
	CPU   float64
	Cores []float64
	// MemUsed is in bytes, and MemPercent the share of the physical memory.
	MemUsed    uint64
	MemPercent float64
	// The rates are in bytes per second.
	NetIn, NetOut   float64
	IORead, IOWrite float64
}

// ProcPoint is one sample of a process, for its chart.
type ProcPoint struct {
	Time    time.Time
	CPU     float64
	Private uint64
}

// Store holds the samples. Its signals are set from the sampling goroutine and
// read on the UI thread.
type Store struct {
	// Latest is the newest snapshot.
	Latest *ui.Signal[sample.Snapshot]
	// History holds the last HistoryLen points, oldest first.
	History *ui.Signal[[]Point]
	// Paused stops the sampling while it holds true.
	Paused *ui.Signal[bool]
	// Interval is the time between samples.
	Interval *ui.Signal[time.Duration]
	// Err is the last failure to sample, or "".
	Err *ui.Signal[string]
	// Rules are the alert rules, and Alerts the alerts they raised, newest first.
	Rules  *ui.Signal[Rules]
	Alerts *ui.Signal[[]Alert]
	// Firing lists the kinds of the rules whose condition holds now.
	Firing *ui.Signal[[]AlertKind]
	// OnAlert, when set, runs on the sampling goroutine for each new alert, and
	// OnSample after each sample.
	OnAlert  func(Alert)
	OnSample func(sample.Snapshot)

	sampler *sample.Sampler
	src     sample.Source
	mu      sync.Mutex // guards procs and the rule state, and keeps two samples apart
	procs   map[string][]ProcPoint
	over    map[AlertKind]time.Time // when each rule's condition started to hold
	raised  map[AlertKind]bool      // the rules that raised an alert since they last held false
	wake    chan struct{}
}

// New returns a store that samples src every interval.
func New(src sample.Source, interval time.Duration) *Store {
	return &Store{
		Latest:   ui.NewSignal(sample.Snapshot{}),
		History:  ui.NewSignal([]Point(nil)),
		Paused:   ui.NewSignal(false),
		Interval: ui.NewSignal(interval),
		Err:      ui.NewSignal(""),
		Rules:    ui.NewSignal(DefaultRules),
		Alerts:   ui.NewSignal([]Alert(nil)),
		Firing:   ui.NewSignal([]AlertKind(nil)),
		sampler:  sample.NewSampler(src),
		src:      src,
		procs:    map[string][]ProcPoint{},
		over:     map[AlertKind]time.Time{},
		raised:   map[AlertKind]bool{},
		wake:     make(chan struct{}, 1),
	}
}

// Source returns the source the store samples.
func (s *Store) Source() sample.Source { return s.src }

// Run samples until ctx is done: at once, then every Interval while not Paused.
func (s *Store) Run(ctx context.Context) {
	for {
		if !s.Paused.Peek() {
			s.Step()
		}
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-time.After(s.Interval.Peek()):
		}
	}
}

// Wake makes Run sample now instead of at the end of its wait, after a change of
// the interval or a resume.
func (s *Store) Wake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Step takes one sample and records it.
func (s *Store) Step() {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.sampler.Next()
	if err != nil {
		s.Err.Set(err.Error())
		return
	}
	if s.Err.Peek() != "" {
		s.Err.Set("")
	}
	p := Point{Time: snap.Time, CPU: snap.CPU, Cores: snap.Cores, MemUsed: snap.Mem.Used(),
		MemPercent: snap.Mem.Percent(), NetIn: snap.NetIn, NetOut: snap.NetOut,
		IORead: snap.IORead, IOWrite: snap.IOWrite}
	hist := s.History.Peek()
	next := make([]Point, 0, min(len(hist)+1, HistoryLen))
	next = append(next, hist[max(0, len(hist)+1-HistoryLen):]...)
	next = append(next, p)

	seen := make(map[string]bool, len(snap.Procs))
	for _, pr := range snap.Procs {
		seen[pr.Key] = true
		h := s.procs[pr.Key]
		if len(h) >= HistoryLen {
			h = slices.Delete(h, 0, len(h)+1-HistoryLen)
		}
		s.procs[pr.Key] = append(h, ProcPoint{Time: snap.Time, CPU: pr.CPU, Private: pr.Private})
	}
	for k := range s.procs {
		if !seen[k] {
			delete(s.procs, k)
		}
	}
	alerts := s.check(snap)

	s.History.Set(next)
	s.Latest.Set(snap)
	if len(alerts) > 0 {
		all := slices.Concat(alerts, s.Alerts.Peek())
		s.Alerts.Set(all[:min(len(all), maxAlerts)])
		if s.OnAlert != nil {
			for _, a := range alerts {
				s.OnAlert(a)
			}
		}
	}
	if s.OnSample != nil {
		s.OnSample(snap)
	}
}

// ProcessHistory returns the samples of the process with key, oldest first.
func (s *Store) ProcessHistory(key string) []ProcPoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.procs[key])
}

// Process returns the process with key in the latest snapshot.
func (s *Store) Process(key string) (sample.Process, bool) {
	for _, p := range s.Latest.Get().Procs {
		if p.Key == key {
			return p, true
		}
	}
	return sample.Process{}, false
}
