### Fixed

- Linux: the Vulkan renderer no longer crashes when a frame's text fills the glyph atlas, which then
  grows. Text drawn before and after the growth shares one texture.
