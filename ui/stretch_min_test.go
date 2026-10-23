package ui_test

import (
	"testing"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// A child that stretches across a 24 px row is still at least its MinHeight tall,
// as CSS does: the row's height is the stretch, and the minimum wins over it.
func TestStretchedChildKeepsMinHeight(t *testing.T) {
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Child(
			ui.Div().Key("row").Row().Height(ui.Px(24)).Child(
				ui.Div().Key("item").Width(ui.Px(40)).MinHeight(ui.Px(32)),
			),
		)
	}, uitest.Size(200, 100))
	if h := u.Get("item").Bounds().H; h != 32 {
		t.Errorf("stretched item is %v px tall, want its MinHeight of 32", h)
	}
}
