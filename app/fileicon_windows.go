//go:build windows

package app

import (
	"image"

	"github.com/tyler-builds/jamb/internal/platform/windows"
)

func fileIcon(path string, size int) (image.Image, error) { return windows.FileIcon(path, size) }
