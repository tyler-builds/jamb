//go:build !tinygo

package ui

// copyLarge sets *dst to *src. It is for large structs (a Theme, a widget's
// styles): TinyGo turns a plain assignment of one into code for every field, so
// its build copies them through reflect instead (see copylarge_tinygo.go).
func copyLarge[T any](dst, src *T) { *dst = *src }
