//go:build windows

package windows

import "testing"

// TestPinterfaceGUIDMatchesTheSDK checks the parameterized IID computation against
// IReference<Int32>, whose IID is 548CEFBD-BC8A-5FA0-8DF2-957440FC8BF4 in the
// Windows Runtime, and whose signature is built from IReference's generic ID
// (61C17706-2D65-11E0-9AE8-D48564015472, in cppwinrt's windows.foundation.0.h)
// and Int32's "i4". It runs without COM.
func TestPinterfaceGUIDMatchesTheSDK(t *testing.T) {
	want := guid{0x548CEFBD, 0xBC8A, 0x5FA0, [8]byte{0x8D, 0xF2, 0x95, 0x74, 0x40, 0xFC, 0x8B, 0xF4}}
	got := pinterfaceGUID("pinterface({61c17706-2d65-11e0-9ae8-d48564015472};i4)")
	if got != want {
		t.Fatalf("pinterfaceGUID(IReference<Int32>) = %v, want %v", got, want)
	}
}

// TestEventIIDsAreVersion5 checks that the delegate IIDs carry the version and
// variant bits of a name-based UUID, so a change to the signature shows up here.
func TestEventIIDsAreVersion5(t *testing.T) {
	for i, g := range eventIIDs {
		if v := g.Data3 >> 12; v != 5 {
			t.Errorf("eventIIDs[%d] has version %d, want 5", i, v)
		}
		if v := g.Data4[0] >> 6; v != 2 {
			t.Errorf("eventIIDs[%d] has variant bits %d, want 2 (RFC 4122)", i, v)
		}
	}
	if eventIIDs[eventActivated] == eventIIDs[eventFailed] {
		t.Error("the Activated and Failed delegates have the same IID")
	}
}
