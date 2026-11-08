//go:build linux

package linux

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"

	"github.com/tyler-builds/jamb/internal/dmabuf"
	"github.com/tyler-builds/jamb/internal/platform/linux/wl"
	"golang.org/x/sys/unix"
)

// linuxDmabuf is the compositor's zwp_linux_dmabuf_v1: the formats and
// modifiers of the GPU buffers it imports, and the device it composites with.
// A GPU renderer presents through it (see dmabufSurface).
type linuxDmabuf struct {
	p *wl.Proxy
	// mods are the modifiers the compositor imports, by DRM format.
	mods    map[uint32][]uint64
	main    dmabuf.Device
	hasMain bool

	// The default feedback (version 4) being received: its format table, and
	// the modifiers of the tranches so far. done replaces mods with them.
	feedback *wl.Proxy
	table    []formatEntry
	pending  map[uint32][]uint64
	tranche  []uint16
}

type formatEntry struct {
	format   uint32
	modifier uint64
}

// bindDmabuf binds zwp_linux_dmabuf_v1. Version 3 announces the modifiers as
// events when bound; version 4 sends them through a feedback object instead,
// with the compositor's main device. Both arrive by the next roundtrip.
func bindDmabuf(c *wl.Conn, registry *wl.Proxy, name, version uint32) *linuxDmabuf {
	m := &linuxDmabuf{mods: map[uint32][]uint64{}}
	m.p = c.Bind(registry, name, "zwp_linux_dmabuf_v1", minU(version, 4), m.event)
	if m.p.Version >= 4 {
		m.feedback = c.NewProxy("zwp_linux_dmabuf_feedback_v1", 4, m.feedbackEvent)
		m.p.Send(2, wl.NewID{P: m.feedback}) // get_default_feedback
	}
	return m
}

func (m *linuxDmabuf) event(op uint16, d *wl.Decoder) {
	if op == 1 { // modifier(format, modifier_hi, modifier_lo), version 3
		format, hi, lo := d.Uint(), d.Uint(), d.Uint()
		addModifier(m.mods, format, uint64(hi)<<32|uint64(lo))
	}
}

func addModifier(mods map[uint32][]uint64, format uint32, mod uint64) {
	if mod == dmabuf.ModInvalid || slices.Contains(mods[format], mod) {
		return
	}
	mods[format] = append(mods[format], mod)
}

func (m *linuxDmabuf) feedbackEvent(op uint16, d *wl.Decoder) {
	switch op {
	case 0: // done
		if m.pending != nil {
			m.mods, m.pending = m.pending, nil
		}
	case 1: // format_table(fd, size)
		fd, size := d.FD(), d.Uint()
		if fd < 0 {
			return
		}
		m.table = readFormatTable(fd, int(size))
		unix.Close(fd)
	case 2: // main_device(device)
		if dev, ok := devT(d.Array()); ok {
			m.main, m.hasMain = dev, true
		}
	case 3: // tranche_done
		if m.pending == nil {
			m.pending = map[uint32][]uint64{}
		}
		for _, i := range m.tranche {
			if int(i) < len(m.table) {
				e := m.table[i]
				addModifier(m.pending, e.format, e.modifier)
			}
		}
		m.tranche = m.tranche[:0]
	case 5: // tranche_formats(indices)
		b := d.Array()
		for i := 0; i+2 <= len(b); i += 2 {
			m.tranche = append(m.tranche, binary.LittleEndian.Uint16(b[i:]))
		}
	}
}

// readFormatTable reads the feedback's format table: 16-byte entries of a
// format, padding and a modifier.
func readFormatTable(fd, size int) []formatEntry {
	if size <= 0 {
		return nil
	}
	mem, err := unix.Mmap(fd, 0, size, unix.PROT_READ, unix.MAP_PRIVATE)
	if err != nil {
		return nil
	}
	defer unix.Munmap(mem)
	table := make([]formatEntry, 0, size/16)
	for i := 0; i+16 <= len(mem); i += 16 {
		table = append(table, formatEntry{
			format:   binary.LittleEndian.Uint32(mem[i:]),
			modifier: binary.LittleEndian.Uint64(mem[i+8:]),
		})
	}
	return table
}

// devT decodes a dev_t array argument.
func devT(b []byte) (dmabuf.Device, bool) {
	if len(b) != 8 {
		return dmabuf.Device{}, false
	}
	dev := binary.LittleEndian.Uint64(b)
	return dmabuf.Device{Major: unix.Major(dev), Minor: unix.Minor(dev)}, true
}

// dmabufSurface implements dmabuf.Surface for a window.
type dmabufSurface struct{ w *window }

// DmabufSurface returns the window's surface for GPU buffers, or nil when the
// compositor imports no ARGB8888 dma-buf with an explicit modifier, in which
// case frames are drawn on the CPU.
func (w *window) DmabufSurface() dmabuf.Surface {
	m := w.d.dmabuf
	if m == nil || len(m.mods[dmabuf.FormatARGB8888]) == 0 {
		return nil
	}
	return dmabufSurface{w}
}

func (s dmabufSurface) Modifiers(format uint32) []uint64 {
	return slices.Clone(s.w.d.dmabuf.mods[format])
}

func (s dmabufSurface) MainDevice() (dmabuf.Device, bool) {
	m := s.w.d.dmabuf
	return m.main, m.hasMain
}

// params creates a zwp_linux_buffer_params_v1 holding img's planes.
func (s dmabufSurface) params(img dmabuf.Image, on wl.Handler) (*wl.Proxy, error) {
	d := s.w.d
	if len(img.Planes) == 0 || len(img.Planes) > 4 {
		return nil, errors.New("platform/linux: a dma-buf has 1 to 4 planes")
	}
	if !slices.Contains(d.dmabuf.mods[img.Format], img.Modifier) {
		return nil, errors.New("platform/linux: the compositor does not import this dma-buf format and modifier")
	}
	params := d.conn.NewProxy("zwp_linux_buffer_params_v1", d.dmabuf.p.Version, on)
	d.dmabuf.p.Send(1, wl.NewID{P: params}) // create_params
	hi, lo := uint32(img.Modifier>>32), uint32(img.Modifier)
	for i, pl := range img.Planes {
		params.Send(1, wl.FD(img.FD), uint32(i), pl.Offset, pl.Stride, hi, lo) // add
	}
	return params, nil
}

// probeRoundtrips bounds the wait for the answer to a probe. Compositors answer
// within the roundtrip after the request; one that never answers rejects.
const probeRoundtrips = 4

// Probe implements dmabuf.Surface with the create request, which the
// compositor answers with created or failed instead of a protocol error.
func (s dmabufSurface) Probe(img dmabuf.Image) error {
	c := s.w.d.conn
	answered, ok := false, false
	params, err := s.params(img, func(op uint16, d *wl.Decoder) {
		switch op {
		case 0: // created(buffer): a buffer the compositor made; let it go
			b := c.Adopt(d.Uint(), "wl_buffer", 1, nil)
			b.Send(0) // destroy
			b.Destroyed()
			answered, ok = true, true
		case 1: // failed
			answered = true
		}
	})
	if err != nil {
		return err
	}
	params.Send(2, int32(img.Width), int32(img.Height), img.Format, uint32(0)) // create
	for i := 0; i < probeRoundtrips && !answered; i++ {
		if err := c.Roundtrip(); err != nil {
			return err
		}
	}
	params.Send(0) // destroy
	params.Destroyed()
	if !ok {
		return fmt.Errorf("platform/linux: the compositor cannot import a dma-buf with modifier %#x", img.Modifier)
	}
	return nil
}

// Import implements dmabuf.Surface with create_immed, which needs no
// roundtrip: the compositor reports a buffer it cannot import as a protocol
// error, so the renderer sends only kinds of image that passed Probe.
func (s dmabufSurface) Import(img dmabuf.Image) (dmabuf.Buffer, error) {
	c := s.w.d.conn
	params, err := s.params(img, nil)
	if err != nil {
		return nil, err
	}
	b := &gpuBuffer{width: img.Width, height: img.Height}
	b.buf = c.NewProxy("wl_buffer", 1, b.event)
	params.Send(3, wl.NewID{P: b.buf}, int32(img.Width), int32(img.Height), img.Format, uint32(0)) // create_immed
	params.Send(0)                                                                                 // destroy
	params.Destroyed()
	// Flush now: the caller closes the descriptor when this returns.
	if err := c.Flush(); err != nil {
		return nil, err
	}
	return b, nil
}

func (s dmabufSurface) Present(buf dmabuf.Buffer) error {
	b, ok := buf.(*gpuBuffer)
	if !ok || b.buf == nil {
		return errors.New("platform/linux: present of a buffer this window did not import")
	}
	b.busy = true
	s.w.presentBuffer(b.buf, b.width, b.height)
	return nil
}

// gpuBuffer is a wl_buffer of a dma-buf.
type gpuBuffer struct {
	buf           *wl.Proxy
	width, height int
	busy          bool // attached and not yet released by the compositor
	destroyed     bool // Destroy was called while busy
}

func (b *gpuBuffer) event(op uint16, d *wl.Decoder) {
	if op == 0 { // release
		b.busy = false
		if b.destroyed {
			b.destroy()
		}
	}
}

func (b *gpuBuffer) Busy() bool { return b.busy }

func (b *gpuBuffer) Destroy() {
	b.destroyed = true
	if !b.busy {
		b.destroy()
	}
}

func (b *gpuBuffer) destroy() {
	if b.buf != nil {
		b.buf.Send(0) // destroy
		b.buf.Destroyed()
		b.buf = nil
	}
}
