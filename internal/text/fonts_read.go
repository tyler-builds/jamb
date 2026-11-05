//go:build !windows && !unix

package text

import "os"

// readFontFile reads a font file into memory.
func readFontFile(path string) ([]byte, error) { return os.ReadFile(path) }
