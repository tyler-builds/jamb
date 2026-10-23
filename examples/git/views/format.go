package views

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/ui"
)

// ago describes t relative to now, as "5 min ago" or "3 days ago". Past a
// year it gives the date.
func ago(t, now time.Time) string {
	d := now.Sub(t)
	plural := func(n int, unit string) string {
		if n == 1 {
			return fmt.Sprintf("1 %s ago", unit)
		}
		return fmt.Sprintf("%d %ss ago", n, unit)
	}
	switch {
	case t.IsZero():
		return ""
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return plural(int(d.Hours()), "hour")
	case d < 30*24*time.Hour:
		return plural(int(d.Hours()/24), "day")
	case d < 365*24*time.Hour:
		return plural(int(d.Hours()/24/30), "month")
	}
	return ui.FormatDate(t)
}

// stamp is the full date and time of t, for tooltips and the detail pane.
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return ui.FormatDate(t) + " " + ui.FormatTime(t)
}

// initials are the first letters of a name's first and last words, as
// ui.Avatar draws them, for the graph's nodes.
func initials(name string) string {
	words := strings.Fields(name)
	first := func(w string) string {
		r, _ := utf8.DecodeRuneInString(w)
		return strings.ToUpper(string(r))
	}
	switch len(words) {
	case 0:
		return ""
	case 1:
		return first(words[0])
	}
	return first(words[0]) + first(words[len(words)-1])
}

// splitPath splits a slash path into its folder (with the trailing slash) and its base name.
func splitPath(p string) (dir, base string) {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i+1], p[i+1:]
	}
	return "", p
}

// matches reports whether the commit fields contain every word of the query,
// ignoring case. A hash matches by its prefix.
func matches(query, hash, subject, author string) bool {
	for w := range strings.FieldsSeq(strings.ToLower(query)) {
		if !strings.HasPrefix(hash, w) && !strings.Contains(strings.ToLower(subject), w) &&
			!strings.Contains(strings.ToLower(author), w) {
			return false
		}
	}
	return true
}
func itoa(n int) string { return strconv.Itoa(n) }
