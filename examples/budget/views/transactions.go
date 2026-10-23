package views

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/budget/ledger"
)

// The keys of the transaction table's columns.
const (
	colDate     = "date"
	colPayee    = "payee"
	colCategory = "category"
	colAccount  = "account"
	colAmount   = "amount"
	colMemo     = "memo"
	colCleared  = "cleared"
)

// isoDay is how a date picker's editor hands its day to the table: as text that the
// edit reads back.
const isoDay = "2006-01-02"

// shownTransactions returns the transactions the filters let through: of the month
// shown, unless the table shows every month, of the account and the category chosen,
// and matching the search.
func (a *App) shownTransactions(d ledger.Data) []ledger.Transaction {
	m, all := a.month.Get(), a.allTime.Get()
	q, acct, cat := a.txQuery.Get(), a.txAccount.Get(), a.txCategory.Get()
	needs := map[string]bool{}
	if cat == filterUncategorized {
		for _, t := range d.NeedCategory(m) {
			needs[t.ID] = true
		}
		if all {
			for _, t := range d.Transactions {
				if t.Category == ledger.Uncategorized && !d.IsTransfer(t) {
					needs[t.ID] = true
				}
			}
		}
	}
	var out []ledger.Transaction
	for _, t := range d.Transactions {
		switch {
		case !all && !m.Contains(t.Date):
		case acct != "" && t.Account != acct:
		case cat == filterUncategorized && !needs[t.ID]:
		case cat != "" && cat != filterUncategorized && t.Category != cat:
		case q != "" && !matches(q, t, d.CategoryName(t.Category)):
		default:
			out = append(out, t)
		}
	}
	return out
}

// transactionsPage is the table of the transactions, with its filters above it and
// its sums below it. Every cell but the cleared mark edits in place: Enter or F2 on a
// cell, or typing over the payee, memo or amount.
func (a *App) transactionsPage(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	hc := ctx.HighContrast()
	d := a.s.Data.Get()
	rows := a.shownTransactions(d)
	sel := a.txSel.Get()

	catOpts := []ui.Option[string]{{Value: ledger.Uncategorized, Label: "Uncategorized"}}
	for _, c := range d.Categories {
		catOpts = append(catOpts, ui.Option[string]{Value: c.ID, Label: c.Group + ": " + c.Name})
	}
	acctOpts := make([]ui.Option[string], 0, len(d.Accounts))
	for _, ac := range d.Accounts {
		acctOpts = append(acctOpts, ui.Option[string]{Value: ac.ID, Label: ac.Name})
	}
	accountName := func(id string) string {
		if ac, ok := d.Account(id); ok {
			return ac.Name
		}
		return ""
	}
	update := func(label string, id string, fn func(*ledger.Transaction)) {
		cur := a.s.Data.Peek()
		i := slices.IndexFunc(cur.Transactions, func(x ledger.Transaction) bool { return x.ID == id })
		if i < 0 {
			return
		}
		tx := cur.Transactions[i]
		fn(&tx)
		a.s.UpdateTransaction(label, tx)
	}
	choose := func(opts []ui.Option[string], label string) func(ledger.Transaction, string, func(string), func(), func()) ui.Element {
		return func(_ ledger.Transaction, value string, change func(string), commit, _ func()) ui.Element {
			return ui.Component(func(ctx *ui.Context) ui.Element {
				v := ui.Local(ctx, "v", value)
				return ui.Select(v, opts...).Small().Label(label).Width(ui.Full).
					OnChange(func(s string) { change(s); commit() })
			})
		}
	}

	grid := ui.DataGrid(rows, func(x ledger.Transaction) string { return x.ID },
		ui.DataColumn[ledger.Transaction]{Key: colDate, Header: "Date",
			Text:    func(x ledger.Transaction) string { return ui.FormatDate(x.Date) },
			Value:   func(x ledger.Transaction) string { return x.Date.Format(isoDay) },
			Compare: ui.SortBy(func(x ledger.Transaction) int64 { return x.Date.Unix() }),
			Editor: func(_ ledger.Transaction, value string, change func(string), commit, _ func()) ui.Element {
				return ui.Component(func(ctx *ui.Context) ui.Element {
					day, _ := time.ParseInLocation(isoDay, value, time.Local)
					v := ui.Local(ctx, "v", day)
					return ui.DatePicker(v).Small().Label("Date").Width(ui.Full).
						OnChange(func(t time.Time) { change(t.Format(isoDay)) })
				})
			},
			Edit: func(x ledger.Transaction, value string) error {
				day, err := time.ParseInLocation(isoDay, value, time.Local)
				if err != nil {
					return errText("Enter a date.")
				}
				update("Change date", x.ID, func(t *ledger.Transaction) { t.Date = day })
				return nil
			},
			Width: 110, Resizable: true},
		ui.DataColumn[ledger.Transaction]{Key: colPayee, Header: "Payee",
			Text:    func(x ledger.Transaction) string { return x.Payee },
			Compare: ui.SortByText(func(x ledger.Transaction) string { return x.Payee }),
			Edit: func(x ledger.Transaction, value string) error {
				if strings.TrimSpace(value) == "" {
					return errText("Enter a payee.")
				}
				update("Change payee", x.ID, func(t *ledger.Transaction) { t.Payee = strings.TrimSpace(value) })
				return nil
			},
			Width: 200, Grow: 1, Resizable: true},
		ui.DataColumn[ledger.Transaction]{Key: colCategory, Header: "Category",
			Text:  func(x ledger.Transaction) string { return d.CategoryName(x.Category) },
			Value: func(x ledger.Transaction) string { return x.Category },
			Cell: func(x ledger.Transaction) ui.Element {
				name := ui.Text(d.CategoryName(x.Category)).Truncate()
				if x.Category == ledger.Uncategorized && !d.IsTransfer(x) {
					return ui.Div().Row().Align(ui.Center).Gap(6).
						Child(ui.Icon(icons.TriangleAlert).Size(14).Color(t.Warning)).Child(name.TextColor(t.TextMuted))
				}
				if x.Category == ledger.Uncategorized {
					return ui.Text("Transfer").TextColor(t.TextMuted)
				}
				return name
			},
			Compare: ui.SortByText(func(x ledger.Transaction) string { return d.CategoryName(x.Category) }),
			Editor:  choose(catOpts, "Category"),
			Edit: func(x ledger.Transaction, value string) error {
				update("Change category", x.ID, func(t *ledger.Transaction) { t.Category = value })
				return nil
			},
			Width: 170, Resizable: true},
		ui.DataColumn[ledger.Transaction]{Key: colAccount, Header: "Account",
			Text:    func(x ledger.Transaction) string { return accountName(x.Account) },
			Value:   func(x ledger.Transaction) string { return x.Account },
			Compare: ui.SortByText(func(x ledger.Transaction) string { return accountName(x.Account) }),
			Editor:  choose(acctOpts, "Account"),
			Edit: func(x ledger.Transaction, value string) error {
				update("Change account", x.ID, func(t *ledger.Transaction) { t.Account = value })
				return nil
			},
			Width: 150, Resizable: true},
		ui.DataColumn[ledger.Transaction]{Key: colMemo, Header: "Memo",
			Text:    func(x ledger.Transaction) string { return x.Memo },
			Compare: ui.SortByText(func(x ledger.Transaction) string { return x.Memo }),
			Edit: func(x ledger.Transaction, value string) error {
				update("Change memo", x.ID, func(t *ledger.Transaction) { t.Memo = strings.TrimSpace(value) })
				return nil
			},
			Width: 160, Resizable: true},
		ui.DataColumn[ledger.Transaction]{Key: colAmount, Header: "Amount",
			Text:  func(x ledger.Transaction) string { return money(x.Amount) },
			Value: func(x ledger.Transaction) string { return ui.FormatNumber(x.Amount.Float(), 2) },
			Cell: func(x ledger.Transaction) ui.Element {
				return ui.Text(money(x.Amount)).TextColor(amountColor(t, hc, x.Amount))
			},
			Compare: ui.SortBy(func(x ledger.Transaction) ledger.Money { return x.Amount }),
			Edit: func(x ledger.Transaction, value string) error {
				v, ok := ui.ParseNumber(value)
				if !ok || ledger.FromFloat(v) == 0 {
					return errText("Enter an amount other than zero, such as " + ui.FormatNumber(-12.5, 2) + " for money out.")
				}
				update("Change amount", x.ID, func(t *ledger.Transaction) { t.Amount = ledger.FromFloat(v) })
				return nil
			},
			Width: 120, Align: ui.End, Resizable: true},
		ui.DataColumn[ledger.Transaction]{Key: colCleared, Header: "Cleared",
			Text: func(x ledger.Transaction) string {
				return map[bool]string{true: "Cleared", false: "Not cleared"}[x.Cleared]
			},
			Cell: func(x ledger.Transaction) ui.Element {
				ic, label := icons.Circle, "Mark cleared"
				if x.Cleared {
					ic, label = icons.CircleCheck, "Mark not cleared"
				}
				id := x.ID
				return ui.IconButton(ic).Ghost().Small().Label(label).AccessChecked(x.Cleared).
					OnClick(func() { a.toggleCleared(id) })
			},
			Interactive: true,
			Compare:     ui.SortBy(func(x ledger.Transaction) int { return map[bool]int{true: 1}[x.Cleared] }),
			Width:       80, Align: ui.Center},
	).Key("transactions").Label("Transactions").
		Selection(ui.SelectionMultiple, a.txSel).Sort(a.txSort).Handle(a.txGrid).
		Density(ui.DataGridCompact).Movable().
		OnActivate(func(x ledger.Transaction) { a.editTransaction(x.ID) }).
		ContextMenu(func(x ledger.Transaction, _ string) []ui.MenuItem { return a.transactionMenu(x) }).
		Empty(ui.Empty("No transactions").Icon(icons.Receipt).
			Description("Nothing matches the filters for this period. Add a transaction, import a CSV file, or change the filters."))

	return ui.Div().Column().Size(ui.Full, ui.Full).
		Child(pageHeader(t, "Transactions", periodName(a.month.Get(), a.allTime.Get()),
			ui.Button("Import CSV…").Key("import").Icon(icons.FileUp).Secondary().OnClick(func() { a.importing.Set(true) }),
			ui.Button("Add transaction").Key("add").Icon(icons.Plus).Primary().Tooltip("Add transaction (Ctrl+N)").
				OnClick(a.newTransaction))).
		Child(a.txToolbar(t, d, catOpts, sel)).
		Child(ui.Div().Grow(1).MinHeight(ui.Px(0)).PaddingX(24).KeyContext("Transactions").
			OnAction(DeleteSelected, a.askDelete).
			OnAction(EditSelected, func() {
				if s := a.txSel.Peek(); len(s) == 1 {
					a.editTransaction(s[0])
				}
			}).
			Child(grid)).
		Child(txSums(rows))
}

// txToolbar holds the search and the filters, or, while rows are selected, what can
// be done to all of them at once.
func (a *App) txToolbar(t *ui.Theme, d ledger.Data, catOpts []ui.Option[string], sel []string) ui.Element {
	bar := ui.Div().Row().Align(ui.Center).Gap(8).PaddingX(24).PaddingBottom(10).Wrap()
	if len(sel) > 1 {
		bulk := ui.NewSignal("\x00")
		opts := append([]ui.Option[string]{{Value: "\x00", Label: "Set category…"}}, catOpts...)
		return bar.Key("bulk").
			Child(ui.Text(strconv.Itoa(len(sel)) + " selected").FontWeight(ui.FontMedium)).
			Child(ui.Select(bulk, opts...).Key("bulk-category").Small().Label("Set the category of the selected transactions").
				Width(ui.Px(220)).OnChange(func(cat string) {
				if cat != "\x00" {
					a.setCategory(a.txSel.Peek(), cat)
				}
			})).
			Child(ui.Button("Delete…").Key("bulk-delete").Danger().Small().Icon(icons.Trash).OnClick(a.askDelete)).
			Child(ui.Button("Clear selection").Ghost().Small().OnClick(func() { a.txSel.Set(nil) }))
	}
	accts := []ui.Option[string]{{Value: "", Label: "All accounts"}}
	for _, ac := range d.Accounts {
		accts = append(accts, ui.Option[string]{Value: ac.ID, Label: ac.Name})
	}
	cats := append([]ui.Option[string]{{Value: "", Label: "All categories"},
		{Value: filterUncategorized, Label: "Needs a category"}}, catOpts[1:]...)
	return bar.Key("filters").
		Child(ui.TextInput(a.txQuery).Key("search").Small().Label("Search transactions").
			Placeholder("Search payee, memo, category or amount").Icon(icons.Search).Clearable().
			FocusHandle(a.search).Width(ui.Px(300))).
		Child(ui.Select(a.txAccount, accts...).Key("filter-account").Small().Label("Account").Width(ui.Px(180))).
		Child(ui.Select(a.txCategory, cats...).Key("filter-category").Small().Label("Category").Width(ui.Px(200))).
		Child(ui.ToggleGroup(a.allTime,
			ui.Option[bool]{Value: false, Label: "This month"},
			ui.Option[bool]{Value: true, Label: "All time"},
		).Key("period").Label("Period"))
}

// txSums is the foot of the table: how many rows it shows, and their money in, out
// and the difference.
func txSums(rows []ledger.Transaction) ui.Element {
	var in, out ledger.Money
	for _, t := range rows {
		if t.Amount > 0 {
			in += t.Amount
		} else {
			out -= t.Amount
		}
	}
	return ui.StatusBar().Key("sums").Label("Sums of the transactions shown").
		Start(ui.Text(strconv.Itoa(len(rows))+" transactions")).
		End(ui.Text("In "+money(in)), ui.Text("Out "+money(out)), ui.Text("Net "+money(in-out)))
}

// periodName names the period the transactions show.
func periodName(m ledger.Month, all bool) string {
	if all {
		return "All time"
	}
	return ui.FormatMonthYear(m.Start())
}

// transactionMenu is the menu of a row of the table.
func (a *App) transactionMenu(x ledger.Transaction) []ui.MenuItem {
	id := x.ID
	cleared := "Mark cleared"
	if x.Cleared {
		cleared = "Mark not cleared"
	}
	n := len(a.txSel.Peek())
	del := "Delete…"
	if n > 1 {
		del = "Delete " + strconv.Itoa(n) + " transactions…"
	}
	return []ui.MenuItem{
		{Label: "Edit…", Icon: icons.Pencil, OnClick: func() { a.editTransaction(id) }},
		{Label: "Duplicate", Icon: icons.Copy, OnClick: func() { a.duplicate(id) }},
		{Label: cleared, Icon: icons.CircleCheck, OnClick: func() { a.toggleCleared(id) }},
		ui.MenuSeparator(),
		{Label: del, Icon: icons.Trash, OnClick: a.askDelete},
	}
}

func (a *App) toggleCleared(id string) {
	d := a.s.Data.Peek()
	if i := slices.IndexFunc(d.Transactions, func(x ledger.Transaction) bool { return x.ID == id }); i >= 0 {
		tx := d.Transactions[i]
		tx.Cleared = !tx.Cleared
		label := "Mark cleared"
		if !tx.Cleared {
			label = "Mark not cleared"
		}
		a.s.UpdateTransaction(label, tx)
	}
}

func (a *App) duplicate(id string) {
	d := a.s.Data.Peek()
	if i := slices.IndexFunc(d.Transactions, func(x ledger.Transaction) bool { return x.ID == id }); i >= 0 {
		tx := d.Transactions[i]
		tx.Cleared = false
		nid := a.s.AddTransaction(tx)
		a.txSel.Set([]string{nid})
		a.txGrid.ScrollToRow(nid)
	}
}

// setCategory gives the transactions with the IDs the category cat, as one step.
func (a *App) setCategory(ids []string, cat string) {
	a.s.Apply("Set category of "+strconv.Itoa(len(ids))+" transactions", func(d *ledger.Data) {
		for i := range d.Transactions {
			if slices.Contains(ids, d.Transactions[i].ID) {
				d.Transactions[i].Category = cat
			}
		}
	})
}

// askDelete asks before it deletes the selected transactions.
func (a *App) askDelete() {
	if sel := a.txSel.Peek(); len(sel) > 0 {
		a.confirm.Set(slices.Clone(sel))
	}
}

// confirmDialog asks to delete the transactions in a.confirm. The deletion is a step
// that Undo takes back, and the toast that reports it offers the undo too.
func (a *App) confirmDialog(ctx *ui.Context) ui.Element {
	ids := a.confirm.Get()
	if len(ids) == 0 {
		return ui.Div()
	}
	title, what := "Delete this transaction?", "It"
	if len(ids) > 1 {
		title, what = "Delete "+strconv.Itoa(len(ids))+" transactions?", "They"
	}
	return ui.AlertDialog(title, what+" will be removed from the ledger and its sums. Undo can bring "+
		strings.ToLower(what[:1])+what[1:]+" back.").
		Intent(ui.AlertDialogDestructive).
		Confirm("Delete", func() {
			a.confirm.Set(nil)
			a.s.DeleteTransactions(ids)
			a.txSel.Set(nil)
			msg := "Deleted the transaction"
			if len(ids) > 1 {
				msg = "Deleted " + strconv.Itoa(len(ids)) + " transactions"
			}
			ctx.Toast(ui.ToastSuccess, msg).Action("Undo", a.s.Undo)
		}).
		Cancel("", func() { a.confirm.Set(nil) })
}

// errText is an error with a message the table shows under the cell.
type errText string

func (e errText) Error() string { return string(e) }
