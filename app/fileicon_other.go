//go:build !windows

package app

import (
	"errors"
	"image"
)

func fileIcon(string, int) (image.Image, error) {
	return nil, errors.New("app.FileIcon is supported on Windows only")
}
