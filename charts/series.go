package charts

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// Series is one series of a chart: its points, and the functions that read the
// position and the value of each point. Set one of Time, Category and X for the
// horizontal position of a point. Time places it on a time axis, which shows
// dates; Category names its place on a category axis, such as a day or a
// product, and the categories are placed in the order they first appear; X is a
// number. With none of them a point is placed at its index in Data.
//
//	charts.Line(charts.Series[Visit]{
//		Name:     "Visits",
//		Data:     visits,
//		Category: func(v Visit) string { return v.Day },
//		Y:        func(v Visit) float64 { return float64(v.Count) },
//	})
type Series[T any] struct {
	// Name is the name of the series, shown in the legend and in the table.
	Name string
	// Data are the data points of the series, in order.
	Data []T
	// X is the position of a point, for a chart with numeric positions.
	X func(T) float64
	// Time is the time of a point, for a chart with a time axis.
	Time func(T) time.Time
	// Category is the category of a point, for a chart with category positions.
	Category func(T) string
	// Y is the value of a point: the height of a line point, a bar or an area, or
	// the size of a slice. It is required.
	Y func(T) float64
	// Color is the color of the series' line, bars, area or markers, in place of the
	// palette's. The zero value takes the color of the palette for the series' place.
	// Under high contrast it is ignored, and the series is the text color like every
	// other. A pie or a donut colors its slices from the palette and ignores it. The
	// chart does not check that the colors an app sets differ from one another or
	// stand out against the surface.
	Color ui.Color
}

// CandleSeries is the data of a candlestick chart: one candle for each point,
// with its open, high, low and close values. Its position is set as for a
// Series, and the four value functions are required.
type CandleSeries[T any] struct {
	// Name is the name of the series, shown in the legend and in the table.
	Name string
	// Data are the candles of the series, in order.
	Data []T
	// X is the position of a candle, for numeric positions.
	X func(T) float64
	// Time is the time of a candle, for a time axis.
	Time func(T) time.Time
	// Category is the category of a candle, for category positions.
	Category func(T) string
	// Open is the opening value of a candle.
	Open func(T) float64
	// High is the highest value of a candle.
	High func(T) float64
	// Low is the lowest value of a candle.
	Low func(T) float64
	// Close is the closing value of a candle.
	Close func(T) float64
	// Rising is the color of a candle that rises, whose close is at or above its open.
	// The zero value is the theme's Success color. Under high contrast it is ignored: a
	// rising candle is hollow, in the text color.
	Rising ui.Color
	// Falling is the color of a candle that falls, whose close is below its open. The
	// zero value is the theme's Danger color. Under high contrast it is ignored: a
	// falling candle is solid, in the text color.
	Falling ui.Color
}

// xKind is how the points of a chart are placed on its horizontal axis.
type xKind uint8

const (
	xNumber xKind = iota
	xTime
	xCategory
)

// point is one point of a series. The candlestick values are set only for a
// candlestick chart.
type point struct {
	x, y                   float64
	open, high, low, close float64
}

// seriesData is the converted data of one series, with its points sorted by x
// unless the chart keeps their order (a pie does). lo and hi are the least and
// the greatest value, which are taken once here rather than on every draw.
type seriesData struct {
	name   string
	color  ui.Color // the Series' Color; zero takes the palette's
	pts    []point
	lo, hi float64
	// xs and ys are the coordinates of pts, kept apart for the downsampling.
	xs, ys []float64
}

// valueRange sets lo and hi to the least and the greatest value of the points.
// For a candlestick, candles is set and the range runs from the lowest low to the
// highest high.
func (s *seriesData) valueRange(candles bool) {
	s.lo, s.hi = 0, 0
	for i, p := range s.pts {
		a, b := p.y, p.y
		if candles {
			a, b = p.low, p.high
		}
		if i == 0 {
			s.lo, s.hi = a, b
			continue
		}
		s.lo, s.hi = min(s.lo, a), max(s.hi, b)
	}
}

// dataset is the data of a chart, converted from the caller's series. Every
// series places its points the same way: x is the axis kind, cats holds the
// categories by position, and loc is the time zone of the times. rising and
// falling are the candle colors of a candlestick chart, zero when unset.
type dataset struct {
	x               xKind
	cats            []string
	loc             *time.Location
	series          []seriesData
	rising, falling ui.Color
}

// placer assigns horizontal positions to points while the data is converted.
type placer struct {
	ds     *dataset
	catIdx map[string]int
}

// placeTime returns the position of the time t and records its time zone.
func (pl *placer) placeTime(t time.Time) float64 {
	if pl.ds.loc == nil {
		pl.ds.loc = t.Location()
	}
	return float64(t.Unix()) + float64(t.Nanosecond())/1e9
}

// placeCategory returns the position of the category name, which is added to
// the categories the first time it is seen.
func (pl *placer) placeCategory(name string) float64 {
	i, ok := pl.catIdx[name]
	if !ok {
		i = len(pl.ds.cats)
		pl.catIdx[name] = i
		pl.ds.cats = append(pl.ds.cats, name)
	}
	return float64(i)
}

// axisKind returns the kind of the axis that the functions of one series select.
func axisKind(hasTime, hasCategory bool) xKind {
	switch {
	case hasTime:
		return xTime
	case hasCategory:
		return xCategory
	}
	return xNumber
}

// setKind sets the kind of the chart's axis from the kinds of its series. A chart
// has one kind of axis, so series of different kinds panic, as a programming error.
func (ds *dataset) setKind(kinds []xKind) {
	for i, k := range kinds {
		if i == 0 {
			ds.x = k
		} else if k != ds.x {
			panic("charts: the series place their points on different kinds of axis (time, category or number)")
		}
	}
}

// finish sorts the points of each series by x, stably, unless keep is set, and
// takes the value range of each series.
func (ds *dataset) finish(keep, candles bool) {
	for i := range ds.series {
		s := &ds.series[i]
		if !keep {
			slices.SortStableFunc(s.pts, func(a, b point) int { return cmp.Compare(a.x, b.x) })
		}
		s.valueRange(candles)
		s.xs = make([]float64, len(s.pts))
		s.ys = make([]float64, len(s.pts))
		for j, p := range s.pts {
			s.xs[j], s.ys[j] = p.x, p.y
		}
	}
}

// newPlacer returns a placer for ds. Series of the chart are converted with it.
func newPlacer(ds *dataset) *placer {
	return &placer{ds: ds, catIdx: map[string]int{}}
}

// fromSeries converts the series of a chart. A point with a NaN position or
// value is left out, so it draws nothing. keep sets whether the points keep the
// order of the data, which a pie needs.
func fromSeries[T any](series []Series[T], keep bool) dataset {
	kinds := make([]xKind, len(series))
	for i, s := range series {
		if s.Y == nil {
			panic("charts: a series has no Y function")
		}
		kinds[i] = axisKind(s.Time != nil, s.Category != nil)
	}
	var ds dataset
	ds.setKind(kinds)
	pl := newPlacer(&ds)
	for _, s := range series {
		sd := seriesData{name: s.Name, color: s.Color, pts: make([]point, 0, len(s.Data))}
		for i, item := range s.Data {
			p := point{y: s.Y(item)}
			switch ds.x {
			case xTime:
				p.x = pl.placeTime(s.Time(item))
			case xCategory:
				p.x = pl.placeCategory(s.Category(item))
			default:
				p.x = float64(i)
				if s.X != nil {
					p.x = s.X(item)
				}
			}
			if math.IsNaN(p.x) || math.IsNaN(p.y) {
				continue
			}
			sd.pts = append(sd.pts, p)
		}
		ds.series = append(ds.series, sd)
	}
	ds.finish(keep, false)
	return ds
}

// fromCandles converts the candlestick series of a chart, as fromSeries does.
func fromCandles[T any](s CandleSeries[T]) dataset {
	if s.Open == nil || s.High == nil || s.Low == nil || s.Close == nil {
		panic("charts: a candlestick series has no Open, High, Low or Close function")
	}
	ds := dataset{rising: s.Rising, falling: s.Falling}
	ds.setKind([]xKind{axisKind(s.Time != nil, s.Category != nil)})
	pl := newPlacer(&ds)
	sd := seriesData{name: s.Name, pts: make([]point, 0, len(s.Data))}
	for i, item := range s.Data {
		p := point{open: s.Open(item), high: s.High(item), low: s.Low(item), close: s.Close(item)}
		p.y = p.close
		switch ds.x {
		case xTime:
			p.x = pl.placeTime(s.Time(item))
		case xCategory:
			p.x = pl.placeCategory(s.Category(item))
		default:
			p.x = float64(i)
			if s.X != nil {
				p.x = s.X(item)
			}
		}
		if math.IsNaN(p.x) || math.IsNaN(p.open) || math.IsNaN(p.high) || math.IsNaN(p.low) || math.IsNaN(p.close) {
			continue
		}
		sd.pts = append(sd.pts, p)
	}
	ds.series = []seriesData{sd}
	ds.finish(false, true)
	return ds
}

// bounds returns the least and the greatest x of the points of the series, and
// whether there are any.
func (ds *dataset) bounds(visible []int) (lo, hi float64, ok bool) {
	for _, k := range visible {
		pts := ds.series[k].pts
		if len(pts) == 0 {
			continue
		}
		a, b := pts[0].x, pts[len(pts)-1].x
		if !ok {
			lo, hi, ok = a, b, true
			continue
		}
		lo, hi = min(lo, a), max(hi, b)
	}
	return lo, hi, ok
}

// slots returns the distinct x positions of the visible series, ascending. They are
// the slots of a bar chart, and the rows of a table.
func (ds *dataset) slots(visible []int) []float64 {
	var xs []float64
	for _, k := range visible {
		for _, p := range ds.series[k].pts {
			xs = append(xs, p.x)
		}
	}
	slices.Sort(xs)
	return slices.Compact(xs)
}
