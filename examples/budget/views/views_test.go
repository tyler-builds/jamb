package views

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui/uitest"

	"github.com/tyler-builds/jamb/examples/budget/ledger"
)

var now = time.Date(2026, time.October, 20, 10, 0, 0, 0, time.Local)

// open renders the app over the sample ledger.
func open(t *testing.T) (*uitest.UI, *App, *ledger.Store) {
	t.Helper()
	s := ledger.New(ledger.Sample(now), "")
	a := New(s, Options{Now: func() time.Time { return now }})
	u := uitest.New(t, a.Root(), uitest.Size(1280, 820), uitest.Bindings(Bindings()...), uitest.Start(now))
	return u, a, s
}

func TestOverviewSumsUpTheMonth(t *testing.T) {
	u, _, s := open(t)
	tot := s.Data.Peek().MonthTotals(ledger.MonthOf(now))
	if got := u.Get("overview-tiles").Text(); !strings.Contains(got, money(tot.Income)) || !strings.Contains(got, money(tot.Spent)) {
		t.Errorf("tiles = %q, want the income %s and the spending %s", got, money(tot.Income), money(tot.Spent))
	}
	if u.Has("need-category") {
		t.Error("the sample's transfers are counted as needing a category")
	}
	if w := u.Warnings(); len(w) > 0 {
		t.Errorf("warnings: %q", w)
	}
}

func TestATransactionWithoutACategoryIsFlaggedAndReviewed(t *testing.T) {
	u, a, s := open(t)
	s.AddTransaction(ledger.Transaction{Date: now, Payee: "Hardware Store", Account: "a1", Amount: -42_00})
	u.Frame()
	u.Get("need-category")
	u.ByRole(access.RoleButton, "Review").Click()
	if a.page.Peek() != pageTransactions || a.txCategory.Peek() != filterUncategorized {
		t.Fatalf("Review shows %s filtered by %q", a.page.Peek(), a.txCategory.Peek())
	}
	if n := u.ByRole(access.RoleDataGrid, "Transactions").Node().GridRows; n != 1 {
		t.Errorf("%d rows need a category, want the one added", n)
	}
}

func TestAddTransactionChecksTheFormAndUndoes(t *testing.T) {
	u, a, s := open(t)
	before := len(s.Data.Peek().Transactions)
	u.Press("ctrl+n")
	if !u.Has("tx-form") {
		t.Fatal("Ctrl+N did not open the form")
	}
	u.Get("save").Click()
	if n := len(s.Data.Peek().Transactions); n != before {
		t.Fatal("an empty form saved a transaction")
	}
	u.ByName("2 fields need attention.")
	u.Get("amount").Type("23.4")
	u.ByRole(access.RoleComboBox, "Payee").Type("Corner Cafe")
	u.Get("save").Click()
	if u.Has("tx-form") {
		t.Fatal("the form stayed open after a valid save")
	}
	txs := s.Data.Peek().Transactions
	if len(txs) != before+1 {
		t.Fatalf("%d transactions, want one more", len(txs))
	}
	added := txs[len(txs)-1]
	// The payee was used before, so its category came with it, and money went out.
	if added.Amount != -23_40 || s.Data.Peek().CategoryName(added.Category) != "Dining out" {
		t.Errorf("added %+v", added)
	}
	if sel := a.txSel.Peek(); len(sel) != 1 || sel[0] != added.ID {
		t.Errorf("selection = %v, want the new transaction", sel)
	}
	u.Press("ctrl+z")
	if n := len(s.Data.Peek().Transactions); n != before {
		t.Errorf("%d transactions after Ctrl+Z, want %d", n, before)
	}
	u.Press("ctrl+y")
	if n := len(s.Data.Peek().Transactions); n != before+1 {
		t.Errorf("%d transactions after Ctrl+Y, want %d", n, before+1)
	}
}

func TestEditAnAmountInTheTable(t *testing.T) {
	u, a, s := open(t)
	a.page.Set(pageTransactions)
	u.Frame()
	tx := s.Data.Peek().Transactions[len(s.Data.Peek().Transactions)-1]
	a.txGrid.FocusCell(tx.ID, colAmount)
	u.Frame()
	u.Press("f2")
	u.Press("ctrl+a")
	u.Type("-77.25")
	u.Press("enter")
	got := s.Data.Peek().Transactions[len(s.Data.Peek().Transactions)-1]
	if got.ID != tx.ID || got.Amount != -77_25 {
		t.Errorf("after the edit: %+v, want an amount of -77.25", got)
	}
	if s.UndoLabel.Peek() != "Change amount" {
		t.Errorf("undo label = %q", s.UndoLabel.Peek())
	}
}

func TestDeleteAsksFirstAndOffersUndo(t *testing.T) {
	u, a, s := open(t)
	a.page.Set(pageTransactions)
	u.Frame()
	txs := s.Data.Peek().Transactions
	ids := []string{txs[len(txs)-1].ID, txs[len(txs)-2].ID}
	a.txSel.Set(ids)
	u.Frame()
	if !u.Has("bulk") {
		t.Fatal("selecting two rows did not show what can be done to them")
	}
	u.Get("bulk-delete").Click()
	u.ByName("Delete 2 transactions?")
	u.ByRole(access.RoleButton, "Cancel").Click()
	if len(s.Data.Peek().Transactions) != len(txs) {
		t.Fatal("Cancel deleted")
	}
	a.askDelete()
	u.Frame()
	u.ByRole(access.RoleButton, "Delete").Click()
	if n := len(s.Data.Peek().Transactions); n != len(txs)-2 {
		t.Fatalf("%d transactions after the delete, want %d", n, len(txs)-2)
	}
	u.ByRole(access.RoleButton, "Undo").Click()
	if n := len(s.Data.Peek().Transactions); n != len(txs) {
		t.Errorf("%d transactions after the toast's Undo, want %d", n, len(txs))
	}
}

func TestBudgetEditsInPlaceAndKeepsItsGroups(t *testing.T) {
	u, a, s := open(t)
	a.page.Set(pageBudget)
	u.Frame()
	m := ledger.MonthOf(now)
	var groceries string
	for _, c := range s.Data.Peek().Categories {
		if c.Name == "Groceries" {
			groceries = c.ID
		}
	}
	grid := u.ByRole(access.RoleDataGrid, "Budget of "+u.Get("month-name").Text())
	if n := grid.Node().GridRows; n != 14 {
		t.Errorf("%d rows, want three groups and their eleven categories", n)
	}
	// Edit the groceries' budget in place: the first cell that shows 520.00 is its
	// Budgeted cell.
	u.ByRole(access.RoleDataItem, money(520_00)).Click()
	u.Press("f2")
	u.Press("ctrl+a")
	u.Type("600")
	u.Press("enter")
	if got := s.Data.Peek().Budgeted(m, groceries); got != 600_00 {
		t.Errorf("groceries budget = %v, want 600.00", got)
	}
}

func TestImportACSVFile(t *testing.T) {
	u, a, s := open(t)
	before := len(s.Data.Peek().Transactions)
	u.Press("ctrl+i")
	file := "Date,Description,Amount\n2026-10-21,Corner Cafe,-12.50\n2026-10-22,Bookshop,-30\nnot a date,Oops,-1\n"
	a.loadCSV("bank.csv", strings.NewReader(file))
	u.Frame()
	if got := u.Get("import-summary").Text(); !strings.Contains(got, "2 transactions to import from bank.csv") {
		t.Errorf("summary = %q", got)
	}
	u.Get("import-problems")
	u.Get("do-import").Click()
	if n := len(s.Data.Peek().Transactions); n != before+2 {
		t.Fatalf("%d transactions after the import, want %d", n, before+2)
	}
	if a.page.Peek() != pageTransactions || u.Has("import-dialog") {
		t.Error("the import did not close and show the transactions")
	}
	// The same file again finds nothing new.
	u.Press("ctrl+i")
	a.loadCSV("bank.csv", strings.NewReader(file))
	u.Frame()
	if got := u.Get("import-summary").Text(); !strings.Contains(got, "0 transactions to import") || !strings.Contains(got, "2 rows already") {
		t.Errorf("second import summary = %q", got)
	}
	if !u.Get("do-import").Disabled() {
		t.Error("an import of nothing can be pressed")
	}
}

func TestMonthSwitcherMovesThePages(t *testing.T) {
	u, a, _ := open(t)
	first := u.Get("month-name").Text()
	u.Press("alt+left")
	if a.month.Peek() != ledger.MonthOf(now).Add(-1) || u.Get("month-name").Text() == first {
		t.Fatalf("Alt+Left shows %v", a.month.Peek())
	}
	u.Get("this-month").Click()
	if a.month.Peek() != ledger.MonthOf(now) || u.Has("this-month") {
		t.Error("This month did not come back")
	}
}

func TestAccountsShowBalancesAndRefuseToDeleteAUsedOne(t *testing.T) {
	u, a, s := open(t)
	a.page.Set(pageAccounts)
	u.Frame()
	d := s.Data.Peek()
	if got := u.Get("balance-a1").Text(); got != money(d.Balance("a1")) {
		t.Errorf("checking balance = %q, want %q", got, money(d.Balance("a1")))
	}
	a.deleteAccount("a1")
	u.Frame()
	if !slices.ContainsFunc(s.Data.Peek().Accounts, func(x ledger.Account) bool { return x.ID == "a1" }) {
		t.Error("an account with transactions was deleted")
	}
	u.Get("add-account").Click()
	u.Get("account-name").Type("Holiday fund")
	u.Get("save-account").Click()
	if n := len(s.Data.Peek().Accounts); n != len(d.Accounts)+1 {
		t.Errorf("%d accounts, want one more", n)
	}
}

func TestEveryPageAndDialogBuildsWithoutWarnings(t *testing.T) {
	u, a, _ := open(t)
	for _, p := range []string{pageOverview, pageTransactions, pageBudget, pageAccounts} {
		a.page.Set(p)
		u.Frame()
	}
	a.newTransaction()
	u.Frame()
	a.form.Set(nil)
	a.accountDlg.Set(&ledger.Account{Kind: ledger.Savings})
	u.Frame()
	a.accountDlg.Set(nil)
	a.importing.Set(true)
	u.Frame()
	a.loadCSV("bank.csv", strings.NewReader("Date,Payee,Amount\n2026-10-21,Cafe,-3\n"))
	u.Frame()
	a.closeImport()
	u.Frame()
	if w := u.Warnings(); len(w) > 0 {
		t.Errorf("warnings: %q", w)
	}
}
