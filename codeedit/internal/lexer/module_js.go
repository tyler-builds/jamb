//go:build js

package lexer

import (
	"errors"
	"fmt"
	"sync"
	"syscall/js"
)

// In a js build the module runs in the WebAssembly of the JavaScript host (a
// browser, or Node): wazero does not build there, and the host compiles the
// module to native code itself.
//
// The module is compiled and instantiated with the promise APIs
// (WebAssembly.compile and WebAssembly.instantiate), and the calling goroutine
// waits for them while the JavaScript event loop runs. Browsers refuse the
// synchronous APIs on their main thread for a module of this size (Chrome above
// 4 KB). So New must not be called from inside a JavaScript callback, where a
// goroutine cannot wait; jamb calls it from the UI goroutine.

var (
	compileOnce sync.Once
	compiledMod js.Value // the WebAssembly.Module
	compileErr  error
)

// compiled returns the compiled module, compiling it on first use.
func compiled() (js.Value, error) {
	compileOnce.Do(func() {
		wa := js.Global().Get("WebAssembly")
		if wa.IsUndefined() {
			compileErr = errors.New("lexer: the JavaScript host has no WebAssembly")
			return
		}
		src := js.Global().Get("Uint8Array").New(len(wasm))
		js.CopyBytesToJS(src, wasm)
		compiledMod, compileErr = await(wa.Call("compile", src))
		if compileErr != nil {
			compileErr = fmt.Errorf("lexer: compile module: %w", compileErr)
		}
	})
	return compiledMod, compileErr
}

// jsModule is an instance of the module in the JavaScript host.
type jsModule struct {
	mem        js.Value // the exported WebAssembly.Memory
	fns        [numExports]js.Value
	uint8Array js.Value // the Uint8Array constructor
	host       [2]js.Func
}

// newModule compiles the module if this is its first use in the process, and
// returns a new instance of it.
func newModule() (module, error) {
	cm, err := compiled()
	if err != nil {
		return nil, err
	}
	m := &jsModule{uint8Array: js.Global().Get("Uint8Array")}
	hostFunc := func(f func([]byte) uint32) js.Func {
		return js.FuncOf(func(_ js.Value, args []js.Value) any {
			b, _ := m.read(uint32(args[0].Int()), uint32(args[1].Int()))
			return f(b)
		})
	}
	m.host = [2]js.Func{hostFunc(idStartAt), hostFunc(idContinueAt)}
	object := js.Global().Get("Object")
	env := object.New()
	env.Set("is_id_start", m.host[0])
	env.Set("is_id_continue", m.host[1])
	imports := object.New()
	imports.Set("env", env)
	inst, err := await(js.Global().Get("WebAssembly").Call("instantiate", cm, imports))
	if err != nil {
		m.close()
		return nil, fmt.Errorf("lexer: instantiate module: %w", err)
	}
	exports := inst.Get("exports")
	for i, name := range exportNames {
		if m.fns[i] = exports.Get(name); m.fns[i].Type() != js.TypeFunction {
			err = errors.Join(err, fmt.Errorf("lexer: module does not export %s", name))
		}
	}
	if m.mem = exports.Get("memory"); m.mem.IsUndefined() {
		err = errors.Join(err, errors.New("lexer: module exports no memory"))
	}
	if err != nil {
		m.close()
		return nil, err
	}
	return m, nil
}

func (m *jsModule) call(f export, args ...uint64) (r uint32, err error) {
	// A trap in the module is a JavaScript exception, which syscall/js raises as
	// a panic.
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%s: %v", exportNames[f], e)
		}
	}()
	vals := make([]any, len(args))
	for i, a := range args {
		vals[i] = uint32(a)
	}
	v := m.fns[f].Invoke(vals...)
	if v.Type() != js.TypeNumber {
		return 0, nil
	}
	// An i32 result arrives as a signed JavaScript number.
	return uint32(int64(v.Float())), nil
}

// read copies the bytes out of the memory.
func (m *jsModule) read(p, n uint32) ([]byte, bool) {
	buf := m.mem.Get("buffer")
	if uint64(p)+uint64(n) > uint64(buf.Get("byteLength").Int()) {
		return nil, false
	}
	b := make([]byte, n)
	if n > 0 {
		js.CopyBytesToGo(b, m.uint8Array.New(buf, p, n))
	}
	return b, true
}

func (m *jsModule) write(p uint32, b []byte) bool {
	buf := m.mem.Get("buffer")
	if uint64(p)+uint64(len(b)) > uint64(buf.Get("byteLength").Int()) {
		return false
	}
	if len(b) > 0 {
		js.CopyBytesToJS(m.uint8Array.New(buf, p, len(b)), b)
	}
	return true
}

func (m *jsModule) size() uint32 { return uint32(m.mem.Get("buffer").Get("byteLength").Int()) }

func (m *jsModule) grow(pages uint32) (ok bool) {
	// Memory.grow throws a RangeError when the memory cannot grow.
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	m.mem.Call("grow", pages)
	return true
}

// close releases the host functions. The JavaScript host frees the instance and
// its memory once nothing refers to them.
func (m *jsModule) close() error {
	for _, f := range m.host {
		f.Release()
	}
	*m = jsModule{}
	return nil
}

// await waits for the promise p to settle, and returns its value or its error.
func await(p js.Value) (js.Value, error) {
	type result struct {
		v   js.Value
		err error
	}
	done := make(chan result, 1)
	resolve := js.FuncOf(func(_ js.Value, args []js.Value) any {
		done <- result{v: args[0]}
		return nil
	})
	reject := js.FuncOf(func(_ js.Value, args []js.Value) any {
		done <- result{err: js.Error{Value: args[0]}}
		return nil
	})
	defer resolve.Release()
	defer reject.Release()
	p.Call("then", resolve, reject)
	r := <-done
	return r.v, r.err
}
