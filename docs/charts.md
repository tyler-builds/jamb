# Charts

The `charts` package draws charts on `ui.Canvas`: line, area, bar (grouped and
stacked, vertical and horizontal), scatter, pie and donut, radar, candlestick and
Sankey charts. The package doc (`go doc github.com/tyler-builds/jamb/charts`) is
the reference for every method; this page covers how the pieces fit.

## Building a chart

A chart is built from data with a constructor, configured with chained methods,
and placed in a tree as its `Element`:

```go
ui.Div().Child(charts.Line(charts.Series[Visit]{
	Name: "Visits",
	Data: visits,
	Time: func(v Visit) time.Time { return v.Day },
	Y:    func(v Visit) float64 { return float64(v.Count) },
}).Title("Visits this week").
	YAxis(charts.Axis{Label: "Visits"}.Min(0).Max(400)).
	Size(ui.Full, ui.Px(300)))
```

A chart is a `ui.Element` (it embeds `ui.Composite`), so it goes straight into a tree, and it keeps its state between renders; give it a `Key` when its place in the tree may change.

| Constructor | Data | Options that apply |
| --- | --- | --- |
| `Line`, `Area` | `Series[T]` per line | `Smooth`; `Stacked` (area) |
| `Bar` | `Series[T]` per bar group | `Stacked`, `Horizontal` |
| `Scatter` | `Series[T]` with `X` and `Y` | axes |
| `Pie`, `Donut` | one `Series[T]`, one slice per point | `Legend` |
| `Radar` | `Series[T]` per polygon, one axis per position | `Stacked` and `Smooth` do nothing |
| `Candlestick` | `CandleSeries[T]` with four value functions | axes |
| `Sankey` | `SankeyLink` values | none (no legend) |

Options that a chart does not use do nothing. `Horizontal` turns bars only.

## Data and axes

A `Series[T]` places each point on the horizontal axis with one of `Time`,
`Category` or `X`, and gives its value with `Y`. A chart's series share one kind
of horizontal axis: time, category or number. Mixing kinds panics, as a
programming error. With none of the three functions, a point is placed at its
index in `Data`.

- **Category** axes place the categories in the order they first appear.
- **Time** axes show ticks on whole seconds, days, months or years of the time
  zone of the data. The labels follow the user's region settings: `ui.FormatDate` for
  days, `ui.FormatMonthYear` for months, `ui.FormatTime` for times of day, and
  the year alone for years. Ticks under a minute apart show `15:04:05`.
- **Number** axes use nice ticks in steps of 1, 2 or 5 times a power of ten.

`Axis` configures an axis: `Label`, `Format`, the desired `Ticks`, `Grid`
(`GridAuto` draws grid lines on the value axis only), and `Min` and `Max` to fix
the ends of the range. Tick labels are measured with the painter, so the margins hold them,
and labels that would overlap are thinned out.

Tooltips, the summary and the keyboard announcements format times the same way.
Use `Axis.Format` or `Chart.Format` for another form.

## Interaction

- **Hover** shows a tooltip at the pointer: the position's label, and each shown
  series' value there. The tooltip is a real element, an overlay on the chart.
  A line, area or scatter plot shows a crosshair, and a bar chart highlights the
  slot.
- **Keyboard.** The chart is one focus stop. Its first shown point is the start.
  Left and Right move along the points of the series. Up and Down switch to the
  next or previous series at the same place. Each step is announced with
  `ui.Announce`, such as "Tuesday, Visits 180". A focus ring marks the point.

  | Chart | Keys |
  | --- | --- |
  | Line, area, scatter, candlestick | Left and Right walk; Up and Down switch series |
  | Bar, vertical | Left and Right walk the slots; Up and Down switch series |
  | Bar, horizontal | Up and Down walk the slots (they run down the chart); Left and Right switch series |
  | Radar | Left and Right walk the axes; Up and Down switch series |
  | Pie, donut | any arrow moves to the previous or next slice |
  | Sankey | any arrow moves to the previous or next link |

- **Legend.** Each legend item is a toggle button: its pressed state is whether
  its series or slice is shown, and a click or Enter hides or shows it. A hidden
  item is faded; its text keeps its weight.

Lines and areas are drawn from a downsampled copy of their points, so the pointer
and the keyboard use the points of the data, not the downsampled ones.

## Accessibility

A chart is an image named by its summary, made from its data, for example:

> Line chart, 2 series, 7 points each, from Monday to Sunday; Visits from 120 to 300, Returns from 10 to 40

`Label` replaces the summary. `charts.DataTable(chart)` shows the same data in a
`ui.DataGrid`, with a column for each series, so a screen reader reads the exact
values. It shows every series, including hidden ones. The app can show it in
place of the chart, as the demo's Charts section does.

## Colors and high contrast

Series colors come from a palette for each kind of theme: light, dark and high
contrast. Every palette color is at least 3:1 against the light and the dark
surfaces, as the package tests check. Series also differ in shape: each has its
own dash pattern and marker, and under high contrast every series is the text
color, with its own hatch on bars, areas and slices. Axis labels use the theme's
`TextMuted`, and grid lines the theme's `Border`.

## Motion

On the first draw a chart animates in over 600 milliseconds: lines and areas reveal
from the left, bars, slices and radar polygons grow from their base, and a Sankey
diagram fades in. Scatter plots and candlesticks are drawn at once. With the
reduced motion setting the chart is drawn in full at once.

## Parts

The parts of a chart are set with `Chart.Styles(charts.Style{...})`. A part's style
merges over the built-in defaults, then over the chart's `Styles`. Under high
contrast the colors are the system colors, whatever is set; sizes, spacing and
fonts still apply.

| Part | Applies to | Notes |
| --- | --- | --- |
| `Chart` | the box around the title, canvas and legend | background, border, radius, padding, gap |
| `Title` | the title text | text color, size, weight |
| `Axis` | axis labels and axis titles | `TextColor`, `FontSize` |
| `Grid` | grid lines | `BorderColor` is the line color, `BorderWidth` its width |
| `Legend` | the row or column of legend items | gap, wrap |
| `LegendItem` | each legend button | padding, radius, gap, text |
| `Tooltip` | the tooltip box | background, border, shadow, padding, text color |

The parts are documented here and in the package doc. They are not listed in
`docs/styling.md`, so `TestEveryPartIsApplied` does not cover them.

## Performance

Lines and areas are reduced with Largest-Triangle-Three-Buckets to about two points
per pixel of the plot's length, before they are drawn. The reduction of 100,000
points takes about 0.3 ms (`BenchmarkLTTB`).

The crosshair, the hover highlight and the tooltip are drawn on a layer of their
own, above the plot. Moving the pointer re-renders only that layer, so the plot's
canvas is not redrawn: a hover frame on a 100,000-point line or area takes about
0.5 ms on the development machine. A frame that redraws the plot itself, such as
a resize or a change of data, takes about 12 ms for 100,000 points; most of that
is the vector rasterizer in `internal/vector` sorting the crossings of the stroke.

## Limits

- Scatter plots draw every point. They are not downsampled.
- Candlestick and scatter charts do not animate on their first draw.
- A Sankey diagram has no node tooltips; its links carry the tooltips and the
  keyboard walk.
- A pie without `Category` labels its slices with their index numbers.
- The downsampled copy is drawn for lines and areas. The table and the keyboard
  use every point.
- A chart's summary and labels are written from its data as supplied. Name and
  label values are not checked.
