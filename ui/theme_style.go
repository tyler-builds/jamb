package ui

import "maps"

// themeStyleKey is the key that a value of type T is stored under on a theme. Each
// type argument makes a distinct key type, so two types never share a slot, even
// when their names are the same in different packages.
type themeStyleKey[T any] struct{}

// SetThemeStyle stores v on the theme t, under the type T. It is how a theme holds
// the style of a widget that the Theme struct cannot name, such as charts.Style or
// codeedit.Style, or the style type of a third-party widget: the widget reads it with
// ThemeStyle, and merges it over its built-in defaults and under the Styles of an
// instance. Storing again under the same type replaces the value.
//
// The value belongs to the theme t, not to the app: a window that switches between a
// light and a dark theme needs it set on both. Set it before the theme is given to a
// window or to SetTheme, as with the other fields of a theme. To change it while the
// theme is in use, set it on a Clone and pass the clone to SetTheme. Copies made with
// Clone, with a struct copy, or by HighContrastTheme keep the values set before the
// copy, and a later SetThemeStyle on one does not change the other.
func SetThemeStyle[T any](t *Theme, v T) {
	m := make(map[any]any, len(t.styles)+1)
	maps.Copy(m, t.styles)
	m[themeStyleKey[T]{}] = v
	t.styles = m
}

// ThemeStyle returns the value that SetThemeStyle stored on the theme t under the
// type T, and true. It returns the zero value and false when none is stored, or when
// t is nil. A widget calls it while it renders, with ctx.Theme(); the theme is not
// changed then (see SetThemeStyle), so the read needs no lock.
//
// Under high contrast ctx.Theme() is the high contrast theme, which keeps the values
// of the app's theme. A widget should treat them as it treats the Styles of an
// instance there: keep their sizes and spacing, and take its colors from the system
// colors, as the built-in widgets do.
func ThemeStyle[T any](t *Theme) (T, bool) {
	if t != nil {
		if v, ok := t.styles[themeStyleKey[T]{}]; ok {
			s, _ := v.(T) // a nil stored under an interface type is the zero value
			return s, true
		}
	}
	var zero T
	return zero, false
}
