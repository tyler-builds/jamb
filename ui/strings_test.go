package ui

import (
	"reflect"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// Every message of the English text is set, so that a Strings value with the English
// fields is complete: each built-in control shows some text in every language.
func TestEnglishStringsIsComplete(t *testing.T) {
	checkComplete(t, "English", englishStrings())
}

// Each pack sets only what it translates, and StringsFor fills in the rest with English.
func TestPacksAreCompletedWithEnglish(t *testing.T) {
	for name, pack := range map[string]*Strings{
		"it":    italianStrings(),
		"zh-CN": chineseMainlandStrings(),
		"zh-TW": chineseTaiwanStrings(),
		"zh-HK": chineseHongKongStrings(),
	} {
		checkComplete(t, name, completeStrings(pack))
	}
}

// checkComplete reports each field of s that is empty or nil.
func checkComplete(t *testing.T, name string, s *Strings) {
	t.Helper()
	v := reflect.ValueOf(s).Elem()
	for i := range v.NumField() {
		if v.Field(i).IsZero() {
			t.Errorf("%s leaves %s empty", name, v.Type().Field(i).Name)
		}
	}
}

// EnglishStrings is a copy: a change to it does not reach the next copy.
func TestEnglishStringsIsACopy(t *testing.T) {
	s := EnglishStrings()
	s.Close = "Fermer"
	if got := EnglishStrings().Close; got != "Close" {
		t.Errorf("a copy of the English text has Close %q after a change to another copy", got)
	}
}

// The tag of a language selects its pack. Simplified Chinese is the default for zh,
// Traditional Chinese is for Taiwan and for the Hant script, Hong Kong and Macao have
// their own, and a tag without a pack is English.
func TestStringsForMatchesLanguageTags(t *testing.T) {
	cases := []struct {
		tag, navigation string
	}{
		{"it-IT", "Navigazione"},
		{"it", "Navigazione"},
		{"zh-CN", "导航"},
		{"zh", "导航"},
		{"zh-Hans", "导航"},
		{"zh-SG", "导航"},
		{"zh-TW", "導覽"},
		{"zh-Hant", "導覽"},
		{"zh-Hant-TW", "導覽"},
		{"zh-HK", "導航"},
		{"zh-Hant-HK", "導航"},
		{"zh-MO", "導航"},
		{"it_IT", "Navigazione"},
		{"fr-FR", "Navigation"},
		{"en-US", "Navigation"},
		{"", "Navigation"},
	}
	for _, c := range cases {
		if got := StringsFor(c.tag).Navigation; got != c.navigation {
			t.Errorf("StringsFor(%q).Navigation = %q, want %q", c.tag, got, c.navigation)
		}
	}
}

// A pack translates only some messages: the others are English in that language.
func TestPacksFallBackFieldByField(t *testing.T) {
	it := StringsFor("it")
	if it.Close != "Chiudi" {
		t.Errorf("Italian Close = %q", it.Close)
	}
	if got := it.MinLength(3); got != "Enter at least 3 characters." {
		t.Errorf("Italian MinLength, which is not translated, = %q", got)
	}
}

// A Strings value set on a runtime is completed with English, and a copy of it is kept:
// changing the value afterwards does not change the runtime.
func TestSetStringsCompletesAndCopies(t *testing.T) {
	rt := NewRuntime(func(ctx *Context) Element { return Div() }, DarkTheme(), nil)
	if rt.strings.Close != "Close" {
		t.Fatalf("a new runtime's Close = %q, want English", rt.strings.Close)
	}
	s := &Strings{Close: "Fermer"}
	rt.SetStrings(s)
	s.Close = "Quitter"
	if rt.strings.Close != "Fermer" || rt.strings.Minimize != "Minimize" || rt.strings.PageOf == nil {
		t.Errorf("after SetStrings: Close %q, Minimize %q", rt.strings.Close, rt.strings.Minimize)
	}
	rt.SetStrings(nil)
	if rt.strings.Close != "Close" {
		t.Errorf("after SetStrings(nil): Close %q, want English", rt.strings.Close)
	}
}

// The controls show the text of the language they are given, in a rendered tree: the
// default name of a pagination, a page button, a current page and the remove button
// of a tag. The Italian and the Chinese words are the ones of the packs.
func TestWidgetsShowTheirLanguage(t *testing.T) {
	page := NewSignal(4)
	cases := []struct {
		tag, pager, previous, page3, current, remove string
	}{
		{"it-IT", "Paginazione", "Pagina precedente", "Pagina 3", "Pagina 4, pagina corrente", "Rimuovi Sales"},
		{"zh-CN", "分页", "上一页", "第 3 页", "第 4 页，当前页", "移除 Sales"},
		{"zh-TW", "分頁", "上一頁", "第 3 頁", "第 4 頁，目前頁", "移除 Sales"},
	}
	for _, c := range cases {
		r := newAccessRig(func(ctx *Context) Element {
			return Div().Column().
				Child(Pagination(page, 20).Key("pager")).
				Child(Tag("Sales").Key("tag").OnRemove(func() {}))
		})
		r.rt.SetStrings(StringsFor(c.tag))
		tr := r.tree()
		accesstest.Find(t, tr, access.RoleGroup, c.pager)
		accesstest.Find(t, tr, access.RoleButton, c.previous)
		accesstest.Find(t, tr, access.RoleButton, c.page3)
		accesstest.Find(t, tr, access.RoleStaticText, c.current)
		accesstest.Find(t, tr, access.RoleButton, c.remove)
		if len(r.warnings) != 0 {
			t.Errorf("%s: accessibility warnings: %v", c.tag, r.warnings)
		}
	}
}

// The validation messages follow the language of the window that shows them: a
// built-in message is read from the Strings given to errorText, and a message without
// a translation is English there.
func TestValidationMessagesFollowTheLanguage(t *testing.T) {
	if got := errorText(Required[string]()(" "), StringsFor("it")); got != "Questo campo è obbligatorio." {
		t.Errorf("Italian required message = %q", got)
	}
	if got := errorText(MinLength(3)("ab"), StringsFor("zh-TW")); got != "Enter at least 3 characters." {
		t.Errorf("zh-TW minimum length message, which is not translated, = %q", got)
	}
	if got := MinLength(3)("ab").Error(); got != "Enter at least 3 characters." {
		t.Errorf("the English Error of a minimum length message = %q", got)
	}
}

// TestPackCoverage logs how many of the messages each pack translates, for the README
// and the review. It fails when a pack translates nothing.
func TestPackCoverage(t *testing.T) {
	total := reflect.TypeOf(Strings{}).NumField()
	for name, pack := range map[string]*Strings{
		"it":    italianStrings(),
		"zh-CN": chineseMainlandStrings(),
		"zh-TW": chineseTaiwanStrings(),
		"zh-HK": chineseHongKongStrings(),
	} {
		n := 0
		v := reflect.ValueOf(pack).Elem()
		for i := range v.NumField() {
			if !v.Field(i).IsZero() {
				n++
			}
		}
		t.Logf("%s translates %d of %d messages", name, n, total)
		if n == 0 {
			t.Errorf("the %s pack translates nothing", name)
		}
	}
}
