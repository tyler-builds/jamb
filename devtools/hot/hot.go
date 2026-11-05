// Package hot keeps an app's state across the restarts of `jamb dev`: every
// signal the app makes with ui.NewSignal, and every ui.Local, has the value it
// had before the restart, with no code in the app. See docs/tools.md, "State
// across restarts".
//
// `jamb dev` rewrites, in copies it builds from, each ui.NewSignal call of the
// app's own module into a call of Signal, with a key named after the source:
// the package's directory, the function, and the variable or field the signal
// is assigned to. Before it stops the app, it asks for a checkpoint over the
// control channel, and it starts the new build with JAMB_STATE_FILE naming it.
// Each signal then takes its old value as it is made, and each local as its
// component first renders. Code that sets a signal after making it, such as
// startup code loading a file, sets it after that, as a React effect runs after
// its state is restored.
//
// A value whose type changed so that the old one does not fit, and one that
// cannot be kept (it holds a func or a channel), starts from its initial value,
// and the app says which. Nothing of this package is in a program that does not
// link package devtools, and a program not run by `jamb dev` keeps nothing.
package hot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"weak"

	"github.com/tyler-builds/jamb/internal/devhook"
	"github.com/tyler-builds/jamb/ui"
)

// stateVersion is the version of the checkpoint file.
const stateVersion = 2

// file is a checkpoint.
type file struct {
	Version int      `json:"version"`
	Entries []*entry `json:"entries"`
}

// entry is one kept value: a signal, by its key and how many signals of that
// key were made before it, or a component's local, by the component and name.
type entry struct {
	ID    string `json:"id"`
	Value *node  `json:"value"` // the signal pointer: a kPtr node holding a kSignal
	// Base is the value the app gave the signal itself: what it held when the
	// app had started, or when it was made, for one made later. The next run
	// compares its own with it (see restore).
	Base json.RawMessage `json:"base"`
}

// tracked is a signal or local of this process that a checkpoint keeps.
type tracked struct {
	id   string
	get  func() (reflect.Value, bool) // the *ui.Signal, while it is alive
	base json.RawMessage              // see entry.Base; nil until the app has started
	// waiting is the checkpoint's entry for a signal made before the app had
	// started, which is restored then.
	waiting *entry
}

var state = struct {
	sync.Mutex
	counts   map[string]int // signals made per key
	tracked  []*tracked
	restored map[string]*entry // the entries of JAMB_STATE_FILE not yet used, by ID
	dec      *decoder
	started  bool // the app's startup code has run (see started)
	kept     int
	fresh    int      // values that took the app's new startup value
	reset    []string // why values were not restored
	summary  bool     // the summary is scheduled
}{counts: map[string]int{}, restored: map[string]*entry{}}

func init() {
	devhook.Local = local
	devhook.Checkpoint = Checkpoint
	devhook.Started = started
	if path := os.Getenv("JAMB_STATE_FILE"); path != "" {
		if err := load(path); err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "jamb dev: the state from before the restart was not read:", err)
		}
	}
}

// load reads a checkpoint.
func load(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	if f.Version != stateVersion {
		return fmt.Errorf("version %d, not %d", f.Version, stateVersion)
	}
	state.dec = newDecoder()
	for _, e := range f.Entries {
		state.restored[e.ID] = e
		state.dec.index(e.Value)
	}
	return nil
}

// Signal is what `jamb dev` makes of each ui.NewSignal call: it gives s the
// value that the signal with the same key had before the restart, and keeps
// track of s for the next one. key names the call in the source; the signals
// made by one call are told apart by the order they are made in.
//
// A signal made while the app starts, by package variables or code before
// app.Run, gets its value when the app has started, after any code that sets it
// at startup (see restore). One made later gets it at once.
func Signal[T any](s *ui.Signal[T], key string) *ui.Signal[T] {
	state.Lock()
	defer state.Unlock()
	state.counts[key]++
	w := weak.Make(s)
	track(key+"#"+strconv.Itoa(state.counts[key]), reflect.ValueOf(s), func() (reflect.Value, bool) {
		p := w.Value()
		return reflect.ValueOf(p), p != nil
	})
	return s
}

// local is ui.Local's hook: a component's local, by the component's ID and the
// local's name. Components render after the app has started, so it is
// restored at once.
func local(component uint64, name string, signal any, current func() any) {
	state.Lock()
	defer state.Unlock()
	track("local:"+strconv.FormatUint(component, 16)+":"+name, reflect.ValueOf(signal), func() (reflect.Value, bool) {
		s := current()
		return reflect.ValueOf(s), s != nil
	})
}

// track starts keeping the signal p, and restores it now or, while the app
// is starting, once it has. It is called with state locked.
func track(id string, p reflect.Value, get func() (reflect.Value, bool)) {
	t := &tracked{id: id, get: get}
	state.tracked = append(state.tracked, t)
	e := state.restored[id]
	delete(state.restored, id)
	if !state.started {
		t.waiting = e
		return
	}
	t.base = baseOf(p)
	if e != nil {
		restore(e, p, t.base)
	}
}

// started is called by the app when its startup code has run, just before
// its first frame (devhook.Started). The signals made until then take their
// values now.
func started() {
	state.Lock()
	defer state.Unlock()
	if state.started {
		return
	}
	state.started = true
	for _, t := range state.tracked {
		p, ok := t.get()
		if !ok {
			continue
		}
		t.base = baseOf(p)
		if e := t.waiting; e != nil {
			t.waiting = nil
			restore(e, p, t.base)
		}
	}
}

// baseOf encodes the signal p's value as the base of its next checkpoint, or
// nil when it cannot be encoded.
func baseOf(p reflect.Value) json.RawMessage {
	n, err := newEncoder().encode(p.Elem())
	if err != nil {
		return nil
	}
	b, _ := json.Marshal(n)
	return b
}

// restore gives the signal p its value from the checkpoint's entry e, unless
// the app gave it something else than last time. base is what the app gave it
// this time: the value it was made with, after what the startup code set. If
// that is what the last run's app gave it too, the value in the checkpoint is
// the user's, and it wins. If it differs, the code or the data it was read
// from changed, and the app's new value wins: a default changed in the code
// takes effect, and data read at startup is not overwritten with older data.
// It is called with state locked.
func restore(e *entry, p reflect.Value, base json.RawMessage) {
	defer func() {
		if !state.summary {
			state.summary = true
			time.AfterFunc(time.Second, summarize)
		}
	}()
	if e.Value == nil || e.Value.K != kPtr || e.Value.V == nil {
		return
	}
	if !sameBases(base, e.Base) {
		state.fresh++
		state.dec.bind(e.Value.P, p) // what shared it before shares it still
		return
	}
	// Decode into a copy of the value, so that a value that no longer fits
	// leaves the signal as it is, and a field added since keeps its new default.
	cp := reflect.New(p.Type().Elem())
	f, _ := cp.Elem().Type().FieldByName("value")
	writable(cp.Elem().FieldByIndex(f.Index)).Set(writable(p.Elem().FieldByIndex(f.Index)))
	if err := state.dec.decode(e.Value.V, cp.Elem()); err != nil {
		state.reset = append(state.reset, e.ID+": "+err.Error())
		return
	}
	writable(p.Elem().FieldByIndex(f.Index)).Set(writable(cp.Elem().FieldByIndex(f.Index)))
	state.dec.bind(e.Value.P, p)
	state.kept++
}

// sameBases reports whether two encoded bases are the same value (see sameBase).
func sameBases(a, b json.RawMessage) bool {
	if a == nil || b == nil {
		return false
	}
	if bytes.Equal(a, b) {
		return true
	}
	var x, y node
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return sameBase(&x, &y)
}

// summarize says what the restart kept and what it did not.
func summarize() {
	state.Lock()
	defer state.Unlock()
	msg := "jamb dev: kept " + plural(state.kept, "value")
	if state.fresh > 0 {
		msg += "; " + plural(state.fresh, "value") + " took the app's new start"
	}
	if n := len(state.reset); n > 0 {
		msg += fmt.Sprintf("; %d started again from their initial values:", n)
		for _, r := range state.reset {
			msg += "\n  " + r
		}
	}
	fmt.Fprintln(os.Stderr, msg)
}

// Checkpoint writes the values of the signals and locals alive now to the
// file at path, and returns how many it wrote. Call it on the UI thread, so
// that no render is half done. Values that cannot be kept are left out and
// named on standard error.
func Checkpoint(path string) (int, error) {
	state.Lock()
	defer state.Unlock()
	enc := newEncoder()
	f := file{Version: stateVersion, Entries: []*entry{}}
	var skipped []string
	alive := state.tracked[:0]
	for _, t := range state.tracked {
		p, ok := t.get()
		if !ok {
			continue
		}
		alive = append(alive, t)
		n, err := enc.encode(p)
		if err != nil {
			skipped = append(skipped, t.id+": "+err.Error())
			continue
		}
		base := t.base
		if !state.started {
			base = baseOf(p)
		}
		f.Entries = append(f.Entries, &entry{ID: t.id, Value: n, Base: base})
	}
	state.tracked = alive
	b, err := json.Marshal(f)
	if err != nil {
		return 0, err
	}
	if err := writeAtomic(path, b); err != nil {
		return 0, err
	}
	reportSkipped(skipped)
	return len(f.Entries), nil
}

func writeAtomic(path string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".state-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// reportSkipped names the values a checkpoint left out. The app's signals are
// named; component locals, many of which are the caches of jamb's own widgets,
// are counted, and named with JAMB_TRACE=1.
func reportSkipped(skipped []string) {
	if len(skipped) == 0 {
		return
	}
	sort.Strings(skipped)
	var app, locals []string
	for _, s := range skipped {
		if strings.HasPrefix(s, "local:") {
			locals = append(locals, s)
		} else {
			app = append(app, s)
		}
	}
	trace := os.Getenv("JAMB_TRACE") == "1"
	if len(app) > 0 || trace {
		list := app
		if trace {
			list = skipped
		}
		msg := "jamb dev: " + plural(len(list), "value") + " cannot be kept across the restart, and start again:"
		for _, s := range list {
			msg += "\n  " + s
		}
		fmt.Fprintln(os.Stderr, msg)
	}
	if len(locals) > 0 && !trace {
		fmt.Fprintf(os.Stderr, "jamb dev: %s of components start again (they hold funcs or the like); JAMB_TRACE=1 names them\n",
			plural(len(locals), "local"))
	}
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}
