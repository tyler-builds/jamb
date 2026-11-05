//go:build unix

package text

import (
	"os"

	"golang.org/x/sys/unix"
)

// readFontFile maps a font file read-only instead of copying it onto the Go
// heap, as on Windows: the pages are file-backed and shared with other
// processes, so they do not count as this process's private memory, and the
// kernel can drop them under pressure. Fonts are never unloaded, so the mapping
// stays for the life of the process. A file that cannot be mapped is read.
func readFontFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.Size() == 0 || st.Size() > 1<<30 {
		return os.ReadFile(path) // nothing to map, or not a plausible font
	}
	b, err := unix.Mmap(int(f.Fd()), 0, int(st.Size()), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return os.ReadFile(path)
	}
	return b, nil
}
