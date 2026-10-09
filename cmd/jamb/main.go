// Command jamb is the development companion for jamb applications.
//
//	jamb dev [package dir]
//
// dev builds the application, runs it, and watches the source of every
// non-standard-library package it depends on (including the framework in a
// go.work workspace). When a file changes it rebuilds — only then — and
// restarts the app, keeping the window where you left it. A failed build
// prints the compiler errors and leaves the running app alone.
//
// Run it from the repository root with:
//
//	go run ./cmd/jamb dev ./examples/demo
package main

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

func usage() {
	fmt.Fprintln(os.Stderr, `usage: jamb dev [package dir]

  dev   build, run, and rebuild/restart on source changes`)
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 || os.Args[1] != "dev" {
		usage()
	}
	dir := "."
	if len(os.Args) > 2 {
		dir = os.Args[2]
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		fatal(err)
	}
	if err := dev(abs); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "jamb:", err)
	os.Exit(1)
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "jamb dev: "+format+"\n", args...)
}

type runner struct {
	dir      string
	work     string
	geometry string
	slot     int
	child    *exec.Cmd
	exited   chan struct{}
	watched  []string
}

func dev(dir string) error {
	work, err := os.MkdirTemp("", "jamb-dev-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	r := &runner{dir: dir, work: work, geometry: filepath.Join(work, "geometry")}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	if err := r.refreshWatchList(); err != nil {
		return err
	}
	changedAt := time.Now()
	exe, ok := r.build()
	if !ok {
		logf("fix the errors above; waiting for changes")
	} else {
		r.start(exe, changedAt)
	}
	sig := r.signature()

	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-interrupt:
			r.stop()
			return nil
		case <-r.exitedChan():
			logf("app exited; stopping")
			return nil
		case <-tick.C:
			s := r.signature()
			if s == sig {
				continue
			}
			changedAt := time.Now()
			// Let editors finish writing (save-all, formatters) before building.
			time.Sleep(60 * time.Millisecond)
			sig = r.signature()
			exe, ok := r.build()
			if !ok {
				continue
			}
			r.stop()
			r.start(exe, changedAt)
			if err := r.refreshWatchList(); err != nil {
				logf("warning: %v", err)
			}
			sig = r.signature()
		}
	}
}

func (r *runner) exitedChan() <-chan struct{} {
	if r.exited == nil {
		return nil
	}
	return r.exited
}

// refreshWatchList asks the go tool which source directories the app depends
// on, skipping the standard library and the read-only module cache.
func (r *runner) refreshWatchList() error {
	cmd := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.Dir}}{{end}}", ".")
	cmd.Dir = r.dir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("go list in %s failed: %v (is it a Go package directory?)", r.dir, err)
	}
	modcache, _ := exec.Command("go", "env", "GOMODCACHE").Output()
	mc := strings.TrimSpace(string(modcache))
	var dirs []string
	for _, d := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		d = strings.TrimSpace(d)
		if d == "" || (mc != "" && strings.HasPrefix(d, mc)) {
			continue
		}
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	r.watched = dirs
	return nil
}

// signature hashes names, sizes and modification times of every file in the
// watched package directories (sources and embedded assets alike).
func (r *runner) signature() uint64 {
	h := fnv.New64a()
	for _, d := range r.watched {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || strings.HasPrefix(name, ".") || strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "~") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			fmt.Fprintf(h, "%s/%s:%d:%d;", d, name, info.Size(), info.ModTime().UnixNano())
		}
	}
	return h.Sum64()
}

// build compiles the app to an alternating path, so the running executable
// (which Windows keeps locked) is never overwritten.
func (r *runner) build() (string, bool) {
	r.slot ^= 1
	name := "app" + strconv.Itoa(r.slot)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	exe := filepath.Join(r.work, name)
	t0 := time.Now()
	cmd := exec.Command("go", "build", "-o", exe, ".")
	cmd.Dir = r.dir
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		os.Stderr.Write(out.Bytes())
		logf("build failed after %v; keeping the previous version running", time.Since(t0).Round(time.Millisecond))
		return "", false
	}
	logf("built in %v", time.Since(t0).Round(time.Millisecond))
	return exe, true
}

func (r *runner) start(exe string, changedAt time.Time) {
	cmd := exec.Command(exe)
	cmd.Dir = r.dir
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	cmd.Env = append(os.Environ(),
		"JAMB_GEOMETRY_FILE="+r.geometry,
		"JAMB_DEV_CHANGED_AT="+strconv.FormatInt(changedAt.UnixNano(), 10),
	)
	if err := cmd.Start(); err != nil {
		logf("cannot start %s: %v", exe, err)
		return
	}
	exited := make(chan struct{})
	r.child, r.exited = cmd, exited
	go func() {
		cmd.Wait()
		close(exited)
	}()
}

// stop terminates the running app. The window geometry is already on disk:
// the app saves it whenever the window is moved or resized.
func (r *runner) stop() {
	if r.child == nil {
		return
	}
	exited := r.exited
	r.child.Process.Kill()
	<-exited
	r.child, r.exited = nil, nil
}
