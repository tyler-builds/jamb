### Fixed

- In a web page, a touch drags sliders, split dividers and the other elements that capture the
  pointer, instead of scrolling the page, and a touch scroll leaves no hover behind. An adapter asks
  the app with the new platform.TouchHandler whether a touch drags what is under it.
- In a web page, a touch scroll and its momentum move once per display frame, by what the finger
  moved since the last frame and by the momentum up to the frame's time, so they no longer judder
  when touch events and frames fall out of step.
