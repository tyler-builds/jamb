package ui

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Validator checks the value of a form control. It returns nil when the value is
// valid, and an error whose text is the message the field shows. A control takes
// its validators with Validate (TextInput, TextArea, NumberInput, Select,
// RadioGroup and Checkbox); a Field takes a check of its own with Check.
//
// The validators follow the value types of the controls:
//   - Required for any comparable value: the empty text (after trimming spaces),
//     the zero number, false for a check box (so it is "must accept"), and the
//     zero option of a Select or RadioGroup, which is the placeholder.
//   - MinLength, MaxLength and Pattern for text, which count runes. MinLength and
//     Pattern pass the empty text, so Required decides whether a field must be
//     filled in, and an optional field can use them.
//   - Range for numbers, from lo to hi inclusive.
//   - Custom for any value, from a function of your own.
type Validator[T any] func(v T) error

// messageError is the error of a built-in validator. Its text is read from the
// Strings of the window that shows it, so that it follows the window's language. Its
// Error method gives the English text.
type messageError struct{ text func(s *Strings) string }

// Error returns the English text of the message.
func (e *messageError) Error() string { return e.text(englishStrings()) }

// errorText returns the text that err shows in a window with the Strings s: the text
// of a built-in validator's message in s, and the text of any other error as it is.
func errorText(err error, s *Strings) string {
	var m *messageError
	if errors.As(err, &m) {
		return m.text(s)
	}
	return err.Error()
}

// requiredError is the message of Required.
var requiredError = &messageError{text: func(s *Strings) string { return s.Required }}

// Required rejects the zero value of T: the empty text, zero, false and the zero
// option. A text value is trimmed first, so spaces alone are empty. A number does
// not need it, since a number input always holds a number; use Range to say which
// numbers are allowed.
func Required[T comparable]() Validator[T] {
	return func(v T) error {
		if s, ok := any(v).(string); ok {
			if strings.TrimSpace(s) == "" {
				return requiredError
			}
			return nil
		}
		var zero T
		if v == zero {
			return requiredError
		}
		return nil
	}
}

// MinLength rejects text with fewer than n runes. The empty text passes, so that
// Required decides whether the field must be filled in.
func MinLength(n int) Validator[string] {
	return func(s string) error {
		if s != "" && utf8.RuneCountInString(s) < n {
			return &messageError{text: func(t *Strings) string { return t.MinLength(n) }}
		}
		return nil
	}
}

// MaxLength rejects text with more than n runes.
func MaxLength(n int) Validator[string] {
	return func(s string) error {
		if utf8.RuneCountInString(s) > n {
			return &messageError{text: func(t *Strings) string { return t.MaxLength(n) }}
		}
		return nil
	}
}

// Pattern rejects text that re does not match, with the message msg. The empty
// text passes (see MinLength).
func Pattern(re *regexp.Regexp, msg string) Validator[string] {
	return func(s string) error {
		if s != "" && !re.MatchString(s) {
			return errors.New(msg)
		}
		return nil
	}
}

// Range rejects a number outside lo to hi, both included.
func Range[T int | float64](lo, hi T) Validator[T] {
	return func(v T) error {
		if v < lo || v > hi {
			return &messageError{text: func(t *Strings) string { return t.ValueRange(fmt.Sprint(lo), fmt.Sprint(hi)) }}
		}
		return nil
	}
}

// Custom makes a validator from fn, which returns the error for a value it
// rejects, or nil.
func Custom[T any](fn func(v T) error) Validator[T] { return fn }

// firstError returns the error of the first validator that rejects v, or nil.
func firstError[T any](v T, rules []Validator[T]) error {
	for _, r := range rules {
		if err := r(v); err != nil {
			return err
		}
	}
	return nil
}
