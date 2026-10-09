package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
)

// packageOptions are the flags and the directory of jamb package.
type packageOptions struct {
	dir       string
	arch      string
	cert      string
	testCert  bool
	timestamp string
	noMSIX    bool
	sdkBin    string
}

// packageCmd parses the arguments of jamb package and builds the release.
//
//	jamb package [flags] [package dir]
func packageCmd(args []string) error {
	var o packageOptions
	fs := flag.NewFlagSet("jamb package", flag.ContinueOnError)
	fs.StringVar(&o.arch, "arch", "amd64", "target architecture: amd64 or arm64")
	fs.StringVar(&o.cert, "cert", "", "sign with this PFX file; its password is read from the "+passwordEnv+" environment variable")
	fs.BoolVar(&o.testCert, "test-cert", false, "sign with a self-signed test certificate kept in dist (for install tests on this PC only)")
	fs.StringVar(&o.timestamp, "timestamp", "", "RFC 3161 timestamp server URL for the signatures")
	fs.BoolVar(&o.noMSIX, "no-msix", false, "build the zip only")
	fs.StringVar(&o.sdkBin, "sdk-bin", "", "Windows SDK bin directory with makeappx.exe and signtool.exe (default: the newest SDK under Windows Kits)")
	if err := fs.Parse(args); err != nil {
		return helpOrError(err)
	}
	// The directory may come before or after the flags.
	o.dir = "."
	if fs.NArg() > 0 {
		o.dir = fs.Arg(0)
		if err := fs.Parse(fs.Args()[1:]); err != nil {
			return helpOrError(err)
		}
		if fs.NArg() > 0 {
			return fmt.Errorf("unexpected argument %q", fs.Arg(0))
		}
	}

	if o.arch != "amd64" && o.arch != "arm64" {
		return fmt.Errorf("--arch must be amd64 or arm64, not %q", o.arch)
	}
	if o.cert != "" && o.testCert {
		return errors.New("--cert and --test-cert cannot be used together")
	}
	if o.timestamp != "" && o.cert == "" && !o.testCert {
		return errors.New("--timestamp needs --cert or --test-cert")
	}
	abs, err := filepath.Abs(o.dir)
	if err != nil {
		return err
	}
	o.dir = abs
	return packageApp(o)
}

// helpOrError treats a request for help as success, since flag has already
// printed the usage.
func helpOrError(err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}

// cleaner runs the removals of one packaging run: the staging directory, the
// resource object in the app's directory, and the temporary signing PFX. It
// also runs them on Ctrl+C, which ends the process without running deferred
// calls, so the decrypted signing key is not left in the temp directory.
type cleaner struct {
	mu  sync.Mutex
	fns []func()
}

// add registers a removal. The last one added runs first.
func (c *cleaner) add(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fns = append(c.fns, fn)
}

// run performs the registered removals, once.
func (c *cleaner) run() {
	c.mu.Lock()
	fns := c.fns
	c.fns = nil
	c.mu.Unlock()
	for i := len(fns) - 1; i >= 0; i-- {
		fns[i]()
	}
}

// watchInterrupt runs the removals and exits when the user presses Ctrl+C. The
// returned function ends the watch when packaging returns normally.
func (c *cleaner) watchInterrupt() (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	done := make(chan struct{})
	go func() {
		select {
		case <-ch:
			c.run()
			os.Exit(130)
		case <-done:
		}
	}()
	return func() {
		signal.Stop(ch)
		close(done)
	}
}

// packageApp builds the release of the app in o.dir into o.dir/dist: a zip,
// and, when it is signed and the Windows SDK is installed, an MSIX. Signing with
// --cert or --test-cert covers the executable and the MSIX.
func packageApp(o packageOptions) error {
	var c cleaner
	defer c.run()
	defer c.watchInterrupt()()

	cfg, err := loadConfig(o.dir)
	if err != nil {
		return err
	}
	icon, err := loadIcon(filepath.Join(o.dir, cfg.Icon))
	if err != nil {
		return err
	}
	if side := icon.Bounds().Dx(); side < recommendedIconSize {
		pkgf("warning: the icon is %d px; %d px or larger is recommended", side, recommendedIconSize)
	}
	winVersion, err := windowsVersion(cfg.Version)
	if err != nil {
		return err
	}
	// Check the tools before anything is written, so a missing SDK leaves
	// nothing behind.
	signing := o.cert != "" || o.testCert
	var signtool string
	if signing {
		if signtool, err = findSDKTool("signtool.exe", o.sdkBin); err != nil {
			return err
		}
	}
	dist := filepath.Join(o.dir, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		return err
	}

	var sign *signer
	if signing {
		if sign, err = newSigner(o, cfg, dist, signtool); err != nil {
			return err
		}
		c.add(sign.close)
	}
	// An MSIX that is not signed cannot be installed, so it is built only when
	// a signing option is given.
	var makeappx string
	switch {
	case o.noMSIX:
	case sign == nil:
		pkgf("no MSIX: a package must be signed to install, so add --test-cert for a local test or --cert for a release (the MSIX also needs the Windows SDK)")
	default:
		if makeappx, err = findSDKTool("makeappx.exe", o.sdkBin); err != nil {
			pkgf("warning: no MSIX will be built: %v", err)
		}
	}

	stage, err := os.MkdirTemp("", "jamb-package-")
	if err != nil {
		return err
	}
	c.add(func() { os.RemoveAll(stage) })
	// The layout holds what goes into the package: the executable, and for
	// the MSIX its manifest and logos.
	layout := filepath.Join(stage, "layout")
	if err := os.Mkdir(layout, 0o755); err != nil {
		return err
	}
	exe := filepath.Join(layout, cfg.Name+".exe")
	pkgf("building %s for windows/%s", cfg.Name, o.arch)
	if err := buildExe(o.dir, exe, cfg, icon, winVersion, o.arch, &c); err != nil {
		return err
	}
	if sign != nil {
		if err := sign.sign(exe); err != nil {
			return err
		}
	}

	base := fmt.Sprintf("%s-%s-windows-%s", cfg.Name, cfg.Version, o.arch)
	zipPath := filepath.Join(dist, base+".zip")
	if err := writeZip(zipPath, o.dir, exe); err != nil {
		return err
	}
	pkgf("wrote %s", zipPath)

	if makeappx == "" {
		return nil
	}
	msixPath := filepath.Join(dist, base+".msix")
	if err := buildMSIX(makeappx, layout, msixPath, cfg, icon, winVersion, o.arch); err != nil {
		return err
	}
	if sign != nil {
		if err := sign.sign(msixPath); err != nil {
			return err
		}
	}
	pkgf("wrote %s", msixPath)
	if sign != nil && sign.testCert {
		printTestCertInstructions(sign, cfg, msixPath)
	}
	return nil
}

// buildExe writes the app's resources as a resource object in dir and builds
// the app into exe. The object is removed as soon as the build ends, on every
// path, and c removes it too if packaging is interrupted.
func buildExe(dir, exe string, cfg *config, icon image.Image, winVersion, arch string, c *cleaner) error {
	rs, err := buildResources(cfg, icon, winVersion)
	if err != nil {
		return err
	}
	syso := filepath.Join(dir, sysoName(arch))
	if _, err := os.Stat(syso); err == nil {
		return fmt.Errorf("%s already exists in the app directory; jamb package writes that file and needs the name, so move it away", syso)
	}
	remove := func() { os.Remove(syso) }
	c.add(remove)
	defer remove()
	if err := writeSyso(syso, rs, arch); err != nil {
		return err
	}

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w -H=windowsgui", "-o", exe, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH="+arch, "CGO_ENABLED=0")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		os.Stderr.Write(out.Bytes())
		return errors.New("go build failed; the errors are above")
	}
	return nil
}

// writeZip writes the release zip. It holds the executable and any LICENSE or
// NOTICE files of the app's directory, all at the root of the archive.
func writeZip(path, dir, exe string) (err error) {
	files := []string{exe}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		upper := strings.ToUpper(e.Name())
		if !e.IsDir() && (strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "NOTICE")) {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()
	zw := zip.NewWriter(out)
	for _, file := range files {
		if err := addZipFile(zw, file); err != nil {
			return err
		}
	}
	return zw.Close()
}

// addZipFile copies one file into zw under its base name, keeping its
// modification time.
func addZipFile(zw *zip.Writer, file string) error {
	src, err := os.Open(file)
	if err != nil {
		return err
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name = filepath.Base(file)
	hdr.Method = zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, src)
	return err
}

// printTestCertInstructions shows how to trust the test certificate for an
// install on this PC. The steps change system state, so jamb only prints them.
func printTestCertInstructions(s *signer, cfg *config, msix string) {
	fmt.Fprintln(os.Stderr)
	pkgf("The MSIX is signed with a test certificate that Windows does not trust yet.")
	pkgf("To install it on this PC, run these in an administrator PowerShell:")
	pkgf(`  certutil -addstore -f TrustedPeople "%s"`, s.cer)
	pkgf(`  Add-AppxPackage -Path "%s"`, msix)
	pkgf("To remove it again:")
	pkgf(`  Get-AppxPackage -Name "%s" | Remove-AppxPackage`, cfg.IdentityName)
	pkgf("  certutil -delstore TrustedPeople %s", s.thumbprint)
}

// pkgf prints a progress or warning line for jamb package.
func pkgf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "jamb package: "+format+"\n", args...)
}
