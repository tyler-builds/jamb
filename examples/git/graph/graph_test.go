package graph

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

func commits(spec ...string) []Commit {
	// Each spec is "hash parent parent...".
	var cs []Commit
	for _, s := range spec {
		var c Commit
		var fields []string
		for f := range splitFields(s) {
			fields = append(fields, f)
		}
		c.Hash, c.Parents = fields[0], fields[1:]
		cs = append(cs, c)
	}
	return cs
}

func splitFields(s string) func(func(string) bool) {
	return func(yield func(string) bool) {
		start := -1
		for i := 0; i <= len(s); i++ {
			if i == len(s) || s[i] == ' ' {
				if start >= 0 && !yield(s[start:i]) {
					return
				}
				start = -1
			} else if start < 0 {
				start = i
			}
		}
	}
}

func TestLinearHistoryStaysInOneLane(t *testing.T) {
	var b Builder
	rows := b.Add(commits("c b", "b a", "a"))
	for i, r := range rows {
		if r.Lane != 0 || r.Lanes != 1 {
			t.Errorf("row %d: lane %d of %d, want lane 0 of 1", i, r.Lane, r.Lanes)
		}
	}
	if len(rows[0].Lines) != 1 || rows[0].Lines[0] != (Line{Part: Lower, From: 0, To: 0}) {
		t.Errorf("tip lines = %v, want one line down", rows[0].Lines)
	}
	if len(rows[2].Lines) != 1 || rows[2].Lines[0].Part != Upper {
		t.Errorf("root lines = %v, want one line in from above", rows[2].Lines)
	}
}

func TestBranchAndMerge(t *testing.T) {
	// m merges f into main; f and b both come from a.
	var b Builder
	rows := b.Add(commits("m b f", "f a", "b a", "a"))
	if !rows[0].Merge {
		t.Error("the merge row is not marked as a merge")
	}
	if rows[1].Lane != 1 {
		t.Errorf("the feature commit is in lane %d, want 1", rows[1].Lane)
	}
	if rows[2].Lane != 0 {
		t.Errorf("the main commit is in lane %d, want 0", rows[2].Lane)
	}
	// Both lanes wait for a, so both end at its node.
	var upper int
	for _, l := range rows[3].Lines {
		if l.Part == Upper {
			upper++
		}
	}
	if upper != 2 || rows[3].Lane != 0 {
		t.Errorf("root: %d lines end at its node in lane %d, want 2 in lane 0", upper, rows[3].Lane)
	}
	if b.Widest() != 2 {
		t.Errorf("Widest = %d, want 2", b.Widest())
	}
}

func TestFreedLaneIsReused(t *testing.T) {
	// x is a second tip whose branch ends at a; y is a third tip after it.
	var b Builder
	rows := b.Add(commits("c b", "x a", "b a", "a", "y"))
	if rows[4].Lane != 0 {
		t.Errorf("a tip after every lane ended is in lane %d, want 0", rows[4].Lane)
	}
}

// edges returns the lanes a row's lines touch at its top and bottom edges.
func edges(r Row) (top, bottom []int) {
	for _, l := range r.Lines {
		switch l.Part {
		case Upper:
			top = append(top, l.From)
		case Lower:
			bottom = append(bottom, l.To)
		case Through:
			top = append(top, l.From)
			bottom = append(bottom, l.To)
		}
	}
	slices.Sort(top)
	slices.Sort(bottom)
	return slices.Compact(top), slices.Compact(bottom)
}

// The lines leaving the bottom of a row are the lines entering the top of the
// next, for any history and however it is paged.
func TestRowsJoinUp(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for trial := range 50 {
		// A random history: each commit's parents are later commits.
		n := 200
		cs := make([]Commit, n)
		for i := range cs {
			cs[i].Hash = fmt.Sprint(i)
		}
		for i := range n - 1 {
			parents := 1
			if r.IntN(6) == 0 {
				parents = 2
			}
			if r.IntN(25) == 0 {
				parents = 0
			}
			for range parents {
				p := i + 1 + r.IntN(min(8, n-i-1))
				if !slices.Contains(cs[i].Parents, cs[p].Hash) {
					cs[i].Parents = append(cs[i].Parents, cs[p].Hash)
				}
			}
		}
		var b Builder
		var rows []Row
		for start := 0; start < n; start += 37 {
			rows = append(rows, b.Add(cs[start:min(start+37, n)])...)
		}
		for i := 1; i < len(rows); i++ {
			_, below := edges(rows[i-1])
			above, _ := edges(rows[i])
			if !slices.Equal(below, above) {
				t.Fatalf("trial %d: row %d leaves at %v, row %d enters at %v", trial, i-1, below, i, above)
			}
		}
		_, last := edges(rows[len(rows)-1])
		if len(last) != 0 {
			t.Fatalf("trial %d: lines leave the last row at %v", trial, last)
		}
		for i, row := range rows {
			for _, l := range row.Lines {
				if max(l.From, l.To) >= row.Lanes || row.Lane >= row.Lanes {
					t.Fatalf("trial %d: row %d has a line %v outside its %d lanes", trial, i, l, row.Lanes)
				}
			}
		}
	}
}
