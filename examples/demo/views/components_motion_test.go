package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// The motion samples of the Components page show their labels, and the delayed
// entrance of the delay sample finishes, which runs its OnDone. The runtime times its
// delays on the wall clock, so the frames here start from it.
func TestMotionSamplesShowAndDelayedEntranceFinishes(t *testing.T) {
	r := newDemoRig(t)
	r.now = time.Now()
	showComponents(r)
	tr := r.tree()
	for _, label := range []string{
		"Spring: Move, then Move back", "Keyframes: Play", "Timeline: Sequence, Then, Parallel",
		"Repeat: a timeline that loops", "Stagger: siblings enter one after another", "Delay and OnDone",
	} {
		accesstest.Find(t, tr, access.RoleStaticText, label)
	}
	for i := 0; i < 10 && !hasText(r, "Finished: the tile is in place."); i++ {
		r.frame()
	}
	if !hasText(r, "Finished: the tile is in place.") {
		t.Fatal("the delayed entrance does not finish, so its OnDone does not run")
	}
	if len(r.warnings) > 0 {
		t.Errorf("development warnings: %v", r.warnings)
	}
}
