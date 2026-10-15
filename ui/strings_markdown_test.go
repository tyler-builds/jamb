package ui

import (
	"reflect"
	"strings"
	"testing"
)

// The words of the Markdown view are English in every pack: the packs leave the
// Markdown fields empty, so StringsFor fills them in with English.
func TestMarkdownStringsAreEnglishInEveryPack(t *testing.T) {
	for name, pack := range map[string]*Strings{
		"it":    italianStrings(),
		"zh-CN": chineseMainlandStrings(),
		"zh-TW": chineseTaiwanStrings(),
		"zh-HK": chineseHongKongStrings(),
	} {
		v := reflect.ValueOf(pack).Elem()
		for i := range v.NumField() {
			if f := v.Type().Field(i).Name; strings.HasPrefix(f, "Markdown") && !v.Field(i).IsZero() {
				t.Errorf("the %s pack sets %s; the Markdown text is English only", name, f)
			}
		}
	}
	s := StringsFor("it")
	for _, c := range []struct{ got, want string }{
		{s.MarkdownImageNotShown(""), "Image not shown"},
		{s.MarkdownImageNotShown("Build status"), "Image not shown: Build status"},
		{s.MarkdownCode("go"), "Code, go"},
		{s.MarkdownCode(""), "Code"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}
