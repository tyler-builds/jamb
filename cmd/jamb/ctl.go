package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const ctlUsage = `usage: jamb ctl list
       jamb ctl [-app PID] OP [name=value ...]
       jamb ctl [-app PID] -

ctl reads and drives a running app through its control channel, which jamb dev
opens (see docs/tools.md, "Control channel"). Each answer is JSON on stdout; a
failed request exits with status 1.

  list     the running apps that have the channel open
  OP       one request: info, tree, find, click, move, drag, type, key, scroll,
           shot, wait, actions, dispatch, commands, run or clock,
           with its arguments as name=value, such as
             jamb ctl find role=button name=Save
             jamb ctl click key=save
             jamb ctl type text="hello world"
             jamb ctl wait for=found key=done timeout_ms=2000
             jamb ctl dispatch action=notes.new
             jamb ctl run command=notes.create args='{"title":"Hi"}'
             jamb ctl clock advance_ms=500
  -        requests as JSON lines on stdin, answers as JSON lines on stdout

clock drives the app's clock, so that time moves only when the agent moves it:
  jamb ctl clock           the clock's mode (live or seek) and time
  jamb ctl clock +500      moves it 500 ms forward (advance_ms=500)
  jamb ctl clock settle    moves it until the UI comes to rest (settle=true)
  jamb ctl clock seek      stops it where it is (mode=seek)
  jamb ctl clock live      gives it back to the wall clock (mode=live)

With one app running, -app is not needed.`

// ctlNumbers are the request fields that take numbers; the others take text.
var ctlNumbers = map[string]bool{"window": true, "x": true, "y": true, "dx": true, "dy": true,
	"count": true, "limit": true, "offset": true, "timeout_ms": true, "advance_ms": true}

// ctlBools are the request fields that take true or false.
var ctlBools = map[string]bool{"freeze": true, "settle": true}

// ctlDial is how long ctl waits for an app to answer a connection.
const ctlDial = 2 * time.Second

func ctlCmd(args []string) error {
	fs := flag.NewFlagSet("ctl", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	pid := fs.Int("app", 0, "the process ID of the app")
	if err := fs.Parse(args); err != nil || fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, ctlUsage)
		os.Exit(2)
	}
	op, rest := fs.Arg(0), fs.Args()[1:]
	if op == "list" {
		return ctlList(os.Stdout)
	}
	app, err := ctlPick(*pid)
	if err != nil {
		return err
	}
	if op == "-" {
		return ctlStream(app.Socket, os.Stdin, os.Stdout)
	}
	req, err := ctlRequest(op, rest)
	if err != nil {
		return err
	}
	ok, err := ctlOne(app.Socket, req, os.Stdout)
	if err != nil {
		return err
	}
	if !ok {
		os.Exit(1)
	}
	return nil
}

// ctlRequest builds a request from an op and its name=value arguments.
func ctlRequest(op string, args []string) (map[string]any, error) {
	req := map[string]any{"op": op}
	for _, a := range args {
		if op == "clock" && !strings.Contains(a, "=") {
			if err := ctlClockWord(req, a); err != nil {
				return nil, err
			}
			continue
		}
		name, value, ok := strings.Cut(a, "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("argument %q is not name=value", a)
		}
		switch {
		case ctlNumbers[name]:
			n, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("%s must be a number, not %q", name, value)
			}
			req[name] = n
		case name == "args":
			if !json.Valid([]byte(value)) {
				return nil, fmt.Errorf("args must be JSON, such as args='{\"title\":\"Hi\"}', not %q", value)
			}
			req[name] = json.RawMessage(value)
		case ctlBools[name]:
			b, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("%s must be true or false, not %q", name, value)
			}
			req[name] = b
		case name == "path" && value != "":
			abs, err := filepath.Abs(value) // the app's working directory may differ
			if err != nil {
				return nil, err
			}
			req[name] = abs
		default:
			req[name] = value
		}
	}
	return req, nil
}

// ctlClockWord reads a word of the clock op's short form, as Exact writes it:
// +N advances the clock by N milliseconds, and settle, live and seek stand for
// settle=true, mode=live and mode=seek.
func ctlClockWord(req map[string]any, word string) error {
	switch word {
	case "settle":
		req["settle"] = true
		return nil
	case "live", "seek":
		req["mode"] = word
		return nil
	}
	if ms, ok := strings.CutPrefix(word, "+"); ok {
		n, err := strconv.ParseUint(ms, 10, 31)
		if err != nil || n == 0 {
			return fmt.Errorf("%q is not a number of milliseconds to advance by, such as +500", word)
		}
		req["advance_ms"] = float64(n)
		return nil
	}
	return fmt.Errorf("clock takes +MS, settle, live, seek or name=value, not %q", word)
}

// ctlDir is the directory of the apps' sockets; app/control.go names it the same way.
func ctlDir() string {
	if d := os.Getenv("JAMB_CONTROL_DIR"); d != "" {
		return d
	}
	name := "jamb-control"
	if uid := os.Getuid(); uid >= 0 {
		name = "jamb-" + strconv.Itoa(uid)
	}
	return filepath.Join(os.TempDir(), name)
}

// ctlApp is an app's description, from the PID.json file beside its socket.
type ctlApp struct {
	Protocol int    `json:"protocol"`
	PID      int    `json:"pid"`
	Socket   string `json:"socket"`
	Exe      string `json:"exe"`
	Dir      string `json:"dir"`
	Started  string `json:"started"`
}

// ctlApps returns the apps whose socket answers, by process ID. Descriptions
// that a crashed app left behind are skipped.
func ctlApps() ([]ctlApp, error) {
	files, err := filepath.Glob(filepath.Join(ctlDir(), "*.json"))
	if err != nil {
		return nil, err
	}
	var apps []ctlApp
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var a ctlApp
		if json.Unmarshal(b, &a) != nil || a.Socket == "" {
			continue
		}
		if c, err := net.DialTimeout("unix", a.Socket, ctlDial); err == nil {
			c.Close()
			apps = append(apps, a)
		}
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].PID < apps[j].PID })
	return apps, nil
}

func ctlList(w io.Writer) error {
	apps, err := ctlApps()
	if err != nil {
		return err
	}
	if apps == nil {
		apps = []ctlApp{}
	}
	b, _ := json.MarshalIndent(apps, "", "  ")
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}

// ctlPick returns the app with the process ID, or the only app running.
func ctlPick(pid int) (ctlApp, error) {
	apps, err := ctlApps()
	if err != nil {
		return ctlApp{}, err
	}
	if pid != 0 {
		for _, a := range apps {
			if a.PID == pid {
				return a, nil
			}
		}
		return ctlApp{}, fmt.Errorf("app %d has no control channel open; jamb ctl list shows those that do", pid)
	}
	switch len(apps) {
	case 0:
		return ctlApp{}, errors.New("no running app has its control channel open; run the app with jamb dev")
	case 1:
		return apps[0], nil
	}
	pids := make([]string, len(apps))
	for i, a := range apps {
		pids[i] = strconv.Itoa(a.PID)
	}
	return ctlApp{}, fmt.Errorf("%d apps are running (%s); choose one with -app PID", len(apps), strings.Join(pids, ", "))
}

// ctlOne sends one request and writes its answer, indented. It reports whether
// the request succeeded.
func ctlOne(socket string, req map[string]any, w io.Writer) (bool, error) {
	conn, err := net.DialTimeout("unix", socket, ctlDial)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return false, err
	}
	in := bufio.NewReader(conn)
	line, err := in.ReadBytes('\n')
	if err != nil {
		return false, fmt.Errorf("the app did not answer: %w", err)
	}
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		return false, fmt.Errorf("the app's answer is not JSON: %w", err)
	}
	out := map[string]json.RawMessage{}
	if resp.Error != nil {
		out["error"] = resp.Error
	} else {
		out["result"] = resp.Result
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	_, err = fmt.Fprintf(w, "%s\n", b)
	return resp.Error == nil, err
}

// ctlStream copies requests from r to the app and its answers to w, a line each.
func ctlStream(socket string, r io.Reader, w io.Writer) error {
	conn, err := net.DialTimeout("unix", socket, ctlDial)
	if err != nil {
		return err
	}
	defer conn.Close()
	answers := bufio.NewScanner(conn)
	answers.Buffer(make([]byte, 64<<10), 16<<20)
	requests := bufio.NewScanner(r)
	requests.Buffer(make([]byte, 64<<10), 1<<20)
	for requests.Scan() {
		line := strings.TrimSpace(requests.Text())
		if line == "" {
			continue
		}
		if _, err := io.WriteString(conn, line+"\n"); err != nil {
			return err
		}
		if !answers.Scan() {
			if err := answers.Err(); err != nil {
				return err
			}
			return errors.New("the app closed the connection")
		}
		if _, err := fmt.Fprintf(w, "%s\n", answers.Bytes()); err != nil {
			return err
		}
	}
	return requests.Err()
}
