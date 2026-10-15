package ui

import (
	"reflect"
	"strings"
	"testing"
)

// The words of the charts are English in every pack: the packs leave the Chart fields
// empty, so StringsFor fills them in with English, and an app that wants them in
// another language sets them itself.
func TestChartStringsAreEnglishInEveryPack(t *testing.T) {
	for name, pack := range map[string]*Strings{
		"it":    italianStrings(),
		"zh-CN": chineseMainlandStrings(),
		"zh-TW": chineseTaiwanStrings(),
		"zh-HK": chineseHongKongStrings(),
	} {
		v := reflect.ValueOf(pack).Elem()
		for i := range v.NumField() {
			if f := v.Type().Field(i).Name; strings.HasPrefix(f, "Chart") && !v.Field(i).IsZero() {
				t.Errorf("the %s pack sets %s; the chart text is English only", name, f)
			}
		}
	}
	if got := StringsFor("it").ChartSeriesSummary("Line chart", 2, 7, 7); got != "Line chart, 2 series, 7 points each" {
		t.Errorf("Italian ChartSeriesSummary = %q, want the English fallback", got)
	}
}
