//go:build windows

package win32

import (
	"testing"
	"unsafe"
)

// TestShellStructLayout checks the sizes and offsets of the structs against the
// 64-bit layouts of the Windows SDK. It calls nothing in the shell.
func TestShellStructLayout(t *testing.T) {
	var nid NOTIFYICONDATAW
	for name, tc := range map[string]struct{ got, want uintptr }{
		"NOTIFYICONDATAW size":  {unsafe.Sizeof(nid), 976},
		"hIcon offset":          {unsafe.Offsetof(nid.HIcon), 32},
		"szTip offset":          {unsafe.Offsetof(nid.SzTip), 40},
		"uVersion offset":       {unsafe.Offsetof(nid.UVersion), 816},
		"guidItem offset":       {unsafe.Offsetof(nid.GuidItem), 952},
		"hBalloonIcon offset":   {unsafe.Offsetof(nid.HBalloonIcon), 968},
		"ICONINFO size":         {unsafe.Sizeof(ICONINFO{}), 32},
		"BITMAPINFOHEADER size": {unsafe.Sizeof(BITMAPINFOHEADER{}), 40},
		"SHFILEINFOW size":      {unsafe.Sizeof(SHFILEINFOW{}), 696},
		"BITMAP size":           {unsafe.Sizeof(BITMAP{}), 32},
		"NIN_KEYSELECT":         {uintptr(NIN_KEYSELECT), 0x401},
		"NOTIFYICON_VERSION_4":  {NOTIFYICON_VERSION_4, 4},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %d; want %d", name, tc.got, tc.want)
		}
	}
}
