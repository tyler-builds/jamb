//go:build windows

package windows

import (
	"crypto/sha1"
	"encoding/binary"
	"testing"
)

// pinterfaceNamespace is the namespace of the parameterized interface IDs of the
// Windows Runtime, 11F47AD5-7B73-42C0-ABAE-878B1E16ADEE, in its byte order.
var pinterfaceNamespace = [16]byte{0x11, 0xF4, 0x7A, 0xD5, 0x7B, 0x73, 0x42, 0xC0, 0xAB, 0xAE, 0x87, 0x8B, 0x1E, 0x16, 0xAD, 0xEE}

// pinterfaceGUID returns the IID of a parameterized interface or delegate from its
// signature, as the Windows Runtime defines it: a version 5 UUID over the namespace
// and the signature's bytes, as cppwinrt's base.h computes it (generate_guid). It is
// test code only, so the package links no crypto package.
func pinterfaceGUID(signature string) guid {
	h := sha1.New()
	h.Write(pinterfaceNamespace[:])
	h.Write([]byte(signature))
	sum := h.Sum(nil)
	var b [16]byte
	copy(b[:], sum)
	b[6] = b[6]&0x0F | 0x50 // version 5
	b[8] = b[8]&0x3F | 0x80 // RFC 4122 variant
	var g guid
	g.Data1 = binary.BigEndian.Uint32(b[0:4])
	g.Data2 = binary.BigEndian.Uint16(b[4:6])
	g.Data3 = binary.BigEndian.Uint16(b[6:8])
	copy(g.Data4[:], b[8:16])
	return g
}

// The written-out delegate IIDs are the ones their signatures give.
func TestEventIIDsMatchTheirSignatures(t *testing.T) {
	sigs := [2]string{
		"pinterface({9de1c534-6ae1-11e0-84e1-18a905bcc53f};" + toastSignature + ";cinterface(IInspectable))",
		"pinterface({9de1c534-6ae1-11e0-84e1-18a905bcc53f};" + toastSignature + ";{35176862-cfd4-44f8-ad64-f500fd896c3b})",
	}
	for i, sig := range sigs {
		if got := pinterfaceGUID(sig); got != eventIIDs[i] {
			t.Errorf("eventIIDs[%d] = %v, but its signature gives %v", i, eventIIDs[i], got)
		}
	}
}

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
