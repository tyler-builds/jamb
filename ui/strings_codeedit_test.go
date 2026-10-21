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

// The messages of the code editor's diagnostics have English text in every language until a
// pack translates them.
func TestCodeEditorDiagnosticStringsFallBackToEnglish(t *testing.T) {
	for _, locale := range []string{"en", "it", "zh-CN", "zh-TW", "zh-HK"} {
		s := StringsFor(locale)
		for name, v := range map[string]string{"CodeEditorError": s.CodeEditorError, "CodeEditorWarning": s.CodeEditorWarning,
			"CodeEditorInfo": s.CodeEditorInfo, "CodeEditorHint": s.CodeEditorHint} {
			if v == "" {
				t.Errorf("%s: %s is empty", locale, name)
			}
		}
		if got := s.CodeEditorDiagnostic(s.CodeEditorError, 3, "undefined: x"); got != "Error on line 3: undefined: x" {
			t.Errorf("%s: the diagnostic message is %q", locale, got)
		}
	}
}

// The names and messages of the code actions menu are translated where the packs say so, and
// every locale has a non-empty text for each of them.
func TestCodeEditorCodeActionStrings(t *testing.T) {
	for _, locale := range []string{"en", "it", "zh-CN", "zh-TW", "zh-HK"} {
		s := StringsFor(locale)
		for name, v := range map[string]string{"CodeEditorCodeActions": s.CodeEditorCodeActions,
			"CodeEditorQuickFix": s.CodeEditorQuickFix, "CodeEditorRefactor": s.CodeEditorRefactor,
			"CodeEditorSourceAction": s.CodeEditorSourceAction, "CodeEditorMoreActions": s.CodeEditorMoreActions,
			"CodeEditorNoCodeActions": s.CodeEditorNoCodeActions, "CodeEditorLoadingActions": s.CodeEditorLoadingActions,
			"CodeEditorActionFailed": s.CodeEditorActionFailed} {
			if v == "" {
				t.Errorf("%s: %s is empty", locale, name)
			}
		}
		if s.CodeEditorActionCount == nil || s.CodeEditorActionCount(3) == "" {
			t.Errorf("%s: CodeEditorActionCount is not set", locale)
		}
		if s.CodeEditorActionUnavailable == nil || s.CodeEditorActionUnavailable("Add import", "x") == "" {
			t.Errorf("%s: CodeEditorActionUnavailable is not set", locale)
		}
	}
	if got := StringsFor("en").CodeEditorActionUnavailable("Add import", "no file"); got != "Add import, unavailable: no file" {
		t.Errorf("English unavailable = %q", got)
	}
	if got := StringsFor("en").CodeEditorActionCount(3); got != "3 actions" {
		t.Errorf("English count = %q, want %q", got, "3 actions")
	}
	it := StringsFor("it")
	if it.CodeEditorCodeActions != "Azioni codice" || it.CodeEditorQuickFix != "Correzione rapida" ||
		it.CodeEditorNoCodeActions != "Nessuna azione disponibile" {
		t.Errorf("Italian code action strings = %q, %q, %q", it.CodeEditorCodeActions, it.CodeEditorQuickFix, it.CodeEditorNoCodeActions)
	}
	if got := StringsFor("zh-CN").CodeEditorCodeActions; got != "代码操作" {
		t.Errorf("zh-CN CodeEditorCodeActions = %q, want %q", got, "代码操作")
	}
	if got := StringsFor("zh-TW").CodeEditorCodeActions; got != "程式碼動作" {
		t.Errorf("zh-TW CodeEditorCodeActions = %q, want %q", got, "程式碼動作")
	}
}
