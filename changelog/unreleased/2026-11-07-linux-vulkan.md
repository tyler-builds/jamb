### Added

- Linux: windows are drawn on the GPU with Vulkan, loaded at run time, into dma-bufs handed to the
  Wayland compositor. A busy 1080p frame takes about 1.2 ms instead of 11 ms on the CPU. Without
  Vulkan, a qualifying GPU or the compositor's dma-buf support, windows are drawn on the CPU as
  before, and `JAMB_RENDERER=cpu` asks for that.
