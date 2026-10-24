package ledger

import (
	"math/rand"
	"slices"
	"time"
)

// Sample returns a made-up household's ledger for the six months up to now, so the
// app has something to show on its first run. The payees are made up. The same now
// gives the same ledger.
func Sample(now time.Time) Data {
	d := Data{Budgets: map[string]map[string]Money{}}
	add := func(a Account) string { a.ID = d.newID("a"); d.Accounts = append(d.Accounts, a); return a.ID }
	checking := add(Account{Name: "Everyday checking", Kind: Checking, Opening: 1850_00})
	savings := add(Account{Name: "Savings", Kind: Savings, Opening: 6200_00})
	card := add(Account{Name: "Credit card", Kind: Credit, Opening: -320_00})
	cat := func(name, group string, income bool) string {
		c := Category{ID: d.newID("c"), Name: name, Group: group, Income: income}
		d.Categories = append(d.Categories, c)
		return c.ID
	}
	salary := cat("Salary", "Income", true)
	otherIn := cat("Other income", "Income", true)
	rent := cat("Rent", "Bills", false)
	power := cat("Utilities", "Bills", false)
	phone := cat("Phone and internet", "Bills", false)
	insurance := cat("Insurance", "Bills", false)
	groceries := cat("Groceries", "Everyday", false)
	dining := cat("Dining out", "Everyday", false)
	transport := cat("Transport", "Everyday", false)
	shopping := cat("Shopping", "Everyday", false)
	fun := cat("Entertainment", "Everyday", false)
	emergency := cat("Emergency fund", "Savings goals", false)
	vacation := cat("Vacation", "Savings goals", false)

	budget := map[string]Money{
		rent: 1400_00, power: 140_00, phone: 95_00, insurance: 120_00, groceries: 520_00,
		dining: 180_00, transport: 160_00, shopping: 150_00, fun: 90_00, emergency: 300_00, vacation: 150_00,
	}
	rng := rand.New(rand.NewSource(int64(now.Year())*12 + int64(now.Month())))
	tx := func(day time.Time, payee, category, account string, amount Money, cleared bool) {
		d.Transactions = append(d.Transactions, Transaction{ID: d.newID("t"), Date: day, Payee: payee,
			Category: category, Account: account, Amount: amount, Cleared: cleared})
	}
	this := MonthOf(now)
	for back := 5; back >= 0; back-- {
		m := this.Add(-back)
		b := make(map[string]Money, len(budget))
		for c, v := range budget {
			b[c] = v
		}
		d.Budgets[m.Key()] = b
		day := func(n int) time.Time { return m.Start().AddDate(0, 0, n-1) }
		last := 28
		if m == this {
			last = max(1, now.Day())
		}
		// cleared is true for the days well before now, as a bank shows them.
		cleared := func(t time.Time) bool { return now.Sub(t) > 72*time.Hour }
		when := func(n int) (time.Time, bool) {
			t := day(n)
			return t, n <= last
		}
		if t, ok := when(1); ok {
			tx(t, "Maple Street Apartments", rent, checking, -1400_00, cleared(t))
			tx(t, "Acme Payroll", salary, checking, 2650_00, cleared(t))
		}
		if t, ok := when(15); ok {
			tx(t, "Acme Payroll", salary, checking, 2650_00, cleared(t))
			tx(t, "Savings transfer", Uncategorized, savings, 300_00, cleared(t))
			tx(t, "Savings transfer", emergency, checking, -300_00, cleared(t))
		}
		if t, ok := when(8); ok {
			tx(t, "City Power and Water", power, checking, -Money(110_00+rng.Intn(50_00)), cleared(t))
		}
		if t, ok := when(12); ok {
			tx(t, "Signal Mobile", phone, card, -95_00, cleared(t))
		}
		if t, ok := when(20); ok {
			tx(t, "Harbor Insurance", insurance, checking, -120_00, cleared(t))
		}
		for _, n := range []int{3, 7, 10, 14, 17, 21, 24, 27} {
			if t, ok := when(n); ok {
				tx(t, []string{"Green Basket Market", "Corner Grocer"}[rng.Intn(2)], groceries, card,
					-Money(35_00+rng.Intn(60_00)), cleared(t))
			}
		}
		for _, n := range []int{5, 11, 19, 26} {
			if t, ok := when(n); ok {
				tx(t, []string{"Corner Cafe", "Noodle House", "Bistro Nine"}[rng.Intn(3)], dining, card,
					-Money(18_00+rng.Intn(40_00)), cleared(t))
			}
		}
		for _, n := range []int{2, 9, 16, 23} {
			if t, ok := when(n); ok {
				tx(t, "Metro Transit", transport, card, -Money(25_00+rng.Intn(20_00)), cleared(t))
			}
		}
		if t, ok := when(13); ok {
			tx(t, "Northwind Outfitters", shopping, card, -Money(40_00+rng.Intn(150_00)), cleared(t))
		}
		if t, ok := when(18); ok {
			tx(t, "Lumen Cinema", fun, card, -Money(24_00+rng.Intn(30_00)), cleared(t))
		}
		if t, ok := when(22); ok && back%2 == 0 {
			tx(t, "Market refund", otherIn, checking, Money(15_00+rng.Intn(40_00)), cleared(t))
		}
		if t, ok := when(25); ok {
			tx(t, "Card payment", Uncategorized, card, 650_00, cleared(t))
			tx(t, "Card payment", Uncategorized, checking, -650_00, cleared(t))
		}
	}
	slices.SortStableFunc(d.Transactions, func(a, b Transaction) int { return a.Date.Compare(b.Date) })
	return d
}
