//go:build linux

// Package wl is a small Wayland client: the wire protocol over the
// compositor's Unix socket, in pure Go, with no libwayland.
//
// An object is a *Proxy with an id and an event handler. Requests are sent with
// Proxy.Send, whose arguments are encoded by their Go type (see Send), and
// events are decoded by the handler from a *Decoder in the order the protocol
// lists their arguments. The package knows only wl_display; the platform
// adapter speaks the other interfaces it uses, with opcodes from their
// protocol XML.
//
// A Conn is used from one goroutine, the UI thread.
package wl

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"
)

// Fixed is a wl_fixed_t argument: a signed 24.8 fixed-point number.
type Fixed float64

// FD is a file descriptor argument. The descriptor is sent with the message
// and stays open on the sending side; the caller closes it.
type FD int

// NewID is a new_id argument for a request whose interface the protocol
// fixes: the new object's proxy.
type NewID struct{ P *Proxy }

// Handler decodes the events of an object. d holds the event's arguments.
type Handler func(opcode uint16, d *Decoder)

// Proxy is the client side of a protocol object.
type Proxy struct {
	c  *Conn
	ID uint32
	// Interface names the object's interface, for errors.
	Interface string
	// On receives the object's events; nil ignores them.
	On Handler
	// Version is the version the object was bound or created with.
	Version uint32
	dead    bool
}

// Conn is a connection to a Wayland compositor.
type Conn struct {
	fd      int
	objects map[uint32]*Proxy
	free    []uint32
	next    uint32
	out     []byte
	outFDs  []int
	in      []byte
	inFDs   []int
	buf     []byte
	oob     []byte
	// Display is wl_display, object 1.
	Display *Proxy
	err     error
}

// Connect opens the compositor's socket: WAYLAND_SOCKET if set, else
// WAYLAND_DISPLAY (default wayland-0), relative to XDG_RUNTIME_DIR unless it
// is absolute.
func Connect() (*Conn, error) {
	if s := os.Getenv("WAYLAND_SOCKET"); s != "" {
		fd, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("wl: WAYLAND_SOCKET %q is not a descriptor", s)
		}
		os.Unsetenv("WAYLAND_SOCKET")
		unix.CloseOnExec(fd)
		return newConn(fd), nil
	}
	name := os.Getenv("WAYLAND_DISPLAY")
	if name == "" {
		name = "wayland-0"
	}
	path := name
	if !filepath.IsAbs(name) {
		dir := os.Getenv("XDG_RUNTIME_DIR")
		if dir == "" {
			return nil, errors.New("wl: XDG_RUNTIME_DIR is not set")
		}
		path = filepath.Join(dir, name)
	}
	fd, err := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("wl: socket: %w", err)
	}
	if err := unix.Connect(fd, &unix.SockaddrUnix{Name: path}); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("wl: connecting to %s: %w", path, err)
	}
	return newConn(fd), nil
}

func newConn(fd int) *Conn {
	c := &Conn{fd: fd, objects: map[uint32]*Proxy{}, next: 2,
		buf: make([]byte, 64<<10), oob: make([]byte, unix.CmsgSpace(28*4))}
	c.Display = &Proxy{c: c, ID: 1, Interface: "wl_display", Version: 1}
	c.objects[1] = c.Display
	c.Display.On = c.displayEvent
	return c
}

// FD returns the socket, for polling.
func (c *Conn) FD() int { return c.fd }

// Err returns the error that ended the connection, or nil.
func (c *Conn) Err() error { return c.err }

// Close closes the connection.
func (c *Conn) Close() error {
	if c.fd < 0 {
		return nil
	}
	err := unix.Close(c.fd)
	c.fd = -1
	for _, fd := range c.inFDs {
		unix.Close(fd)
	}
	c.inFDs = nil
	return err
}

// ProtocolError is a fatal error the compositor reported for an object.
type ProtocolError struct {
	Interface string
	Object    uint32
	Code      uint32
	Message   string
}

func (e *ProtocolError) Error() string {
	return fmt.Sprintf("wl: protocol error %d on %s@%d: %s", e.Code, e.Interface, e.Object, e.Message)
}

func (c *Conn) displayEvent(op uint16, d *Decoder) {
	switch op {
	case 0: // error(object_id: object, code: uint, message: string)
		id, code, msg := d.Uint(), d.Uint(), d.String()
		iface := "?"
		if p := c.objects[id]; p != nil {
			iface = p.Interface
		}
		c.err = &ProtocolError{Interface: iface, Object: id, Code: code, Message: msg}
	case 1: // delete_id(id: uint)
		id := d.Uint()
		if p := c.objects[id]; p != nil {
			p.dead = true
		}
		delete(c.objects, id)
		if id < 0xff000000 {
			c.free = append(c.free, id)
		}
	}
}

// NewProxy makes a client object for a new_id argument. The id is taken now;
// it is used once the request that creates it is sent.
func (c *Conn) NewProxy(iface string, version uint32, on Handler) *Proxy {
	var id uint32
	if n := len(c.free); n > 0 {
		id = c.free[n-1]
		c.free = c.free[:n-1]
	} else {
		id = c.next
		c.next++
	}
	p := &Proxy{c: c, ID: id, Interface: iface, Version: version, On: on}
	c.objects[id] = p
	return p
}

// Adopt makes the client side of an object the compositor created, whose id
// arrived as a new_id argument of an event (for example wl_data_offer).
func (c *Conn) Adopt(id uint32, iface string, version uint32, on Handler) *Proxy {
	p := &Proxy{c: c, ID: id, Interface: iface, Version: version, On: on}
	c.objects[id] = p
	return p
}

// Destroyed marks p as gone after a destructor request, so its events are
// dropped. A client id stays reserved until the compositor's delete_id frees
// it; the id of an object the compositor created is forgotten at once.
func (p *Proxy) Destroyed() {
	p.dead = true
	if p.ID >= 0xff000000 && p.c.objects[p.ID] == p {
		delete(p.c.objects, p.ID)
	}
}

// Send queues a request. Its arguments are encoded by type:
//
//	int32 → int; uint32 → uint; Fixed → fixed; string → string (NUL-terminated);
//	[]byte → array; *Proxy → object (nil for a null object); NewID → new_id;
//	FD → fd.
//
// Requests are written by Flush.
func (p *Proxy) Send(opcode uint16, args ...any) {
	c := p.c
	start := len(c.out)
	c.out = binary.LittleEndian.AppendUint32(c.out, p.ID)
	c.out = binary.LittleEndian.AppendUint32(c.out, 0) // size and opcode, below
	for _, a := range args {
		switch v := a.(type) {
		case int32:
			c.out = binary.LittleEndian.AppendUint32(c.out, uint32(v))
		case uint32:
			c.out = binary.LittleEndian.AppendUint32(c.out, v)
		case Fixed:
			c.out = binary.LittleEndian.AppendUint32(c.out, uint32(int32(math.Round(float64(v)*256))))
		case string:
			c.out = binary.LittleEndian.AppendUint32(c.out, uint32(len(v)+1))
			c.out = append(c.out, v...)
			c.out = append(c.out, 0)
			c.out = pad(c.out)
		case []byte:
			c.out = binary.LittleEndian.AppendUint32(c.out, uint32(len(v)))
			c.out = append(c.out, v...)
			c.out = pad(c.out)
		case *Proxy:
			var id uint32
			if v != nil {
				id = v.ID
			}
			c.out = binary.LittleEndian.AppendUint32(c.out, id)
		case NewID:
			c.out = binary.LittleEndian.AppendUint32(c.out, v.P.ID)
		case FD:
			c.outFDs = append(c.outFDs, int(v))
		default:
			panic(fmt.Sprintf("wl: argument of type %T", a))
		}
	}
	size := len(c.out) - start
	binary.LittleEndian.PutUint32(c.out[start+4:], uint32(size)<<16|uint32(opcode))
}

func pad(b []byte) []byte {
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

// Bind binds a global from wl_registry: registry.bind(name, interface,
// version, new_id). The interface and version travel with the new id, since
// the request's new_id has no fixed interface.
func (c *Conn) Bind(registry *Proxy, name uint32, iface string, version uint32, on Handler) *Proxy {
	p := c.NewProxy(iface, version, on)
	registry.Send(0, name, iface, version, NewID{p})
	return p
}

// Flush writes the queued requests.
func (c *Conn) Flush() error {
	for len(c.out) > 0 {
		var oob []byte
		if len(c.outFDs) > 0 {
			oob = unix.UnixRights(c.outFDs...)
		}
		n, err := unix.SendmsgN(c.fd, c.out, oob, nil, unix.MSG_NOSIGNAL)
		if err == unix.EINTR {
			continue
		}
		if err == unix.EAGAIN {
			// The socket is full: wait until it drains.
			fds := []unix.PollFd{{Fd: int32(c.fd), Events: unix.POLLOUT}}
			if _, err := unix.Poll(fds, -1); err != nil && err != unix.EINTR {
				return c.fail(err)
			}
			continue
		}
		if err != nil {
			return c.fail(err)
		}
		c.outFDs = c.outFDs[:0]
		c.out = c.out[n:]
	}
	c.out = c.out[:0]
	return nil
}

func (c *Conn) fail(err error) error {
	if c.err == nil {
		c.err = fmt.Errorf("wl: %w", err)
	}
	return c.err
}

// Read reads what the socket holds, waiting for data if block is set, and
// dispatches every complete event to its object's handler.
func (c *Conn) Read(block bool) error {
	flags := unix.MSG_CMSG_CLOEXEC
	if !block {
		flags |= unix.MSG_DONTWAIT
	}
	n, oobn, _, _, err := unix.Recvmsg(c.fd, c.buf, c.oob, flags)
	if err == unix.EAGAIN || err == unix.EINTR {
		return nil
	}
	if err != nil {
		return c.fail(err)
	}
	if n == 0 {
		return c.fail(errors.New("the compositor closed the connection"))
	}
	if oobn > 0 {
		msgs, err := unix.ParseSocketControlMessage(c.oob[:oobn])
		if err == nil {
			for _, m := range msgs {
				if fds, err := unix.ParseUnixRights(&m); err == nil {
					c.inFDs = append(c.inFDs, fds...)
				}
			}
		}
	}
	c.in = append(c.in, c.buf[:n]...)
	c.dispatch()
	return c.err
}

func (c *Conn) dispatch() {
	for len(c.in) >= 8 && c.err == nil {
		id := binary.LittleEndian.Uint32(c.in)
		word := binary.LittleEndian.Uint32(c.in[4:])
		size, op := int(word>>16), uint16(word)
		if size < 8 || size%4 != 0 {
			c.err = fmt.Errorf("wl: malformed message of %d bytes", size)
			return
		}
		if len(c.in) < size {
			return
		}
		d := &Decoder{c: c, b: c.in[8:size]}
		if p := c.objects[id]; p != nil && !p.dead && p.On != nil {
			p.On(op, d)
		}
		// The wire does not say which event a descriptor belongs to, so the
		// handlers of every event that carries one (wl_keyboard.keymap,
		// wl_data_source.send) must take it with FD, or later events would get
		// the wrong descriptors.
		c.in = c.in[size:]
	}
	if len(c.in) == 0 {
		c.in = c.in[:0:0]
	}
}

// Roundtrip sends wl_display.sync and dispatches events until its callback
// is done, so every request before it has been handled.
func (c *Conn) Roundtrip() error {
	done := false
	cb := c.NewProxy("wl_callback", 1, func(op uint16, d *Decoder) { done = true })
	c.Display.Send(0, NewID{cb}) // sync
	if err := c.Flush(); err != nil {
		return err
	}
	for !done {
		if err := c.Read(true); err != nil {
			return err
		}
	}
	return nil
}

// Decoder reads an event's arguments in order.
type Decoder struct {
	c   *Conn
	b   []byte
	bad bool
}

func (d *Decoder) word() uint32 {
	if len(d.b) < 4 {
		d.bad = true
		return 0
	}
	v := binary.LittleEndian.Uint32(d.b)
	d.b = d.b[4:]
	return v
}

// Uint reads a uint, object or new_id argument.
func (d *Decoder) Uint() uint32 { return d.word() }

// Int reads an int argument.
func (d *Decoder) Int() int32 { return int32(d.word()) }

// Fixed reads a fixed argument.
func (d *Decoder) Fixed() float64 { return float64(int32(d.word())) / 256 }

// String reads a string argument.
func (d *Decoder) String() string {
	b := d.Array()
	if n := len(b); n > 0 && b[n-1] == 0 {
		b = b[:n-1]
	}
	return string(b)
}

// Array reads an array argument. The bytes are valid only during the handler.
func (d *Decoder) Array() []byte {
	n := int(d.word())
	padded := (n + 3) &^ 3
	if n < 0 || len(d.b) < padded {
		d.bad = true
		return nil
	}
	b := d.b[:n]
	d.b = d.b[padded:]
	return b
}

// FD takes the next descriptor that arrived with the events. The handler owns
// it and must close it.
func (d *Decoder) FD() int {
	if len(d.c.inFDs) == 0 {
		d.bad = true
		return -1
	}
	fd := d.c.inFDs[0]
	d.c.inFDs = d.c.inFDs[1:]
	return fd
}

// Bad reports whether an argument was missing or malformed.
func (d *Decoder) Bad() bool { return d.bad }
