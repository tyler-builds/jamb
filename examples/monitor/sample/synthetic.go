package sample

import (
	"math"
	"slices"
	"sync"
	"time"
)

// Synthetic is a made-up machine: four processors and a dozen processes whose load
// rises and falls in waves. Each Read moves it on by one step, so its readings are
// the same from run to run. It stands in for the machine in the tests, and on the
// platforms without a reader.
type Synthetic struct {
	mu    sync.Mutex
	step  int
	now   time.Time
	every time.Duration
	raw   Raw
	ended map[uint32]bool
	// Spike, when it is above zero, adds that many percent of load to every
	// processor, for the tests of the alerts.
	Spike float64
}

// synthProc is a process of the made-up machine: a name, a parent, a share of the
// load and a private working set.
type synthProc struct {
	pid, ppid uint32
	name      string
	load      float64
	mem       uint64
}

// syntheticProcs are the processes of the made-up machine.
var syntheticProcs = []synthProc{
	{4, 0, "System", 0.02, 140 << 10},
	{612, 4, "smss.exe", 0, 1 << 20},
	{820, 612, "csrss.exe", 0.01, 2 << 20},
	{904, 612, "wininit.exe", 0, 1 << 20},
	{1040, 904, "services.exe", 0.01, 6 << 20},
	{1236, 1040, "svchost.exe", 0.03, 24 << 20},
	{2210, 904, "explorer.exe", 0.04, 96 << 20},
	{3120, 2210, "editor.exe", 0.12, 420 << 20},
	{3488, 2210, "browser.exe", 0.18, 860 << 20},
	{3492, 3488, "browser.exe", 0.06, 210 << 20},
	{3510, 3488, "browser.exe", 0.03, 120 << 20},
	{4020, 2210, "build.exe", 0.25, 640 << 20},
	{4108, 2210, "terminal.exe", 0.01, 48 << 20},
}

// NewSynthetic returns a made-up machine whose clock starts at start and moves
// on by every on each Read.
func NewSynthetic(start time.Time, every time.Duration) *Synthetic {
	s := &Synthetic{now: start, every: every, ended: map[uint32]bool{}}
	s.raw.Mem = Memory{Total: 16 << 30, CommitLimit: 24 << 30}
	s.raw.Cores = make([]CoreTimes, 4)
	for _, p := range syntheticProcs {
		s.raw.Procs = append(s.raw.Procs, RawProc{PID: p.pid, PPID: p.ppid, Name: p.name,
			Created: start.Add(-time.Hour).UnixNano()/100 + epochDiff + int64(p.pid)})
	}
	return s
}

// Read moves the machine on by one step and reads it.
func (s *Synthetic) Read() (Raw, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.step++
	s.now = s.now.Add(s.every)
	t := float64(s.step)
	tick := s.every.Nanoseconds() / 100 // one step of one processor, in 100 ns
	var used uint64
	procs := s.raw.Procs[:0:0]
	for i, p := range s.raw.Procs {
		if s.ended[p.PID] {
			continue
		}
		def := syntheticProcs[slices.IndexFunc(syntheticProcs, func(d synthProc) bool { return d.pid == p.PID })]
		wave := 0.5 + 0.5*math.Sin(t/7+float64(i))
		share := def.load * (0.4 + 1.2*wave)
		p.CPU += int64(share * float64(tick) * float64(len(s.raw.Cores)))
		p.Private = def.mem + uint64(float64(def.mem)*0.1*wave)
		p.WorkingSet = p.Private + p.Private/3
		p.Threads = uint32(4 + i*3)
		p.Handles = uint32(120 + i*90)
		p.ReadBytes += uint64(share * (4 << 20))
		p.WriteBytes += uint64(share * (1 << 20))
		used += p.Private
		// The editor, the terminal, the file manager and the browser's first process
		// have windows.
		p.HasWindow = p.PID == 3120 || p.PID == 4108 || p.PID == 2210 || p.PID == 3488
		procs = append(procs, p)
	}
	s.raw.Procs = procs
	for i := range s.raw.Cores {
		load := 0.15 + 0.5*(0.5+0.5*math.Sin(t/5+float64(i))) + s.Spike/100
		load = max(0, min(1, load))
		busy := int64(load * float64(tick))
		c := &s.raw.Cores[i]
		c.User += busy * 2 / 3
		c.Kernel += tick - busy*2/3 // kernel time includes idle time
		c.Idle += tick - busy
	}
	s.raw.Mem.Available = s.raw.Mem.Total - min(s.raw.Mem.Total, 3<<30+used)
	s.raw.Mem.CommitTotal = s.raw.Mem.Total - s.raw.Mem.Available + 2<<30
	s.raw.NetIn += uint64((1 + math.Sin(t/3)) * (900 << 10))
	s.raw.NetOut += uint64((1 + math.Cos(t/4)) * (120 << 10))
	s.raw.Time = s.now
	out := s.raw
	out.Cores = slices.Clone(s.raw.Cores)
	out.Procs = slices.Clone(s.raw.Procs)
	return out, nil
}

// syntheticDescriptions are the programs' names of the made-up executables. The
// build tool gives none, as some programs do.
var syntheticDescriptions = map[string]string{
	"explorer.exe": "Windows Explorer", "editor.exe": "Editor", "browser.exe": "Browser",
	"terminal.exe": "Terminal",
}

// Describe returns the made-up program of the process with pid. Windows' own
// processes run from the Windows folder, but System, as on Windows, has no
// executable.
func (s *Synthetic) Describe(pid uint32) Program {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.raw.Procs {
		if p.PID != pid || s.ended[pid] {
			continue
		}
		switch {
		case p.Name == "System":
			return Program{Windows: true}
		case p.Name == "explorer.exe" || p.Name == "build.exe" || syntheticDescriptions[p.Name] != "":
			return Program{Path: `C:\Programs\` + p.Name, Description: syntheticDescriptions[p.Name]}
		}
		return Program{Path: `C:\Windows\System32\` + p.Name, Windows: true}
	}
	return Program{}
}

// End takes the process with pid off the made-up machine.
func (s *Synthetic) End(pid uint32, created int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.raw.Procs {
		if p.PID == pid && p.Created == created && !s.ended[pid] {
			s.ended[pid] = true
			return nil
		}
	}
	return ErrGone
}
