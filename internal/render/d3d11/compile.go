//go:build windows

package d3d11

import (
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	modCompiler    = syscall.NewLazyDLL("d3dcompiler_47.dll")
	procD3DCompile = modCompiler.NewProc("D3DCompile")
)

const (
	d3dCompileOptimization3 = 1 << 15
)

// compileShader returns bytecode for one entry point of shaderSource.
// Bytecode is cached under the user cache directory keyed by a hash of the
// source, so only the very first launch pays for HLSL compilation.
func compileShader(entry, target string) ([]byte, error) {
	// FNV is plenty for naming a cache file. crypto/sha256 would pull Go's
	// whole FIPS crypto module into every jamb binary, with a 32 MB buffer.
	h := fnv.New128a()
	h.Write([]byte(shaderSource + "|" + entry + "|" + target))
	sum := h.Sum(nil)
	cachePath := ""
	if dir, err := os.UserCacheDir(); err == nil {
		cachePath = filepath.Join(dir, "jamb", "shaders", hex.EncodeToString(sum)+".dxbc")
		if b, err := os.ReadFile(cachePath); err == nil && len(b) > 0 {
			return b, nil
		}
	}
	code, err := d3dCompile(entry, target)
	if err != nil {
		return nil, err
	}
	if cachePath != "" {
		if os.MkdirAll(filepath.Dir(cachePath), 0o755) == nil {
			_ = os.WriteFile(cachePath, code, 0o644)
		}
	}
	return code, nil
}

func d3dCompile(entry, target string) ([]byte, error) {
	if err := procD3DCompile.Find(); err != nil {
		return nil, fmt.Errorf("d3d11: cannot load d3dcompiler_47.dll to compile shaders (%v); it ships with Windows 10 and later", err)
	}
	src := []byte(shaderSource)
	name := []byte("jamb.hlsl\x00")
	entryZ := append([]byte(entry), 0)
	targetZ := append([]byte(target), 0)
	var code, errs *com
	hr, _, _ := syscall.SyscallN(procD3DCompile.Addr(),
		uintptr(unsafe.Pointer(&src[0])), uintptr(len(src)), uintptr(unsafe.Pointer(&name[0])),
		0, 0, uintptr(unsafe.Pointer(&entryZ[0])), uintptr(unsafe.Pointer(&targetZ[0])),
		d3dCompileOptimization3, 0, uintptr(unsafe.Pointer(&code)), uintptr(unsafe.Pointer(&errs)))
	if hresult(hr).failed() {
		msg := ""
		if errs != nil {
			msg = string(blobBytes(errs))
			errs.Release()
		}
		return nil, fmt.Errorf("d3d11: HLSL compile of %s (%s) failed: %s", entry, target, msg)
	}
	if errs != nil {
		errs.Release()
	}
	defer code.Release()
	return append([]byte(nil), blobBytes(code)...), nil
}

func blobBytes(b *com) []byte {
	p, _, _ := syscall.SyscallN(b.fn(3), b.this()) // GetBufferPointer
	n, _, _ := syscall.SyscallN(b.fn(4), b.this()) // GetBufferSize
	return unsafe.Slice((*byte)(ptrFrom(p)), int(n))
}
