package ui

import (
	"reflect"
	"strings"
	"testing"
)

// The high contrast theme keeps every widget style of its base theme (the
// fields whose types end in Style or Variants), so an app's overrides are not
// lost when the user turns high contrast on. A new widget's theme field fails
// here until HighContrastTheme copies it.
func TestHighContrastKeepsEveryWidgetStyle(t *testing.T) {
	base := DarkTheme()
	bv := reflect.ValueOf(base).Elem()
	for i := range bv.NumField() {
		f := bv.Type().Field(i)
		name := f.Type.Name()
		if !f.IsExported() || f.Type.Kind() != reflect.Struct || !(strings.HasSuffix(name, "Style") || strings.HasSuffix(name, "Variants")) {
			continue
		}
		// Mark the first Style found in the field, at any depth.
		var mark func(v reflect.Value) bool
		mark = func(v reflect.Value) bool {
			if v.Type() == reflect.TypeOf(Style{}) {
				v.Set(reflect.ValueOf(Style{Gap: 7.5}))
				return true
			}
			for j := range v.NumField() {
				if v.Type().Field(j).IsExported() && v.Field(j).Kind() == reflect.Struct && mark(v.Field(j)) {
					return true
				}
			}
			return false
		}
		if !mark(bv.Field(i)) {
			continue
		}
		hc := reflect.ValueOf(HighContrastTheme(SystemColors{}, base)).Elem()
		if !reflect.DeepEqual(hc.Field(i).Interface(), bv.Field(i).Interface()) {
			t.Errorf("HighContrastTheme drops Theme.%s", f.Name)
		}
	}
}
