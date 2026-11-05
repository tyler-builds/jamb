//go:build !unix

package app

import "os"

// checkPrivate accepts the control directory. On Windows it is in the user's
// own temporary directory, which other users cannot open.
func checkPrivate(dir string, fi os.FileInfo) error { return nil }
