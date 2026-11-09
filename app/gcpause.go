//go:build !tinygo

package app

import "runtime"

// gcPauseTotal returns the total GC pause time, which TinyGo's MemStats lacks.
func gcPauseTotal(m *runtime.MemStats) uint64 { return m.PauseTotalNs }
