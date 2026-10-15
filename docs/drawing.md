# Drawing

Gradients, borders, the canvas, charts, SVG images and the built-in icons.

## Gradients

```go
ui.Div().BackgroundGradient(ui.LinearGradient(135, from, to))  // replaces Background
ui.LinearGradient(90, from, to).Stops(0.2, 0.8)  // the colors sit at 20% and 80% of the gradient line
ui.LinearGradient(135, from, to).InOklab()       // interpolate in Oklab
```

- The angle is CSS's: 0 points up (the gradient runs from bottom to top), 90
  points right, 180 points down, 135 points to the bottom right. The gradient
  line passes through the center of the box, and its length is the box
  projected onto it, so the corners get the end colors.
- Colors are mixed in sRGB by default, as in GPUI. `InOklab()` mixes in Oklab,
  which stays bright between saturated colors: blue to yellow passes through
  green rather than gray.
- Opacity fades both colors. Borders and shadows stay solid. A gradient is also
  a `Style` field, so `Hover`, `Pressed` and `Focused` may set one, and it
  replaces `Background` when both are set.
- Gradient colors do not animate. `Transition` does not ease them, so a hover
  that changes the gradient snaps to the new one.

## Borders and rounded clipping

```go
ui.Div().Border(1, c).Radius(12)                          // one width on every side
ui.Div().BorderEdges(ui.Edges{Bottom: 2}, accent)         // an underline: a width per side
ui.Div().BorderEdges(ui.Edges{Left: 3}, accent).Radius(8) // a left accent bar; its corners curve into the sides
ui.Div().Radius(14).ScrollY()                             // children are clipped to the rounded shape
```

- `BorderEdges(e, c)` takes the width of each side (top, right, bottom, left);
  a zero side has no border. One color serves every side. `Border` and
  `BorderEdges` replace each other: the one set last wins, which is also how
  `Style.Merge` treats the `BorderWidth` and `BorderEdges` fields.
- A border side takes up space like padding, so the content moves in by its
  width, and each corner's inner edge curves from one side into the next. The
  inner radius of a corner is its radius less the average of its two sides'
  widths: exact for a uniform border, as in CSS, and an approximation for
  mixed widths.
- Border widths are layout, so they do not ease with `Transition`. The color
  does.
- A container with a `Radius` that clips (`ClipContent`, `ScrollY`, `ScrollX`)
  clips its children to the rounded shape inside its border. Edges are
  anti-aliased, and text, images and gradients are clipped the same way.
- Only the innermost rounded clip keeps its corners. Where a rounded container
  sits inside another rounded one, the inner corners are applied and the outer
  ones are not, so content can show past the outer corners near them. Avoid
  nesting rounded clips closely, or leave the outer container square.
- Hit testing and scrolling still use the rectangle, so a press in a corner
  that the rounding hides reaches the content there.

## Canvas

`ui.Canvas(draw)` is an element you paint with shapes, as an HTML canvas: charts,
graphs, diagrams and custom widgets. `draw` gets a `*ui.Painter` and the canvas's
content size in logical pixels:

```go
ui.Canvas(func(p *ui.Painter, w, h float32) {
	area := ui.NewPath().MoveTo(0, h)
	for i, v := range values {
		area.LineTo(float32(i)*w/float32(len(values)-1), h-v*h)
	}
	p.Stroke(area, ui.StrokeStyle{Width: 2, Color: t.Accent, Join: ui.JoinRound})
	area.LineTo(w, h).Close()
	p.Fill(area, ui.LinearPaint(0, 0, 0, h,
		ui.ColorStop{At: 0, Color: t.Accent.Alpha(0.4)}, ui.ColorStop{At: 1, Color: t.Accent.Alpha(0)}))
	p.Text(4, 4, "Visits", ui.TextFormat{Color: t.TextMuted})
}).Size(ui.Full, ui.Px(160)).Label("Visits this week, rising")
```

- **Paths.** `MoveTo`, `LineTo`, `QuadTo`, `CubicTo`, `Arc`, SVG's `ArcTo`, `Close`,
  and `Rect`, `RoundRect`, `Ellipse` and `Circle`. `EvenOdd` makes nested subpaths
  holes.
- **Painting.** `Fill` and `FillRect` take a `Paint`: a `Color`, `LinearPaint` or
  `RadialPaint` (with `Focus`). `Stroke` takes a `StrokeStyle`: width, caps, joins,
  miter limit and dashes.
- **State.** `Save` and `Restore` around `Translate`, `Scale`, `Rotate`, `Clip` and
  `Alpha`.
- **Text.** `Text` places one line at a point, aligned with `Align` and `VAlign`,
  in the inherited text color unless `Color` is set (`p.TextColor()` returns it,
  to draw shapes that match), and `MeasureText` sizes it. Text uses the text renderer (ClearType where it is
  on) and is drawn over the shapes; it follows the transform's position but is
  not rotated, scaled or clipped.

Shapes are drawn on the CPU (`internal/vector`) at the display's resolution, and
the drawing is kept: `draw` runs again only when the component that made the
canvas renders, or when its size, the display scale or the zoom changes. Read the
signals the drawing depends on in the component's render. A chart of 1000 points
at 800x400 pixels takes about 10 ms to draw.

The canvas is a `Div` (300x150 by default): size it with `Size`, take the pointer
with `OnMouseMove` and the others, whose `LocalX` and `LocalY` are canvas
coordinates when it has no padding or border, and add children to lay elements
over the drawing. Assistive technology sees an image named by `Label`.

## Charts

The `charts` package builds line, area, bar, scatter, pie and donut, radar, candlestick
and Sankey charts on the canvas, with tooltips, a keyboard walk that announces each
point, legend toggles, a data table for the exact values, and downsampling for large
series. See [docs/charts.md](charts.md).

## SVG images

`ui.SVG(src)` draws an SVG file, sharp at any size, scale and zoom:

```go
//go:embed logo.svg
var logoSVG []byte
var logo = ui.MustSVG(logoSVG) // parse once

ui.SVG(logo).Size(ui.Px(120), ui.Px(40)).Alt("Acme")
```

It takes the image's own size (width and height, else the viewBox) unless `Size`
sets one, and fits the image in its box as its `preserveAspectRatio` says.
`Fit(ui.FitCover)`, `Fit(ui.FitContain)` and `Fit(ui.FitFill)` place it as they place
an `ui.Image` instead: cover fills the box and crops the image, contain shows all of
it, letterboxed, and fill stretches it. `Radius(r)` rounds its corners (those of the
letterboxed image under contain), and `Opacity(o)` fades it, as for an `ui.Image`.
`currentColor` is the inherited text color, as for an `Icon`, so monochrome icon
sets such as Lucide's or Heroicons' SVG files follow the text and the theme;
`Color` sets it instead, and high contrast always uses the text color. Without
`Alt` the image is decorative and left out of the access tree.

What is drawn: paths and the basic shapes, groups, `use` and `symbol`, transforms,
fills and strokes (caps, joins, dashes) in colors or linear and radial gradients,
fill rules, opacity, clip paths, and styles from presentation attributes, `style`
attributes and style sheets whose selectors are a type, a class or an id (as
Illustrator and Figma write them). What is not: text, filters, masks, patterns,
markers, embedded images and animation. An SVG is drawn with the canvas's
rasterizer and drawn again only when its size, scale or color changes, so a list
of icons does not redraw when its component renders.

## Icons

`icons.All` lists all of them:

> activity, alert, archive, arrow-left, arrow-right, arrow-up-right, bell,
> bookmark, box, calendar, check, chevron-down, chevron-left, chevron-right,
> chevron-up, circle, clock, copy, dashboard, download, eye, file, filter,
> folder, git-branch, hash, heart, home, inbox, info, layers, link, lock,
> maximize, menu, minimize, minus, moon, more-horizontal, pencil, pin, plus,
> refresh, restore, search, settings, star, star-filled, sun, tag, trash, upload, users, x, zap

The Go names are the CamelCase forms: `icons.ArrowLeft`, `icons.GitBranch`,
and so on.
