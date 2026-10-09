package main

import (
	"bytes"
	"encoding/xml"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

// xmlNode is one element of a parsed document.
type xmlNode struct {
	space string
	name  string
	attrs map[string]string
	text  string
}

// parseXML parses data and fails the test if it is not well-formed. It returns
// the elements in document order; text is the element's character data.
func parseXML(t *testing.T, data []byte) []xmlNode {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(data))
	var nodes []xmlNode
	var open []int
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("not well-formed XML: %v\n%s", err, data)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			n := xmlNode{space: el.Name.Space, name: el.Name.Local, attrs: map[string]string{}}
			for _, a := range el.Attr {
				n.attrs[a.Name.Local] = a.Value
			}
			nodes = append(nodes, n)
			open = append(open, len(nodes)-1)
		case xml.EndElement:
			open = open[:len(open)-1]
		case xml.CharData:
			if len(open) > 0 {
				i := open[len(open)-1]
				nodes[i].text += string(el)
			}
		}
	}
	for i := range nodes {
		nodes[i].text = strings.TrimSpace(nodes[i].text)
	}
	return nodes
}

// findNode returns the first element with the local name, or fails the test.
func findNode(t *testing.T, nodes []xmlNode, name string) xmlNode {
	t.Helper()
	for _, n := range nodes {
		if n.name == name {
			return n
		}
	}
	t.Fatalf("no <%s> element", name)
	return xmlNode{}
}

func TestManifestResource(t *testing.T) {
	cfg := validConfig()
	rs, err := buildResources(&cfg, image.NewNRGBA(image.Rect(0, 0, 256, 256)), "0.1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	data := rs.Get(winres.RT_MANIFEST, winres.ID(1), winres.LCIDDefault)
	if data == nil {
		t.Fatal("no application manifest resource")
	}
	nodes := parseXML(t, data)

	if id := findNode(t, nodes, "assemblyIdentity"); id.attrs["name"] != "Microsoft.Windows.Common-Controls" || id.attrs["version"] != "6.0.0.0" {
		t.Errorf("common controls dependency = %v; want version 6.0.0.0", id.attrs)
	}
	if sos := findNode(t, nodes, "supportedOS"); sos.attrs["Id"] != "{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}" {
		t.Errorf("supportedOS Id = %q; want the Windows 10 and 11 GUID", sos.attrs["Id"])
	}
	if d := findNode(t, nodes, "dpiAwareness"); d.text != "permonitorv2,system" {
		t.Errorf("dpiAwareness = %q; want per-monitor v2 (as EnablePerMonitorDPI sets at run time)", d.text)
	}
	if d := findNode(t, nodes, "dpiAware"); d.text != "true" {
		t.Errorf("dpiAware = %q; want true", d.text)
	}
	if l := findNode(t, nodes, "longPathAware"); l.text != "true" {
		t.Errorf("longPathAware = %q; want true", l.text)
	}
	if e := findNode(t, nodes, "requestedExecutionLevel"); e.attrs["level"] != "asInvoker" {
		t.Errorf("requestedExecutionLevel = %q; want asInvoker", e.attrs["level"])
	}
}

func TestVersionInfoResource(t *testing.T) {
	cfg := validConfig()
	cfg.Copyright = "Copyright 2026 The jamb Authors"
	rs, err := buildResources(&cfg, image.NewNRGBA(image.Rect(0, 0, 256, 256)), "0.1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	vi, err := version.FromBytes(rs.Get(winres.RT_VERSION, winres.ID(1), version.LangDefault))
	if err != nil {
		t.Fatalf("version resource: %v", err)
	}
	if vi.FileVersion != [4]uint16{0, 1, 0, 0} || vi.ProductVersion != [4]uint16{0, 1, 0, 0} {
		t.Errorf("file version %v, product version %v; want 0.1.0.0", vi.FileVersion, vi.ProductVersion)
	}
	strs := vi.Table()[version.LangDefault]
	if strs == nil {
		t.Fatal("no en-US string table")
	}
	want := map[string]string{
		version.CompanyName:      "Jamb Examples",
		version.FileDescription:  cfg.Description,
		version.InternalName:     "jamb-demo",
		version.LegalCopyright:   "Copyright 2026 The jamb Authors",
		version.OriginalFilename: "jamb-demo.exe",
		version.ProductName:      "Jamb Demo",
	}
	for key, value := range want {
		if got := (*strs)[key]; got != value {
			t.Errorf("%s = %q; want %q", key, got, value)
		}
	}
}

func TestWriteSyso(t *testing.T) {
	cfg := validConfig()
	rs, err := buildResources(&cfg, image.NewNRGBA(image.Rect(0, 0, 256, 256)), "0.1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	for arch, machine := range map[string][2]byte{"amd64": {0x64, 0x86}, "arm64": {0x64, 0xAA}} {
		path := filepath.Join(t.TempDir(), sysoName(arch))
		if err := writeSyso(path, rs, arch); err != nil {
			t.Fatalf("writeSyso(%s): %v", arch, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		// The COFF header starts with the machine type, little-endian.
		if len(data) < 2 || data[0] != machine[0] || data[1] != machine[1] {
			t.Errorf("%s: machine bytes % x; want % x", arch, data[:2], machine)
		}
	}
	if got := sysoName("amd64"); got != "jamb_package_windows_amd64.syso" {
		t.Errorf("sysoName(amd64) = %q", got)
	}
}
