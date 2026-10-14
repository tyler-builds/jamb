package main

import (
	"strings"
	"testing"
)

const (
	nsPackage = "http://schemas.microsoft.com/appx/manifest/foundation/windows10"
	nsUAP     = "http://schemas.microsoft.com/appx/manifest/uap/windows10"
	nsRescap  = "http://schemas.microsoft.com/appx/manifest/foundation/windows10/restrictedcapabilities"
)

func TestAppxManifest(t *testing.T) {
	cfg := validConfig()
	// Characters that must be escaped in both attributes and text.
	cfg.DisplayName = `Tom & "Jerry" <demo>`
	data, err := appxManifest(&cfg, "0.1.0.0", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	nodes := parseXML(t, data)

	pkg := findNode(t, nodes, "Package")
	if pkg.space != nsPackage {
		t.Errorf("Package namespace = %q", pkg.space)
	}
	id := findNode(t, nodes, "Identity")
	if id.attrs["Name"] != "JambExamples.Demo" || id.attrs["Publisher"] != "CN=Jamb Examples" ||
		id.attrs["Version"] != "0.1.0.0" || id.attrs["ProcessorArchitecture"] != "x64" {
		t.Errorf("Identity = %v", id.attrs)
	}
	if n := findNode(t, nodes, "DisplayName"); n.text != cfg.DisplayName {
		t.Errorf("DisplayName text = %q; want %q", n.text, cfg.DisplayName)
	}
	if n := findNode(t, nodes, "PublisherDisplayName"); n.text != "Jamb Examples" {
		t.Errorf("PublisherDisplayName = %q", n.text)
	}
	if n := findNode(t, nodes, "Logo"); n.text != `Assets\StoreLogo.png` {
		t.Errorf("Logo = %q", n.text)
	}
	if n := findNode(t, nodes, "TargetDeviceFamily"); n.attrs["Name"] != "Windows.Desktop" || n.attrs["MinVersion"] != "10.0.17763.0" {
		t.Errorf("TargetDeviceFamily = %v", n.attrs)
	}

	app := findNode(t, nodes, "Application")
	if app.attrs["Executable"] != "jamb-demo.exe" || app.attrs["EntryPoint"] != "Windows.FullTrustApplication" {
		t.Errorf("Application = %v", app.attrs)
	}
	ve := findNode(t, nodes, "VisualElements")
	if ve.space != nsUAP {
		t.Errorf("VisualElements namespace = %q; want uap", ve.space)
	}
	if ve.attrs["DisplayName"] != cfg.DisplayName {
		t.Errorf("VisualElements DisplayName = %q; want %q", ve.attrs["DisplayName"], cfg.DisplayName)
	}
	if ve.attrs["Square150x150Logo"] != `Assets\Square150x150Logo.png` || ve.attrs["Square44x44Logo"] != `Assets\Square44x44Logo.png` {
		t.Errorf("VisualElements logos = %v", ve.attrs)
	}

	capability := findNode(t, nodes, "Capability")
	if capability.space != nsRescap || capability.attrs["Name"] != "runFullTrust" {
		t.Errorf("Capability = %v in %q; want rescap runFullTrust", capability.attrs, capability.space)
	}
	if !strings.Contains(string(data), `IgnorableNamespaces="uap rescap"`) {
		t.Error("uap and rescap are not declared as ignorable")
	}
}

func TestAppxManifestProtocolOnlyWhenSet(t *testing.T) {
	cfg := validConfig()
	off, err := appxManifest(&cfg, "0.1.0.0", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(off), "windows.protocol") {
		t.Error("the manifest declares a protocol without notificationProtocol")
	}

	cfg.NotificationProtocol = true
	data, err := appxManifest(&cfg, "0.1.0.0", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	nodes := parseXML(t, data)
	ext := findNode(t, nodes, "Extension")
	if ext.space != nsUAP || ext.attrs["Category"] != "windows.protocol" {
		t.Errorf("Extension = %v in namespace %q; want uap windows.protocol", ext.attrs, ext.space)
	}
	if p := findNode(t, nodes, "Protocol"); p.space != nsUAP || p.attrs["Name"] != "jamb-jambexamples.demo" {
		t.Errorf("Protocol = %v in namespace %q; want the scheme of identityName", p.attrs, p.space)
	}
	if app := findNode(t, nodes, "Application"); app.attrs["Executable"] != "jamb-demo.exe" {
		t.Errorf("Application = %v; the protocol must not change the application", app.attrs)
	}
}

func TestAppxManifestArm64(t *testing.T) {
	cfg := validConfig()
	data, err := appxManifest(&cfg, "0.1.0.0", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if id := findNode(t, parseXML(t, data), "Identity"); id.attrs["ProcessorArchitecture"] != "arm64" {
		t.Errorf("ProcessorArchitecture = %q; want arm64", id.attrs["ProcessorArchitecture"])
	}
}
