package d3d11

import (
	"errors"
	"testing"

	"github.com/tyler-builds/jamb/render"
)

// ReleaseDevice and Recover create and release the real device, so the
// recreated pipelines and states are the ones a frame would use. No reset is
// forced: the device is released by the process itself.
func TestRecoverRebuildsDevice(t *testing.T) {
	d, err := NewDevice()
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}
	defer d.Close()

	d.ReleaseDevice()
	if d.device != nil || d.ctx != nil || d.pipes[pipeQuad].vs != nil {
		t.Fatal("ReleaseDevice left device objects behind")
	}
	if _, err := d.NewRenderer(0, 1, 1); !errors.Is(err, errDeviceReleased) {
		t.Fatalf("NewRenderer on a released device: err = %v; want errDeviceReleased", err)
	}

	if err := d.Recover(); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if d.device == nil || d.ctx == nil || d.pipes[pipeQuad].vs == nil || d.pipes[pipeSubpixel].ps == nil {
		t.Fatal("Recover did not create the device and its pipelines")
	}
	if d.raster == nil || d.blend == nil || d.cbuf == nil || d.samplers[0] == nil {
		t.Fatal("Recover did not create the states and the constant buffer")
	}
	if len(d.textures) != 0 {
		t.Fatalf("Recover kept %d GPU textures; they must be uploaded again from their pixels", len(d.textures))
	}
	if err := d.checkDevice("healthy device", 0x80070057); errors.Is(err, render.ErrDeviceLost) {
		t.Fatalf("a healthy device reported as lost: %v", err)
	}
}

// A removal or reset HRESULT is a device loss on its own. Any other failure is
// a loss only when the device reports that it was removed.
func TestCheckDeviceClassifiesLoss(t *testing.T) {
	d, err := NewDevice()
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}
	defer d.Close()

	for _, hr := range []uintptr{dxgiErrorDeviceRemoved, dxgiErrorDeviceReset} {
		if err := d.checkDevice("Present", hr); !errors.Is(err, render.ErrDeviceLost) {
			t.Errorf("HRESULT 0x%08X: err = %v; want render.ErrDeviceLost", hr, err)
		}
	}
	if err := d.checkDevice("Present", 0x80070057); err == nil || errors.Is(err, render.ErrDeviceLost) {
		t.Errorf("E_INVALIDARG on a healthy device: err = %v; want a plain error", err)
	}
	if err := d.checkDevice("Present", 0); err != nil {
		t.Errorf("S_OK: err = %v; want nil", err)
	}
}
