package app

import (
	"bytes"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestInstanceNamesFor(t *testing.T) {
	mutex, pipe := instanceNamesFor("JambExamples.Notes", "S-1-5-21-1", 1)
	if want := `Local\jamb-JambExamples.Notes`; mutex != want {
		t.Errorf("mutex = %q, want %q", mutex, want)
	}
	if prefix := `\\.\pipe\jamb-JambExamples.Notes-`; !strings.HasPrefix(pipe, prefix) {
		t.Errorf("pipe = %q, want the prefix %q", pipe, prefix)
	}
	if _, again := instanceNamesFor("JambExamples.Notes", "S-1-5-21-1", 1); again != pipe {
		t.Errorf("the same user and session gave two pipe names: %q and %q", pipe, again)
	}
	for name, other := range map[string]string{
		"another user":    mustPipe("JambExamples.Notes", "S-1-5-21-2", 1),
		"another session": mustPipe("JambExamples.Notes", "S-1-5-21-1", 2),
		"another app":     mustPipe("JambExamples.Other", "S-1-5-21-1", 1),
	} {
		if other == pipe {
			t.Errorf("%s shares the pipe name %q", name, pipe)
		}
	}
}

// mustPipe returns the pipe name of instanceNamesFor.
func mustPipe(appID, sid string, session uint32) string {
	_, pipe := instanceNamesFor(appID, sid, session)
	return pipe
}

func TestHandoffRoundTrip(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	want := handoff{Version: handoffVersion, Dir: `C:\Work dir`, Args: []string{
		"--new-note", "two words", `say "hi"`, "line\nbreak", "naïve 文字", "",
	}}
	var got handoff
	served := make(chan error, 1)
	go func() {
		served <- serveHandoff(server, func(h handoff) bool {
			got = h
			return true
		})
	}()
	ok, err := sendHandoff(client, want, time.Second)
	if err != nil {
		t.Fatalf("sendHandoff: %v", err)
	}
	if !ok {
		t.Fatal("sendHandoff reported the request refused")
	}
	if err := <-served; err != nil {
		t.Fatalf("serveHandoff: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the server read %+v, want %+v", got, want)
	}
}

func TestHandoffRefused(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	served := make(chan error, 1)
	go func() {
		served <- serveHandoff(server, func(handoff) bool { return false })
	}()
	ok, err := sendHandoff(client, handoff{Version: handoffVersion, Args: []string{"x"}}, time.Second)
	if err != nil || ok {
		t.Fatalf("sendHandoff = %v, %v; want false and no error", ok, err)
	}
	if err := <-served; err != nil {
		t.Errorf("serveHandoff: %v", err)
	}
}

func TestHandoffWrongVersionIsRefused(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	called := false
	served := make(chan error, 1)
	go func() {
		served <- serveHandoff(server, func(handoff) bool {
			called = true
			return true
		})
	}()
	ok, err := sendHandoff(client, handoff{Version: handoffVersion + 1, Args: []string{"x"}}, time.Second)
	if err != nil || ok {
		t.Fatalf("sendHandoff = %v, %v; want false and no error", ok, err)
	}
	if err := <-served; err == nil {
		t.Error("serveHandoff accepted a request of another protocol version")
	}
	if called {
		t.Error("deliver ran for a request of another protocol version")
	}
}

func TestSendHandoffTimesOut(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	// Nothing reads the server end, so the request is never answered.
	ok, err := sendHandoff(client, handoff{Version: handoffVersion}, 20*time.Millisecond)
	client.Close() // ends the write that is still waiting
	if ok || !errors.Is(err, errHandoffTimeout) {
		t.Fatalf("sendHandoff = %v, %v; want false and errHandoffTimeout", ok, err)
	}
}

func TestWriteHandoffRejectsLargeRequests(t *testing.T) {
	big := handoff{Version: handoffVersion, Args: []string{strings.Repeat("x", maxHandoff)}}
	if err := writeHandoff(io.Discard, big); err == nil {
		t.Error("writeHandoff accepted a request over the limit")
	}
}

func TestReadHandoffLimits(t *testing.T) {
	for name, frame := range map[string][]byte{
		"empty":    {0, 0, 0, 0},
		"too long": {0x7f, 0xff, 0xff, 0xff}, // the body is never read
	} {
		if _, err := readHandoff(bytes.NewReader(frame)); err == nil {
			t.Errorf("%s frame: readHandoff accepted it", name)
		}
	}
	if _, err := readHandoff(bytes.NewReader([]byte{0, 0})); err == nil {
		t.Error("a truncated header was accepted")
	}
}

func TestHandoffFrameLayout(t *testing.T) {
	var buf bytes.Buffer
	if err := writeHandoff(&buf, handoff{Version: handoffVersion, Args: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if n := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3]); n != len(b)-4 {
		t.Errorf("the length prefix is %d; the body is %d bytes", n, len(b)-4)
	}
	h, err := readHandoff(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a"}; !reflect.DeepEqual(h.Args, want) {
		t.Errorf("Args = %q, want %q", h.Args, want)
	}
}
