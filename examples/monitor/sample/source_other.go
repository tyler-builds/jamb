//go:build !windows

package sample

import "time"

// Machine returns a made-up machine: the reader of the machine's counters is
// written for Windows only.
func Machine() Source { return NewSynthetic(time.Now(), time.Second) }
