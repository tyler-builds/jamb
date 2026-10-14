//go:build windows

package windows

import (
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/tyler-builds/jamb/internal/win32"
)

// TestMenuFlags checks the MF_ flags of each kind of item.
func TestMenuFlags(t *testing.T) {
	for name, tc := range map[string]struct {
		item MenuItem
		want uint32
	}{
		"plain":            {MenuItem{Label: "Show"}, win32.MF_STRING},
		"disabled":         {MenuItem{Label: "Show", Disabled: true}, win32.MF_STRING | win32.MF_GRAYED},
		"checked":          {MenuItem{Label: "Show", Checked: true}, win32.MF_STRING | win32.MF_CHECKED},
		"disabled checked": {MenuItem{Label: "Show", Disabled: true, Checked: true}, win32.MF_STRING | win32.MF_GRAYED | win32.MF_CHECKED},
		"separator":        {MenuItem{Separator: true}, win32.MF_SEPARATOR},
	} {
		if got := menuFlags(tc.item); got != tc.want {
			t.Errorf("%s: menuFlags = %#x; want %#x", name, got, tc.want)
		}
	}
}

// TestMenuText checks that an '&' is doubled, so a label shows it as itself.
func TestMenuText(t *testing.T) {
	for in, want := range map[string]string{
		"Show window": "Show window",
		"Save & exit": "Save && exit",
		"a&&b":        "a&&&&b",
		"":            "",
	} {
		if got := menuText(in); got != want {
			t.Errorf("menuText(%q) = %q; want %q", in, got, want)
		}
	}
}

// TestTipUnits checks the tooltip's units: at most 127, with a terminating NUL,
// and never half of a surrogate pair.
func TestTipUnits(t *testing.T) {
	tip := tipUnits("Notes")
	if got := tipString(tip[:]); got != "Notes" {
		t.Errorf("tipUnits(Notes) = %q", got)
	}
	long := tipUnits(strings.Repeat("a", 200))
	if long[126] != 'a' || long[127] != 0 {
		t.Errorf("a 200-character tooltip: unit 126 = %#x, unit 127 = %#x; want 'a' and 0", long[126], long[127])
	}
	// U+10400 is a surrogate pair; cut after the first unit, it is dropped whole.
	pair := tipUnits(strings.Repeat("a", 126) + "\U00010400")
	if pair[126] != 0 {
		t.Errorf("a pair cut at the limit: unit 126 = %#x; want 0, the pair dropped", pair[126])
	}
}

// tipString converts the NUL-terminated UTF-16 units of a tooltip to a string.
func tipString(u []uint16) string {
	n := 0
	for n < len(u) && u[n] != 0 {
		n++
	}
	return string(utf16.Decode(u[:n]))
}

// TestGUIDMemory checks that the first three fields of an RFC 4122 UUID are
// turned little-endian, as the GUID structure holds them.
func TestGUIDMemory(t *testing.T) {
	// 886313e1-3b8a-5372-9b90-0c9aee199e5d
	u := [16]byte{0x88, 0x63, 0x13, 0xe1, 0x3b, 0x8a, 0x53, 0x72, 0x9b, 0x90, 0x0c, 0x9a, 0xee, 0x19, 0x9e, 0x5d}
	want := [16]byte{0xe1, 0x13, 0x63, 0x88, 0x8a, 0x3b, 0x72, 0x53, 0x9b, 0x90, 0x0c, 0x9a, 0xee, 0x19, 0x9e, 0x5d}
	if got := guidMemory(u); got != want {
		t.Errorf("guidMemory = % x; want % x", got, want)
	}
}

// The icon asks for the standard tooltip: under NOTIFYICON_VERSION_4 the shell shows the tip
// text only when NIF_SHOWTIP is set as well as NIF_TIP.
func TestTrayDataShowsTheTooltip(t *testing.T) {
	ic := &TrayIcon{}
	ic.opts.Tooltip = "Jamb Demo"
	nid := ic.data()
	if want := uint32(win32.NIF_TIP | win32.NIF_SHOWTIP); nid.UFlags&want != want {
		t.Errorf("flags %#x lack NIF_TIP|NIF_SHOWTIP", nid.UFlags)
	}
}
