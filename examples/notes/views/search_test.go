package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/scene"
	"github.com/tyler-builds/jamb/ui"
)

// Searching highlights the matches in the note list rows: the Languages row
// gets a background quad in the match color.
func TestSearchHighlightsMatchesInNoteRows(t *testing.T) {
	r := newRig(t)
	soft := scene.Color(matchHighlight(ui.DarkTheme()).Background)
	count := func() int {
		n := 0
		for _, q := range r.rt.Frame(time.Now()).Quads {
			if q.Background == soft {
				n++
			}
		}
		return n
	}
	base := count()
	r.s.Query.Set("lang")
	r.frame()
	if got := count(); got <= base {
		t.Fatalf("searching for lang painted %d highlight quads, want more than the %d before", got, base)
	}
}
