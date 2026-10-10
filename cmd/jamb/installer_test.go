package main

import (
	"bytes"
	"debug/pe"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// sampleInstallerFields are the values of validConfig, as the installer uses them.
func sampleInstallerFields() installerFields {
	cfg := validConfig()
	cfg.Copyright = "Copyright 2026 The jamb Authors"
	return installerFields{
		DisplayName:   cfg.DisplayName,
		Description:   cfg.Description,
		Publisher:     cfg.Publisher,
		Copyright:     cfg.Copyright,
		Version:       cfg.Version,
		WinVersion:    "0.1.0.0",
		Executable:    cfg.Name + ".exe",
		ExePath:       `C:\build\layout\jamb-demo.exe`,
		Files:         []installerFile{{Path: `C:\app\LICENSE`, Name: "LICENSE"}},
		Icon:          `C:\build\installer.ico`,
		OutFile:       `C:\dist\jamb-demo-0.1.0-windows-amd64-setup.exe`,
		UninstallKey:  `Software\Microsoft\Windows\CurrentVersion\Uninstall\JambExamples.Demo`,
		EstimatedSize: 12,
	}
}

func TestInstallerScriptIsWellFormed(t *testing.T) {
	data, err := installerScript(sampleInstallerFields())
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	if strings.Contains(script, "{{") || strings.Contains(script, "}}") {
		t.Error("the template left a placeholder unfilled")
	}
	// Every string is closed on its line. An escaped quote, $\", is not a delimiter.
	escaped := regexp.MustCompile(`\$\\"`)
	for i, line := range strings.Split(script, "\n") {
		if n := strings.Count(escaped.ReplaceAllString(line, ""), `"`); n%2 != 0 {
			t.Errorf("line %d has an unclosed string: %s", i+1, line)
		}
	}
	if n := strings.Count(script, "\nSection "); n != 3 {
		t.Errorf("sections = %d; want the main, desktop and uninstall sections", n)
	}
	if n := strings.Count(script, "SectionEnd"); n != 3 {
		t.Errorf("SectionEnd = %d; want 3", n)
	}
	for _, want := range []string{
		"RequestExecutionLevel user",
		`InstallDir "$LOCALAPPDATA\Programs\Jamb Demo"`,
		`Section /o "Desktop shortcut"`,
		`CreateShortcut "$SMPROGRAMS\Jamb Demo\Jamb Demo.lnk" "$INSTDIR\jamb-demo.exe"`,
		`WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\JambExamples.Demo" "DisplayName" "Jamb Demo"`,
		`WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\JambExamples.Demo" "DisplayVersion" "0.1.0"`,
		`WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\JambExamples.Demo" "Publisher" "Jamb Examples"`,
		`"UninstallString" "$\"$INSTDIR\uninstall.exe$\""`,
		`WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\JambExamples.Demo" "EstimatedSize" 12`,
		`DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\JambExamples.Demo"`,
		`VIProductVersion "0.1.0.0"`,
		`VIAddVersionKey /LANG=1033 "LegalCopyright" "Copyright 2026 The jamb Authors"`,
		`Delete "$INSTDIR\LICENSE"`,
		`File "C:\app\LICENSE"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script lacks %s", want)
		}
	}
	if strings.Contains(script, "HKLM") || strings.Contains(script, "RequestExecutionLevel admin") {
		t.Error("the per-user installer refers to HKLM or asks for administrator rights")
	}
}

func TestInstallerOmitsCopyrightWhenUnset(t *testing.T) {
	f := sampleInstallerFields()
	f.Copyright = ""
	data, err := installerScript(f)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "LegalCopyright") {
		t.Error("an empty copyright is written as a version key")
	}
}

func TestInstallerEscapesNSISCharacters(t *testing.T) {
	f := sampleInstallerFields()
	f.Description = `Shows $HOME and "quotes"`
	data, err := installerScript(f)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"Comments" "Shows $$HOME and $\"quotes$\""`; !strings.Contains(string(data), want) {
		t.Errorf("escaped description missing; want %s", want)
	}
}

func TestInstallerRefusesDisplayNameThatIsNotAFolderName(t *testing.T) {
	for _, name := range []string{`Jamb/Demo`, `Jamb: Demo`, `Jamb "Demo"`} {
		f := sampleInstallerFields()
		f.DisplayName = name
		if _, err := installerScript(f); err == nil || !strings.Contains(err.Error(), "folder name") {
			t.Errorf("displayName %q: err = %v; want a folder-name error", name, err)
		}
	}
}

func TestFindMakensisExplicitPathMustExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "makensis.exe")
	if _, err := findMakensis(missing); err == nil || !strings.Contains(err.Error(), "--makensis") {
		t.Errorf("err = %v; want the missing --makensis path reported", err)
	}
	present := filepath.Join(t.TempDir(), "makensis.exe")
	if err := os.WriteFile(present, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if p, err := findMakensis(present); err != nil || p != present {
		t.Errorf("findMakensis = %q, %v; want the given path", p, err)
	}
}

func TestFindMakensisMissingSaysHowToInstall(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("ProgramFiles", t.TempDir())
	t.Setenv("ProgramFiles(x86)", t.TempDir())
	_, err := findMakensis("")
	if err == nil || !strings.Contains(err.Error(), "winget install NSIS.NSIS") || !strings.Contains(err.Error(), "--makensis") {
		t.Errorf("err = %v; want the install hints", err)
	}
}

func TestWriteICOHasTheIconSizes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.ico")
	src := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	if err := writeICO(path, src); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// ICONDIR: reserved 0, type 1 (icon), then the number of images.
	if len(data) < 6 || !bytes.Equal(data[:4], []byte{0, 0, 1, 0}) || int(data[4])|int(data[5])<<8 != len(icoSizes) {
		t.Errorf("header = % x; want an icon with %d images", data[:min(6, len(data))], len(icoSizes))
	}
}

// TestInstallerCompilesWithMakensis builds a setup.exe when NSIS is installed. It is
// skipped otherwise. The executable is a placeholder, since the installer only copies it.
func TestInstallerCompilesWithMakensis(t *testing.T) {
	makensis, err := findMakensis("")
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	stage := filepath.Join(dir, "stage")
	for _, d := range []string{appDir, stage} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(stage, "jamb-demo.exe")
	if err := os.WriteFile(exe, []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "LICENSE"), []byte("license text"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "setup.exe")
	cfg := validConfig()
	icon := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	if err := buildInstaller(makensis, stage, out, appDir, exe, &cfg, icon, "0.1.0.0"); err != nil {
		t.Fatal(err)
	}
	f, err := pe.Open(out)
	if err != nil {
		t.Fatalf("setup.exe is not a PE file: %v", err)
	}
	f.Close()
}
