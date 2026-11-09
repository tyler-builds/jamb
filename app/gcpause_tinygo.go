//go:build tinygo

package app

import "runtime"

// gcPauseTotal returns 0: TinyGo's MemStats has no pause total.
func gcPauseTotal(*runtime.MemStats) uint64 { return 0 }
