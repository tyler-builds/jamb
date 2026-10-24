package ui

import (
	"math"
	"testing"
)

func TestFormatNumberInLocale(t *testing.T) {
	for _, c := range []struct {
		name     string
		l        *dateLocale
		v        float64
		decimals int
		want     string
	}{
		{"en-US", testLocaleEN(), 12345, 0, "12,345"},
		{"en-US", testLocaleEN(), 12345.678, 2, "12,345.68"},
		{"en-US", testLocaleEN(), -1234.5, 1, "-1,234.5"},
		{"de-DE", localeDE(), 12345.678, 2, "12.345,68"},
		{"de-DE", localeDE(), -1234.5, 1, "-1.234,5"},
		{"hi-IN", localeHI(), 1234567, 0, "12,34,567"},
	} {
		if got := formatNumber(c.v, c.decimals, c.l); got != c.want {
			t.Errorf("%s: formatNumber(%v, %d) = %q, want %q", c.name, c.v, c.decimals, got, c.want)
		}
	}
}

func TestFormatNumberMatchesNumberFormat(t *testing.T) {
	useLocale(t, localeDE())
	f := NumberFormat(NumberOptions{Decimals: 2, Signed: true})
	for _, c := range []struct {
		v   float64
		raw string // the raw value the field holds
	}{
		{0, "0.00"},
		{7, "7.00"},
		{1234.5, "1234.50"},
		{-98765.43, "-98765.43"},
	} {
		want := f.Display(c.raw)
		if got := FormatNumber(c.v, 2); got != want {
			t.Errorf("FormatNumber(%v, 2) = %q, but NumberFormat shows %q", c.v, got, want)
		}
	}
}

func TestFormatNumberRoundsToDecimals(t *testing.T) {
	l := testLocaleEN()
	for _, c := range []struct {
		v        float64
		decimals int
		want     string
	}{
		{2.5, 0, "3"}, // ties go away from zero, as Windows rounds
		{3.5, 0, "4"},
		{0.125, 2, "0.13"},
		{1.005, 2, "1.01"}, // the digits as written, though the float is a little less
		{0.126, 2, "0.13"},
		{999.999, 2, "1,000.00"},
		{1.5, -3, "2"}, // a negative decimals is 0
	} {
		if got := formatNumber(c.v, c.decimals, l); got != c.want {
			t.Errorf("formatNumber(%v, %d) = %q, want %q", c.v, c.decimals, got, c.want)
		}
	}
}

func TestFormatNumberNoNegativeZero(t *testing.T) {
	l := testLocaleEN()
	for _, c := range []struct {
		v        float64
		decimals int
		want     string
	}{
		{-0.001, 2, "0.00"},
		{-0.4, 0, "0"},
		{math.Copysign(0, -1), 0, "0"},
		{-0.5, 1, "-0.5"},
	} {
		if got := formatNumber(c.v, c.decimals, l); got != c.want {
			t.Errorf("formatNumber(%v, %d) = %q, want %q", c.v, c.decimals, got, c.want)
		}
	}
}

func TestFormatNumberNonFinite(t *testing.T) {
	l := testLocaleEN()
	if got := formatNumber(math.NaN(), 2, l); got != "" {
		t.Errorf("NaN = %q, want empty", got)
	}
	if got := formatNumber(math.Inf(1), 2, l); got != "∞" {
		t.Errorf("+Inf = %q, want ∞", got)
	}
	if got := formatNumber(math.Inf(-1), 2, l); got != "-∞" {
		t.Errorf("-Inf = %q, want -∞", got)
	}
	if got := formatNumber(math.Inf(-1), 2, localeDE()); got != "-∞" {
		t.Errorf("-Inf in de-DE = %q, want -∞", got)
	}
}

func TestFormatBytesBoundaries(t *testing.T) {
	en := testLocaleEN()
	s := englishStrings()
	for _, c := range []struct {
		n    uint64
		want string
	}{
		{0, "0 bytes"},
		{1, "1 byte"},
		{2, "2 bytes"},
		{512, "512 bytes"},
		{1023, "1023 bytes"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{10239, "9.99 KB"},
		{10240, "10.0 KB"},
		{102400, "100 KB"},
		{1000 * 1024, "0.97 MB"},
		{1048575, "0.99 MB"},
		{1048576, "1.00 MB"},
		{12*1048576 + 330*1024, "12.3 MB"},
		{10 * 1048576, "10.0 MB"},
		{100 * 1048576, "100 MB"},
		{1073741824, "1.00 GB"},
		{1234567890, "1.14 GB"}, // 1.1499..., cut to two digits
		{1 << 40, "1.00 TB"},
		{1000 * 1073741824, "0.97 TB"},
		{1 << 50, "1.00 PB"},
		{math.MaxUint64, "16383 PB"},
	} {
		if got := formatBytes(c.n, en, s); got != c.want {
			t.Errorf("formatBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestFormatBytesDecimalPointFollowsLocale(t *testing.T) {
	s := englishStrings()
	if got := formatBytes(1536, localeDE(), s); got != "1,50 KB" {
		t.Errorf("de-DE 1.5 KB = %q, want 1,50 KB", got)
	}
	if got := formatBytes(12*1048576+330*1024, localeDE(), s); got != "12,3 MB" {
		t.Errorf("de-DE 12.3 MB = %q, want 12,3 MB", got)
	}
	if got := formatBytes(512, localeDE(), s); got != "512 bytes" {
		t.Errorf("de-DE 512 bytes = %q, want 512 bytes", got)
	}
}

func TestFormatBytesUsesTheWordsOfStrings(t *testing.T) {
	s := englishStrings()
	s.Byte, s.Bytes, s.KB = "octet", "octets", "Ko"
	if got := formatBytes(1, testLocaleEN(), s); got != "1 octet" {
		t.Errorf("one byte = %q, want 1 octet", got)
	}
	if got := formatBytes(3, testLocaleEN(), s); got != "3 octets" {
		t.Errorf("three bytes = %q, want 3 octets", got)
	}
	if got := formatBytes(2048, testLocaleEN(), s); got != "2.00 Ko" {
		t.Errorf("2 KB = %q, want 2.00 Ko", got)
	}
}

func TestFormatBytesAndNumberReadTheLocaleOverride(t *testing.T) {
	useLocale(t, localeDE())
	if got := FormatNumber(1234.5, 1); got != "1.234,5" {
		t.Errorf("FormatNumber under the de-DE override = %q", got)
	}
	if got := FormatBytes(1536); got != "1,50 KB" {
		t.Errorf("FormatBytes under the de-DE override = %q", got)
	}
}

// TestStringsBytesAreInEveryPack checks that every built-in pack has the size words,
// its own or the English ones that an empty field falls back to.
func TestStringsBytesAreInEveryPack(t *testing.T) {
	for name, pack := range map[string]*Strings{
		"en":    englishStrings(),
		"it":    italianStrings(),
		"zh-CN": chineseMainlandStrings(),
		"zh-TW": chineseTaiwanStrings(),
		"zh-HK": chineseHongKongStrings(),
	} {
		pack = completeStrings(pack)
		if pack.Byte == "" || pack.Bytes == "" || pack.KB == "" || pack.MB == "" ||
			pack.GB == "" || pack.TB == "" || pack.PB == "" {
			t.Errorf("the %s pack leaves a size word empty", name)
		}
	}
}
