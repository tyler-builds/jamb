package main

import (
	"image"
	"os"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

// sysoName is the resource object jamb package writes into the app's directory
// for one architecture. The name is specific to this tool, so a file the app
// keeps for itself is never overwritten.
func sysoName(arch string) string {
	return "jamb_package_windows_" + arch + ".syso"
}

// buildResources returns the resources linked into the app: the icon, the
// VERSIONINFO that Windows shows in a file's Details tab, and the application
// manifest.
func buildResources(cfg *config, icon image.Image, winVersion string) (*winres.ResourceSet, error) {
	ico, err := buildIcon(icon)
	if err != nil {
		return nil, err
	}
	rs := &winres.ResourceSet{}
	if err := rs.SetIcon(winres.ID(1), ico); err != nil {
		return nil, err
	}

	vi := version.Info{}
	fields := []struct{ key, value string }{
		{version.CompanyName, cfg.Publisher},
		{version.FileDescription, cfg.Description},
		{version.InternalName, cfg.Name},
		{version.LegalCopyright, cfg.Copyright},
		{version.OriginalFilename, cfg.Name + ".exe"},
		{version.ProductName, cfg.DisplayName},
	}
	for _, f := range fields {
		if f.value == "" {
			continue
		}
		if err := vi.Set(version.LangDefault, f.key, f.value); err != nil {
			return nil, err
		}
	}
	vi.SetFileVersion(winVersion)
	vi.SetProductVersion(winVersion)
	rs.SetVersionInfo(vi)

	// The manifest matches what the framework does at run time: per-monitor
	// v2 DPI awareness (see platform/windows, EnablePerMonitorDPI), the
	// Common Controls v6 library that TaskDialog needs, Windows 10 and 11 as
	// the supported OS, and long path names.
	rs.SetManifest(winres.AppManifest{
		Description:         cfg.Description,
		Compatibility:       winres.Win10AndAbove,
		ExecutionLevel:      winres.AsInvoker,
		DPIAwareness:        winres.DPIPerMonitorV2,
		LongPathAware:       true,
		UseCommonControlsV6: true,
	})
	return rs, nil
}

// buildIcon returns the .ico image set of src, with each size of icoSizes.
func buildIcon(src image.Image) (*winres.Icon, error) {
	return winres.NewIconFromImages(iconImages(src))
}

// writeICO writes src as an .ico file at path, for the installer's icon.
func writeICO(path string, src image.Image) (err error) {
	ico, err := buildIcon(src)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	return ico.SaveICO(f)
}

// writeSyso writes rs as a COFF object for arch. The go tool links a .syso
// file in the package directory into the next build.
func writeSyso(path string, rs *winres.ResourceSet, arch string) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	return rs.WriteObject(f, winres.Arch(arch))
}
