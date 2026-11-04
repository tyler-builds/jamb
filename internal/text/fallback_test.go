package text

import (
	"testing"

	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
)

// Fallback fonts the app adds come before the platform's, in the order they
// were added, and bad data is an error.
func TestAddFallbackComesFirst(t *testing.T) {
	r := NewRegistry()
	if err := r.AddFallback("first", gomono.TTF); err != nil {
		t.Fatal(err)
	}
	if err := r.AddFallback("second", goregular.TTF); err != nil {
		t.Fatal(err)
	}
	if f := r.fallbackFor('A'); f == nil || f.Name != "first" {
		t.Fatalf("fallbackFor('A') = %v, want the first app fallback", f)
	}
	if r.fallbacks[1].font.font.Name != "second" {
		t.Error("the second app fallback is not second")
	}
	if err := r.AddFallback("broken", []byte("not a font")); err == nil {
		t.Error("data that is not a font was accepted")
	}
}

// The color-table probe reads the table directory.
func TestHasColorTables(t *testing.T) {
	if hasColorTables(goregular.TTF, 0) {
		t.Error("Go Regular has color tables")
	}
	if hasColorTables([]byte("ttcf"), 0) || hasColorTables(nil, 0) {
		t.Error("truncated data has color tables")
	}
}
