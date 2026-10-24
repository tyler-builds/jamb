// Package ledger holds a household's money: its accounts, the categories of its
// spending and income, its transactions, and the amount it budgets for each
// category each month. Data is a value; a Store holds it in a signal, records an
// undo step for each change, and saves it as JSON.
package ledger

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Money is an amount in hundredths of the currency, such as cents: -1250 is a
// spending of 12.50. Whole hundredths keep sums exact.
type Money int64

// Float returns the amount in units of the currency, for formatting.
func (m Money) Float() float64 { return float64(m) / 100 }

// FromFloat returns the amount of v units of the currency, rounded to a hundredth.
func FromFloat(v float64) Money { return Money(math.Round(v * 100)) }

// Raw returns the amount as a number field holds it, such as "-12.5", or "" for none.
func (m Money) Raw() string {
	if m == 0 {
		return ""
	}
	return strconv.FormatFloat(m.Float(), 'f', -1, 64)
}

// ParseRaw reads the raw value of a number field, such as "-12.5".
func ParseRaw(s string) (Money, bool) {
	if strings.TrimSpace(s) == "" {
		return 0, true
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, false
	}
	return FromFloat(v), true
}

// Kind is the kind of an account.
type Kind string

const (
	Checking Kind = "checking"
	Savings  Kind = "savings"
	Credit   Kind = "credit"
	Cash     Kind = "cash"
)

// Kinds lists the kinds of account, in the order the app offers them.
var Kinds = []Kind{Checking, Savings, Credit, Cash}

// Label names a kind for people.
func (k Kind) Label() string {
	switch k {
	case Savings:
		return "Savings"
	case Credit:
		return "Credit card"
	case Cash:
		return "Cash"
	}
	return "Checking"
}

// Account is where money is kept. Opening is its balance before its first
// transaction.
type Account struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    Kind   `json:"kind"`
	Opening Money  `json:"opening"`
}

// Category sorts transactions. Group gathers categories in the budget, such as
// "Bills"; Income marks the categories that money comes in under.
type Category struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Group  string `json:"group"`
	Income bool   `json:"income,omitempty"`
}

// Uncategorized is the category of a transaction that has none.
const Uncategorized = ""

// Transaction is money in (a positive Amount) or out (negative) of an account on a
// day, from or to a payee.
type Transaction struct {
	ID       string    `json:"id"`
	Date     time.Time `json:"date"`
	Payee    string    `json:"payee"`
	Category string    `json:"category,omitempty"`
	Account  string    `json:"account"`
	Amount   Money     `json:"amount"`
	Memo     string    `json:"memo,omitempty"`
	Cleared  bool      `json:"cleared,omitempty"`
}

// Month is a calendar month, the period of a budget.
type Month struct {
	Year  int
	Month time.Month
}

// MonthOf returns the month of t.
func MonthOf(t time.Time) Month { return Month{t.Year(), t.Month()} }

// Start returns the first day of the month, at local midnight.
func (m Month) Start() time.Time { return time.Date(m.Year, m.Month, 1, 0, 0, 0, 0, time.Local) }

// Add returns the month n months after m (before, for a negative n).
func (m Month) Add(n int) Month { return MonthOf(m.Start().AddDate(0, n, 0)) }

// Contains reports whether t falls in the month.
func (m Month) Contains(t time.Time) bool { return MonthOf(t) == m }

// Key returns the month as the budgets are keyed, such as "2026-10".
func (m Month) Key() string { return fmt.Sprintf("%04d-%02d", m.Year, int(m.Month)) }

// Compare orders months.
func (m Month) Compare(o Month) int {
	return cmp.Or(cmp.Compare(m.Year, o.Year), cmp.Compare(m.Month, o.Month))
}

// Data is everything the ledger holds. Treat it as a value: change a copy, with
// the Store's methods, and never a slice of it in place.
type Data struct {
	Accounts     []Account     `json:"accounts"`
	Categories   []Category    `json:"categories"`
	Transactions []Transaction `json:"transactions"`
	// Budgets holds the amount budgeted for each category, by month key ("2026-10")
	// and category ID.
	Budgets map[string]map[string]Money `json:"budgets"`
	// NextID numbers the next new item.
	NextID int `json:"nextId"`
}

// clone returns a copy of d that shares no slice or map with it.
func (d Data) clone() Data {
	d.Accounts = slices.Clone(d.Accounts)
	d.Categories = slices.Clone(d.Categories)
	d.Transactions = slices.Clone(d.Transactions)
	b := make(map[string]map[string]Money, len(d.Budgets))
	for k, m := range d.Budgets {
		inner := make(map[string]Money, len(m))
		for c, v := range m {
			inner[c] = v
		}
		b[k] = inner
	}
	d.Budgets = b
	return d
}

// newID returns an ID with prefix that no item has, and counts it.
func (d *Data) newID(prefix string) string {
	d.NextID++
	return prefix + strconv.Itoa(d.NextID)
}

// Account returns the account with id.
func (d Data) Account(id string) (Account, bool) {
	i := slices.IndexFunc(d.Accounts, func(a Account) bool { return a.ID == id })
	if i < 0 {
		return Account{}, false
	}
	return d.Accounts[i], true
}

// Category returns the category with id.
func (d Data) Category(id string) (Category, bool) {
	i := slices.IndexFunc(d.Categories, func(c Category) bool { return c.ID == id })
	if i < 0 {
		return Category{}, false
	}
	return d.Categories[i], true
}

// CategoryName returns the name of the category with id, or "Uncategorized".
func (d Data) CategoryName(id string) string {
	if c, ok := d.Category(id); ok {
		return c.Name
	}
	return "Uncategorized"
}

// Balance returns the balance of the account with id: its opening balance and
// every transaction in it.
func (d Data) Balance(id string) Money {
	a, _ := d.Account(id)
	b := a.Opening
	for _, t := range d.Transactions {
		if t.Account == id {
			b += t.Amount
		}
	}
	return b
}

// NetWorth returns the balances of all the accounts together.
func (d Data) NetWorth() Money {
	var n Money
	for _, a := range d.Accounts {
		n += d.Balance(a.ID)
	}
	return n
}

// Budgeted returns the amount budgeted for the category in the month.
func (d Data) Budgeted(m Month, category string) Money { return d.Budgets[m.Key()][category] }

// Spent returns what went out under the category in the month, less refunds, as a
// positive amount.
func (d Data) Spent(m Month, category string) Money {
	var s Money
	for _, t := range d.Transactions {
		if t.Category == category && m.Contains(t.Date) {
			s -= t.Amount
		}
	}
	return s
}

// Totals are the sums of a month.
type Totals struct {
	// Income is what came in under the income categories, and Spent what went out
	// under the others, less refunds. Transactions with no category, such as
	// transfers between accounts, count in neither; Uncategorized counts them.
	Income, Spent Money
	Uncategorized int
	// Budgeted is the sum of the month's budgets of the spending categories.
	Budgeted Money
}

// MonthTotals returns the sums of the month m.
func (d Data) MonthTotals(m Month) Totals {
	income := map[string]bool{}
	for _, c := range d.Categories {
		income[c.ID] = c.Income
	}
	var t Totals
	for _, tx := range d.Transactions {
		if !m.Contains(tx.Date) {
			continue
		}
		if tx.Category == Uncategorized {
			t.Uncategorized++
			continue
		}
		if income[tx.Category] {
			t.Income += tx.Amount
		} else {
			t.Spent -= tx.Amount
		}
	}
	for id, v := range d.Budgets[m.Key()] {
		if !income[id] {
			t.Budgeted += v
		}
	}
	return t
}

// IsTransfer reports whether t moves money between two of the accounts: it has no
// category, and another account has the opposite amount on the same day.
func (d Data) IsTransfer(t Transaction) bool {
	if t.Category != Uncategorized {
		return false
	}
	return slices.ContainsFunc(d.Transactions, func(o Transaction) bool {
		return o.Account != t.Account && o.Amount == -t.Amount && o.Date.Equal(t.Date)
	})
}

// NeedCategory returns the transactions of the month m that have no category and are
// not transfers, which the budget cannot count.
func (d Data) NeedCategory(m Month) []Transaction {
	var out []Transaction
	for _, t := range d.Transactions {
		if t.Category == Uncategorized && m.Contains(t.Date) && !d.IsTransfer(t) {
			out = append(out, t)
		}
	}
	return out
}

// Payees returns the payees of the transactions, most used first, for suggestions.
func (d Data) Payees() []string {
	n := map[string]int{}
	for _, t := range d.Transactions {
		if t.Payee != "" {
			n[t.Payee]++
		}
	}
	out := make([]string, 0, len(n))
	for p := range n {
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b string) int { return cmp.Or(cmp.Compare(n[b], n[a]), strings.Compare(a, b)) })
	return out
}

// LastCategory returns the category of the latest transaction to payee, so that a
// new one can start with it.
func (d Data) LastCategory(payee string) (string, bool) {
	var best Transaction
	found := false
	for _, t := range d.Transactions {
		if strings.EqualFold(t.Payee, payee) && (!found || t.Date.After(best.Date)) {
			best, found = t, true
		}
	}
	return best.Category, found
}
