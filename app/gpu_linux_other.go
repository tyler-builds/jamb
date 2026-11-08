//go:build linux && !(amd64 || arm64)

package app

import (
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/render"
)

// The Vulkan renderer needs a 64-bit target; here frames are drawn on the CPU.

func osPrepareDevice() {}

func osNewRenderer(w platform.Window, width, height int) (render.Renderer, error) {
	return newSoftRenderer(w)
}

func osReleaseDevice() {}

func osRebuildDevice() error { return nil }

func osGpuAdapter() string { return "CPU" }
