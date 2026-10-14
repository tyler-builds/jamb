package charts

// Axis configures an axis of a chart: its label, its range, its tick format and
// count, and its grid lines. The zero value is an automatic axis. Set the range
// with Range, which returns the axis so that it chains:
//
//	charts.YAxis(charts.Axis{Label: "Visits", Format: formatCount}.Range(0, 400))
type Axis struct {
	// Label is the title of the axis, drawn beside its labels.
	Label string
	// Format returns the text of a tick. Nil means the chart's Format for a value
	// axis, and the date, time or number of a position axis.
	Format func(float64) string
	// Ticks is how many ticks to aim for. Zero means as many as fit without their
	// labels overlapping.
	Ticks int
	// Grid says whether the axis draws grid lines across the plot. GridAuto draws
	// them on the value axis only.
	Grid GridLines

	lo, hi float64
	ranged bool
}

// GridLines says whether an axis draws grid lines across the plot.
type GridLines uint8

const (
	GridAuto GridLines = iota // on the value axis, off on the position axis
	GridOn
	GridOff
)

// Range fixes the range of the axis to lo to hi, instead of the range of the data.
// The ticks then run from lo to hi, and the data outside it is not drawn.
func (a Axis) Range(lo, hi float64) Axis {
	a.lo, a.hi, a.ranged = lo, hi, true
	return a
}

// LegendPosition is where the legend of a chart is shown.
type LegendPosition uint8

const (
	LegendBottom LegendPosition = iota // below the chart (the default)
	LegendTop                          // above the chart
	LegendRight                        // beside the chart, in a column
	LegendNone                         // not shown; the chart is still named and walkable
)
