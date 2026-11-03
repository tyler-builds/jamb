//go:build linux

package linux

import (
	"fmt"

	"github.com/tyler-builds/jamb/internal/platform/linux/wl"
	"golang.org/x/sys/unix"
)

// wl_shm formats: ARGB8888 is premultiplied BGRA in memory on a little-endian
// machine, which is what render/soft draws.
const shmFormatARGB8888 = 0

// shmBuffer is one wl_buffer in shared memory that the CPU renderer draws into.
type shmBuffer struct {
	buf           *wl.Proxy
	pool          *wl.Proxy
	mem           []byte
	width, height int
	stride        int
	busy          bool // attached and not yet released by the compositor
}

// swapchain hands the renderer a buffer the compositor is not reading, and
// attaches it to the surface on present. Two buffers are enough when frames
// are paced by frame callbacks; a third is made if both are still busy.
type swapchain struct {
	w       *window
	bufs    []*shmBuffer
	current *shmBuffer
}

// Buffer implements soft.Target.
func (s *swapchain) Buffer(width, height int) ([]byte, int, error) {
	for i, b := range s.bufs {
		if b.busy {
			continue
		}
		if b.width != width || b.height != height {
			b.destroy()
			nb, err := s.newBuffer(width, height)
			if err != nil {
				s.bufs = append(s.bufs[:i], s.bufs[i+1:]...)
				return nil, 0, err
			}
			s.bufs[i], b = nb, nb
		}
		s.current = b
		return b.mem, b.stride, nil
	}
	if len(s.bufs) >= 4 {
		// The compositor holds every buffer; draw into a new one rather than
		// waiting, and let the old ones go when they are released.
		for _, b := range s.bufs {
			b.destroyWhenReleased()
		}
		s.bufs = s.bufs[:0]
	}
	nb, err := s.newBuffer(width, height)
	if err != nil {
		return nil, 0, err
	}
	s.bufs = append(s.bufs, nb)
	s.current = nb
	return nb.mem, nb.stride, nil
}

// Present implements soft.Target: it attaches the drawn buffer and commits.
func (s *swapchain) Present() error {
	b := s.current
	if b == nil {
		return nil
	}
	s.current = nil
	b.busy = true
	s.w.present(b)
	return nil
}

func (s *swapchain) newBuffer(width, height int) (*shmBuffer, error) {
	stride := width * 4
	size := stride * height
	fd, err := unix.MemfdCreate("jamb-frame", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		return nil, fmt.Errorf("platform/linux: memfd_create: %w", err)
	}
	defer unix.Close(fd)
	if err := unix.Ftruncate(fd, int64(size)); err != nil {
		return nil, fmt.Errorf("platform/linux: sizing a frame buffer: %w", err)
	}
	mem, err := unix.Mmap(fd, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("platform/linux: mapping a frame buffer: %w", err)
	}
	c := s.w.d.conn
	b := &shmBuffer{mem: mem, width: width, height: height, stride: stride}
	b.pool = c.NewProxy("wl_shm_pool", 1, nil)
	s.w.d.shm.Send(0, wl.NewID{P: b.pool}, wl.FD(fd), int32(size)) // create_pool
	b.buf = c.NewProxy("wl_buffer", 1, nil)
	b.pool.Send(0, wl.NewID{P: b.buf}, int32(0), int32(width), int32(height), int32(stride), uint32(shmFormatARGB8888)) // create_buffer
	b.pool.Send(1)                                                                                                      // destroy: the buffer keeps the memory
	b.pool.Destroyed()
	b.pool = nil
	b.buf.On = func(op uint16, d *wl.Decoder) {
		if op == 0 { // release
			b.busy = false
		}
	}
	// Flush now: the descriptor is closed when this returns.
	if err := c.Flush(); err != nil {
		unix.Munmap(mem)
		return nil, err
	}
	return b, nil
}

func (b *shmBuffer) destroy() {
	if b.buf != nil {
		b.buf.Send(0) // destroy
		b.buf.Destroyed()
		b.buf = nil
	}
	if b.mem != nil {
		unix.Munmap(b.mem)
		b.mem = nil
	}
}

// destroyWhenReleased destroys the buffer now if the compositor is done with
// it, or when it releases it.
func (b *shmBuffer) destroyWhenReleased() {
	if !b.busy {
		b.destroy()
		return
	}
	b.buf.On = func(op uint16, d *wl.Decoder) {
		if op == 0 {
			b.destroy()
		}
	}
}

func (s *swapchain) close() {
	for _, b := range s.bufs {
		b.destroyWhenReleased()
	}
	s.bufs = nil
}
