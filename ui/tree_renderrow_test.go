package ui_test

import (
	"fmt"
	"testing"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// RenderRow content keeps its width beside a node's description, so a row stays one
// line high and the rows, which are a fixed height apart, do not overlap. The content
// was shrunk by the description, which grew from its own width, and its text wrapped
// to two lines. In a tree too narrow for it, single-line content (NoWrap) is clipped
// and the row keeps its height.
func TestTreeRenderRowStaysBesideTheDescription(t *testing.T) {
	nodes := []ui.TreeNode{
		{Key: "cust", Label: "Customers", Description: "1,024 rows"},
		{Key: "ord", Label: "Orders", Description: "18,311 rows"},
		{Key: "ol", Label: "OrderLines", Description: "92,004 rows"},
	}
	for _, width := range []float32{200, 120} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			sel := ui.NewSignal("")
			u := uitest.New(t, func(ctx *ui.Context) ui.Element {
				tv := ui.Tree(nodes, sel).Key("tree").Label("Tables").Grow(1).
					RenderRow(func(n ui.TreeNode, s ui.PartState) ui.Element {
						label := ui.Text(n.Label + " (dbo)")
						if width < 200 {
							label.NoWrap()
						}
						return ui.Div().Row().Align(ui.Center).Gap(6).Key("content/"+n.Key).Children(
							ui.Icon(icons.Table).Size(16), label)
					})
				return ui.Div().Row().Grow(1).Child(ui.Div().Column().Width(ui.Px(width)).Child(tv))
			}, uitest.Size(320, 200))
			var prev ui.Rect
			for i, k := range []string{"cust", "ord", "ol"} {
				row := u.Get(k).Bounds()
				if row.H > 26.5 {
					t.Errorf("row %s is %.1f px high, want one line (26)", k, row.H)
				}
				if i > 0 && row.Y < prev.Y+prev.H-0.5 {
					t.Errorf("row %s at y=%.1f overlaps the row above it, which ends at %.1f", k, row.Y, prev.Y+prev.H)
				}
				prev = row
				c := u.Get("content/" + k).Bounds()
				if c.Y < row.Y || c.Y+c.H > row.Y+row.H+0.5 {
					t.Errorf("the content of row %s (%+v) is not inside the row (%+v)", k, c, row)
				}
			}
			// In the wider tree the content keeps its own width, and the description
			// takes the room that is left.
			if width == 200 {
				nat := uitest.New(t, func(ctx *ui.Context) ui.Element {
					return ui.Div().Row().Child(ui.Div().Row().Align(ui.Center).Gap(6).Key("nat").Children(
						ui.Icon(icons.Table).Size(16), ui.Text("Orders (dbo)")))
				})
				if got, want := u.Get("content/ord").Bounds().W, nat.Get("nat").Bounds().W; got < want-0.5 {
					t.Errorf("the content of Orders is %.1f px wide, want its own width %.1f", got, want)
				}
			}
		})
	}
}
