package ui

import (
	"fmt"
	"reflect"
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

	// The labels of the controls that jamb draws.
	LoadingEllipsis string
	// LoadingSuggestions is the text shown while suggestions load.
	LoadingSuggestions string
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
	// PreviousPage is the name of the button that goes to the previous page.
	PreviousPage string
	// NextPage is the name of the button that goes to the next page.
	NextPage string
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
	// Minimize is the name of the button that minimizes a window.
	Minimize string
	// Maximize is the name of the button that maximizes a window.
	Maximize string
	// Restore is the name of the button that restores a maximized window.
	Restore string
	// Increase is the name of the button that increases a number.
	Increase string
	// Decrease is the name of the button that decreases a number.
	Decrease string

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
	// Page is the name of a page button, such as "Page 3". CurrentPage names the page
	// that is shown, and PageOf is the text of a compact pagination, such as "Page 3 of 9".
	Page func(n int) string
	// CurrentPage builds the name of the current page, from its number.
	CurrentPage func(n int) string
	// PageOf builds the name of a page, from its number and the number of pages.
	PageOf func(page, pages int) string

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

		LoadingEllipsis:      "Loading…",
		LoadingSuggestions:   "Loading suggestions",
		SaturationBrightness: "Saturation and brightness",
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
		PreviousPage:        "Previous page",
		NextPage:            "Next page",
		PreviousYear:        "Previous year",
		PreviousMonth:       "Previous month",
		NextMonth:           "Next month",
		NextYear:            "Next year",
		Close:               "Close",
		Minimize:            "Minimize",
		Maximize:            "Maximize",
		Restore:             "Restore",
		Increase:            "Increase",
		Decrease:            "Decrease",

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

		Today:        "Today",
		StartOfRange: "Start of range",
		EndOfRange:   "End of range",
		InRange:      "In range",

		NoResults:               "No results",
		NoRows:                  "No rows",
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
	}
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
