// Command fields is a page of text fields for the web build's browser check
// (scripts/web/browser.cjs): on an iPhone, the editable element over each lays
// its text out as the field does, which the check compares, at every character
// boundary, with where the field puts its caret. The fields are 120 pixels
// apart, the first at the top of the page:
//
//  0. a text input;
//  1. a text area;
//  2. a text area with Markdown's marks (markdown.EditMarks), as a chat
//     composer has, whose code is monospace and whose syntax hides;
//  3. a text area with text of 18 pixels.
package main

import (
	"github.com/tyler-builds/jamb/app"
	"github.com/tyler-builds/jamb/markdown"
	"github.com/tyler-builds/jamb/ui"
)

func main() {
	line, area, md, big := ui.NewSignal(""), ui.NewSignal(""), ui.NewSignal(""), ui.NewSignal("")
	marks := markdown.EditMarks(markdown.EditStyle{})
	app.New().Run(app.Window{
		Title: "fields",
		Root: func(ctx *ui.Context) ui.Element {
			slot := func(el ui.Element) ui.Element {
				return ui.Div().Height(ui.Px(108)).Shrink(0).Child(el)
			}
			return ui.Div().Column().Gap(12).Padding(16).Background(ctx.Theme().Background).Grow(1).
				Child(slot(ui.TextInput(line).Label("Line").Width(ui.Px(330)))).
				Child(slot(ui.TextArea(area).Label("Area").Width(ui.Px(330)).Height(ui.Px(100)))).
				Child(slot(ui.TextArea(md).Label("Markdown").Marks(marks).Width(ui.Px(330)).Height(ui.Px(100)))).
				Child(slot(ui.TextArea(big).Label("Large").Style(ui.Style{FontSize: 18}).Width(ui.Px(330)).Height(ui.Px(100))))
		},
	})
}
