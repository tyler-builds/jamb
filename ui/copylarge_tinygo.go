//go:build tinygo

package ui

import "reflect"

// copyLarge sets *dst to *src as one block of memory. A plain assignment of a
// large struct becomes code for every one of its fields under TinyGo: hundreds
// of KB for a Theme.
func copyLarge[T any](dst, src *T) { reflect.ValueOf(dst).Elem().Set(reflect.ValueOf(src).Elem()) }
