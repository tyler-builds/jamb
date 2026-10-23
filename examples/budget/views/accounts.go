package views

import (
	"errors"
	"strings"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/budget/ledger"
)

// accountsPage lists the accounts with their balances, and the net worth, which is
// their sum. A card's menu edits or deletes its account.
func (a *App) accountsPage(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	hc := ctx.HighContrast()
	d := a.s.Data.Get()
	grid := ui.Div().Row().Wrap().Gap(16).PaddingX(24).Key("account-cards")
	for _, ac := range d.Accounts {
		grid.Child(a.accountCard(t, hc, d, ac))
	}
	return ui.Div().Column().Size(ui.Full, ui.Full).ScrollY().
		Child(pageHeader(t, "Accounts", "Net worth "+money(d.NetWorth()),
			ui.Button("Add account").Key("add-account").Icon(icons.Plus).Primary().
				OnClick(func() { a.accountDlg.Set(&ledger.Account{Kind: ledger.Checking}) }))).
		Child(grid)
}

func kindIcon(k ledger.Kind) *icons.Icon {
	switch k {
	case ledger.Savings:
		return icons.PiggyBank
	case ledger.Credit:
		return icons.CreditCard
	case ledger.Cash:
		return icons.Banknote
	}
	return icons.Landmark
}

// accountCard is one account: its name, kind, balance and how many transactions it
// has. Its button opens the account's transactions.
func (a *App) accountCard(t *ui.Theme, hc bool, d ledger.Data, ac ledger.Account) ui.Element {
	bal := d.Balance(ac.ID)
	n := 0
	for _, tx := range d.Transactions {
		if tx.Account == ac.ID {
			n++
		}
	}
	id := ac.ID
	edit := ac
	return ui.Div().Column().Gap(10).Width(ui.Px(280)).Padding(16).Radius(10).Key("account-"+id).
		Background(t.Surface).Border(1, t.Border).
		ContextMenu(func() []ui.MenuItem {
			return []ui.MenuItem{
				{Label: "Edit…", Icon: icons.Pencil, OnClick: func() { a.accountDlg.Set(&edit) }},
				{Label: "Delete", Icon: icons.Trash, OnClick: func() { a.deleteAccount(id) }},
			}
		}).
		Child(ui.Div().Row().Align(ui.Center).Gap(10).
			Child(ui.Div().Center().Size(ui.Px(32), ui.Px(32)).Radius(8).Background(t.SurfaceHover).
				Child(ui.Icon(kindIcon(ac.Kind)).Size(16).Color(t.TextMuted))).
			Child(ui.Div().Column().Grow(1).MinWidth(ui.Px(0)).
				Child(ui.Text(ac.Name).FontWeight(ui.FontMedium).Truncate()).
				Child(ui.Text(ac.Kind.Label()).FontSize(12).TextColor(t.TextMuted))).
			Child(ui.IconButton(icons.Pencil).Ghost().Small().Label("Edit " + ac.Name).
				OnClick(func() { a.accountDlg.Set(&edit) }))).
		Child(ui.Text(money(bal)).Key("balance-" + id).FontSize(22).FontWeight(ui.FontSemibold).
			TextColor(amountColor(t, hc, bal))).
		Child(ui.Div().Row().Align(ui.Center).
			Child(ui.Text(pluralize(n, "transaction")).FontSize(12).TextColor(t.TextMuted)).
			Child(ui.Spacer()).
			Child(ui.Button("Show").Ghost().Small().OnClick(func() {
				a.txAccount.Set(id)
				a.allTime.Set(true)
				a.page.Set(pageTransactions)
			})))
}

func (a *App) deleteAccount(id string) {
	if err := a.s.DeleteAccount(id); err != nil {
		a.toast(ui.ToastWarning, "The account was not deleted: "+err.Error())
		return
	}
	a.toast(ui.ToastSuccess, "Deleted the account").Action("Undo", a.s.Undo)
}

// accountDialog adds or edits the account in a.accountDlg: its name, its kind and its
// opening balance.
func (a *App) accountDialog(ctx *ui.Context) ui.Element {
	ac := a.accountDlg.Get()
	if ac == nil {
		return ui.Div()
	}
	return ui.Component(func(ctx *ui.Context) ui.Element {
		name := ui.Local(ctx, "name", ac.Name)
		kind := ui.Local(ctx, "kind", ac.Kind)
		opening := ui.Local(ctx, "opening", ac.Opening.Raw())
		form := ui.Local(ctx, "form", (*ui.Form)(nil))
		if form.Peek() == nil {
			form.Set(ui.NewForm().OnSubmit(func() error {
				open, ok := ledger.ParseRaw(opening.Peek())
				if !ok {
					return errors.New("Enter the opening balance.")
				}
				a.s.SaveAccount(ledger.Account{ID: ac.ID, Name: strings.TrimSpace(name.Peek()), Kind: kind.Peek(), Opening: open})
				a.accountDlg.Set(nil)
				return nil
			}))
		}
		f := form.Peek()
		kinds := make([]ui.Option[ledger.Kind], len(ledger.Kinds))
		for i, k := range ledger.Kinds {
			kinds[i] = ui.Option[ledger.Kind]{Value: k, Label: k.Label(), Icon: kindIcon(k)}
		}
		title := "Add account"
		if ac.ID != "" {
			title = "Edit account"
		}
		save := ui.Button("Save").Key("save-account").Primary().OnClick(f.Submit)
		return ui.Modal(title, ui.Div().Column().Gap(14).Width(ui.Px(380)).
			Child(f.Summary()).
			Child(f.Field("name", "Name", ui.TextInput(name).Key("account-name").Placeholder("Everyday checking").
				Validate(ui.Required[string](), ui.MaxLength(60))).Required()).
			Child(f.Field("kind", "Kind", ui.RadioGroup(kind, kinds...).Key("account-kind"))).
			Child(f.Field("opening", "Opening balance", ui.TextInput(opening).Key("account-opening").
				Format(ui.NumberFormat(ui.NumberOptions{Currency: true, Signed: true}))).
				Description("What the account held before its first transaction here. A card's debt is negative."))).
			Key("account-form").
			Footer(ui.Button("Cancel").Secondary().OnClick(func() { a.accountDlg.Set(nil) }), save).
			Default(save).
			OnClose(func() { a.accountDlg.Set(nil) })
	})
}

// pluralize writes n and the noun, with an s for any number but one.
func pluralize(n int, noun string) string {
	s := ui.FormatNumber(float64(n), 0) + " " + noun
	if n != 1 {
		s += "s"
	}
	return s
}
