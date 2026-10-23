package views

import (
	"strconv"
	"strings"
)

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
