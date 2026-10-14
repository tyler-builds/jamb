//go:build windows

package windows

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// TestMapKeyPunctuation checks that the OEM virtual keys of the US layout map to the punctuation keys
// named after the characters they print there, with Shift held or not.
func TestMapKeyPunctuation(t *testing.T) {
	for _, tc := range []struct {
		name string
		vk   uintptr
		want input.Key
	}{
		{"VK_OEM_1", 0xBA, input.KeySemicolon},
		{"VK_OEM_PLUS", 0xBB, input.KeyEqual},
		{"VK_OEM_COMMA", 0xBC, input.KeyComma},
		{"VK_OEM_MINUS", 0xBD, input.KeyMinus},
		{"VK_OEM_PERIOD", 0xBE, input.KeyPeriod},
		{"VK_OEM_2", 0xBF, input.KeySlash},
		{"VK_OEM_3", 0xC0, input.KeyBacktick},
		{"VK_OEM_4", 0xDB, input.KeyLeftBracket},
		{"VK_OEM_5", 0xDC, input.KeyBackslash},
		{"VK_OEM_6", 0xDD, input.KeyRightBracket},
		{"VK_OEM_7", 0xDE, input.KeyApostrophe},
	} {
		if got := mapKey(tc.vk); got != tc.want {
			t.Errorf("mapKey(%s, %#x) = %d, want %d", tc.name, tc.vk, got, tc.want)
		}
	}
}
