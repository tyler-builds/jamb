//go:build !windows

package watch

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Watch calls onChange (on its own goroutine) after files in dir change, until
// stop is called. Changes arriving within debounce of each other produce one
// call. Other platforms have no change notifications yet, so it polls the
// directory once a second.
func Watch(dir string, debounce time.Duration, onChange func()) (stop func(), err error) {
	last, err := snapshot(dir)
	if err != nil {
		return nil, err
	}
	quit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-quit:
				return
			case <-tick.C:
			}
			cur, err := snapshot(dir)
			if err != nil || cur == last {
				continue
			}
			last = cur
			select {
			case <-quit:
				return
			case <-time.After(debounce):
			}
			onChange()
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(quit)
			<-done
		})
	}, nil
}

// snapshot describes the entries of dir by name, size and modification time.
func snapshot(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "%s %d %d\n", e.Name(), info.Size(), info.ModTime().UnixNano())
	}
	return b.String(), nil
}
