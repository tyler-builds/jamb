package views

import "github.com/tyler-builds/jamb/ui"

// customizingSection shows how a built-in widget is styled without rebuilding
// it: a part set for the whole app in the theme (see DemoDarkTheme in theme.go), a part changed
// on one instance, the variants, and a render slot. Each sample is the built-in
// widget, so its keyboard, focus and accessibility behavior are the framework's.
// See docs/styling.md.
func customizingSection() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		themed := ui.Local(ctx, "cust.theme", "overview")
		instance := ui.Local(ctx, "cust.instance", "overview")
		segmented := ui.Local(ctx, "cust.segmented", "all")
		inbox := ui.Local(ctx, "cust.inbox", "open")
		swatch := ui.Local(ctx, "cust.swatch", "accent")
		inline, stacked := ui.Local(ctx, "cust.inline", 8), ui.Local(ctx, "cust.stacked", 8)
		notify, sync := ui.Local(ctx, "cust.notify", true), ui.Local(ctx, "cust.sync", false)

		overview := ui.Tab[string]{Value: "overview", Label: "Overview", Panel: func() ui.Element { return panelText(t, "Overview panel") }}
		usage := ui.Tab[string]{Value: "usage", Label: "Usage", Panel: func() ui.Element { return panelText(t, "Usage panel") }}
		billing := ui.Tab[string]{Value: "billing", Label: "Billing", Panel: func() ui.Element { return panelText(t, "Billing panel") }}

		// The render slot's content: a visible name and a count, drawn in the tab.
		// The tab's label carries the same words for assistive technology.
		counts := map[string]int{"open": 4, "closed": 12}
		names := map[string]string{"open": "Open", "closed": "Closed"}

		// The swatches are the theme's colors, so under high contrast they are the
		// system colors, like everything else.
		swatches := []struct {
			value, label string
			color        ui.Color
		}{
			{"accent", "Accent", t.Accent}, {"success", "Success", t.Success},
			{"warning", "Warning", t.Warning}, {"danger", "Danger", t.Danger},
		}
		swatchOptions := make([]ui.Option[string], len(swatches))
		colorOf := map[string]ui.Color{}
		for i, sw := range swatches {
			swatchOptions[i] = ui.Option[string]{Value: sw.value, Label: sw.label}
			colorOf[sw.value] = sw.color
		}
		swatchRow := func(value, label string) ui.Element {
			return ui.Div().Row().Align(ui.Center).Gap(8).
				Child(ui.Div().Size(ui.Px(12), ui.Px(12)).Radius(3).Background(colorOf[value])).
				Child(ui.Text(label).NoWrap())
		}

		samples := []ui.Element{
			sample(t, "Theme: Indicator 3 px", ui.Div().Width(ui.Px(360)).Child(ui.Tabs(themed, overview, usage, billing).
				Key("cust.themeTabs").Label("Theme example"))),
			sample(t, "Instance: Styles", ui.Div().Width(ui.Px(360)).Child(ui.Tabs(instance, overview, usage, billing).
				Key("cust.instanceTabs").Label("Instance example").Styles(ui.TabsStyle{
				Indicator:   ui.Style{Background: t.Success},
				TabSelected: ui.Style{TextColor: t.Success},
			}))),
			sample(t, "Variant: Segmented", ui.Tabs(segmented,
				ui.Tab[string]{Value: "all", Label: "All"},
				ui.Tab[string]{Value: "unread", Label: "Unread"},
				ui.Tab[string]{Value: "starred", Label: "Starred"},
			).Key("cust.segmentedTabs").Label("Segmented example").Variant(ui.TabsSegmented)),
			sample(t, "Render slot: count in the tab", ui.Tabs(inbox,
				ui.Tab[string]{Value: "open", Label: "Open, 4 items"},
				ui.Tab[string]{Value: "closed", Label: "Closed, 12 items"},
			).Key("cust.slotTabs").Label("Render slot example").Variant(ui.TabsSegmented).
				RenderTab(func(tab ui.Tab[string], _ ui.PartState) ui.Element {
					return ui.Div().Row().Align(ui.Center).Gap(6).
						Child(ui.Text(names[tab.Value]).NoWrap()).
						Child(ui.Badge(counts[tab.Value]))
				})),
			sample(t, "Theme: button radius", ui.Button("Save").Primary().OnClick(func() {
				ctx.Toast(ui.ToastSuccess, "Saved")
			})),
			sample(t, "Instance: Checkbox Styles", ui.Checkbox("Notify me", notify).
				Styles(ui.CheckboxStyle{BoxChecked: ui.Style{Background: t.Success, BorderColor: t.Success}})),
			sample(t, "Render slot: collapsible header", ui.Collapsible("Sync, 3 items", sync, ui.Text("Last sync 10:42")).
				Key("cust.syncSection").RenderHeader(func(_ string, _ ui.PartState) ui.Element {
				return ui.Div().Row().Align(ui.Center).Gap(6).
					Child(ui.Text("Sync").NoWrap()).
					Child(ui.Badge(3))
			})),
			sample(t, "Spin buttons: Inline", ui.NumberField(inline, 0, 23, 1).Label("Hour, inline").SpinButtons(ui.SpinInline)),
			sample(t, "Spin buttons: Stacked", ui.NumberField(stacked, 0, 23, 1).Label("Hour, stacked").SpinButtons(ui.SpinStacked)),
			sample(t, "Render slot: swatch in a select", ui.Select(swatch, swatchOptions...).Label("Swatch").Width(ui.Px(200)).
				RenderOption(func(o ui.Option[string], _ ui.PartState) ui.Element {
					return swatchRow(o.Value, o.Label)
				}).
				RenderValue(func(o ui.Option[string], _ ui.PartState) ui.Element {
					return swatchRow(o.Value, o.Label)
				})),
		}
		return Card(t).Gap(14).
			Child(SectionTitle(t, "Customizing")).
			Child(ui.Text("Each part of a built-in widget can be styled: set it for the app in the theme, change it on one instance, or pick a variant. A render slot draws the content of one part. The widget keeps its keys, focus, selection and accessible names. Under high contrast the colors are the system colors, and sizes still apply.").
				Size(12).Color(t.TextMuted)).
			Child(ui.Div().Row().Wrap().Gap(22).Align(ui.Start).Children(samples...)).
			Child(ui.Text("A tab's accessible name is its label, so put in the label what a user must hear, such as a count.").
				Size(12).Color(t.TextMuted))
	})
}
