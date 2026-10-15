package charts

// Axis configures an axis of a chart: its label, its range, its tick format and
// count, and its grid lines. The zero value is an automatic axis. Fix either end of
// the range with Min and Max, which return the axis so that they chain:
//
//	chart.YAxis(charts.Axis{Label: "Visits", Format: formatCount}.Min(0).Max(400))
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

	min, max       float64
	hasMin, hasMax bool
}

// GridLines says whether an axis draws grid lines across the plot.
type GridLines uint8

const (
	GridAuto GridLines = iota // on the value axis, off on the position axis
	// GridOn draws grid lines across the plot, whatever the axis is.
	GridOn
	// GridOff draws no grid lines across the plot.
	GridOff
)

// Min fixes the start of the axis at lo, instead of the start of the data. The ticks
// then start at lo, and the data below it is not drawn. Max does the same for the end
// of the axis. Either one can be set alone; the other end follows the data.
func (a Axis) Min(lo float64) Axis {
	a.min, a.hasMin = lo, true
	return a
}

// Max fixes the end of the axis at hi, instead of the end of the data (see Min).
func (a Axis) Max(hi float64) Axis {
	a.max, a.hasMax = hi, true
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
