//go:build unix

package app

import (
	"fmt"
	"os"
	"syscall"
)

// checkPrivate refuses a control directory that another user owns or may use.
func checkPrivate(dir string, fi os.FileInfo) error {
	if fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s may be used by other users (mode %v); make it 0700", dir, fi.Mode().Perm())
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return fmt.Errorf("%s belongs to another user", dir)
	}
	return nil
}
