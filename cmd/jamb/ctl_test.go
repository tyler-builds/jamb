package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCtlRequest(t *testing.T) {
	req, err := ctlRequest("click", []string{"key=save", "count=2", "text=3", "path=a.png", `name=a=b`})
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs("a.png")
	want := map[string]any{"op": "click", "key": "save", "count": 2.0, "text": "3", "path": abs, "name": "a=b"}
	for k, v := range want {
		if req[k] != v {
			t.Errorf("%s = %#v; want %#v", k, req[k], v)
		}
	}
	for _, bad := range []string{"novalue", "=x", "x=1e", "limit=many"} {
		if _, err := ctlRequest("tree", []string{bad}); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

// fakeApp listens like an app's control channel and answers each request with
// the answer it is given, the request's op echoed in the result.
func fakeApp(t *testing.T, dir string, pid int) string {
	t.Helper()
	socket := filepath.Join(dir, strconv.Itoa(pid)+".sock")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	b, _ := json.Marshal(ctlApp{Protocol: 1, PID: pid, Socket: socket})
	if err := os.WriteFile(filepath.Join(dir, strconv.Itoa(pid)+".json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				in := bufio.NewScanner(c)
				for in.Scan() {
					var req map[string]any
					json.Unmarshal(in.Bytes(), &req)
					if req["op"] == "fail" {
						c.Write([]byte(`{"error":{"code":"bad_request","message":"no"}}` + "\n"))
						continue
					}
					out, _ := json.Marshal(map[string]any{"result": map[string]any{"op": req["op"]}})
					c.Write(append(out, '\n'))
				}
			}()
		}
	}()
	return socket
}

func TestCtlFindsAndAsks(t *testing.T) {
	dir, err := os.MkdirTemp("", "jc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("JAMB_CONTROL_DIR", dir)

	if _, err := ctlPick(0); err == nil {
		t.Fatal("picked an app when none runs")
	}
	// A description left by an app that crashed is skipped.
	stale, _ := json.Marshal(ctlApp{PID: 8, Socket: filepath.Join(dir, "gone.sock")})
	os.WriteFile(filepath.Join(dir, "8.json"), stale, 0o600)
	socket := fakeApp(t, dir, 3)

	app, err := ctlPick(0)
	if err != nil || app.Socket != socket {
		t.Fatalf("pick: %+v %v", app, err)
	}
	if _, err := ctlPick(8); err == nil {
		t.Error("picked the app that is gone")
	}

	var out bytes.Buffer
	ok, err := ctlOne(socket, map[string]any{"op": "info"}, &out)
	if err != nil || !ok || !strings.Contains(out.String(), `"op": "info"`) {
		t.Fatalf("one: %v %v %s", ok, err, out.String())
	}
	out.Reset()
	if ok, err := ctlOne(socket, map[string]any{"op": "fail"}, &out); err != nil || ok || !strings.Contains(out.String(), "bad_request") {
		t.Errorf("a failed request: %v %v %s", ok, err, out.String())
	}

	out.Reset()
	in := strings.NewReader("{\"op\":\"tree\"}\n\n{\"op\":\"find\"}\n")
	if err := ctlStream(socket, in, &out); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(strings.TrimSpace(out.String()), "\n"); len(lines) != 2 || !strings.Contains(lines[1], "find") {
		t.Errorf("stream: %q", out.String())
	}

	fakeApp(t, dir, 4)
	if _, err := ctlPick(0); err == nil || !strings.Contains(err.Error(), "-app") {
		t.Errorf("two apps: %v", err)
	}
}
