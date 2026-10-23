// Package sample reads the state of the machine: the load of each processor, the
// memory, the network traffic and the running processes. A Source reads the raw
// counters, and a Sampler turns two readings in a row into rates.
package sample

import (
	"errors"
	"slices"
	"strconv"
	"time"
)

// Raw is one reading of the machine's counters. The times are cumulative, in
// 100-nanosecond units, and the byte counts are cumulative since boot.
type Raw struct {
	Time time.Time
	// Cores holds the cumulative times of each logical processor.
	Cores []CoreTimes
	// Utility, when HasUtility is set, is the processor utility since the last
	// reading, from 0 to 100, and CoreUtility that of each logical processor, as
	// Task Manager measures them (see utilityReader). Without it the load is the
	// processors' busy time.
	HasUtility  bool
	Utility     float64
	CoreUtility []float64
	Mem         Memory
	// NetIn and NetOut are the bytes received and sent by the hardware network
	// interfaces.
	NetIn, NetOut uint64
	Procs         []RawProc
}

// CoreTimes are the cumulative times of a logical processor. Kernel includes Idle,
// as Windows reports it.
type CoreTimes struct {
	Idle, Kernel, User int64
}

// Memory is the physical memory and the commit charge, in bytes.
type Memory struct {
	Total, Available         uint64
	CommitTotal, CommitLimit uint64
}

// Used returns the physical memory in use.
func (m Memory) Used() uint64 { return m.Total - min(m.Available, m.Total) }

// Percent returns the physical memory in use as a percentage of the total.
func (m Memory) Percent() float64 {
	if m.Total == 0 {
		return 0
	}
	return float64(m.Used()) / float64(m.Total) * 100
}

// RawProc is one process in a reading.
type RawProc struct {
	PID, PPID uint32
	Name      string
	// Created is when the process started, as a Windows file time. With the PID it
	// tells a process from a later one that reuses its PID.
	Created int64
	// CPU is the process's cumulative user and kernel time.
	CPU int64
	// Private is the private working set, the memory Task Manager shows; WorkingSet
	// includes the pages shared with other processes.
	Private, WorkingSet uint64
	Threads, Handles    uint32
	// ReadBytes and WriteBytes are the process's cumulative I/O, to files, devices
	// and the network.
	ReadBytes, WriteBytes uint64
	// HasWindow is set when the process has a window on the taskbar.
	HasWindow bool
}

// Snapshot is the state of the machine at one sample, with the rates since the
// sample before it.
type Snapshot struct {
	Time time.Time
	// CPU is the load of all processors, from 0 to 100; Cores is the load of each.
	CPU   float64
	Cores []float64
	Mem   Memory
	// NetIn and NetOut are bytes per second; IORead and IOWrite are the processes'
	// I/O in bytes per second.
	NetIn, NetOut   float64
	IORead, IOWrite float64
	Procs           []Process
}

// Threads returns the number of threads of every process.
func (s Snapshot) Threads() (n int) {
	for _, p := range s.Procs {
		n += int(p.Threads)
	}
	return n
}

// Handles returns the number of handles of every process.
func (s Snapshot) Handles() (n int) {
	for _, p := range s.Procs {
		n += int(p.Handles)
	}
	return n
}

// Process is one process at a sample.
type Process struct {
	// Key identifies the process across samples: its PID and when it started.
	Key       string
	PID, PPID uint32
	// Name is the executable's file name; Program says what runs it (see
	// Source.Describe).
	Name string
	Program
	Started time.Time
	// Created is when the process started as a Windows file time, which End takes.
	Created int64
	// CPU is the share of all processors the process used since the last sample,
	// from 0 to 100, scaled as the total is, so the processes add up to it.
	CPU                 float64
	Private, WorkingSet uint64
	Threads, Handles    uint32
	// IORead and IOWrite are bytes per second.
	IORead, IOWrite float64
	// HasWindow is set when the process has a window on the taskbar, which makes
	// its program an app, as Task Manager counts them.
	HasWindow bool
}

// Program is what a process runs: its executable's full path, the program's name
// from the executable's version information, as Task Manager shows it ("Helium"
// for a chrome.exe that names itself so), and whether it is part of Windows. The
// path and the name are "" when Windows does not give them.
type Program struct {
	Path, Description string
	// Windows is set for a part of Windows: a process with no executable, such as
	// System, or one whose executable is in the Windows folder and published by
	// Microsoft. Drivers' programs, which Windows keeps in its folder too, are
	// not.
	Windows bool
}

// Label returns the name to show for the process: the program's name, or the
// executable's when the program gives none.
func (p Process) Label() string {
	if p.Description != "" {
		return p.Description
	}
	return p.Name
}

// ProcessKey returns the key of the process with pid that started at created.
func ProcessKey(pid uint32, created int64) string {
	return strconv.FormatUint(uint64(pid), 10) + ":" + strconv.FormatInt(created, 36)
}

// Source reads the machine's counters.
type Source interface {
	Read() (Raw, error)
	// Describe returns the program of the process with pid.
	Describe(pid uint32) Program
	// End ends the process with pid, if it is still the one that started at created.
	End(pid uint32, created int64) error
}

// ErrGone is the error of End for a process that has already exited.
var ErrGone = errors.New("the process has exited")

// Sampler turns readings of a Source into snapshots.
type Sampler struct {
	src  Source
	prev *Raw
	// programs holds each process's program, by key, so a process is described
	// once, when it first appears.
	programs map[string]Program
}

// NewSampler returns a sampler that reads src.
func NewSampler(src Source) *Sampler { return &Sampler{src: src, programs: map[string]Program{}} }

// Source returns the source the sampler reads.
func (s *Sampler) Source() Source { return s.src }

// Next reads the counters and returns the snapshot. The first snapshot has no
// rates, since there is no reading before it to measure from.
func (s *Sampler) Next() (Snapshot, error) {
	raw, err := s.src.Read()
	if err != nil {
		return Snapshot{}, err
	}
	snap := Rates(s.prev, raw)
	s.prev = &raw
	seen := make(map[string]bool, len(snap.Procs))
	for i := range snap.Procs {
		p := &snap.Procs[i]
		seen[p.Key] = true
		prog, ok := s.programs[p.Key]
		if !ok {
			prog = s.src.Describe(p.PID)
			s.programs[p.Key] = prog
		}
		p.Program = prog
	}
	for k := range s.programs {
		if !seen[k] {
			delete(s.programs, k)
		}
	}
	return snap, nil
}

// Rates returns the snapshot of cur, with its rates measured from prev. A nil prev
// gives zero rates.
func Rates(prev *Raw, cur Raw) Snapshot {
	snap := Snapshot{Time: cur.Time, Mem: cur.Mem, Cores: make([]float64, len(cur.Cores))}
	var secs float64
	var total int64 // the processor time that passed, summed over the processors
	if prev != nil {
		secs = cur.Time.Sub(prev.Time).Seconds()
		var busy int64
		for i, c := range cur.Cores {
			if i >= len(prev.Cores) {
				break
			}
			p := prev.Cores[i]
			all := (c.Kernel - p.Kernel) + (c.User - p.User)
			idle := c.Idle - p.Idle
			if all > 0 {
				snap.Cores[i] = clamp(float64(all-idle) / float64(all) * 100)
			}
			total += all
			busy += all - idle
		}
		if total > 0 {
			snap.CPU = clamp(float64(busy) / float64(total) * 100)
		}
		if secs > 0 {
			snap.NetIn = perSecond(prev.NetIn, cur.NetIn, secs)
			snap.NetOut = perSecond(prev.NetOut, cur.NetOut, secs)
		}
	}
	// With the utility, the load is Task Manager's, and the processes' shares of the
	// busy time are scaled to it.
	scale := 1.0
	if cur.HasUtility && prev != nil {
		if snap.CPU > 0 {
			scale = clamp(cur.Utility) / snap.CPU
		}
		snap.CPU = clamp(cur.Utility)
		if len(cur.CoreUtility) == len(snap.Cores) {
			for i, v := range cur.CoreUtility {
				snap.Cores[i] = clamp(v)
			}
		}
	}
	var before map[string]*RawProc
	if prev != nil {
		before = make(map[string]*RawProc, len(prev.Procs))
		for i := range prev.Procs {
			p := &prev.Procs[i]
			before[ProcessKey(p.PID, p.Created)] = p
		}
	}
	snap.Procs = make([]Process, 0, len(cur.Procs))
	for _, r := range cur.Procs {
		p := Process{Key: ProcessKey(r.PID, r.Created), PID: r.PID, PPID: r.PPID, Name: r.Name,
			Started: fileTime(r.Created), Created: r.Created, Private: r.Private, WorkingSet: r.WorkingSet,
			Threads: r.Threads, Handles: r.Handles, HasWindow: r.HasWindow}
		if b := before[p.Key]; b != nil {
			if total > 0 && r.CPU > b.CPU {
				p.CPU = clamp(float64(r.CPU-b.CPU) / float64(total) * 100 * scale)
			}
			if secs > 0 {
				p.IORead = perSecond(b.ReadBytes, r.ReadBytes, secs)
				p.IOWrite = perSecond(b.WriteBytes, r.WriteBytes, secs)
			}
		}
		snap.IORead += p.IORead
		snap.IOWrite += p.IOWrite
		snap.Procs = append(snap.Procs, p)
	}
	slices.SortFunc(snap.Procs, func(a, b Process) int { return int(a.PID) - int(b.PID) })
	return snap
}

// perSecond is the rate of a counter that went from a to b in secs seconds. A
// counter that went back, as one does when an interface goes away, counts as zero.
func perSecond(a, b uint64, secs float64) float64 {
	if b < a {
		return 0
	}
	return float64(b-a) / secs
}

func clamp(v float64) float64 { return max(0, min(100, v)) }

// epochDiff is the time from 1601 to 1970, in 100-nanosecond units.
const epochDiff = 116444736000000000

// fileTime converts a Windows file time, 100-nanosecond intervals since 1601, to a
// time. Zero is the zero time.
func fileTime(ft int64) time.Time {
	if ft == 0 {
		return time.Time{}
	}
	return time.Unix(0, (ft-epochDiff)*100)
}
