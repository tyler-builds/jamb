package ui

import (
	"math"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
)

// currencyDE is a de-DE currency: the euro after the amount with a space, a minus
// before the amount, and the comma for the decimal point.
func currencyDE() *dateLocale {
	l := localeDE()
	l.currency, l.currDigits, l.currPositive, l.currNeg = "€", 2, 3, 8
	l.monDecimal, l.monGroup, l.monGrouping = ",", ".", []int{3}
	return l
}

func TestFormatCurrencyFollowsTheRegion(t *testing.T) {
	en := testLocaleEN()
	paren := testLocaleEN()
	paren.currNeg = 0
	yen := testLocaleEN()
	yen.currency, yen.currDigits = "¥", 0
	// Amounts of money may group differently from other numbers.
	swiss := testLocaleEN()
	swiss.currency, swiss.currPositive, swiss.currNeg = "CHF", 2, 12
	swiss.monDecimal, swiss.monGroup = ".", "'"
	for _, c := range []struct {
		name string
		loc  *dateLocale
		v    float64
		want string
	}{
		{"en-US", en, 1234.5, "$1,234.50"},
		{"en-US negative", en, -5, "-$5.00"},
		{"en-US rounds", en, 0.125, "$0.13"},
		{"en-US rounds the digits as written", en, 1.005, "$1.01"},
		{"en-US rounds up a whole", en, 9.995, "$10.00"},
		{"en-US rounds to zero", en, -0.004, "$0.00"},
		{"parentheses", paren, -1234.5, "($1,234.50)"},
		{"de-DE", currencyDE(), 1234.5, "1.234,50 €"},
		{"de-DE negative", currencyDE(), -5, "-5,00 €"},
		{"no decimals", yen, 1234.4, "¥1,234"},
		{"monetary separators", swiss, -1234.5, "CHF -1'234.50"},
		{"NaN", en, math.NaN(), ""},
	} {
		if got := formatCurrency(c.v, c.loc); got != c.want {
			t.Errorf("%s: formatCurrency(%v) = %q, want %q", c.name, c.v, got, c.want)
		}
	}
}

// TestFormatCurrencyPatterns checks Windows' four positive and sixteen negative
// shapes of an amount (LOCALE_ICURRENCY and LOCALE_INEGCURR).
func TestFormatCurrencyPatterns(t *testing.T) {
	positive := []string{"$1.10", "1.10$", "$ 1.10", "1.10 $"}
	negative := []string{"($1.10)", "-$1.10", "$-1.10", "$1.10-", "(1.10$)", "-1.10$", "1.10-$", "1.10$-",
		"-1.10 $", "-$ 1.10", "1.10 $-", "$ 1.10-", "$ -1.10", "1.10- $", "($ 1.10)", "(1.10 $)"}
	for i, want := range positive {
		l := testLocaleEN()
		l.currPositive = i
		if got := formatCurrency(1.1, l); got != want {
			t.Errorf("positive pattern %d = %q, want %q", i, got, want)
		}
	}
	for i, want := range negative {
		l := testLocaleEN()
		l.currNeg = i
		if got := formatCurrency(-1.1, l); got != want {
			t.Errorf("negative pattern %d = %q, want %q", i, got, want)
		}
	}
}

// TestCurrencyFieldTypesAndShowsTheSymbol checks that a currency field reads and
// shows the amount with the monetary separators, takes the currency's digits, and
// shows the symbol where the region puts it, hidden from assistive technology.
func TestCurrencyFieldTypesAndShowsTheSymbol(t *testing.T) {
	useLocale(t, currencyDE())
	f := NumberFormat(NumberOptions{Currency: true, Signed: true})
	if f.opts.Decimals != 2 {
		t.Fatalf("decimals = %d, want the currency's 2", f.opts.Decimals)
	}
	if got := f.Display("1234.5"); got != "1.234,5" {
		t.Errorf("Display = %q, want 1.234,5", got)
	}
	if raw, ok := f.fromDisplay("1.234,56"); !ok || raw != "1234.56" {
		t.Errorf("fromDisplay = %q, %v, want 1234.56", raw, ok)
	}
	amount := NewSignal("12.5")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Child(TextInput(amount).Key("amount").Label("Amount").Format(f))
	})
	if got := nodeText(r.rt.tree); !strings.HasSuffix(got, "€") {
		t.Errorf("the field shows %q, want the euro after the amount", got)
	}
	r.tree().Walk(func(n *access.Node) bool {
		if strings.Contains(n.Name, "€") || strings.Contains(n.Value, "€") {
			t.Errorf("assistive technology reads the symbol: %q %q", n.Name, n.Value)
		}
		return true
	})
	useLocale(t, testLocaleEN())
	r2 := newAccessRig(func(ctx *Context) Element {
		return Div().Child(TextInput(NewSignal("5")).Label("Amount").Format(NumberFormat(NumberOptions{Currency: true})))
	})
	if got := nodeText(r2.rt.tree); !strings.HasPrefix(got, "$") {
		t.Errorf("en-US field shows %q, want the dollar before the amount", got)
	}
}
