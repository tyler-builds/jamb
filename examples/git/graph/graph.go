// Package graph lays out a commit history as lanes: each commit gets a column,
// and each row lists the lines that cross it, so a list can draw the graph one
// row at a time and the rows join up.
//
// A lane keeps its column for as long as it is in use, so a line that only
// passes a row is straight. A lane that ends is free, and the next branch that
// needs a column takes the leftmost free one.
package graph

import "slices"

// Part is the piece of a row that a line crosses.
type Part uint8

const (
	// Upper runs from the top edge of the row, in lane From, to the row's node.
	Upper Part = iota
	// Lower runs from the row's node to the bottom edge, in lane To.
	Lower
	// Through runs from the top edge to the bottom edge in lane From, past the node.
	Through
)

// Line is one line of a row. Color is the lane whose color it takes.
type Line struct {
	Part     Part
	From, To int
	Color    int
}

// Row is the graph's slice for one commit.
type Row struct {
	// Lane is the column of the commit's node.
	Lane int
	// Lanes is how many columns the row uses, its node and every line.
	Lanes int
	// Merge is true for a commit with more than one parent.
	Merge bool
	Lines []Line
}

// Commit is what the layout needs to know of a commit.
type Commit struct {
	Hash    string
	Parents []string
}

// Builder lays out a history a page at a time: the rows of a page continue the
// lanes of the page before it. The zero value is ready to use.
type Builder struct {
	// lanes holds, for each column, the hash of the commit the lane waits for,
	// or "" for a free column.
	lanes []string
	// most is the widest row so far.
	most int
}

// Widest is the most columns any row has used so far.
func (b *Builder) Widest() int { return b.most }

// Add lays out the next commits, which must come in topological order (every
// commit before its parents), and returns one row for each.
func (b *Builder) Add(commits []Commit) []Row {
	rows := make([]Row, 0, len(commits))
	for _, c := range commits {
		rows = append(rows, b.add(c))
	}
	return rows
}

func (b *Builder) add(c Commit) Row {
	row := Row{Merge: len(c.Parents) > 1}
	lane := slices.Index(b.lanes, c.Hash)
	if lane < 0 {
		// A commit no lane waits for is the tip of a branch: it starts a lane.
		lane = b.free()
	}
	row.Lane = lane
	width := len(b.lanes)
	for j, h := range b.lanes {
		switch h {
		case "":
		case c.Hash:
			// Every lane waiting for this commit ends at its node.
			row.Lines = append(row.Lines, Line{Part: Upper, From: j, To: lane, Color: j})
			b.lanes[j] = ""
		default:
			row.Lines = append(row.Lines, Line{Part: Through, From: j, To: j, Color: j})
		}
	}
	for i, p := range c.Parents {
		// A lane already waiting for the parent is joined, except that the first
		// parent keeps this lane when the other is to its right, so a branch's own
		// line stays left of what was merged into it. Two lanes waiting for one
		// commit meet at its node.
		if t := slices.Index(b.lanes, p); t >= 0 && (i > 0 || t < lane) {
			row.Lines = append(row.Lines, Line{Part: Lower, From: lane, To: t, Color: t})
			continue
		}
		t := lane
		if i > 0 {
			t = b.free()
		}
		b.lanes[t] = p
		row.Lines = append(row.Lines, Line{Part: Lower, From: lane, To: t, Color: t})
	}
	width = max(width, len(b.lanes))
	for len(b.lanes) > 0 && b.lanes[len(b.lanes)-1] == "" {
		b.lanes = b.lanes[:len(b.lanes)-1]
	}
	row.Lanes = width
	b.most = max(b.most, width)
	return row
}

// free returns the leftmost free column, adding one at the right when none is.
func (b *Builder) free() int {
	if i := slices.Index(b.lanes, ""); i >= 0 {
		return i
	}
	b.lanes = append(b.lanes, "")
	return len(b.lanes) - 1
}
