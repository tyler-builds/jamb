//go:build !js

package lexer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// cacheSubdir is the directory, under os.UserCacheDir, that holds the compiled
// module.
var cacheSubdir = filepath.Join("jamb", "wasm")

// mu guards the process's runtime and the compiled module, and every call that
// instantiates or closes a module. Calls into a running instance do not take it.
var (
	mu       sync.Mutex
	rt       wazero.Runtime
	compiled wazero.CompiledModule
)

// wazeroModule is an instance of the module in wazero.
type wazeroModule struct {
	mod api.Module
	fns [numExports]api.Function
}

// newModule compiles the module if this is its first use in the process, and
// returns a new instance of it.
func newModule() (module, error) {
	mu.Lock()
	r, cm, err := compiledModule()
	var mod api.Module
	if err == nil {
		mod, err = r.InstantiateModule(context.Background(), cm, wazero.NewModuleConfig().WithName(""))
	}
	mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("lexer: instantiate module: %w", err)
	}
	w := &wazeroModule{mod: mod}
	for i, name := range exportNames {
		if w.fns[i] = mod.ExportedFunction(name); w.fns[i] == nil {
			err = errors.Join(err, fmt.Errorf("lexer: module does not export %s", name))
		}
	}
	if mod.Memory() == nil {
		err = errors.Join(err, errors.New("lexer: module exports no memory"))
	}
	if err != nil {
		w.close()
		return nil, err
	}
	return w, nil
}

func (w *wazeroModule) call(f export, args ...uint64) (uint32, error) {
	r, err := w.fns[f].Call(context.Background(), args...)
	if err != nil {
		return 0, err
	}
	if len(r) == 0 {
		return 0, nil
	}
	return uint32(r[0]), nil
}

// read returns a view of the memory, which a later call may move.
func (w *wazeroModule) read(p, n uint32) ([]byte, bool) { return w.mod.Memory().Read(p, n) }

func (w *wazeroModule) write(p uint32, b []byte) bool { return w.mod.Memory().Write(p, b) }

func (w *wazeroModule) size() uint32 { return w.mod.Memory().Size() }

func (w *wazeroModule) grow(pages uint32) bool {
	_, ok := w.mod.Memory().Grow(pages)
	return ok
}

func (w *wazeroModule) close() error {
	mu.Lock()
	defer mu.Unlock()
	return w.mod.Close(context.Background())
}

// compiledModule returns the compiled module, compiling it on first use. The
// caller holds mu.
func compiledModule() (wazero.Runtime, wazero.CompiledModule, error) {
	if compiled != nil {
		return rt, compiled, nil
	}
	if rt == nil {
		r, err := newRuntime(defaultCache())
		if err != nil {
			return nil, nil, err
		}
		rt = r
	}
	cm, err := rt.CompileModule(context.Background(), wasm)
	if err != nil {
		return nil, nil, fmt.Errorf("lexer: compile module: %w", err)
	}
	compiled = cm
	return rt, compiled, nil
}

// newRuntime returns a runtime with the host functions that the module imports.
// A nil cache leaves the runtime to compile the module from source.
func newRuntime(cache wazero.CompilationCache) (wazero.Runtime, error) {
	cfg := wazero.NewRuntimeConfig()
	if cache != nil {
		cfg = cfg.WithCompilationCache(cache)
	}
	ctx := context.Background()
	r := wazero.NewRuntimeWithConfig(ctx, cfg)
	_, err := r.NewHostModuleBuilder("env").
		NewFunctionBuilder().WithFunc(isIDStart).Export("is_id_start").
		NewFunctionBuilder().WithFunc(isIDContinue).Export("is_id_continue").
		Instantiate(ctx)
	if err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("lexer: host functions: %w", err)
	}
	return r, nil
}

// defaultCache returns the process's compilation cache: a directory named jamb
// under the user's cache directory, or memory when that directory is not
// available.
func defaultCache() wazero.CompilationCache {
	base, err := os.UserCacheDir()
	if err != nil {
		return wazero.NewCompilationCache()
	}
	return openCache(filepath.Join(base, cacheSubdir))
}

// openCache returns a compilation cache in dir, creating dir if needed. It
// falls back to an in-memory cache when dir cannot be used, so an unwritable
// cache never prevents highlighting. Nothing is written outside dir.
func openCache(dir string) wazero.CompilationCache {
	if err := os.MkdirAll(dir, 0o755); err == nil {
		if c, err := wazero.NewCompilationCacheWithDir(dir); err == nil {
			return c
		}
	}
	return wazero.NewCompilationCache()
}

// isIDStart and isIDContinue are the module's imports in wazero (see idStartAt).
func isIDStart(_ context.Context, m api.Module, p, n uint32) uint32 {
	b, _ := m.Memory().Read(p, n)
	return idStartAt(b)
}

func isIDContinue(_ context.Context, m api.Module, p, n uint32) uint32 {
	b, _ := m.Memory().Read(p, n)
	return idContinueAt(b)
}
