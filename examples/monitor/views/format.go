package views

import (
	"strconv"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/monitor/sample"
)

// percent shows a load, such as "12.3%", or "0%" for none.
func percent(v float64) string {
	switch {
	case v <= 0:
		return "0%"
	case v < 10:
		return ui.FormatNumber(v, 1) + "%"
	}
	return ui.FormatNumber(v, 0) + "%"
}

// rate shows bytes per second, such as "1.23 MB/s".
func rate(v float64) string { return ui.FormatBytes(uint64(max(v, 0))) + "/s" }

// count shows a whole number with the locale's group separators.
func count(n int) string { return ui.FormatNumber(float64(n), 0) }

// interval names a time between samples, such as "1 s" or "0.5 s".
func interval(d time.Duration) string {
	return ui.FormatNumber(d.Seconds(), decimalsOf(d)) + " s"
}

func decimalsOf(d time.Duration) int {
	if d%time.Second == 0 {
		return 0
	}
	return 1
}

// matches reports whether process p matches every word of the filter: a word is
// part of its executable's or its program's name, ignoring case, or the start of
// its PID.
func matches(filter string, p sample.Process) bool {
	id := strconv.FormatUint(uint64(p.PID), 10)
	name, desc := strings.ToLower(p.Name), strings.ToLower(p.Description)
	for w := range strings.FieldsSeq(strings.ToLower(filter)) {
		if !strings.Contains(name, w) && !strings.Contains(desc, w) && !strings.HasPrefix(id, w) {
			return false
		}
	}
	return true
}
