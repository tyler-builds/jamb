// Package dmabuf describes the hand-off of GPU-rendered frames to a window
// system as Linux dma-bufs. It holds only types, so the platform adapter that
// shows the buffers (platform/linux) and the renderer that draws them
// (render/vulkan) need not import each other.
package dmabuf

// FormatARGB8888 is DRM_FORMAT_ARGB8888: premultiplied BGRA in memory on a
// little-endian machine, Vulkan's VK_FORMAT_B8G8R8A8_UNORM.
const FormatARGB8888 uint32 = 'A' | 'R'<<8 | '2'<<16 | '4'<<24

// ModLinear is DRM_FORMAT_MOD_LINEAR: rows one after another, which every GPU
// can read, so it is the layout for sharing an image between two GPUs.
const ModLinear uint64 = 0

// ModInvalid is DRM_FORMAT_MOD_INVALID: a buffer whose layout the driver
// implies rather than states. Such buffers are not supported.
const ModInvalid uint64 = 0x00ffffffffffffff

// Plane is one memory plane of an image: where it starts in the buffer and the
// bytes per row.
type Plane struct {
	Offset, Stride uint32
}

// Image is a GPU image exported as a dma-buf.
type Image struct {
	// FD is the dma-buf. Import does not take ownership: the caller closes it
	// after Import returns.
	FD            int
	Width, Height int
	Format        uint32 // a DRM fourcc, such as FormatARGB8888
	Modifier      uint64 // the DRM format modifier: the image's tiling and compression
	Planes        []Plane
}

// Device is a DRM device number (dev_t), split as major and minor.
type Device struct {
	Major, Minor uint32
}

// Buffer is an image the window system has imported.
type Buffer interface {
	// Busy reports whether the window system may still read the buffer: it was
	// presented and not released yet. A renderer must not draw into it.
	Busy() bool
	// Destroy lets go of the buffer, once the window system is done with it.
	Destroy()
}

// Surface shows a window's frames from dma-bufs.
type Surface interface {
	// Modifiers returns the modifiers the window system can import for format.
	Modifiers(format uint32) []uint64
	// MainDevice returns the device the window system composites with, when it
	// says. A renderer on that device avoids a copy between GPUs.
	MainDevice() (Device, bool)
	// Probe reports whether the window system can import img, without
	// keeping it. A rejection is an error, where Import's is fatal to the
	// connection, so a renderer probes a kind of image (its device, modifier
	// and memory) before it imports images of that kind. Probe waits for the
	// window system's answer.
	Probe(img Image) error
	// Import makes a buffer of img. The window system must accept it (see
	// Probe): a rejection ends the connection.
	Import(img Image) (Buffer, error)
	// Present shows buf as the window's next frame. The GPU must have finished
	// drawing it.
	Present(buf Buffer) error
}
