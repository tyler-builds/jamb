//go:build windows

package app

import (
	"image"
	"os"
	"path/filepath"
	"testing"
)

// TestFileIconReadsAnExecutableAndATypeIcon reads the icon of an executable of
// Windows at two sizes, and the icon of a text file through its type. It shows
// nothing on the screen.
func TestFileIconReadsAnExecutableAndATypeIcon(t *testing.T) {
	exe := filepath.Join(os.Getenv("SystemRoot"), "explorer.exe")
	for _, size := range []int{16, 48} {
		img, err := FileIcon(exe, size)
		if err != nil {
			t.Fatalf("FileIcon(explorer.exe, %d): %v", size, err)
		}
		if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
			t.Errorf("FileIcon(explorer.exe, %d) is %v", size, b)
		}
		if !opaqueSomewhere(img) {
			t.Errorf("FileIcon(explorer.exe, %d) is transparent all over", size)
		}
	}
	txt := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(txt, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	img, err := FileIcon(txt, 32)
	if err != nil {
		t.Fatalf("FileIcon(notes.txt): %v", err)
	}
	if b := img.Bounds(); b.Dx() != 32 || !opaqueSomewhere(img) {
		t.Errorf("the text file's icon is %v, or transparent", b)
	}
	if _, err := FileIcon(exe, 0); err == nil {
		t.Error("a size of zero gave no error")
	}
}

// opaqueSomewhere reports whether any pixel of img is more than half opaque.
func opaqueSomewhere(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0x8000 {
				return true
			}
		}
	}
	return false
}
