package main

import (
	"archive/zip"
	"debug/pe"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

// makeApp writes a complete application directory: jamb.json, a 256 px icon,
// a go.mod and a main.go with the given source.
func makeApp(t *testing.T, dir, mainSrc string) {
	t.Helper()
	cfg := validConfig()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		configFile: string(data),
		"go.mod":   "module example.com/app\n\ngo 1.27.1\n",
		"main.go":  mainSrc,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeSolidPNG(t, filepath.Join(dir, "icon.png"), 256, 256)
}

func TestPackageRemovesSysoAfterBuildFailure(t *testing.T) {
	dir := t.TempDir()
	makeApp(t, dir, "package main\n\nfunc main() { undefined() }\n")

	if err := packageApp(packageOptions{dir: dir, arch: "amd64", noMSIX: true}); err == nil {
		t.Fatal("packageApp succeeded on a program that does not compile")
	}
	if _, err := os.Stat(filepath.Join(dir, sysoName("amd64"))); !os.IsNotExist(err) {
		t.Errorf("the resource object survives a failed build: %v", err)
	}
	zips, _ := filepath.Glob(filepath.Join(dir, "dist", "*.zip"))
	if len(zips) != 0 {
		t.Errorf("a failed build wrote %v", zips)
	}
}

func TestPackageKeepsAnExistingSyso(t *testing.T) {
	dir := t.TempDir()
	makeApp(t, dir, "package main\n\nfunc main() {}\n")
	mine := filepath.Join(dir, sysoName("amd64"))
	if err := os.WriteFile(mine, []byte("the app's own resources"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := packageApp(packageOptions{dir: dir, arch: "amd64", noMSIX: true}); err == nil {
		t.Fatal("packageApp overwrote a .syso it did not write")
	}
	data, err := os.ReadFile(mine)
	if err != nil || string(data) != "the app's own resources" {
		t.Errorf("the app's own .syso was changed or removed: %q, %v", data, err)
	}
}

func TestPackageZipHoldsAGUIExecutableWithResources(t *testing.T) {
	if testing.Short() {
		t.Skip("builds an executable")
	}
	dir := t.TempDir()
	makeApp(t, dir, "package main\n\nfunc main() {}\n")
	if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte("license text"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "NOTICE.txt"), []byte("notice text"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Unsigned, so no MSIX is written even though the SDK may be installed.
	if err := packageApp(packageOptions{dir: dir, arch: "amd64"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, sysoName("amd64"))); !os.IsNotExist(err) {
		t.Errorf("the resource object is left in the app directory: %v", err)
	}
	if msix, _ := filepath.Glob(filepath.Join(dir, "dist", "*.msix")); len(msix) != 0 {
		t.Errorf("an unsigned build wrote %v", msix)
	}

	zipPath := filepath.Join(dir, "dist", "jamb-demo-0.1.0-windows-amd64.zip")
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var names []string
	var exeFile *zip.File
	for _, f := range zr.File {
		names = append(names, f.Name)
		if f.Name == "jamb-demo.exe" {
			exeFile = f
		}
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "LICENSE,NOTICE.txt,jamb-demo.exe" {
		t.Fatalf("zip entries = %v", names)
	}

	exe := filepath.Join(t.TempDir(), "jamb-demo.exe")
	src, err := exeFile.Open()
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.Create(exe)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(out, src)
	src.Close()
	out.Close()
	if err != nil {
		t.Fatal(err)
	}

	f, err := pe.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	opt, ok := f.OptionalHeader.(*pe.OptionalHeader64)
	f.Close()
	if !ok || opt.Subsystem != pe.IMAGE_SUBSYSTEM_WINDOWS_GUI {
		t.Errorf("subsystem is not Windows GUI, so a console window would open")
	}

	r, err := os.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	rs, err := winres.LoadFromEXE(r)
	if err != nil {
		t.Fatal(err)
	}
	if m := rs.Get(winres.RT_MANIFEST, winres.ID(1), winres.LCIDDefault); !strings.Contains(string(m), "permonitorv2,system") {
		t.Error("the executable has no per-monitor DPI manifest")
	}
	vi, err := version.FromBytes(rs.Get(winres.RT_VERSION, winres.ID(1), version.LangDefault))
	if err != nil || vi.FileVersion != [4]uint16{0, 1, 0, 0} {
		t.Errorf("version resource: %v, %v", vi, err)
	}
	if icon, err := rs.GetIcon(winres.ID(1)); err != nil || icon == nil {
		t.Errorf("the executable has no icon: %v", err)
	}
}

func TestPackageMSIXWithTestCertificate(t *testing.T) {
	if testing.Short() {
		t.Skip("builds an executable and runs the Windows SDK")
	}
	makeappx, err := findSDKTool("makeappx.exe", "")
	if err != nil {
		t.Skip(err)
	}
	if _, err := findSDKTool("signtool.exe", ""); err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	makeApp(t, dir, "package main\n\nfunc main() {}\n")

	if err := packageApp(packageOptions{dir: dir, arch: "amd64", testCert: true}); err != nil {
		t.Fatal(err)
	}
	dist := filepath.Join(dir, "dist")
	msix := filepath.Join(dist, "jamb-demo-0.1.0-windows-amd64.msix")
	for _, p := range []string{msix, filepath.Join(dist, "jamb-demo-0.1.0-windows-amd64.zip"), filepath.Join(dist, "jamb-demo-test.pfx"), filepath.Join(dist, "jamb-demo-test.cer")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing output: %v", err)
		}
	}

	unpacked := t.TempDir()
	if out, err := exec.Command(makeappx, "unpack", "/p", msix, "/d", unpacked, "/o").CombinedOutput(); err != nil {
		t.Fatalf("makeappx unpack: %v\n%s", err, out)
	}
	for _, p := range []string{"AppxManifest.xml", "AppxSignature.p7x", "jamb-demo.exe", filepath.Join("Assets", "Square44x44Logo.scale-200.png")} {
		if _, err := os.Stat(filepath.Join(unpacked, p)); err != nil {
			t.Errorf("package lacks %s: %v", p, err)
		}
	}
}

func TestPackageCmdArguments(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--arch", "386"}, "--arch must be amd64 or arm64"},
		{[]string{"--cert", "a.pfx", "--test-cert"}, "cannot be used together"},
		{[]string{"--timestamp", "http://t.example"}, "--timestamp needs"},
		{[]string{t.TempDir(), "extra"}, `unexpected argument "extra"`},
	} {
		err := packageCmd(tc.args)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("packageCmd(%q) = %v; want %q", tc.args, err, tc.want)
		}
	}
	if err := packageCmd([]string{"-h"}); err != nil {
		t.Errorf("-h = %v; want no error", err)
	}
}

func TestCleanerRunsOnceInReverseOrder(t *testing.T) {
	var c cleaner
	var order []string
	c.add(func() { order = append(order, "first added") })
	c.add(func() { order = append(order, "second added") })
	c.run()
	c.run()
	if strings.Join(order, ",") != "second added,first added" {
		t.Errorf("order = %v", order)
	}
}
