package views

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/budget/ledger"
)

// txDraft is the transaction the form edits: a new one when id is empty. The amount
// is the raw value of its field, unsigned; income says which way the money goes.
type txDraft struct {
	id                       string
	income                   *ui.Signal[bool]
	amount                   *ui.Signal[string]
	date                     *ui.Signal[time.Time]
	payee, category, account *ui.Signal[string]
	memo                     *ui.Signal[string]
	cleared                  *ui.Signal[bool]
	form                     *ui.Form
}

// newTransaction opens the form for a new transaction: dated today in this month, or
// the first day of the month shown, from the first account.
func (a *App) newTransaction() {
	d := a.s.Data.Peek()
	day := a.month.Peek().Start()
	if today := a.opts.Now(); a.month.Peek().Contains(today) {
		day = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	}
	acct := ""
	if len(d.Accounts) > 0 {
		acct = d.Accounts[0].ID
	}
	a.openForm(&txDraft{income: ui.NewSignal(false), amount: ui.NewSignal(""), date: ui.NewSignal(day),
		payee: ui.NewSignal(""), category: ui.NewSignal(ledger.Uncategorized), account: ui.NewSignal(acct),
		memo: ui.NewSignal(""), cleared: ui.NewSignal(false)})
}

// editTransaction opens the form on the transaction with id.
func (a *App) editTransaction(id string) {
	d := a.s.Data.Peek()
	i := slices.IndexFunc(d.Transactions, func(x ledger.Transaction) bool { return x.ID == id })
	if i < 0 {
		return
	}
	t := d.Transactions[i]
	amount := t.Amount
	if amount < 0 {
		amount = -amount
	}
	a.openForm(&txDraft{id: id, income: ui.NewSignal(t.Amount > 0), amount: ui.NewSignal(amount.Raw()),
		date: ui.NewSignal(t.Date), payee: ui.NewSignal(t.Payee), category: ui.NewSignal(t.Category),
		account: ui.NewSignal(t.Account), memo: ui.NewSignal(t.Memo), cleared: ui.NewSignal(t.Cleared)})
}

// openForm opens the form on dr. Its submit checks every field and saves the
// transaction as one step for Undo.
func (a *App) openForm(dr *txDraft) {
	dr.form = ui.NewForm().OnSubmit(func() error {
		amount, ok := ledger.ParseRaw(dr.amount.Peek())
		if !ok || amount <= 0 {
			return errors.New("Enter the amount.")
		}
		if !dr.income.Peek() {
			amount = -amount
		}
		t := ledger.Transaction{ID: dr.id, Date: dr.date.Peek(), Payee: strings.TrimSpace(dr.payee.Peek()),
			Category: dr.category.Peek(), Account: dr.account.Peek(), Amount: amount,
			Memo: strings.TrimSpace(dr.memo.Peek()), Cleared: dr.cleared.Peek()}
		if dr.id == "" {
			id := a.s.AddTransaction(t)
			a.txSel.Set([]string{id})
			a.txGrid.ScrollToRow(id)
		} else {
			a.s.UpdateTransaction("Edit transaction", t)
		}
		// Show the month of the transaction, so a new one in another month is seen.
		if m := ledger.MonthOf(t.Date); m != a.month.Peek() && !a.allTime.Peek() {
			a.month.Set(m)
		}
		a.form.Set(nil)
		return nil
	})
	a.form.Set(dr)
}

// txFormDialog is the form of a transaction, in a modal: which way the money goes,
// the amount, the day, the payee (with the payees used before), the category, the
// account, a memo and whether the bank has cleared it.
func (a *App) txFormDialog(ctx *ui.Context) ui.Element {
	dr := a.form.Get()
	if dr == nil {
		return ui.Div()
	}
	d := a.s.Data.Peek()
	f := dr.form
	catOpts := []ui.Option[string]{{Value: ledger.Uncategorized, Label: "Uncategorized"}}
	for _, c := range d.Categories {
		catOpts = append(catOpts, ui.Option[string]{Value: c.ID, Label: c.Group + ": " + c.Name})
	}
	acctOpts := []ui.Option[string]{{Value: "", Label: "Choose an account"}}
	for _, ac := range d.Accounts {
		acctOpts = append(acctOpts, ui.Option[string]{Value: ac.ID, Label: ac.Name})
	}
	payees := d.Payees()
	suggest := func(q string) []ui.Option[string] {
		var out []ui.Option[string]
		q = strings.ToLower(q)
		for _, p := range payees {
			if strings.Contains(strings.ToLower(p), q) {
				out = append(out, ui.Option[string]{Value: p, Label: p})
			}
		}
		return out
	}
	title := "Add transaction"
	if dr.id != "" {
		title = "Edit transaction"
	}
	save := ui.Button("Save").Key("save").Primary().OnClick(f.Submit)
	body := ui.Div().Column().Gap(14).Width(ui.Full).
		Child(f.Summary()).
		Child(f.Field("direction", "Type", ui.ToggleGroup(dr.income,
			ui.Option[bool]{Value: false, Label: "Money out", Icon: icons.ArrowUpRight},
			ui.Option[bool]{Value: true, Label: "Money in", Icon: icons.ArrowDownLeft},
		).Key("direction"))).
		Child(ui.Div().Row().Gap(12).
			Child(f.Field("amount", "Amount", ui.TextInput(dr.amount).Key("amount").Format(moneyFormat).Width(ui.Full).
				Validate(ui.Required[string](), ui.Custom(func(s string) error {
					if m, ok := ledger.ParseRaw(s); !ok || m <= 0 {
						return errors.New("Enter an amount above zero.")
					}
					return nil
				}))).Required().Grow(1)).
			Child(f.Field("date", "Date", ui.DatePicker(dr.date).Key("date").Width(ui.Full).Validate(ui.Required[time.Time]())).
				Required().Grow(1))).
		Child(f.Field("payee", "Payee", ui.Combobox(dr.payee, suggest).Key("payee").Width(ui.Full).
			Placeholder("Who was paid, or who paid").Validate(ui.Required[string]()).
			OnChange(func(p string) {
				// A payee used before brings its category, while none is chosen.
				if dr.category.Peek() == ledger.Uncategorized {
					if c, ok := a.s.Data.Peek().LastCategory(p); ok {
						dr.category.Set(c)
						if cat, ok := a.s.Data.Peek().Category(c); ok {
							dr.income.Set(cat.Income)
						}
					}
				}
			})).Required()).
		Child(ui.Div().Row().Gap(12).
			Child(f.Field("category", "Category", ui.Select(dr.category, catOpts...).Key("category").Width(ui.Full)).
				Grow(1)).
			Child(f.Field("account", "Account", ui.Select(dr.account, acctOpts...).Key("account").Width(ui.Full).
				Validate(ui.Required[string]())).Required().Grow(1))).
		Child(f.Field("memo", "Memo", ui.TextArea(dr.memo).Key("memo").Width(ui.Full).MinRows(2).Placeholder("Optional"))).
		Child(ui.Checkbox("Cleared by the bank", dr.cleared).Key("cleared"))
	return ui.Modal(title, body).Key("tx-form").Width(ui.Px(480)).
		Footer(ui.Button("Cancel").Secondary().OnClick(func() { a.form.Set(nil) }), save).
		Default(save).
		OnClose(func() { a.form.Set(nil) })
}
