package main

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"text/template"
)

// appxManifestTemplate is AppxManifest.xml for a full-trust desktop app: the
// Windows.FullTrustApplication entry point, the runFullTrust capability, and
// the logos that Start and the taskbar use. With notificationProtocol set, it also
// declares the app's protocol, so that a click on a notification can start the
// packaged app. Values are escaped with html.
//
// The package layout is the app's executable at the root, AppxManifest.xml
// beside it, and the logos under Assets.
var appxManifestTemplate = template.Must(template.New("AppxManifest.xml").Parse(`<?xml version="1.0" encoding="utf-8"?>
<Package xmlns="http://schemas.microsoft.com/appx/manifest/foundation/windows10"
  xmlns:uap="http://schemas.microsoft.com/appx/manifest/uap/windows10"
  xmlns:rescap="http://schemas.microsoft.com/appx/manifest/foundation/windows10/restrictedcapabilities"
  IgnorableNamespaces="uap rescap">
  <Identity Name="{{.IdentityName | html}}" Publisher="{{.PublisherSubject | html}}" Version="{{.Version | html}}" ProcessorArchitecture="{{.Architecture | html}}" />
  <Properties>
    <DisplayName>{{.DisplayName | html}}</DisplayName>
    <PublisherDisplayName>{{.Publisher | html}}</PublisherDisplayName>
    <Logo>Assets\StoreLogo.png</Logo>
    <Description>{{.Description | html}}</Description>
  </Properties>
  <Dependencies>
    <TargetDeviceFamily Name="Windows.Desktop" MinVersion="10.0.17763.0" MaxVersionTested="10.0.26100.0" />
  </Dependencies>
  <Resources>
    <Resource Language="en-us" />
  </Resources>
  <Applications>
    <Application Id="App" Executable="{{.Executable | html}}" EntryPoint="Windows.FullTrustApplication">
      <uap:VisualElements DisplayName="{{.DisplayName | html}}" Description="{{.Description | html}}" BackgroundColor="transparent" Square150x150Logo="Assets\Square150x150Logo.png" Square44x44Logo="Assets\Square44x44Logo.png" />
      {{- if .Protocol}}
      <Extensions>
        <uap:Extension Category="windows.protocol">
          <uap:Protocol Name="{{.Protocol | html}}" />
        </uap:Extension>
      </Extensions>
      {{- end}}
    </Application>
  </Applications>
  <Capabilities>
    <rescap:Capability Name="runFullTrust" />
  </Capabilities>
</Package>
`))

// appxFields are the values filled into appxManifestTemplate.
type appxFields struct {
	IdentityName     string
	PublisherSubject string
	Version          string
	Architecture     string
	DisplayName      string
	Publisher        string
	Description      string
	Executable       string
	Protocol         string // the protocol the app declares; empty for none
}

// appxManifest returns AppxManifest.xml for the app. winVersion is the
// four-part version from windowsVersion, and arch is amd64 or arm64.
func appxManifest(cfg *config, winVersion, arch string) ([]byte, error) {
	protocol := ""
	if cfg.NotificationProtocol {
		s, err := protocolScheme(cfg.IdentityName)
		if err != nil {
			return nil, err
		}
		protocol = s
	}
	var buf bytes.Buffer
	err := appxManifestTemplate.Execute(&buf, appxFields{
		IdentityName:     cfg.IdentityName,
		PublisherSubject: cfg.PublisherSubject,
		Version:          winVersion,
		Architecture:     processorArchitecture(arch),
		DisplayName:      cfg.DisplayName,
		Publisher:        cfg.Publisher,
		Description:      cfg.Description,
		Executable:       cfg.Name + ".exe",
		Protocol:         protocol,
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// processorArchitecture maps a Go architecture name to the MSIX name.
func processorArchitecture(arch string) string {
	if arch == "arm64" {
		return "arm64"
	}
	return "x64"
}

// buildMSIX completes the package layout with its manifest and logos, and packs
// it into out with makeappx. The layout already holds the app's executable.
func buildMSIX(makeappx, layout, out string, cfg *config, icon image.Image, winVersion, arch string) error {
	manifest, err := appxManifest(cfg, winVersion, arch)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(layout, "AppxManifest.xml"), manifest, 0o644); err != nil {
		return err
	}
	if err := writeLogos(filepath.Join(layout, "Assets"), icon); err != nil {
		return err
	}
	if err := runTool(makeappx, "pack", "/d", layout, "/p", out, "/o"); err != nil {
		return fmt.Errorf("packing the MSIX: %w", err)
	}
	return nil
}
