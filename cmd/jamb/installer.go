package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// installerTemplateSrc is the NSIS script that jamb package fills in for each
// app. See installer.nsi.tmpl.
//
//go:embed installer.nsi.tmpl
var installerTemplateSrc string

var installerTemplate = template.Must(template.New("installer.nsi").Funcs(template.FuncMap{
	"nsis": nsisString,
}).Parse(installerTemplateSrc))

// installerFile is a file the installer copies beside the executable. Path is
// its location, and Name its base name in the install folder.
type installerFile struct {
	Path string
	Name string
}

// installerFields are the values filled into installerTemplate.
type installerFields struct {
	DisplayName   string
	Description   string
	Publisher     string
	Copyright     string
	Version       string
	WinVersion    string
	Executable    string
	ExePath       string
	Files         []installerFile
	Icon          string
	OutFile       string
	UninstallKey  string
	EstimatedSize int64
}

// nsisString escapes s for an NSIS double-quoted string. A dollar sign starts an
// NSIS variable, and a double quote ends the string, so each is escaped.
func nsisString(s string) string {
	s = strings.ReplaceAll(s, "$", "$$")
	return strings.ReplaceAll(s, `"`, `$\"`)
}

// installerScript returns the NSIS script for the app. The install folder and
// shortcuts take their names from displayName, so a name that Windows cannot
// use as a folder name is an error.
func installerScript(f installerFields) ([]byte, error) {
	if strings.ContainsAny(f.DisplayName, `\/:*?"<>|`) {
		return nil, fmt.Errorf(`"displayName" %q is used as a folder name, so it cannot contain \ / : * ? " < > |`, f.DisplayName)
	}
	var buf bytes.Buffer
	if err := installerTemplate.Execute(&buf, f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// findMakensis returns the path of makensis, the NSIS compiler. An explicit path
// must exist. Otherwise makensis is searched for on PATH and in the NSIS folder
// of the Program Files directories, where the NSIS installer puts it by default.
func findMakensis(path string) (string, error) {
	if path != "" {
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("--makensis: %w", err)
		}
		return path, nil
	}
	if p, err := exec.LookPath("makensis"); err == nil {
		return p, nil
	}
	dirs := []string{programFilesX86()}
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		dirs = append(dirs, pf)
	}
	for _, dir := range dirs {
		p := filepath.Join(dir, "NSIS", "makensis.exe")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New(`makensis not found. Install NSIS with "winget install NSIS.NSIS", or "scoop install nsis", or from https://nsis.sourceforge.io/Download; or pass --makensis PATH`)
}

// buildInstaller writes the NSIS script and the icon into dir, and compiles the
// installer to out with makensis. The installer copies the executable and the
// LICENSE and NOTICE files of appDir, the same files as the zip.
func buildInstaller(makensis, dir, out, appDir, exe string, cfg *config, icon image.Image, winVersion string) error {
	licenses, err := licenseFiles(appDir)
	if err != nil {
		return err
	}
	files := make([]installerFile, 0, len(licenses))
	size, err := fileSize(exe)
	if err != nil {
		return err
	}
	for _, l := range licenses {
		files = append(files, installerFile{Path: l, Name: filepath.Base(l)})
		n, err := fileSize(l)
		if err != nil {
			return err
		}
		size += n
	}

	iconPath := filepath.Join(dir, "installer.ico")
	if err := writeICO(iconPath, icon); err != nil {
		return err
	}
	script, err := installerScript(installerFields{
		DisplayName:   cfg.DisplayName,
		Description:   cfg.Description,
		Publisher:     cfg.Publisher,
		Copyright:     cfg.Copyright,
		Version:       cfg.Version,
		WinVersion:    winVersion,
		Executable:    cfg.Name + ".exe",
		ExePath:       exe,
		Files:         files,
		Icon:          iconPath,
		OutFile:       out,
		UninstallKey:  `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + cfg.IdentityName,
		EstimatedSize: (size + 1023) / 1024,
	})
	if err != nil {
		return err
	}
	scriptPath := filepath.Join(dir, "installer.nsi")
	if err := os.WriteFile(scriptPath, script, 0o644); err != nil {
		return err
	}
	pkgf("compiling the installer with %s", filepath.Base(makensis))
	return runTool(makensis, "-V2", scriptPath)
}

// fileSize returns the size of the file at path in bytes.
func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
