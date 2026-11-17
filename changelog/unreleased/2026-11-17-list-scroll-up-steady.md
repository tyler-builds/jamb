### Fixed

- A `ListOf` scrolled up no longer jumps as the rows coming into view are measured. It keeps in place
  the first row on screen that was already measured, at the offset drawn rather than the one a wheel
  scroll eases toward, so the rows being read stay put through wheel, touchpad, finger and fling
  scrolling.
- Dragging a scrollbar's thumb moves the content by as much as the pointer moved, so a `ListOf`'s
  corrections for rows measured on the way are no longer undone by the next move.
- A Bottom `ListOf` that is scrolled away from its end in the frame that items are added no longer
  snaps back to the end.
