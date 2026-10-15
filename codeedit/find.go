package codeedit

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// FindOptions are the options of a search: whether it matches case, whole words only,
// and whether the query is a regular expression (in Go's syntax, RE2).
type FindOptions struct {
	MatchCase bool
	WholeWord bool
	Regex     bool
}

// A Match is a range of the text that a Finder found, in bytes. For a regular
// expression it also keeps the submatches, which Replace expands with $1 or ${name}.
type Match struct {
	From, To int
	groups   []int
}

// A Finder searches a Document for a query. It keeps the matches of the last text it
// searched, and searches again only when the text changes. A Finder is made for one
// query and options; a new query makes a new Finder.
type Finder struct {
	re      *regexp.Regexp
	word    bool
	ver     uint64
	matches []Match
	valid   bool
}

// NewFinder returns a Finder for query. It returns an error if the query is an invalid
// regular expression. An empty query finds nothing.
func NewFinder(query string, opt FindOptions) (*Finder, error) {
	f := &Finder{word: opt.WholeWord}
	if query == "" {
		return f, nil
	}
	pat := query
	if !opt.Regex {
		pat = regexp.QuoteMeta(query)
	}
	if !opt.MatchCase {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, err
	}
	f.re = re
	return f, nil
}

// Matches returns the matches of the query in d, in order. A match that is empty is not
// a match. The result is kept until the text of d changes, and must not be changed.
func (f *Finder) Matches(d *Document) []Match {
	if f.re == nil {
		return nil
	}
	if f.valid && f.ver == d.textVer {
		return f.matches
	}
	text := d.Text()
	f.matches = f.matches[:0]
	for _, g := range f.re.FindAllStringSubmatchIndex(text, -1) {
		from, to := g[0], g[1]
		if from == to {
			continue
		}
		if f.word && !wholeWord(text, from, to) {
			continue
		}
		f.matches = append(f.matches, Match{From: from, To: to, groups: g})
	}
	f.ver, f.valid = d.textVer, true
	return f.matches
}

// wholeWord reports whether the text [from, to) is not next to a word character on either
// side.
func wholeWord(text string, from, to int) bool {
	if from > 0 {
		r, _ := utf8.DecodeLastRuneInString(text[:from])
		if charClass(r) == 1 {
			return false
		}
	}
	if to < len(text) {
		r, _ := utf8.DecodeRuneInString(text[to:])
		if charClass(r) == 1 {
			return false
		}
	}
	return true
}

// Index returns the index of the first match that starts at or after p, and reports
// whether there is one. The matches are those that Matches returns.
func (f *Finder) Index(matches []Match, p int) (int, bool) {
	i := sort.Search(len(matches), func(i int) bool { return matches[i].From >= p })
	return i, i < len(matches)
}

// Replacement returns the text that replaces match m when the replacement is repl. In a
// regular expression search, repl may refer to the groups as $1 or ${name}, as Go's
// regexp.Expand does; otherwise it is used as it is.
func (f *Finder) Replacement(d *Document, m Match, repl string) string {
	if f.re == nil || len(m.groups) == 0 || !strings.ContainsRune(repl, '$') {
		return repl
	}
	text := d.Text()
	return string(f.re.ExpandString(nil, repl, text, m.groups))
}

// ReplaceMatches replaces the given matches with repl, as one step of the undo history.
// The matches must be in order and must not overlap, as Matches returns them. It returns
// the number of matches replaced.
func (d *Document) ReplaceMatches(f *Finder, ms []Match, repl string) (int, error) {
	if len(ms) == 0 {
		return 0, nil
	}
	changes := make([]Change, len(ms))
	for i, m := range ms {
		changes[i] = Change{From: m.From, To: m.To, Text: f.Replacement(d, m, repl)}
	}
	if err := d.Edit(changes...); err != nil {
		return 0, err
	}
	return len(ms), nil
}
