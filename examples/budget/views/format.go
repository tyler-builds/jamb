package views

import (
	"strings"

	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/budget/ledger"
)

// money shows an amount in the user's currency, as Windows' region settings write it.
func money(m ledger.Money) string { return ui.FormatCurrency(m.Float()) }

// amountColor is the color of an amount: money in is the theme's success color, and
// money out the text's. Under high contrast both are the text's.
func amountColor(t *ui.Theme, hc bool, m ledger.Money) ui.Color {
	if m > 0 && !hc {
		return t.Success
	}
	return t.Text
}

// moneyFormat is the format of a field for an amount: the user's currency, signed.
var moneyFormat = ui.NumberFormat(ui.NumberOptions{Currency: true})

// matches reports whether a transaction matches every word of the query: in its
// payee, its memo, its category's name or its amount as shown.
func matches(query string, t ledger.Transaction, category string) bool {
	hay := strings.ToLower(t.Payee + " " + t.Memo + " " + category + " " + money(t.Amount) + " " + money(-t.Amount))
	for w := range strings.FieldsSeq(strings.ToLower(query)) {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}
