//go:build js && !jamb_embedfonts

package text

import (
	"errors"
	"fmt"
	"sync"
	"syscall/js"
)

// A page fetches the Go fonts (see fonts_js.go). The jamb_embedfonts tag
// compiles them in instead, for a host with no server to fetch them from.

func embeddedFonts() []embeddedFont { return nil }

// remoteFont is a font file on its way from the server: fetched once, the
// first time it is asked for.
type remoteFont struct {
	file    string
	once    sync.Once
	arrived chan struct{} // closed when data or err is set
	data    []byte
	err     error
}

// start fetches the file, once. When notify is set, it calls the registry's
// loaded once the file has arrived: text drawn without it is drawn again.
func (f *remoteFont) start(r *Registry, notify bool) {
	f.once.Do(func() {
		go func() {
			f.data, f.err = fetchFont(f.file)
			close(f.arrived)
			if notify && r.loaded != nil {
				r.loaded()
			}
		}()
	})
}

// load returns the font once it has arrived. With wait it waits for it, and
// no text is drawn without it; otherwise it reports errFontPending until then.
func (f *remoteFont) load(r *Registry, name string, wait bool) (*Font, error) {
	f.start(r, !wait)
	if wait {
		<-f.arrived
	} else {
		select {
		case <-f.arrived:
		default:
			return nil, errFontPending
		}
	}
	if f.err != nil {
		return nil, fmt.Errorf("text: fetching the font %s: %w", f.file, f.err)
	}
	return ParseFont(name, f.data)
}

// addPlatformFonts registers the faces a page fetches. Go Regular waits for its
// file, which System.WaitForFonts has fetched before any text is laid out; the
// others are drawn with a nearer face until theirs arrives.
func addPlatformFonts(r *Registry) {
	for i, wf := range webFonts {
		rf := &remoteFont{file: wf.file, arrived: make(chan struct{})}
		name, wait := wf.name, i == 0
		r.fam(wf.family).sources[wf.key] = &lazyFont{
			load:     func() (*Font, error) { return rf.load(r, name, wait) },
			embedded: true,
		}
	}
}

// lastResort is Go Regular, the first of webFonts.
func lastResort(r *Registry) *lazyFont {
	return r.fam(webFonts[0].family).sources[webFonts[0].key]
}

// waitPlatformFonts fetches Go Regular and waits for it.
func waitPlatformFonts(r *Registry) error {
	_, err := r.last.get()
	return err
}

// fetchFont returns the bytes of a font file of the page.
func fetchFont(file string) ([]byte, error) {
	g := js.Global()
	var buf js.Value
	if fn := g.Get("jambFetchFont"); fn.Type() == js.TypeFunction {
		v, err := await(fn.Invoke(file))
		if err != nil {
			return nil, err
		}
		buf = v
	} else {
		base := "fonts/"
		if env := g.Get("jambEnv"); env.Type() == js.TypeObject {
			if v := env.Get("JAMB_FONTS"); v.Type() == js.TypeString {
				base = v.String()
			}
		}
		res, err := await(g.Call("fetch", base+file))
		if err != nil {
			return nil, err
		}
		if !res.Get("ok").Bool() {
			return nil, fmt.Errorf("HTTP %d from %s", res.Get("status").Int(), res.Get("url").String())
		}
		if buf, err = await(res.Call("arrayBuffer")); err != nil {
			return nil, err
		}
	}
	bytes := buf
	if !bytes.InstanceOf(g.Get("Uint8Array")) {
		bytes = g.Get("Uint8Array").New(buf)
	}
	data := make([]byte, bytes.Get("length").Int())
	js.CopyBytesToGo(data, bytes)
	return data, nil
}

// await waits for a promise, and returns its value or its rejection.
func await(p js.Value) (js.Value, error) {
	type result struct {
		v   js.Value
		err error
	}
	done := make(chan result, 1)
	var then, catch js.Func
	then = js.FuncOf(func(_ js.Value, args []js.Value) any {
		done <- result{v: args[0]}
		return nil
	})
	catch = js.FuncOf(func(_ js.Value, args []js.Value) any {
		msg := "the promise was rejected"
		if len(args) > 0 && args[0].Truthy() {
			msg = args[0].Call("toString").String()
		}
		done <- result{err: errors.New(msg)}
		return nil
	})
	defer then.Release()
	defer catch.Release()
	js.Global().Get("Promise").Call("resolve", p).Call("then", then, catch)
	r := <-done
	return r.v, r.err
}
