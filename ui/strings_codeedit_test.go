package ui

import "testing"

// The find bar of the code editor is translated where the packs set its fields, and the fields a
// pack leaves unset show English, so that an unset term is never empty.
func TestCodeEditorFindStringsFollowTheLanguage(t *testing.T) {
	for _, tc := range []struct {
		locale, find, regex string
	}{
		{"it", "Trova", "Espressione regolare"},
		{"zh-CN", "查找", "正则表达式"},
		{"zh-TW", "查找", "正規表示式"},
		{"zh-HK", "查找", "Regular expression"},
	} {
		s := StringsFor(tc.locale)
		if s.CodeEditorFind != tc.find {
			t.Errorf("%s: CodeEditorFind = %q, want %q", tc.locale, s.CodeEditorFind, tc.find)
		}
		if s.CodeEditorRegex != tc.regex {
			t.Errorf("%s: CodeEditorRegex = %q, want %q", tc.locale, s.CodeEditorRegex, tc.regex)
		}
		if s.CodeEditorCount == nil || s.CodeEditorCount(3, 12) == "" {
			t.Errorf("%s: CodeEditorCount is not set", tc.locale)
		}
	}
	if got := StringsFor("it").CodeEditorCount(3, 12); got != "3 di 12" {
		t.Errorf("Italian count = %q, want %q", got, "3 di 12")
	}
	if got := StringsFor("zh-CN").CodeEditorCount(3, 12); got != "3 of 12" {
		t.Errorf("zh-CN count = %q, want the English fallback %q", got, "3 of 12")
	}
}
