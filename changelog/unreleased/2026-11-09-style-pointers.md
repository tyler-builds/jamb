### Changed

- The widgets pass their looks through pointers: the defaults and the resolved look of a widget
  are filled in place, and its parts reach the elements by pointer. TinyGo passes a struct by value
  as one parameter per field and copies one field by field, so every such call was kilobytes of
  code. A web build of the demo is 2.31 MB in Brotli, down from 2.68 MB, and the smallest app
  1.05 MB, down from 1.12 MB. On the desktop a frame full of widgets builds 6% faster, with the
  same allocations.
