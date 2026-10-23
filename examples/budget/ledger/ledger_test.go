package ledger

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, time.October, 20, 10, 0, 0, 0, time.Local)

func TestSampleHasSixMonthsThatAddUp(t *testing.T) {
	d := Sample(now)
	m := MonthOf(now)
	tot := d.MonthTotals(m.Add(-1))
	if tot.Income != 5300_00 && tot.Income < 5300_00 {
		t.Errorf("last month's income = %v, want both paychecks", tot.Income)
	}
	if tot.Spent <= 0 || tot.Budgeted != 3305_00 {
		t.Errorf("last month: spent %v, budgeted %v", tot.Spent, tot.Budgeted)
	}
	// Transfers and card payments have no category; they count in neither.
	if tot.Uncategorized == 0 {
		t.Error("no transfer in last month")
	}
	for back := range 6 {
		if len(d.Budgets[m.Add(-back).Key()]) == 0 {
			t.Errorf("no budget %d months back", back)
		}
	}
	// This month has only the days up to now.
	for _, tx := range d.Transactions {
		if tx.Date.After(now) {
			t.Fatalf("a transaction after now: %+v", tx)
		}
	}
	if Sample(now).Transactions[5] != d.Transactions[5] {
		t.Error("the sample differs between two runs")
	}
}

func TestUndoAndRedo(t *testing.T) {
	s := New(Sample(now), "")
	before := len(s.Data.Peek().Transactions)
	id := s.AddTransaction(Transaction{Date: now, Payee: "Test", Account: "a1", Amount: -5_00})
	ids := []string{id, s.Data.Peek().Transactions[0].ID}
	s.DeleteTransactions(ids)
	if got := s.UndoLabel.Peek(); got != "Delete 2 transactions" {
		t.Errorf("undo label = %q", got)
	}
	if n := len(s.Data.Peek().Transactions); n != before-1 {
		t.Fatalf("%d transactions after add and delete of two, want %d", n, before-1)
	}
	s.Undo()
	if n := len(s.Data.Peek().Transactions); n != before+1 {
		t.Errorf("%d transactions after undo, want %d", n, before+1)
	}
	if got := s.RedoLabel.Peek(); got != "Delete 2 transactions" {
		t.Errorf("redo label = %q", got)
	}
	s.Undo()
	if n := len(s.Data.Peek().Transactions); n != before {
		t.Errorf("%d transactions after two undos, want %d", n, before)
	}
	s.Redo()
	s.Redo()
	if n := len(s.Data.Peek().Transactions); n != before-1 {
		t.Errorf("%d transactions after two redos, want %d", n, before-1)
	}
	// A new change clears what Redo could make again.
	s.Undo()
	s.SetBudget(MonthOf(now), "c3", 1500_00)
	if s.RedoLabel.Peek() != "" {
		t.Error("a change left a step to redo")
	}
	if got := s.Data.Peek().Budgeted(MonthOf(now), "c3"); got != 1500_00 {
		t.Errorf("budget = %v", got)
	}
}

func TestUndoKeepsTheDataItTookBack(t *testing.T) {
	s := New(Sample(now), "")
	first := s.Data.Peek().Transactions[0]
	edited := first
	edited.Payee = "Changed"
	s.UpdateTransaction("Edit payee", edited)
	s.Undo()
	if got := s.Data.Peek().Transactions[0].Payee; got != first.Payee {
		t.Errorf("payee after undo = %q, want %q: a change edited the data Undo keeps", got, first.Payee)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "budget.json")
	s := New(Sample(now), path)
	s.SetBudget(MonthOf(now), "c3", 1234_56)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	d, ok, err := Load(path)
	if err != nil || !ok {
		t.Fatalf("Load = %v, %v", ok, err)
	}
	if got := d.Budgeted(MonthOf(now), "c3"); got != 1234_56 {
		t.Errorf("loaded budget = %v", got)
	}
	if len(d.Transactions) != len(s.Data.Peek().Transactions) || !d.Transactions[3].Date.Equal(s.Data.Peek().Transactions[3].Date) {
		t.Error("the transactions did not round-trip")
	}
	if _, ok, err := Load(filepath.Join(t.TempDir(), "none.json")); ok || err != nil {
		t.Errorf("a missing file = %v, %v; want false, nil", ok, err)
	}
}

// parseISO and parseUS stand in for ui.ParseDate and ui.ParseNumber in en-US.
func parseISO(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", "1/2/2006"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseUS(s string) (float64, bool) {
	s = strings.ReplaceAll(strings.TrimPrefix(s, "$"), ",", "")
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func TestImportCSV(t *testing.T) {
	d := Sample(now)
	file := "Date,Description,Amount,Category\n" +
		"2026-10-21,Corner Cafe,-12.50,Dining out\n" +
		"10/22/2026,\"Acme, Inc.\",\"1,000.00\",salary\n" +
		"not a date,Somewhere,-1,\n" +
		"2026-10-23,Unknown Shop,-5,Hobbies\n" +
		"\n"
	header, rows, err := ReadCSV(strings.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	cols := GuessColumns(header)
	if cols.Date != 0 || cols.Payee != 1 || cols.Amount != 2 || cols.Category != 3 || cols.Outflow != None {
		t.Fatalf("columns = %+v", cols)
	}
	got := Convert(d, rows, cols, "a1", parseISO, parseUS)
	if len(got.Transactions) != 3 || len(got.Problems) != 1 || got.Problems[0].Row != 3 {
		t.Fatalf("imported %d, problems %+v", len(got.Transactions), got.Problems)
	}
	if tx := got.Transactions[1]; tx.Payee != "Acme, Inc." || tx.Amount != 1000_00 || d.CategoryName(tx.Category) != "Salary" {
		t.Errorf("second row = %+v", tx)
	}
	if got.Transactions[2].Category != Uncategorized {
		t.Error("an unknown category was not left uncategorized")
	}
	// The same file again adds nothing.
	s := New(d, "")
	s.Import(got.Transactions)
	again := Convert(s.Data.Peek(), rows, cols, "a1", parseISO, parseUS)
	if len(again.Transactions) != 0 || again.Duplicates != 3 {
		t.Errorf("a second import adds %d and skips %d, want 0 and 3", len(again.Transactions), again.Duplicates)
	}
}

func TestImportCSVWithSemicolonsAndTwoAmountColumns(t *testing.T) {
	file := "Posted;Payee;Debit amount;Credit amount\n2026-10-21;Grocer;12.50;\n2026-10-22;Employer;;900\n2026-10-23;Bank;-3;\n"
	header, rows, err := ReadCSV(strings.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	cols := GuessColumns(header)
	if cols.Outflow != 2 || cols.Inflow != 3 || cols.Amount != None {
		t.Fatalf("columns = %+v: a debit amount column is money out, not the amount", cols)
	}
	got := Convert(Data{}, rows, cols, "a1", parseISO, parseUS)
	want := []Money{-12_50, 900_00, -3_00}
	for i, tx := range got.Transactions {
		if tx.Amount != want[i] {
			t.Errorf("row %d amount = %v, want %v", i+1, tx.Amount, want[i])
		}
	}
}

func TestTransfersNeedNoCategory(t *testing.T) {
	d := Sample(now)
	if n := len(d.NeedCategory(MonthOf(now).Add(-1))); n != 0 {
		t.Errorf("%d transactions of last month need a category; the sample's only uncategorized ones are transfers", n)
	}
	s := New(d, "")
	s.AddTransaction(Transaction{Date: now, Payee: "Somewhere", Account: "a1", Amount: -9_00})
	if n := len(s.Data.Peek().NeedCategory(MonthOf(now))); n != 1 {
		t.Errorf("%d need a category after adding one without, want 1", n)
	}
}
