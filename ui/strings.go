package ui

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Strings is the text that jamb shows and announces: the names and labels of the
// controls, the words of the pickers, the validation messages and the announcements.
// Each field is one message. A message with values is a function of them, so that
// each language sets its own word order and its own plural forms.
//
// An app replaces any of the text with a Strings value, with Runtime.SetStrings or
// app.Window.Strings. A field that is empty (or a function that is nil) shows the
// English text, so a Strings value needs only the messages it changes. The packs of
// StringsFor translate the messages whose translation is short and unambiguous; the
// others stay English in every language.
//
// The messages of programmer errors, such as the panics of the builders and the
// errors of the parsers, and the diagnostics of the accessibility check are not here.
// They are for the developer and stay English. The names of the built-in actions
// (see NewAction) are not here either, since they are identifiers of the keymap.
type Strings struct {
	// The default names of the controls, which assistive technology speaks when the
	// app gives the control no name of its own.
	Avatar string
	// Breadcrumb is the default name of a breadcrumb trail.
	Breadcrumb string
	// Pagination is the default name of a pagination control.
	Pagination string
	// Navigation is the default name of a navigation region, such as a sidebar.
	Navigation string
	// Options is the default name of a list of options.
	Options string
	// Suggestions is the default name of a list of suggestions.
	Suggestions string
	// Times is the default name of a list of times.
	Times string
	// Actions is the default name of a menu of actions.
	Actions string
	// Color is the default name of a color control.
	Color string
	// Progress is the default name of a progress bar.
	Progress string
	// Loading is the default name of a loading indicator.
	Loading string
	// Notifications is the default name of a list of notifications.
	Notifications string
	// ResizePanes is the name of a split's divider, which assistive technology reads.
	ResizePanes string
	// DateField is the default name of a date field.
	DateField string
	// DateRangeField is the default name of a date range field.
	DateRangeField string
	// Steps is the default name of a stepper.
	Steps string
	// CommandPalette is the name of a command palette.
	CommandPalette string
	// Carousel is the default name of a carousel, and the description of one that has a
	// name, which tells assistive technology what the group is.
	Carousel string
	// Slides is the name of the group of a carousel's dots.
	Slides string

	// The labels of the controls that jamb draws.
	LoadingEllipsis string
	// LoadingSuggestions is the text shown while suggestions load.
	LoadingSuggestions string
	// CommandPalettePlaceholder is the text shown in a command palette's empty field.
	CommandPalettePlaceholder string
	// SaturationBrightness is the name of the area of a color picker that sets saturation and brightness.
	SaturationBrightness string
	// SaturationBrightnessValue is the value of the color area, read by assistive technology,
	// such as "Saturation 50%, brightness 80%".
	SaturationBrightnessValue func(saturation, brightness int) string
	// Hue is the name of the hue slider of a color picker.
	Hue string
	// Opacity is the name of the opacity slider of a color picker.
	Opacity string
	// Hex is the name of the hexadecimal color field.
	Hex string
	// ChooseDate is the name of the button that opens a calendar.
	ChooseDate string
	// ChooseDates is the name of the button that opens a calendar for a date range.
	ChooseDates string
	// ShowTimes is the name of the button that opens the list of times.
	ShowTimes string
	// ShowHiddenLocations is the name of the button that shows the locations hidden in a breadcrumb trail.
	ShowHiddenLocations string
	// CollapseNavigation is the name of the button that collapses a sidebar.
	CollapseNavigation string
	// ExpandNavigation is the name of the button that expands a sidebar.
	ExpandNavigation string
	// DismissNotification is the name of the button that dismisses a notification.
	DismissNotification string
	// ToastAnnouncement is the text a toast with a title is announced with, from the
	// title and the message, such as "Upload: Uploaded".
	ToastAnnouncement func(title, text string) string
	// PreviousPage is the name of the button that goes to the previous page.
	PreviousPage string
	// NextPage is the name of the button that goes to the next page.
	NextPage string
	// PreviousSlide is the name of the button that goes to the previous slide of a carousel.
	PreviousSlide string
	// NextSlide is the name of the button that goes to the next slide of a carousel.
	NextSlide string
	// PauseSlides is the name of the button that stops a carousel's AutoPlay.
	PauseSlides string
	// PlaySlides is the name of the button that starts a carousel's AutoPlay again.
	PlaySlides string
	// PreviousYear is the name of the button that goes to the previous year.
	PreviousYear string
	// PreviousMonth is the name of the button that goes to the previous month.
	PreviousMonth string
	// NextMonth is the name of the button that goes to the next month.
	NextMonth string
	// NextYear is the name of the button that goes to the next year.
	NextYear string
	// Close is the name of a close button.
	Close string
	// OK is the label of the button that confirms an alert dialog (see AlertDialog).
	OK string
	// Cancel is the label of the button that cancels an alert dialog.
	Cancel string
	// Minimize is the name of the button that minimizes a window.
	Minimize string
	// Maximize is the name of the button that maximizes a window.
	Maximize string
	// Restore is the name of the button that restores a maximized window.
	Restore string
	// Copy is the name of a copy button that has no text of its own (see CopyButton).
	Copy string
	// Copied is the confirmation of a copy: the label of a copy button with text,
	// and the text announced after a copy (see CopyButton.Copied).
	Copied string
	// Increase is the name of the button that increases a number.
	Increase string
	// Decrease is the name of the button that decreases a number.
	Decrease string
	// ShowPassword is the name of the button that shows the text of a masked field.
	ShowPassword string
	// HidePassword is the name of that button while the text is shown.
	HidePassword string
	// Clear is the name of the button that clears the text of a field.
	Clear string
	// Minimum is the name of the low thumb of a range slider.
	Minimum string
	// Maximum is the name of the high thumb of a range slider.
	Maximum string
	// MoreOptions is the name of the arrow of a split button, which opens its menu.
	MoreOptions string
	// AllTabs is the name of the button of a tab list that opens a menu of all its tabs
	// (see Tabs.OverflowMenu).
	AllTabs string
	// ScrollTabsBack is the name of the button that scrolls a tab list toward its first tab.
	ScrollTabsBack string
	// ScrollTabsForward is the name of the button that scrolls a tab list toward its last tab.
	ScrollTabsForward string

	// The names of a dock's tab strip and of the commands of its tabs' menus.
	PanelGroup string
	// MovePanelLeft is the name of the command that moves a dock panel to the left.
	MovePanelLeft string
	// MovePanelRight is the name of the command that moves a dock panel to the right.
	MovePanelRight string
	// MovePanelUp is the name of the command that moves a dock panel up.
	MovePanelUp string
	// MovePanelDown is the name of the command that moves a dock panel down.
	MovePanelDown string
	// SplitPanelRight is the name of the command that splits a dock group to the right.
	SplitPanelRight string
	// SplitPanelDown is the name of the command that splits a dock group downward.
	SplitPanelDown string
	// FloatPanel is the name of the command that floats a dock panel in its own window.
	FloatPanel string
	// DockPanelBack is the name of the command that docks a floating panel back.
	DockPanelBack string
	// ClosePanel is the name of the command that closes a dock panel.
	ClosePanel string

	// Remove names the button that removes a tag, such as "Remove Sales".
	Remove func(tag string) string
	// ResizeBetween names the divider between two panes that have labels, such as "Resize Sidebar and
	// Editor" (see Split.Label).
	ResizeBetween func(first, second string) string
	// TabMoved is announced after the user moves a tab (see Tabs.Reorderable), such as
	// "Sales moved to position 2 of 3".
	TabMoved func(tab string, position, count int) string
	// ColumnMoved is announced after the user moves a column of a grid (see
	// DataColumn.Movable), such as "Status moved to position 2 of 5".
	ColumnMoved func(column string, position, count int) string
	// Page is the name of a page button, such as "Page 3". CurrentPage names the page
	// that is shown, and PageOf is the text of a compact pagination, such as "Page 3 of 9".
	Page func(n int) string
	// CurrentPage builds the name of the current page, from its number.
	CurrentPage func(n int) string
	// PageOf builds the name of a page, from its number and the number of pages.
	PageOf func(page, pages int) string
	// SlideOf names the slide a carousel shows, and is announced when the user changes it,
	// such as "Slide 2 of 5".
	SlideOf func(n, slides int) string
	// GoToSlide names the dot of a carousel's slide, such as "Go to slide 3".
	GoToSlide func(n int) string
	// StepCompleted, StepCurrent and StepUpcoming name a step of a stepper, from its
	// number, the number of steps and its title, such as "Step 2 of 4: Shipping,
	// current". StepCompleted names a step before the current one.
	StepCompleted func(n, steps int, title string) string
	// StepCurrent names the current step of a stepper.
	StepCurrent func(n, steps int, title string) string
	// StepUpcoming names a step after the current one.
	StepUpcoming func(n, steps int, title string) string
	// RatingValue is the value of a rating that assistive technology reads, such as
	// "3 of 5 stars".
	RatingValue func(value, max int) string

	// The notes of a calendar's days: Today, and the ends and the middle of a range.
	Today string
	// StartOfRange is the name of the start of a date range.
	StartOfRange string
	// EndOfRange is the name of the end of a date range.
	EndOfRange string
	// InRange is the name of a day that lies in a date range.
	InRange string

	// The empty and error texts of the lists, the grid and the suggestions.
	NoResults string
	// NoRows is the message shown when a grid has no rows.
	NoRows string
	// LoadingMore names the row at the end of a grid while it loads more rows, and is
	// announced when the loading starts (see DataGrid.LoadingMore).
	LoadingMore string
	// NoMatchingCommands is the message shown when no command of a command palette matches.
	NoMatchingCommands string
	// CouldNotLoadSuggestions is the message shown when suggestions cannot be loaded.
	CouldNotLoadSuggestions string
	// SuggestionCount builds the message that gives the number of suggestions.
	SuggestionCount func(n int) string
	// NotInList is the message shown when the text of a combo box is not in its list, which is then cleared.
	NotInList string
	// NotInListReverted builds the message shown when the text is not in the list and reverts to a label.
	NotInListReverted func(label string) string
	// SortedAscending is the state of a column sorted in ascending order.
	SortedAscending string
	// SortedDescending is the state of a column sorted in descending order.
	SortedDescending string

	// The text of a badge that counts more than its maximum, such as "99+", and the
	// name it adds to its owner, such as "more than 99 items".
	BadgeOverflow func(max int) string
	// BadgeMoreThan builds the text that is read for a badge whose count is above its maximum.
	BadgeMoreThan func(max int) string
	// BadgeMoreThanNoun builds the text that is read for a badge whose count is above its maximum, with the noun of the count.
	BadgeMoreThanNoun func(max int, noun string) string

	// The validation messages of the built-in validators and the controls.
	Required string
	// MinLength builds the message for text shorter than the minimum length.
	MinLength func(n int) string
	// MaxLength builds the message for text longer than the maximum length.
	MaxLength func(n int) string
	// ValueRange builds the message for a value outside the range, given the ends of the range.
	ValueRange func(lo, hi string) string
	// NumberRange builds the message for a number outside the range, given the ends of the range.
	NumberRange func(lo, hi string) string
	// DateOnOrAfter builds the message for a date before the earliest date that can be chosen.
	DateOnOrAfter func(date string) string
	// DateOnOrBefore builds the message for a date after the latest date that can be chosen.
	DateOnOrBefore func(date string) string
	// DateNotAllowed is the message for a date that cannot be chosen.
	DateNotAllowed string
	// DateExample builds the message that gives an example of a date.
	DateExample func(example string) string
	// RangeExample builds the message that gives an example of a range.
	RangeExample func(example string) string
	// RangeStartChosen is the message announced when the start of a date range is chosen.
	RangeStartChosen string
	// TimeFromTo builds the message for a time outside the range from one time to another.
	TimeFromTo func(from, to string) string
	// TimeExample builds the message that gives an example of a time.
	TimeExample func(example string) string

	// FieldsNeedAttention is the summary of a form with n fields that have a
	// message, and FormSubmitFailed is the message of a form whose submit failed
	// without a message of its own.
	FieldsNeedAttention func(n int) string
	// FormSubmitFailed is the message shown when a form cannot be submitted.
	FormSubmitFailed string

	// The code editor (package codeedit): its default name, the labels of its find and
	// replace bar and of its go-to-line bar, and the words that its matches count shows.
	// CodeEditorCount is the count of matches, such as "3 of 12".
	CodeEditorLabel string
	// CodeEditorFind is the label of the find box of a code editor.
	CodeEditorFind string
	// CodeEditorReplace is the label of the replace box of a code editor.
	CodeEditorReplace string
	// CodeEditorReplaceOne is the name of the button that replaces one match.
	CodeEditorReplaceOne string
	// CodeEditorReplaceAll is the name of the button that replaces every match.
	CodeEditorReplaceAll string
	// CodeEditorNext is the name of the button that goes to the next match.
	CodeEditorNext string
	// CodeEditorPrevious is the name of the button that goes to the previous match.
	CodeEditorPrevious string
	// CodeEditorClose is the name of the button that closes the find bar.
	CodeEditorClose string
	// CodeEditorMatchCase is the label of the option that makes the search case-sensitive.
	CodeEditorMatchCase string
	// CodeEditorWholeWord is the label of the option that matches whole words.
	CodeEditorWholeWord string
	// CodeEditorRegex is the label of the option that treats the query as a regular expression.
	CodeEditorRegex string
	// CodeEditorGoToLine is the label of the box that takes a line number.
	CodeEditorGoToLine string
	// CodeEditorGo is the name of the button that goes to the line.
	CodeEditorGo string
	// CodeEditorNoMatches is the message shown when a find has no matches.
	CodeEditorNoMatches string
	// CodeEditorBadPattern is the message shown for a regular expression that does not parse.
	CodeEditorBadPattern string
	// CodeEditorCount builds the message that gives the position of the current match, from its number and the total.
	CodeEditorCount func(current, total int) string
	// CodeEditorError is the name of the severity of an error diagnostic.
	CodeEditorError string
	// CodeEditorWarning is the name of the severity of a warning diagnostic.
	CodeEditorWarning string
	// CodeEditorInfo is the name of the severity of an information diagnostic.
	CodeEditorInfo string
	// CodeEditorHint is the name of the severity of a hint diagnostic.
	CodeEditorHint string
	// CodeEditorDiagnostic builds the message that announces a diagnostic, from the name of its
	// severity, its line number and its message.
	CodeEditorDiagnostic func(severity string, line int, message string) string
	// CodeEditorCodeActions is the name of the menu of code actions.
	CodeEditorCodeActions string
	// CodeEditorQuickFix is the name of the group of quick fixes in the menu of code actions.
	CodeEditorQuickFix string
	// CodeEditorRefactor is the name of the group of refactorings in the menu of code actions.
	CodeEditorRefactor string
	// CodeEditorSourceAction is the name of the group of source actions in the menu of code actions.
	CodeEditorSourceAction string
	// CodeEditorMoreActions is the name of the group of the other code actions.
	CodeEditorMoreActions string
	// CodeEditorNoCodeActions is the message shown when there are no code actions.
	CodeEditorNoCodeActions string
	// CodeEditorLoadingActions is the message shown while the code actions load.
	CodeEditorLoadingActions string
	// CodeEditorActionFailed is the message shown when the code actions could not load or be applied.
	CodeEditorActionFailed string
	// CodeEditorActionCount builds the message that gives the number of code actions.
	CodeEditorActionCount func(n int) string
	// CodeEditorActionUnavailable builds the message that says a code action is disabled, from its
	// title and the reason it is disabled.
	CodeEditorActionUnavailable func(title, reason string) string

	// The charts (package charts): the names of the chart types, which begin a chart's
	// summary. ChartLine is the name of a line chart.
	ChartLine string
	// ChartArea is the name of an area chart.
	ChartArea string
	// ChartStackedArea is the name of a stacked area chart.
	ChartStackedArea string
	// ChartBar is the name of a bar chart.
	ChartBar string
	// ChartStackedBar is the name of a stacked bar chart.
	ChartStackedBar string
	// ChartHorizontalBar is the name of a horizontal bar chart.
	ChartHorizontalBar string
	// ChartStackedHorizontalBar is the name of a stacked horizontal bar chart.
	ChartStackedHorizontalBar string
	// ChartScatter is the name of a scatter plot.
	ChartScatter string
	// ChartPie is the name of a pie chart.
	ChartPie string
	// ChartDonut is the name of a donut chart.
	ChartDonut string
	// ChartRadar is the name of a radar chart.
	ChartRadar string
	// ChartCandlestick is the name of a candlestick chart.
	ChartCandlestick string
	// ChartSankey is the name of a Sankey diagram.
	ChartSankey string

	// The summary of a chart, which names it for assistive technology. Each message is
	// a whole clause, and the clauses after the first take the summary so far, so that a
	// language sets its own order and punctuation. ChartSeriesSummary begins the
	// summary of a chart of series, from the name of its type, the number of series and
	// the least and the most points of a series, such as "Line chart, 2 series, 7 points
	// each". With no series, least and most are 0.
	ChartSeriesSummary func(chart string, series, least, most int) string
	// ChartCandleSummary begins the summary of a candlestick chart, such as "Candlestick chart, 30 candles".
	ChartCandleSummary func(chart string, candles int) string
	// ChartPieSummary begins the summary of a pie or a donut chart, such as "Pie chart, 7 slices".
	ChartPieSummary func(chart string, slices int) string
	// ChartSankeySummary is the summary of a Sankey diagram, such as "Sankey diagram, 3 nodes, 2 links; total flow 5".
	ChartSankeySummary func(chart string, nodes, links int, total string) string
	// ChartLargestSlice adds the largest slice and its share to the summary of a pie, such as
	// "Pie chart, 7 slices; largest is Saturday at 20%".
	ChartLargestSlice func(summary, slice, share string) string
	// ChartPositionRange adds the first and the last position to a summary, such as
	// "Line chart, 2 series, 7 points each, from Monday to Sunday".
	ChartPositionRange func(summary, from, to string) string
	// ChartValueRanges adds the value ranges of the series, each made by ChartValueRange, to a
	// summary, such as "…, from Monday to Sunday; Visits from 120 to 300, Returns from 10 to 40".
	ChartValueRanges func(summary string, ranges []string) string
	// ChartValueRange is the range of the values of one series, such as "Visits from 120 to 300".
	ChartValueRange func(series, lo, hi string) string

	// The announcements of the keyboard walk and the lines of the tooltips. ChartPointValue
	// announces a point, from its position, its series and its value, such as "Tuesday, Visits 180".
	ChartPointValue func(position, series, value string) string
	// ChartPointNoValue announces a position where the series has no point, such as "Tuesday, Visits no value".
	ChartPointNoValue func(position, series string) string
	// ChartSeriesValue is a line of a tooltip, a name and its value, such as "Visits 180" or "Open 12".
	ChartSeriesValue func(series, value string) string
	// ChartSliceValue announces a slice of a pie, such as "Saturday, 300 (20%)".
	ChartSliceValue func(slice, value, share string) string
	// ChartValueShare is the line of a slice's tooltip, its value and its share, such as "300 (20%)".
	ChartValueShare func(value, share string) string
	// ChartCandleValue announces a candle, such as "Monday, open 10, high 14, low 9, close 12".
	ChartCandleValue func(position, opening, high, low, closing string) string
	// ChartLink names a link of a Sankey diagram in its tooltip, such as "Home to Signup".
	ChartLink func(from, to string) string
	// ChartLinkValue announces a link of a Sankey diagram, such as "Home to Signup, 130".
	ChartLinkValue func(from, to, value string) string

	// The legend and the table of a chart. ChartLegend is the name of the legend.
	ChartLegend string
	// ChartSeriesNumber names a series that has no name, from its number, such as "Series 2".
	ChartSeriesNumber func(n int) string
	// ChartDataTable names the table of a chart's data, from the chart's summary, such as
	// "Data table: Line chart, …".
	ChartDataTable func(summary string) string
	// ChartDate is the header of the column of positions on a time axis.
	ChartDate string
	// ChartCategory is the header of the column of positions on a category axis.
	ChartCategory string
	// ChartPosition is the header of the column of positions on a number axis.
	ChartPosition string
	// ChartSlice is the header of the column of a pie's slices.
	ChartSlice string
	// ChartValue is the header of the column of values of a pie or a Sankey diagram.
	ChartValue string
	// ChartShare is the header of the column of a pie's shares.
	ChartShare string
	// ChartFrom is the header of the column of the sources of a Sankey diagram's links.
	ChartFrom string
	// ChartTo is the header of the column of the targets of a Sankey diagram's links.
	ChartTo string
	// ChartOpen is the opening value of a candle, in its tooltip and its table.
	ChartOpen string
	// ChartHigh is the highest value of a candle, in its tooltip and its table.
	ChartHigh string
	// ChartLow is the lowest value of a candle, in its tooltip and its table.
	ChartLow string
	// ChartClose is the closing value of a candle, in its tooltip and its table.
	ChartClose string

	// The words of the Markdown view (package markdown). MarkdownImage names an image
	// that has no alternative text.
	MarkdownImage string
	// MarkdownImageNotShown is the text of the placeholder of an image that is not
	// shown, from its alternative text, such as "Image not shown: Build status". The
	// alternative text may be empty.
	MarkdownImageNotShown func(alt string) string
	// MarkdownTask names the check box of a task whose item has no text.
	MarkdownTask string
	// MarkdownCode names a code block from the language of its fence, such as
	// "Code, go". The language may be empty.
	MarkdownCode func(language string) string
	// MarkdownFootnotes names the list of footnotes at the end of a document.
	MarkdownFootnotes string

	// RelativeNow is a time within a minute of now, in a relative time such as "now".
	RelativeNow string
	// RelativeYesterday and RelativeTomorrow name the day before and the day after today.
	RelativeYesterday string
	RelativeTomorrow  string
	// RelativeAgo builds a time in the past from its unit and count, such as "3 days ago"
	// (see FormatRelative). RelativeIn builds a time in the future, such as "in 3 days".
	RelativeAgo func(unit RelativeUnit, n int) string
	RelativeIn  func(unit RelativeUnit, n int) string
}

// englishStrings returns the English text. It is shared by the fallbacks, so its
// fields are read and never changed.
func englishStrings() *Strings {
	return &Strings{
		Avatar:         "Avatar",
		Breadcrumb:     "Breadcrumb",
		Pagination:     "Pagination",
		Navigation:     "Navigation",
		Options:        "Options",
		Suggestions:    "Suggestions",
		Times:          "Times",
		Actions:        "Actions",
		Color:          "Color",
		Progress:       "Progress",
		Loading:        "Loading",
		Notifications:  "Notifications",
		ResizePanes:    "Resize panes",
		DateField:      "Date",
		DateRangeField: "Date range",
		Steps:          "Steps",
		CommandPalette: "Command palette",
		Carousel:       "Carousel",
		Slides:         "Slides",

		LoadingEllipsis:           "Loading…",
		LoadingSuggestions:        "Loading suggestions",
		CommandPalettePlaceholder: "Type a command",
		SaturationBrightness:      "Saturation and brightness",
		SaturationBrightnessValue: func(saturation, brightness int) string {
			return fmt.Sprintf("Saturation %d%%, brightness %d%%", saturation, brightness)
		},
		Hue:                 "Hue",
		Opacity:             "Opacity",
		Hex:                 "Hex",
		ChooseDate:          "Choose date",
		ChooseDates:         "Choose dates",
		ShowTimes:           "Show times",
		ShowHiddenLocations: "Show hidden locations",
		CollapseNavigation:  "Collapse navigation",
		ExpandNavigation:    "Expand navigation",
		DismissNotification: "Dismiss notification",
		ToastAnnouncement:   func(title, text string) string { return title + ": " + text },
		PreviousPage:        "Previous page",
		NextPage:            "Next page",
		PreviousSlide:       "Previous slide",
		NextSlide:           "Next slide",
		PauseSlides:         "Pause slides",
		PlaySlides:          "Play slides",
		PreviousYear:        "Previous year",
		PreviousMonth:       "Previous month",
		NextMonth:           "Next month",
		NextYear:            "Next year",
		Close:               "Close",
		OK:                  "OK",
		Cancel:              "Cancel",
		Minimize:            "Minimize",
		Maximize:            "Maximize",
		Restore:             "Restore",
		Copy:                "Copy",
		Copied:              "Copied",
		Increase:            "Increase",
		Decrease:            "Decrease",
		ShowPassword:        "Show password",
		HidePassword:        "Hide password",
		Clear:               "Clear",
		Minimum:             "Minimum",
		Maximum:             "Maximum",
		MoreOptions:         "More options",
		AllTabs:             "All tabs",
		ScrollTabsBack:      "Scroll tabs back",
		ScrollTabsForward:   "Scroll tabs forward",

		PanelGroup:      "Panels",
		MovePanelLeft:   "Move left",
		MovePanelRight:  "Move right",
		MovePanelUp:     "Move up",
		MovePanelDown:   "Move down",
		SplitPanelRight: "Split right",
		SplitPanelDown:  "Split down",
		FloatPanel:      "Float",
		DockPanelBack:   "Dock back",
		ClosePanel:      "Close panel",

		Remove:        func(tag string) string { return "Remove " + tag },
		ResizeBetween: func(first, second string) string { return "Resize " + first + " and " + second },
		Page:          func(n int) string { return fmt.Sprintf("Page %d", n) },
		CurrentPage:   func(n int) string { return fmt.Sprintf("Page %d, current page", n) },
		PageOf:        func(page, pages int) string { return fmt.Sprintf("Page %d of %d", page, pages) },
		SlideOf:       func(n, slides int) string { return fmt.Sprintf("Slide %d of %d", n, slides) },
		GoToSlide:     func(n int) string { return fmt.Sprintf("Go to slide %d", n) },
		RatingValue: func(value, max int) string {
			if max == 1 {
				return fmt.Sprintf("%d of 1 star", value)
			}
			return fmt.Sprintf("%d of %d stars", value, max)
		},

		StepCompleted: func(n, steps int, title string) string {
			return fmt.Sprintf("Step %d of %d: %s, completed", n, steps, title)
		},
		StepCurrent: func(n, steps int, title string) string {
			return fmt.Sprintf("Step %d of %d: %s, current", n, steps, title)
		},
		StepUpcoming: func(n, steps int, title string) string {
			return fmt.Sprintf("Step %d of %d: %s, not started", n, steps, title)
		},

		TabMoved: func(tab string, position, count int) string {
			return fmt.Sprintf("%s moved to position %d of %d", tab, position, count)
		},
		ColumnMoved: func(column string, position, count int) string {
			return fmt.Sprintf("%s moved to position %d of %d", column, position, count)
		},

		Today:        "Today",
		StartOfRange: "Start of range",
		EndOfRange:   "End of range",
		InRange:      "In range",

		NoResults:               "No results",
		NoRows:                  "No rows",
		LoadingMore:             "Loading more rows",
		NoMatchingCommands:      "No matching commands",
		CouldNotLoadSuggestions: "Couldn't load suggestions",
		SuggestionCount: func(n int) string {
			if n == 1 {
				return "1 suggestion"
			}
			return fmt.Sprintf("%d suggestions", n)
		},
		NotInList:         "Not in the list; cleared",
		NotInListReverted: func(label string) string { return "Not in the list; reverted to " + label },
		SortedAscending:   "sorted ascending",
		SortedDescending:  "sorted descending",

		BadgeOverflow:     func(max int) string { return fmt.Sprintf("%d+", max) },
		BadgeMoreThan:     func(max int) string { return fmt.Sprintf("more than %d", max) },
		BadgeMoreThanNoun: func(max int, noun string) string { return fmt.Sprintf("more than %d %s", max, noun) },

		Required:         "This field is required.",
		MinLength:        func(n int) string { return fmt.Sprintf("Enter at least %d characters.", n) },
		MaxLength:        func(n int) string { return fmt.Sprintf("Enter no more than %d characters.", n) },
		ValueRange:       func(lo, hi string) string { return fmt.Sprintf("Enter a value from %s to %s.", lo, hi) },
		NumberRange:      func(lo, hi string) string { return fmt.Sprintf("Enter a number from %s to %s.", lo, hi) },
		DateOnOrAfter:    func(date string) string { return "Enter a date on or after " + date + "." },
		DateOnOrBefore:   func(date string) string { return "Enter a date on or before " + date + "." },
		DateNotAllowed:   "That date cannot be chosen.",
		DateExample:      func(example string) string { return "Enter a date such as " + example + "." },
		RangeExample:     func(example string) string { return "Enter a range such as " + example + "." },
		RangeStartChosen: "Start of range chosen. Choose the end of the range.",
		TimeFromTo:       func(from, to string) string { return "Enter a time from " + from + " to " + to + "." },
		TimeExample:      func(example string) string { return "Enter a time such as " + example + "." },

		FieldsNeedAttention: func(n int) string {
			if n == 1 {
				return "1 field needs attention."
			}
			return fmt.Sprintf("%d fields need attention.", n)
		},
		FormSubmitFailed: "The form could not be submitted.",

		CodeEditorLabel:      "Code editor",
		CodeEditorFind:       "Find",
		CodeEditorReplace:    "Replace",
		CodeEditorReplaceOne: "Replace",
		CodeEditorReplaceAll: "Replace all",
		CodeEditorNext:       "Next",
		CodeEditorPrevious:   "Previous",
		CodeEditorClose:      "Close",
		CodeEditorMatchCase:  "Match case",
		CodeEditorWholeWord:  "Whole word",
		CodeEditorRegex:      "Regular expression",
		CodeEditorGoToLine:   "Line number",
		CodeEditorGo:         "Go",
		CodeEditorNoMatches:  "No results",
		CodeEditorBadPattern: "Invalid pattern",
		CodeEditorCount: func(current, total int) string {
			return fmt.Sprintf("%d of %d", current, total)
		},
		CodeEditorError:   "Error",
		CodeEditorWarning: "Warning",
		CodeEditorInfo:    "Information",
		CodeEditorHint:    "Hint",
		CodeEditorDiagnostic: func(severity string, line int, message string) string {
			return fmt.Sprintf("%s on line %d: %s", severity, line, message)
		},
		CodeEditorCodeActions:    "Code actions",
		CodeEditorQuickFix:       "Quick fix",
		CodeEditorRefactor:       "Refactor",
		CodeEditorSourceAction:   "Source action",
		CodeEditorMoreActions:    "More actions",
		CodeEditorNoCodeActions:  "No code actions available",
		CodeEditorLoadingActions: "Loading actions",
		CodeEditorActionFailed:   "Could not apply the action",
		CodeEditorActionCount: func(n int) string {
			return fmt.Sprintf("%d actions", n)
		},
		CodeEditorActionUnavailable: func(title, reason string) string {
			return fmt.Sprintf("%s, unavailable: %s", title, reason)
		},

		ChartLine:                 "Line chart",
		ChartArea:                 "Area chart",
		ChartStackedArea:          "Stacked area chart",
		ChartBar:                  "Bar chart",
		ChartStackedBar:           "Stacked bar chart",
		ChartHorizontalBar:        "Horizontal bar chart",
		ChartStackedHorizontalBar: "Stacked horizontal bar chart",
		ChartScatter:              "Scatter plot",
		ChartPie:                  "Pie chart",
		ChartDonut:                "Donut chart",
		ChartRadar:                "Radar chart",
		ChartCandlestick:          "Candlestick chart",
		ChartSankey:               "Sankey diagram",

		ChartSeriesSummary: func(chart string, series, least, most int) string {
			s := fmt.Sprintf("%s, %d series", chart, series)
			switch {
			case series == 0:
				return s
			case least == most && least == 1:
				return s + ", 1 point"
			case least == most:
				return fmt.Sprintf("%s, %d points each", s, least)
			}
			return fmt.Sprintf("%s, %d to %d points each", s, least, most)
		},
		ChartCandleSummary: func(chart string, candles int) string {
			return chart + ", " + englishCount(candles, "candle", "candles")
		},
		ChartPieSummary: func(chart string, slices int) string {
			return chart + ", " + englishCount(slices, "slice", "slices")
		},
		ChartSankeySummary: func(chart string, nodes, links int, total string) string {
			return chart + ", " + englishCount(nodes, "node", "nodes") + ", " +
				englishCount(links, "link", "links") + "; total flow " + total
		},
		ChartLargestSlice:  func(summary, slice, share string) string { return summary + "; largest is " + slice + " at " + share },
		ChartPositionRange: func(summary, from, to string) string { return summary + ", from " + from + " to " + to },
		ChartValueRanges:   func(summary string, ranges []string) string { return summary + "; " + strings.Join(ranges, ", ") },
		ChartValueRange:    func(series, lo, hi string) string { return series + " from " + lo + " to " + hi },

		ChartPointValue:   func(position, series, value string) string { return position + ", " + series + " " + value },
		ChartPointNoValue: func(position, series string) string { return position + ", " + series + " no value" },
		ChartSeriesValue:  func(series, value string) string { return series + " " + value },
		ChartSliceValue:   func(slice, value, share string) string { return slice + ", " + value + " (" + share + ")" },
		ChartValueShare:   func(value, share string) string { return value + " (" + share + ")" },
		ChartCandleValue: func(position, opening, high, low, closing string) string {
			return position + ", open " + opening + ", high " + high + ", low " + low + ", close " + closing
		},
		ChartLink:      func(from, to string) string { return from + " to " + to },
		ChartLinkValue: func(from, to, value string) string { return from + " to " + to + ", " + value },

		ChartLegend:       "Legend",
		ChartSeriesNumber: func(n int) string { return fmt.Sprintf("Series %d", n) },
		ChartDataTable:    func(summary string) string { return "Data table: " + summary },
		ChartDate:         "Date",
		ChartCategory:     "Category",
		ChartPosition:     "Position",
		ChartSlice:        "Slice",
		ChartValue:        "Value",
		ChartShare:        "Share",
		ChartFrom:         "From",
		ChartTo:           "To",
		ChartOpen:         "Open",
		ChartHigh:         "High",
		ChartLow:          "Low",
		ChartClose:        "Close",

		MarkdownImage: "Image",
		MarkdownImageNotShown: func(alt string) string {
			if alt == "" {
				return "Image not shown"
			}
			return "Image not shown: " + alt
		},
		MarkdownTask: "Task",
		MarkdownCode: func(language string) string {
			if language == "" {
				return "Code"
			}
			return "Code, " + language
		},
		MarkdownFootnotes: "Footnotes",

		// The English forms follow CLDR 48.2's relativeTime long forms (en).
		RelativeNow:       "now",
		RelativeYesterday: "yesterday",
		RelativeTomorrow:  "tomorrow",
		RelativeAgo: func(u RelativeUnit, n int) string {
			return englishCount(n, englishUnits[u][0], englishUnits[u][1]) + " ago"
		},
		RelativeIn: func(u RelativeUnit, n int) string {
			return "in " + englishCount(n, englishUnits[u][0], englishUnits[u][1])
		},
	}
}

// englishUnits are the one and many nouns of the units of a relative time, by RelativeUnit.
var englishUnits = [...][2]string{
	RelativeMinute: {"minute", "minutes"},
	RelativeHour:   {"hour", "hours"},
	RelativeDay:    {"day", "days"},
	RelativeWeek:   {"week", "weeks"},
	RelativeMonth:  {"month", "months"},
	RelativeYear:   {"year", "years"},
}

// englishCount returns n with the English noun for its number: one for 1, many otherwise.
func englishCount(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// EnglishStrings returns the English text as a new Strings value, which the app can
// change and set. It is a copy, so a change does not reach other windows.
func EnglishStrings() *Strings { return englishStrings() }

// StringsFor returns the text for a language, given as a BCP 47 tag such as "it-IT",
// "zh-Hant-TW" or "zh-HK". The packs are zh-CN (Simplified Chinese, the default for
// any other zh tag), zh-TW and zh-HK (Traditional Chinese, for Taiwan and Hong Kong
// or Macao, or any Hant tag), and it (Italian). A tag without a pack gives the English
// text. A field that the pack leaves empty is English, so the result is complete.
func StringsFor(locale string) *Strings {
	return completeStrings(packFor(locale))
}

// SystemStrings returns the text for the display language of the user's Windows, the
// first of its preferred UI languages (see StringsFor). Elsewhere, and where the
// language cannot be read, it is the English text.
func SystemStrings() *Strings {
	return StringsFor(systemLanguage())
}

// packFor returns the translated pack for a BCP 47 tag, or nil when there is none.
func packFor(tag string) *Strings {
	parts := strings.Split(strings.ToLower(strings.ReplaceAll(tag, "_", "-")), "-")
	switch parts[0] {
	case "it":
		return italianStrings()
	case "zh":
		region, script := "", ""
		for _, p := range parts[1:] {
			switch len(p) {
			case 2:
				region = p
			case 4:
				script = p
			}
		}
		switch {
		case region == "tw", region == "" && script == "hant":
			return chineseTaiwanStrings()
		case region == "hk", region == "mo":
			return chineseHongKongStrings()
		default:
			return chineseMainlandStrings()
		}
	}
	return nil
}

// completeStrings returns a new Strings with the English text, and with each field that
// s sets (a non-empty string or a non-nil function) in place of the English one. A nil
// s is English. The result is never shared with s or with another caller.
func completeStrings(s *Strings) *Strings {
	out := englishStrings()
	if s == nil {
		return out
	}
	src, dst := reflect.ValueOf(s).Elem(), reflect.ValueOf(out).Elem()
	for i := range src.NumField() {
		if !src.Field(i).IsZero() {
			dst.Field(i).Set(src.Field(i))
		}
	}
	return out
}
