package ui

// SyntaxStyle is the look of one syntax capture in a code editor: its color, and
// whether its text is italic or bold. Theme.Syntax is keyed by the tree-sitter
// capture name, such as "keyword" or "string.escape". A capture that is not in
// the map takes the editor foreground.
type SyntaxStyle struct {
	// Color is the color of the text for the capture.
	Color Color
	// Italic sets the text in italic.
	Italic bool
	// Bold sets the text in bold.
	Bold bool
}

// CodeEditorColors are the colors of a code editor, as the predefined themes set
// them from their editor colors. Selection is translucent, so it is drawn over
// the text.
type CodeEditorColors struct {
	// Background is the background of the editor.
	Background Color
	// Foreground is the color of the text that has no syntax color.
	Foreground Color
	// ActiveLine is the background of the line that holds the caret.
	ActiveLine Color
	// LineNumber is the color of the line numbers.
	LineNumber Color
	// ActiveLineNumber is the color of the line number of the caret's line.
	ActiveLineNumber Color
	// Selection is the background of the selected text.
	Selection Color
}

// darkSyntax and lightSyntax are the syntax colors of the built-in themes. Each
// text capture meets 4.5:1 on the Surface of its theme, as the built-in colors do.
var darkSyntax = map[string]SyntaxStyle{
	"attribute":          {Color: Hex("#fde68a")},
	"boolean":            {Color: Hex("#fdba74")},
	"comment":            {Color: Hex("#a1a1aa"), Italic: true},
	"comment.doc":        {Color: Hex("#a1a1aa"), Italic: true},
	"constant":           {Color: Hex("#fdba74")},
	"constructor":        {Color: Hex("#93c5fd")},
	"function":           {Color: Hex("#93c5fd")},
	"keyword":            {Color: Hex("#c4b5fd")},
	"link_text":          {Color: Hex("#a5b4fc")},
	"link_uri":           {Color: Hex("#a1a1aa"), Italic: true},
	"number":             {Color: Hex("#fdba74")},
	"operator":           {Color: Hex("#d4d4d8")},
	"property":           {Color: Hex("#7dd3fc")},
	"punctuation":        {Color: Hex("#a1a1aa")},
	"string":             {Color: Hex("#86efac")},
	"string.escape":      {Color: Hex("#fde68a")},
	"string.special":     {Color: Hex("#fde68a")},
	"tag":                {Color: Hex("#93c5fd")},
	"title":              {Color: Hex("#f4f4f5"), Bold: true},
	"type":               {Color: Hex("#67e8f9")},
	"variable.parameter": {Color: Hex("#f4f4f5")},
	"variable.special":   {Color: Hex("#c4b5fd")},
}

var lightSyntax = map[string]SyntaxStyle{
	"attribute":          {Color: Hex("#a16207")},
	"boolean":            {Color: Hex("#c2410c")},
	"comment":            {Color: Hex("#52525b"), Italic: true},
	"comment.doc":        {Color: Hex("#52525b"), Italic: true},
	"constant":           {Color: Hex("#c2410c")},
	"constructor":        {Color: Hex("#1d4ed8")},
	"function":           {Color: Hex("#1d4ed8")},
	"keyword":            {Color: Hex("#6d28d9")},
	"link_text":          {Color: Hex("#4338ca")},
	"link_uri":           {Color: Hex("#52525b"), Italic: true},
	"number":             {Color: Hex("#c2410c")},
	"operator":           {Color: Hex("#3f3f46")},
	"property":           {Color: Hex("#0369a1")},
	"punctuation":        {Color: Hex("#52525b")},
	"string":             {Color: Hex("#15803d")},
	"string.escape":      {Color: Hex("#b45309")},
	"string.special":     {Color: Hex("#b45309")},
	"tag":                {Color: Hex("#1d4ed8")},
	"title":              {Color: Hex("#18181b"), Bold: true},
	"type":               {Color: Hex("#0e7490")},
	"variable.parameter": {Color: Hex("#18181b")},
	"variable.special":   {Color: Hex("#6d28d9")},
}

// darkEditor and lightEditor are the editor colors of the built-in themes. The
// editor is drawn on the Surface, and its selection is Accent at alpha 0.32.
var (
	darkEditor = CodeEditorColors{
		Background:       Hex("#18181b"),
		Foreground:       Hex("#f4f4f5"),
		ActiveLine:       Hex("#1f1f23"),
		LineNumber:       Hex("#8e8e96"),
		ActiveLineNumber: Hex("#f4f4f5"),
		Selection:        Hex("#6265ed52"),
	}
	lightEditor = CodeEditorColors{
		Background:       Hex("#ffffff"),
		Foreground:       Hex("#18181b"),
		ActiveLine:       Hex("#f4f4f5"),
		LineNumber:       Hex("#62626a"),
		ActiveLineNumber: Hex("#18181b"),
		Selection:        Hex("#4f46e552"),
	}
)
