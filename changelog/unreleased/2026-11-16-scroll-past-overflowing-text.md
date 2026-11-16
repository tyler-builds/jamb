### Fixed

- A scroll container scrolls to the end of a text that runs past its box, such as a `NoWrap` line
  wider than the container: the text's overflow counts toward the container's scrollable size, as
  in CSS. A markdown code block with a long line now scrolls sideways and shows its scrollbar,
  where it used to cut the line off at its edge.
- The part of a text that runs past its box is drawn, and takes clicks and selection, even when
  the box itself is scrolled out of view.
- A wheel or touchpad scroll scrolls each axis in the innermost container that can scroll along
  it: a diagonal scroll over a code block that scrolls only sideways also scrolls the page.
- An absolutely positioned child that fills its parent no longer makes the parent's content wider
  and taller than its box by the parent's padding.
