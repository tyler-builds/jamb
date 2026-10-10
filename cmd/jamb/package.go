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
	dir         string
	arch        string
	installer   bool
	makensis    string
	msix        bool
	signCommand string
	sdkBin      string
}

// removedFlags are the flags of earlier versions of jamb package. Each one is
// refused with what to do instead, rather than reported as an unknown flag.
var removedFlags = map[string]string{
	"cert":      "jamb package no longer reads certificates or their passwords; sign the output with --sign-command (see the README, Packaging)",
	"test-cert": "jamb package no longer creates certificates; sign the MSIX with --sign-command, using a certificate whose subject matches publisherSubject (see the README, Packaging)",
	"timestamp": "put the timestamp server in the sign command, for example with signtool's /tr option (see the README, Packaging)",
	"no-msix":   "the MSIX is no longer built by default; leave out --msix to build the zip only",
}

// packageCmd parses the arguments of jamb package and builds the release.
//
//	jamb package [flags] [package dir]
func packageCmd(args []string) error {
	if err := checkRemovedFlags(args); err != nil {
		return err
	}
	var o packageOptions
	fs := flag.NewFlagSet("jamb package", flag.ContinueOnError)
	fs.StringVar(&o.arch, "arch", "amd64", "target architecture: amd64 or arm64")
	fs.BoolVar(&o.installer, "installer", false, "build a setup.exe installer with NSIS (makensis)")
	fs.StringVar(&o.makensis, "makensis", "", "path of makensis.exe (default: on PATH, or in the NSIS install directory); needs --installer")
	fs.BoolVar(&o.msix, "msix", false, "build an MSIX package; it is unsigned unless --sign-command signs it")
	fs.StringVar(&o.signCommand, "sign-command", "", "command that signs each built file, with "+filePlaceholder+" for its path (default: the "+signCommandEnv+" environment variable)")
	fs.StringVar(&o.sdkBin, "sdk-bin", "", "Windows SDK bin directory with makeappx.exe (default: the newest SDK under Windows Kits)")
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
	if o.makensis != "" && !o.installer {
		return errors.New("--makensis needs --installer")
	}
	if o.signCommand == "" {
		o.signCommand = os.Getenv(signCommandEnv)
	}
	if o.signCommand != "" {
		if _, err := newSigner(o.signCommand); err != nil {
			return err
		}
	}
	abs, err := filepath.Abs(o.dir)
	if err != nil {
		return err
	}
	o.dir = abs
	return packageApp(o)
}

// checkRemovedFlags refuses a flag that earlier versions of jamb package had and
// this one does not, with the reason and the replacement.
func checkRemovedFlags(args []string) error {
	for _, a := range args {
		if a == "--" {
			break
		}
		if !strings.HasPrefix(a, "-") {
			continue
		}
		name := strings.TrimLeft(a, "-")
		if i := strings.Index(name, "="); i >= 0 {
			name = name[:i]
		}
		if msg, ok := removedFlags[name]; ok {
			return fmt.Errorf("--%s was removed: %s", name, msg)
		}
	}
	return nil
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
// work directory in dist, and the resource object in the app's directory. It
// also runs them on Ctrl+C, which ends the process without running deferred
// calls.
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

// packageApp builds the release of the app in o.dir into o.dir/dist: a zip, and
// when requested an installer and an MSIX. When a sign command is set, it signs
// the executable before the zip, the installer after it is built, and the MSIX
// after it is built.
//
// The artifacts are built in a work directory inside dist and moved into dist
// only when every step has succeeded, so a failed signature leaves dist without
// a partly signed set.
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
	// Check the sign command and the tools before anything is written, so a
	// missing tool leaves nothing behind.
	var sign *signer
	if o.signCommand != "" {
		if sign, err = newSigner(o.signCommand); err != nil {
			return err
		}
	}
	var makensis string
	if o.installer {
		if makensis, err = findMakensis(o.makensis); err != nil {
			if o.makensis != "" {
				return err
			}
			pkgf("warning: no installer will be built: %v", err)
		}
	}
	var makeappx string
	if o.msix {
		if makeappx, err = findSDKTool("makeappx.exe", o.sdkBin); err != nil {
			pkgf("warning: no MSIX will be built: %v", err)
		}
	}

	dist := filepath.Join(o.dir, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		return err
	}
	work, err := os.MkdirTemp(dist, ".jamb-package-")
	if err != nil {
		return err
	}
	c.add(func() { os.RemoveAll(work) })

	stage, err := os.MkdirTemp("", "jamb-package-")
	if err != nil {
		return err
	}
	c.add(func() { os.RemoveAll(stage) })
	// The layout holds what goes into the MSIX: the executable, and then its
	// manifest and logos.
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
	zipName := base + ".zip"
	if err := writeZip(filepath.Join(work, zipName), o.dir, exe); err != nil {
		return err
	}
	outputs := []string{zipName}

	if makensis != "" {
		setupName := base + "-setup.exe"
		setupPath := filepath.Join(work, setupName)
		if err := buildInstaller(makensis, stage, setupPath, o.dir, exe, cfg, icon, winVersion); err != nil {
			return err
		}
		if sign != nil {
			if err := sign.sign(setupPath); err != nil {
				return err
			}
		}
		outputs = append(outputs, setupName)
	}

	if makeappx != "" {
		msixName := base + ".msix"
		msixPath := filepath.Join(work, msixName)
		if err := buildMSIX(makeappx, layout, msixPath, cfg, icon, winVersion, o.arch); err != nil {
			return err
		}
		if sign != nil {
			if err := sign.sign(msixPath); err != nil {
				return err
			}
		} else {
			pkgf("warning: %s is unsigned. Upload it to Partner Center, where the Store signs it, or sign it with --sign-command before installing it; Windows does not install unsigned packages", msixName)
		}
		outputs = append(outputs, msixName)
	}

	for _, name := range outputs {
		if err := os.Rename(filepath.Join(work, name), filepath.Join(dist, name)); err != nil {
			return err
		}
		pkgf("wrote %s", filepath.Join(dist, name))
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

// licenseFiles returns the LICENSE and NOTICE files of dir, by full path.
func licenseFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		upper := strings.ToUpper(e.Name())
		if !e.IsDir() && (strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "NOTICE")) {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files, nil
}

// writeZip writes the release zip. It holds the executable and any LICENSE or
// NOTICE files of the app's directory, all at the root of the archive.
func writeZip(path, dir, exe string) (err error) {
	licenses, err := licenseFiles(dir)
	if err != nil {
		return err
	}
	files := append([]string{exe}, licenses...)

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

// pkgf prints a progress or warning line for jamb package.
func pkgf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "jamb package: "+format+"\n", args...)
}
