package views

import "github.com/tyler-builds/jamb/ui"

// The app's themes: quiet neutrals with one indigo accent, a 13 px base size
// and 6 px corners. Both start from the built-in themes, so every widget part the
// app does not mention keeps its default look.
var (
	DarkTheme  = gitTheme(ui.DarkTheme(), true)
	LightTheme = gitTheme(ui.LightTheme(), false)
)

func gitTheme(t *ui.Theme, dark bool) *ui.Theme {
	t.Name = "Git light"
	t.Background = ui.Hex("#ffffff")
	t.Panel = ui.Hex("#f7f7f5")
	t.Surface = ui.Hex("#ffffff")
	t.SurfaceHover = ui.Hex("#f0f0ee")
	t.SurfaceActive = ui.Hex("#e8e8e5")
	t.Border = ui.Hex("#ebebe9")
	t.BorderStrong = ui.Hex("#8c8b87")
	t.Text = ui.Hex("#2f2e2b")
	t.TextMuted = ui.Hex("#5f5e5a")
	t.TextSubtle = ui.Hex("#6f6e6a")
	t.Disabled = t.TextSubtle
	t.Accent = ui.Hex("#5e6ad2")
	t.AccentHover = ui.Hex("#5560c8")
	t.AccentActive = ui.Hex("#4c57bd")
	t.AccentSoft = ui.Hex("#5e6ad21c")
	t.Link = ui.Hex("#4752c4")
	t.FocusRing = ui.Hex("#6f7ae0")
	if dark {
		t.Name = "Git dark"
		t.Background = ui.Hex("#111214")
		t.Panel = ui.Hex("#0c0d0f")
		t.Surface = ui.Hex("#17181b")
		t.SurfaceHover = ui.Hex("#1b1c20")
		t.SurfaceActive = ui.Hex("#22232a")
		t.Border = ui.Hex("#212227")
		t.BorderStrong = ui.Hex("#5c5d66")
		t.Text = ui.Hex("#e6e6e9")
		t.TextMuted = ui.Hex("#9d9ea6")
		t.TextSubtle = ui.Hex("#8a8b94")
		t.Disabled = t.TextSubtle
		t.AccentSoft = ui.Hex("#5e6ad230")
		t.Link = ui.Hex("#a4acf4")
		t.FocusRing = ui.Hex("#7c86e8")
	}
	t.Radius = 6
	t.FontSize = 13
	t.Button = ui.ButtonStyle{Root: ui.Style{Radius: 6}}
	return t
}

// laneColors are the colors of the graph's lanes, a lane's color being its
// column's. They are the same hues in both themes, deeper on the light one.
var (
	darkLanes = []ui.Color{
		ui.Hex("#7c86f0"), ui.Hex("#2fb8d4"), ui.Hex("#d374e8"), ui.Hex("#4fc38a"),
		ui.Hex("#f2a254"), ui.Hex("#ef6f8f"), ui.Hex("#e6c54f"), ui.Hex("#5aa7f7"),
	}
	lightLanes = []ui.Color{
		ui.Hex("#5e6ad2"), ui.Hex("#0e8aa6"), ui.Hex("#a84cc0"), ui.Hex("#22955e"),
		ui.Hex("#c96f1c"), ui.Hex("#cf3f65"), ui.Hex("#a88613"), ui.Hex("#2f7ed8"),
	}
)

// laneColor is the color of lane i on theme t. Under high contrast every lane
// takes the text color, and the graph's shapes still tell the lanes apart.
func laneColor(t *ui.Theme, i int) ui.Color {
	if t.HighContrast {
		return t.Text
	}
	lanes := lightLanes
	if t.Dark {
		lanes = darkLanes
	}
	return lanes[i%len(lanes)]
}
