package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// uploadToasts returns how many toasts named Upload the window shows, and the
// description of the last one.
func uploadToasts(r *demoRig) (int, string) {
	n, desc := 0, ""
	r.tree().Walk(func(nd *access.Node) bool {
		if nd.Role == access.RoleGroup && nd.Name == "Upload" {
			n++
			desc = nd.Description
		}
		return true
	})
	return n, desc
}

// clickButton clicks the button named name, and runs a frame.
func clickButton(r *demoRig, t *testing.T, name string) {
	t.Helper()
	r.bridge.Perform(accesstest.Find(t, r.tree(), access.RoleButton, name).ID, access.ActionClick)
	r.frame()
}

// The upload toast is one toast: each step updates it in place, and the last step
// says it is uploaded. The steps are timed by the window's clock, which starts at
// the real time, so the test's frames start there too. Its frames run the steps
// quickly, so the test checks the end of the upload, not each step.
func TestUploadToastIsUpdatedInPlaceToUploaded(t *testing.T) {
	r := newDemoRig(t)
	r.now = time.Now()
	showComponents(r)
	clickButton(r, t, "Upload")
	done := false
	for range 12 {
		n, desc := uploadToasts(r)
		if n > 1 {
			t.Fatalf("%d toasts named Upload: an update must replace the toast", n)
		}
		if desc == "Uploaded" {
			done = true
			break
		}
		r.frame()
	}
	if !done {
		t.Error("the upload toast never said it was uploaded")
	}
	if len(r.warnings) != 0 {
		t.Errorf("development warnings: %v", r.warnings)
	}
}

// Dismiss removes the upload toast, and the upload stops: no later step shows it.
func TestUploadDismissRemovesTheToastAndStopsTheUpload(t *testing.T) {
	r := newDemoRig(t)
	r.now = time.Now()
	showComponents(r)
	clickButton(r, t, "Upload")
	if n, _ := uploadToasts(r); n != 1 {
		t.Fatalf("%d upload toasts after Upload, want one", n)
	}
	clickButton(r, t, "Dismiss")
	if n, _ := uploadToasts(r); n != 0 {
		t.Fatalf("%d upload toasts after Dismiss, want none", n)
	}
	for range 6 {
		r.frame()
	}
	if n, _ := uploadToasts(r); n != 0 {
		t.Errorf("%d upload toasts after the upload was dismissed, want none", n)
	}
}
