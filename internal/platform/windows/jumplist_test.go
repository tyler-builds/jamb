//go:build windows

package windows

import (
	"fmt"
	"runtime"
	"testing"
	"unsafe"
)

// guidString formats g as the registry form of a GUID, in upper case.
func guidString(g guid) string {
	return fmt.Sprintf("%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X",
		g.Data1, g.Data2, g.Data3, g.Data4[0], g.Data4[1], g.Data4[2], g.Data4[3],
		g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7])
}

// TestJumpListGUIDs checks the identifiers against their SDK strings, from the
// uuid attributes of shobjidl_core.h and propsys.h.
func TestJumpListGUIDs(t *testing.T) {
	for _, tc := range []struct {
		name string
		g    guid
		want string
	}{
		{"CLSID_DestinationList", clsidDestinationList, "77F10CF0-3DB5-4966-B520-B7C54FD35ED6"},
		{"IID_ICustomDestinationList", iidCustomDestinationList, "6332DEBF-87B5-4670-90C0-5E57B408A49E"},
		{"CLSID_EnumerableObjectCollection", clsidEnumerableObjectCollection, "2D3468C1-36A7-43B6-AC24-D3F02FD9607A"},
		{"IID_IObjectArray", iidObjectArray, "92CA9DCD-5622-4BBA-A805-5E9F541BD8C9"},
		{"IID_IObjectCollection", iidObjectCollection, "5632B1A4-E38A-400A-928A-D4CD63230295"},
		{"CLSID_ShellLink", clsidShellLink, "00021401-0000-0000-C000-000000000046"},
		{"IID_IShellLinkW", iidShellLinkW, "000214F9-0000-0000-C000-000000000046"},
		{"IID_IPropertyStore", iidPropertyStore, "886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99"},
		{"PKEY_Title format", pkeyTitle.fmtid, "F29F85E0-4FF9-1068-AB91-08002B27B3D9"},
	} {
		if got := guidString(tc.g); got != tc.want {
			t.Errorf("%s = %s; want %s", tc.name, got, tc.want)
		}
	}
	if pkeyTitle.pid != 2 {
		t.Errorf("PKEY_Title pid = %d; want 2", pkeyTitle.pid)
	}
}

// TestPropVariantSize checks the layout of PROPVARIANT, which is 24 bytes on 64-bit
// Windows.
func TestPropVariantSize(t *testing.T) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("the PROPVARIANT size is checked for 64-bit Windows only")
	}
	if got := unsafe.Sizeof(propVariant{}); got != 24 {
		t.Errorf("sizeof(PROPVARIANT) = %d; want 24", got)
	}
}
