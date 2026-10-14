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
	Avatar         string
	Breadcrumb     string
	Pagination     string
	Navigation     string
	Options        string
	Suggestions    string
	Times          string
	Actions        string
	Color          string
	Progress       string
	Loading        string
	Notifications  string
	ResizePanes    string
	DateField      string
	DateRangeField string

	// The labels of the controls that jamb draws.
	LoadingEllipsis      string
	LoadingSuggestions   string
	SaturationBrightness string
	// SaturationBrightnessValue is the value of the color area, read by assistive technology,
	// such as "Saturation 50%, brightness 80%".
	SaturationBrightnessValue func(saturation, brightness int) string
	Hue                       string
	Opacity                   string
	Hex                       string
	ChooseDate                string
	ChooseDates               string
	ShowTimes                 string
	ShowHiddenLocations       string
	CollapseNavigation        string
	ExpandNavigation          string
	DismissNotification       string
	PreviousPage              string
	NextPage                  string
	PreviousYear              string
	PreviousMonth             string
	NextMonth                 string
	NextYear                  string
	Close                     string
	Minimize                  string
	Maximize                  string
	Restore                   string
	Increase                  string
	Decrease                  string

	// The names of a dock's tab strip and of the commands of its tabs' menus.
	PanelGroup      string
	MovePanelLeft   string
	MovePanelRight  string
	MovePanelUp     string
	MovePanelDown   string
	SplitPanelRight string
	SplitPanelDown  string
	FloatPanel      string
	DockPanelBack   string
	ClosePanel      string

	// Remove names the button that removes a tag, such as "Remove Sales".
	Remove func(tag string) string
	// ResizeBetween names the divider between two panes that have labels, such as "Resize Sidebar and
	// Editor" (see Split.Label).
	ResizeBetween func(first, second string) string
	// Page is the name of a page button, such as "Page 3". CurrentPage names the page
	// that is shown, and PageOf is the text of a compact pagination, such as "Page 3 of 9".
	Page        func(n int) string
	CurrentPage func(n int) string
	PageOf      func(page, pages int) string

	// The notes of a calendar's days: Today, and the ends and the middle of a range.
	Today        string
	StartOfRange string
	EndOfRange   string
	InRange      string

	// The empty and error texts of the lists, the grid and the suggestions.
	NoResults               string
	NoRows                  string
	CouldNotLoadSuggestions string
	SuggestionCount         func(n int) string
	NotInList               string
	NotInListReverted       func(label string) string
	SortedAscending         string
	SortedDescending        string

	// The text of a badge that counts more than its maximum, such as "99+", and the
	// name it adds to its owner, such as "more than 99 items".
	BadgeOverflow     func(max int) string
	BadgeMoreThan     func(max int) string
	BadgeMoreThanNoun func(max int, noun string) string

	// The validation messages of the built-in validators and the controls.
	Required         string
	MinLength        func(n int) string
	MaxLength        func(n int) string
	ValueRange       func(lo, hi string) string
	NumberRange      func(lo, hi string) string
	DateOnOrAfter    func(date string) string
	DateOnOrBefore   func(date string) string
	DateNotAllowed   string
	DateExample      func(example string) string
	RangeExample     func(example string) string
	RangeStartChosen string
	TimeFromTo       func(from, to string) string
	TimeExample      func(example string) string

	// FieldsNeedAttention is the summary of a form with n fields that have a
	// message, and FormSubmitFailed is the message of a form whose submit failed
	// without a message of its own.
	FieldsNeedAttention func(n int) string
	FormSubmitFailed    string

	// The code editor (package codeedit): its default name, the labels of its find and
	// replace bar and of its go-to-line bar, and the words that its matches count shows.
	// CodeEditorCount is the count of matches, such as "3 of 12".
	CodeEditorLabel      string
	CodeEditorFind       string
	CodeEditorReplace    string
	CodeEditorReplaceOne string
	CodeEditorReplaceAll string
	CodeEditorNext       string
	CodeEditorPrevious   string
	CodeEditorClose      string
	CodeEditorMatchCase  string
	CodeEditorWholeWord  string
	CodeEditorRegex      string
	CodeEditorGoToLine   string
	CodeEditorGo         string
	CodeEditorNoMatches  string
	CodeEditorBadPattern string
	CodeEditorCount      func(current, total int) string
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
