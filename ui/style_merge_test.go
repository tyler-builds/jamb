package ui

import (
	"bytes"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// randomStyle returns a style with about half of its fields set to small random
// values, so that pairs of them overlap, clear and override one another.
func randomStyle(r *rand.Rand) Style {
	var s Style
	fillRandom(r, reflect.ValueOf(&s).Elem(), 0)
	lens := []*Length{&s.Width, &s.Height, &s.MinWidth, &s.MinHeight, &s.MaxWidth, &s.MaxHeight, &s.Basis}
	for _, l := range lens {
		switch r.IntN(4) {
		case 1:
			*l = Px(float32(r.IntN(3) * 10))
		case 2:
			*l = Percent(50)
		}
	}
	for i := range s.insetSet {
		s.insetSet[i] = r.IntN(4) == 0
	}
	s.fitWidth, s.contentMin = r.IntN(4) == 0, r.IntN(4) == 0
	return s
}

func fillRandom(r *rand.Rand, v reflect.Value, depth int) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if f := v.Field(i); f.CanSet() && r.IntN(2) == 0 {
				fillRandom(r, f, depth+1)
			}
		}
	case reflect.Float32, reflect.Float64:
		v.SetFloat(float64(r.IntN(3)) * 0.5)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(r.IntN(3)))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(r.IntN(256)))
	case reflect.Bool:
		v.SetBool(r.IntN(2) == 0)
	case reflect.String:
		v.SetString([]string{"", "a", "b"}[r.IntN(3)])
	case reflect.Slice:
		if depth > 3 {
			return
		}
		n := r.IntN(3)
		sl := reflect.MakeSlice(v.Type(), n, n)
		for i := 0; i < n; i++ {
			fillRandom(r, sl.Index(i), depth+1)
		}
		v.Set(sl)
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			fillRandom(r, v.Index(i), depth+1)
		}
	}
}

// TestMergeFromPlain checks that mergeFrom with plain merges what plainStyle
// leaves of a style, without making that copy.
func TestMergeFromPlain(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 5000; i++ {
		a, b := randomStyle(r), randomStyle(r)
		want := a.Merge(plainStyle(b))
		got := a
		got.mergeFrom(&b, true)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("pair %d: mergeFrom(plain) = %+v\nwant %+v", i, got, want)
		}
	}
}

// TestStyleMergeGenUpToDate checks that style_merge_gen.go is what stylegen
// writes for the widget style types as they are now: run go generate ./ui after
// adding or changing one.
func TestStyleMergeGenUpToDate(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the generator with go run")
	}
	out := filepath.Join(t.TempDir(), "gen.go")
	cmd := exec.Command("go", "run", "../internal/stylegen", "-dir", ".", "-out", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("stylegen: %v\n%s", err, b)
	}
	want, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("style_merge_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("style_merge_gen.go is out of date: run go generate ./ui")
	}
}

// look returns the style that fill writes: a widget's defaults and its
// resolver take the style to fill.
func look[T any](fill func(st *T)) T {
	var st T
	fill(&st)
	return st
}

// TestMergeOneSide checks the one-side padding of docs/styling.md: Edges{Left: 24}
// merged over a style with padding sets the left side and keeps the others, on a
// plain Style and on a field's frame, whose default padding is 10 across.
func TestMergeOneSide(t *testing.T) {
	got := Style{Padding: XY(10, 4), Margin: All(2)}.Merge(Style{Padding: Edges{Left: 24}, Margin: Edges{Top: 8}})
	if want := (Edges{Top: 4, Right: 10, Bottom: 4, Left: 24}); got.Padding != want {
		t.Errorf("padding %+v, want %+v", got.Padding, want)
	}
	if want := (Edges{Top: 8, Right: 2, Bottom: 2, Left: 2}); got.Margin != want {
		t.Errorf("margin %+v, want %+v", got.Margin, want)
	}

	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(TextInput(NewSignal("")).Label("Name").Style(Style{Padding: Edges{Left: 24}}))
	})
	if got, want := partNode(t, r.rt, "TextInput.Frame", "").style.Padding, (Edges{Right: 10, Left: 24}); got != want {
		t.Errorf("frame padding %+v, want %+v", got, want)
	}
}
