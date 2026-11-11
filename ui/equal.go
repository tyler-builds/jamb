package ui

import (
	"reflect"
	"sync"
)

// same reports whether a write of b over a changes nothing, so that a Signal or
// a Computed need not tell its readers. It compares with ==, for the types whose
// values == compares without a panic: numbers, strings, booleans, pointers,
// channels, and structs and arrays of those. Values of other types are never the
// same: slices, maps and functions cannot be compared, and an interface (or a
// struct holding one) is compared only when its dynamic type can be. A NaN is
// not the same as itself, so writing one always tells the readers.
func same[T any](a, b T) bool {
	// The common types first, without reflect. The switch is on T itself, not
	// on the dynamic type of a value, which an interface T would give.
	switch any((*T)(nil)).(type) {
	case *string:
		return any(a).(string) == any(b).(string)
	case *bool:
		return any(a).(bool) == any(b).(bool)
	case *int:
		return any(a).(int) == any(b).(int)
	case *int64:
		return any(a).(int64) == any(b).(int64)
	case *float64:
		return any(a).(float64) == any(b).(float64)
	}
	t := reflect.TypeOf((*T)(nil)).Elem()
	switch t.Kind() {
	case reflect.Pointer, reflect.Chan, reflect.UnsafePointer:
		return any(a) == any(b)
	case reflect.Slice, reflect.Map, reflect.Func:
		return false
	case reflect.Interface:
		ia, ib := any(a), any(b)
		ta, tb := reflect.TypeOf(ia), reflect.TypeOf(ib)
		if ta != tb {
			return false
		}
		return ta == nil || canCompare(ta) && ia == ib
	}
	return canCompare(t) && any(a) == any(b)
}

// comparableTypes caches canCompare's answer for structs and arrays.
var comparableTypes sync.Map // reflect.Type → bool

// canCompare reports whether == compares any two values of t without a panic:
// t holds no slice, map, function or interface.
func canCompare(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Slice, reflect.Map, reflect.Func, reflect.Interface:
		return false
	case reflect.Struct, reflect.Array:
	default:
		return true
	}
	if ok, found := comparableTypes.Load(t); found {
		return ok.(bool)
	}
	ok := true
	if t.Kind() == reflect.Array {
		ok = canCompare(t.Elem())
	} else {
		for i := range t.NumField() {
			if !canCompare(t.Field(i).Type) {
				ok = false
				break
			}
		}
	}
	comparableTypes.Store(t, ok)
	return ok
}
