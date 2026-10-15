package charts

import (
	"strconv"

	"github.com/tyler-builds/jamb/ui"
)

// tableRow is a row of a chart's table: its key, and the text of each of its cells.
type tableRow struct {
	key   string
	cells []string
}

// DataTable returns the data of the chart as a table, in a ui.DataGrid, for readers who
// need the exact values: a screen reader reads each cell, and a sighted reader can sort
// the rows. It shows every series, including those hidden in the chart's legend. The
// table is named by the chart's summary.
//
// A line, area, bar, scatter, candlestick or radar chart has a row for each position, with a
// column for each series (a candlestick has its four values); a pie has a row for each slice,
// with its value and share; and a Sankey diagram has a row for each link.
//
//	ui.Div().Column().Child(chart).Child(charts.DataTable(chart).Key("table").Height(ui.Px(240)))
func DataTable(c *Chart) *DataTableElement { return &DataTableElement{c: c} }

// DataTableElement is the table of a chart's data, made by DataTable and configured with the
// methods below, each of which returns the table so that they chain. It is a ui.Element, so it
// is placed in a tree as it is. Its size is the ui.DataGrid's default unless the sizing methods
// say otherwise.
type DataTableElement struct {
	ui.Composite
	c    *Chart
	size sizes // see Width
}

// Key gives the table a stable identity among its siblings, so the state of its grid
// survives reordering.
func (t *DataTableElement) Key(k string) *DataTableElement { t.SetKey(k); return t }

// The sizing methods give the table's grid a size, as they do for a ui.DataGrid. See
// Sizing in docs/styling.md.
func (t *DataTableElement) Width(l ui.Length) *DataTableElement { t.size.setWidth(l); return t }

// Height sets the height of the table.
func (t *DataTableElement) Height(l ui.Length) *DataTableElement { t.size.setHeight(l); return t }

// MinWidth sets the smallest width of the table.
func (t *DataTableElement) MinWidth(l ui.Length) *DataTableElement {
	t.size.setMinWidth(l)
	return t
}

// MinHeight sets the smallest height of the table.
func (t *DataTableElement) MinHeight(l ui.Length) *DataTableElement {
	t.size.setMinHeight(l)
	return t
}

// MaxWidth sets the largest width of the table.
func (t *DataTableElement) MaxWidth(l ui.Length) *DataTableElement {
	t.size.setMaxWidth(l)
	return t
}

// MaxHeight sets the largest height of the table.
func (t *DataTableElement) MaxHeight(l ui.Length) *DataTableElement {
	t.size.setMaxHeight(l)
	return t
}

// Grow makes the table take a share of the free space on its parent's main axis.
func (t *DataTableElement) Grow(f float32) *DataTableElement { t.size.setGrow(f); return t }

// GridSpan sets how many columns and rows the table covers in its parent's grid.
func (t *DataTableElement) GridSpan(cols, rows int) *DataTableElement {
	t.size.setSpan(cols, rows)
	return t
}

// Render builds the table, a ui.DataGrid of the chart's data. It is called by the UI.
func (t *DataTableElement) Render(ctx *ui.Context) ui.Element {
	c := t.c
	s := ctx.Strings()
	header, rows := c.table(s)
	cols := make([]ui.DataColumn[tableRow], len(header))
	for i, h := range header {
		cols[i] = ui.DataColumn[tableRow]{
			Header: h,
			Key:    "column" + strconv.Itoa(i),
			Width:  160,
			Text:   func(r tableRow) string { return r.cells[i] },
		}
	}
	grid := ui.DataGrid(rows, func(r tableRow) string { return r.key }, cols...).
		Label(s.ChartDataTable(c.summary(s)))
	return applySizes(t.size, grid)
}

// table returns the header and the rows of the chart's data, with every series, with the
// headers of s.
func (c *Chart) table(s *ui.Strings) ([]string, []tableRow) {
	switch c.kind {
	case kindSankey:
		header := []string{s.ChartFrom, s.ChartTo, s.ChartValue}
		rows := make([]tableRow, len(c.sk.links))
		for i, e := range c.sk.links {
			rows[i] = tableRow{key: strconv.Itoa(i), cells: []string{c.sk.nodes[e.from].name, c.sk.nodes[e.to].name, c.valueText(e.value)}}
		}
		return header, rows
	case kindPie, kindDonut:
		pts := c.ds.series[0].pts
		total := 0.0
		for _, p := range pts {
			total += max(p.y, 0)
		}
		rows := make([]tableRow, len(pts))
		for i, p := range pts {
			share := "0%"
			if total > 0 {
				share = c.share(p.y, allIndices(len(pts)))
			}
			rows[i] = tableRow{key: strconv.Itoa(i), cells: []string{c.xLabel(p), c.valueText(p.y), share}}
		}
		return []string{s.ChartSlice, s.ChartValue, s.ChartShare}, rows
	}

	header := []string{c.positionHeader(s)}
	if c.kind == kindCandle {
		header = append(header, s.ChartOpen, s.ChartHigh, s.ChartLow, s.ChartClose)
		pts := c.ds.series[0].pts
		rows := make([]tableRow, len(pts))
		for i, p := range pts {
			rows[i] = tableRow{key: strconv.Itoa(i), cells: []string{c.xLabel(p), c.valueText(p.open),
				c.valueText(p.high), c.valueText(p.low), c.valueText(p.close)}}
		}
		return header, rows
	}
	for k := range c.ds.series {
		header = append(header, c.itemName(s, k))
	}
	all := allIndices(len(c.ds.series))
	slots := c.ds.slots(all)
	rows := make([]tableRow, len(slots))
	for i, x := range slots {
		cells := []string{c.positionLabel(x)}
		for k := range c.ds.series {
			cell := ""
			if j, ok := findX(c.ds.series[k].pts, x); ok {
				cell = c.valueText(c.ds.series[k].pts[j].y)
			}
			cells = append(cells, cell)
		}
		rows[i] = tableRow{key: strconv.Itoa(i), cells: cells}
	}
	return header, rows
}

// positionHeader returns the name of the column of positions in the table: the position
// axis's label when it has one, else the kind of the positions, in s.
func (c *Chart) positionHeader(s *ui.Strings) string {
	if c.xa.Label != "" {
		return c.xa.Label
	}
	switch c.ds.x {
	case xTime:
		return s.ChartDate
	case xCategory:
		return s.ChartCategory
	}
	return s.ChartPosition
}
