// Package watch reports changes to the files in a directory. It is the
// platform layer behind live reload of keymap files, so it lives outside the
// ui package, which has no OS code.
package watch
