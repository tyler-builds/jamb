package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The generated file must be the output of the generator for the current upstream
// files. The test writes the output to a temporary directory and compares it with
// ui/themes_gen.go; a difference means that go generate ./ui has not been run.
func TestGeneratedFileIsUpToDate(t *testing.T) {
	data, err := render(filepath.Join("..", "..", "third_party", "gpui-component-themes"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "themes_gen.go")
	if err := os.WriteFile(out, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "ui", "themes_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ui/themes_gen.go is out of date; run go generate ./ui")
	}
}

func TestParseColor(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want rgba
		ok   bool
	}{
		{"#FFFFFF", rgba{255, 255, 255, 255}, true},
		{"#89b4fa15", rgba{0x89, 0xb4, 0xfa, 0x15}, true},
		{"linear-gradient(180deg, #1E293B, #0F172A)", rgba{0x1e, 0x29, 0x3b, 255}, true},
		{"green-600", rgba{}, false},
		{"", rgba{}, false},
	} {
		got, ok := parseColor(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseColor(%q) = %v, %t; want %v, %t", tc.in, got.hex(), ok, tc.want.hex(), tc.ok)
		}
	}
}

// A variant that lacks a required color is an error, not a theme with a zero color.
func TestConvertVariantReportsMissingColor(t *testing.T) {
	th := themeConfig{Name: "Incomplete", Mode: "dark", Colors: map[string]string{"foreground": "#ffffff"}}
	if _, err := convertVariant("Test", th); err == nil {
		t.Errorf("convertVariant of a variant without a background returned no error")
	}
	bad := themeConfig{Name: "Bad mode", Mode: "sepia"}
	if _, err := convertVariant("Test", bad); err == nil {
		t.Errorf("convertVariant of an unknown mode returned no error")
	}
}

// The syntax of a variant is read from the highlight block, and a capture with no
// color is allowed.
func TestConvertSyntaxReadsStyles(t *testing.T) {
	var th themeConfig
	src := `{"name":"x","mode":"dark","colors":{},"highlight":{"syntax":{
		"comment":{"color":"#7c7f93","font_style":"italic"},
		"title":{"color":"#ffffff","font_weight":600},
		"emphasis":{"font_style":"italic"}}}}`
	if err := json.Unmarshal([]byte(src), &th); err != nil {
		t.Fatal(err)
	}
	p := &palette{name: "x", colors: map[string]string{}}
	cv := &converter{p: p}
	bg, edBg, edFg := rgba{12, 12, 14, 255}, rgba{12, 12, 14, 255}, rgba{244, 244, 245, 255}
	got, err := convertSyntax(th, p, cv, bg, edBg, edFg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("convertSyntax returned %d captures, want 3", len(got))
	}
	for _, s := range got {
		switch s.name {
		case "comment":
			if !s.italic || s.bold {
				t.Errorf("comment: italic=%t bold=%t, want italic only", s.italic, s.bold)
			}
		case "title":
			if s.italic || !s.bold {
				t.Errorf("title: italic=%t bold=%t, want bold only", s.italic, s.bold)
			}
		case "emphasis":
			if s.c == (rgba{}) || s.c.a != 255 {
				t.Errorf("emphasis without a color takes the editor foreground, got %s", s.c.hex())
			}
		}
	}
}
