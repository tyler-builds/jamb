//go:build linux

package linux

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"slices"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/internal/platform/linux/wl"
	"golang.org/x/sys/unix"
)

// The clipboard is the seat's selection, through wl_data_device. The app's
// own copy is offered as a wl_data_source, whose data is written when another
// client asks for it; a paste reads the current selection offer through a pipe.

// textTypes are the MIME types of text, preferred first.
var textTypes = []string{"text/plain;charset=utf-8", "UTF8_STRING", "text/plain", "TEXT", "STRING"}

const pngType = "image/png"

// readTimeout bounds a paste from another client that does not answer.
const readTimeout = time.Second

// offer is a wl_data_offer and the MIME types it offers.
type offer struct {
	p     *wl.Proxy
	types []string
}

type clipboard struct {
	d       *display
	manager *wl.Proxy // wl_data_device_manager
	device  *wl.Proxy // wl_data_device

	// The selection another client offers, or nil, and the offers announced
	// by data_offer until selection says which one is current.
	sel     *offer
	pending map[uint32]*offer

	// The app's own selection: its source, and the text or PNG it holds.
	source    *wl.Proxy
	ownText   string
	ownPNG    []byte
	ownsImage bool
}

func newClipboard(d *display, name, version uint32) *clipboard {
	cb := &clipboard{d: d, pending: map[uint32]*offer{}}
	cb.manager = d.conn.Bind(d.registry, name, "wl_data_device_manager", version, nil)
	return cb
}

// attach gets the seat's data device, once the seat exists.
func (cb *clipboard) attach(s *seat) {
	if cb.device != nil || s == nil {
		return
	}
	cb.device = cb.d.conn.NewProxy("wl_data_device", cb.manager.Version, cb.deviceEvent)
	cb.manager.Send(1, wl.NewID{P: cb.device}, s.proxy) // get_data_device
}

func (cb *clipboard) deviceEvent(op uint16, d *wl.Decoder) {
	switch op {
	case 0: // data_offer(id): a new offer; its offer events follow
		id := d.Uint()
		o := &offer{}
		o.p = cb.d.conn.Adopt(id, "wl_data_offer", cb.manager.Version, func(op uint16, d *wl.Decoder) {
			if op == 0 { // offer(mime_type)
				o.types = append(o.types, d.String())
			}
		})
		cb.pending[id] = o
	case 5: // selection(id): the current selection, or null
		id := d.Uint()
		if cb.sel != nil {
			cb.sel.p.Send(2) // destroy
			cb.sel.p.Destroyed()
		}
		cb.sel = cb.pending[id] // nil for a null selection
		delete(cb.pending, id)
		// Other offers (for drag and drop) are not used.
		for _, o := range cb.pending {
			o.p.Send(2)
			o.p.Destroyed()
		}
		clear(cb.pending)
	}
}

// types returns the MIME types of another client's selection.
func (cb *clipboard) types() []string {
	if cb.sel == nil {
		return nil
	}
	return cb.sel.types
}

// owned reports whether the selection is the app's own.
func (cb *clipboard) owned() bool { return cb.source != nil }

// text returns the selection's text, with LF line endings.
func (cb *clipboard) text() (string, bool) {
	if cb.owned() {
		if cb.ownsImage {
			return "", false
		}
		return cb.ownText, true
	}
	for _, t := range textTypes {
		if slices.Contains(cb.types(), t) {
			b, err := cb.receive(t)
			if err != nil {
				return "", false
			}
			return strings.ReplaceAll(string(b), "\r\n", "\n"), true
		}
	}
	return "", false
}

// hasImage reports whether the selection is a PNG image.
func (cb *clipboard) hasImage() bool {
	if cb.owned() {
		return cb.ownsImage
	}
	return slices.Contains(cb.types(), pngType)
}

func (cb *clipboard) image() (image.Image, bool) {
	var data []byte
	if cb.owned() {
		if !cb.ownsImage {
			return nil, false
		}
		data = cb.ownPNG
	} else if slices.Contains(cb.types(), pngType) {
		b, err := cb.receive(pngType)
		if err != nil {
			return nil, false
		}
		data = b
	} else {
		return nil, false
	}
	img, err := png.Decode(bytes.NewReader(data))
	return img, err == nil
}

// receive reads the selection in the given MIME type from the client that
// offers it. The other client writes into a pipe; the read gives up after
// readTimeout.
func (cb *clipboard) receive(mime string) ([]byte, error) {
	if cb.sel == nil {
		return nil, errors.New("no selection")
	}
	var p [2]int
	if err := unix.Pipe2(p[:], unix.O_CLOEXEC); err != nil {
		return nil, err
	}
	r, w := p[0], p[1]
	defer unix.Close(r)
	cb.sel.p.Send(1, mime, wl.FD(w)) // receive
	err := cb.d.conn.Flush()
	unix.Close(w)
	if err != nil {
		return nil, err
	}
	var out []byte
	buf := make([]byte, 64<<10)
	deadline := time.Now().Add(readTimeout)
	for {
		left := time.Until(deadline)
		if left <= 0 {
			return nil, errors.New("the clipboard's owner did not answer")
		}
		fds := []unix.PollFd{{Fd: int32(r), Events: unix.POLLIN}}
		if _, err := unix.Poll(fds, int(left.Milliseconds())+1); err != nil && err != unix.EINTR {
			return nil, err
		}
		if fds[0].Revents == 0 {
			continue
		}
		n, err := unix.Read(r, buf)
		if n > 0 {
			out = append(out, buf[:n]...)
		}
		if n == 0 || (err != nil && err != unix.EINTR && err != unix.EAGAIN) {
			return out, nil
		}
	}
}

// setText makes text the selection.
func (cb *clipboard) setText(text string, serial uint32) {
	cb.own(serial, textTypes, func() { cb.ownText, cb.ownPNG, cb.ownsImage = text, nil, false })
}

// setImage makes img the selection, as a PNG.
func (cb *clipboard) setImage(img image.Image, serial uint32) {
	var buf bytes.Buffer
	if img == nil || png.Encode(&buf, img) != nil {
		return
	}
	cb.own(serial, []string{pngType}, func() { cb.ownText, cb.ownPNG, cb.ownsImage = "", buf.Bytes(), true })
}

// own offers a new source with the MIME types and makes it the selection.
func (cb *clipboard) own(serial uint32, types []string, set func()) {
	if cb.device == nil {
		return
	}
	if cb.source != nil {
		cb.source.Send(1) // destroy
		cb.source.Destroyed()
	}
	set()
	src := cb.d.conn.NewProxy("wl_data_source", cb.manager.Version, nil)
	src.On = func(op uint16, d *wl.Decoder) {
		switch op {
		case 1: // send(mime_type, fd)
			mime := d.String()
			fd := d.FD()
			if fd < 0 {
				return
			}
			var data []byte
			if mime == pngType {
				data = cb.ownPNG
			} else {
				data = []byte(cb.ownText)
			}
			// Written off the UI thread: the reader may be slow, and a pipe holds
			// only so much.
			go writeAll(fd, data)
		case 2: // cancelled: another client took the selection
			if cb.source == src {
				cb.source = nil
			}
			src.Send(1) // destroy
			src.Destroyed()
		}
	}
	cb.manager.Send(0, wl.NewID{P: src}) // create_data_source
	for _, t := range types {
		src.Send(0, t) // offer
	}
	cb.device.Send(1, src, serial) // set_selection
	cb.source = src
	cb.d.conn.Flush()
}

func writeAll(fd int, data []byte) {
	defer unix.Close(fd)
	for len(data) > 0 {
		n, err := unix.Write(fd, data)
		if err == unix.EINTR || err == unix.EAGAIN {
			continue
		}
		if err != nil || n <= 0 {
			return
		}
		data = data[n:]
	}
}
