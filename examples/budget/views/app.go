// Package views is the budget app's UI, built only from public jamb APIs.
package views

import (
	"time"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/budget/ledger"
)

// The pages of the app.
const (
	pageOverview     = "overview"
	pageTransactions = "transactions"
	pageBudget       = "budget"
	pageAccounts     = "accounts"
)

// Options are what the window gives the app.
type Options struct {
	// Now is the clock; nil is time.Now.
	Now func() time.Time
	// Theme is "light", "dark" or "" to follow Windows, and OnTheme runs when the
	// user picks one.
	Theme   string
	OnTheme func(theme string)
}

// App is the window's UI state.
type App struct {
	s    *ledger.Store
	opts Options
	page *ui.Signal[string]
	// month is the month the overview, the budget and the transactions show.
	month *ui.Signal[ledger.Month]

	// The transaction table: its selection, its sort and its filters.
	txSel      *ui.Signal[[]string]
	txSort     *ui.Signal[ui.SortState]
	txQuery    *ui.Signal[string]
	txAccount  *ui.Signal[string] // an account's ID, or "" for every account
	txCategory *ui.Signal[string] // a category's ID, filterUncategorized, or "" for every one
	allTime    *ui.Signal[bool]   // the table shows every month, not the month alone
	txGrid     *ui.DataGridHandle
	search     *ui.FocusHandle

	// The dialogs, each open while its signal holds something.
	form       *ui.Signal[*txDraft]
	confirm    *ui.Signal[[]string] // the transactions the user asked to delete
	accountDlg *ui.Signal[*ledger.Account]
	importing  *ui.Signal[bool]
	importSt   *importState // the file being imported, while the import dialog is open
	palette    *ui.Signal[bool]
	theme      string
	// root is the context of the window's root, for the toasts of handlers.
	root *ui.Context
}

// filterUncategorized is the category filter of the transactions without a category.
const filterUncategorized = "-"

// New returns the UI state of a window over the store s.
func New(s *ledger.Store, opts Options) *App {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &App{s: s, opts: opts, page: ui.NewSignal(pageOverview),
		month: ui.NewSignal(ledger.MonthOf(opts.Now())),
		txSel: ui.NewSignal([]string(nil)), txSort: ui.NewSignal(ui.SortState{Key: colDate, Desc: true}),
		txQuery: ui.NewSignal(""), txAccount: ui.NewSignal(""), txCategory: ui.NewSignal(""),
		allTime: ui.NewSignal(false), txGrid: ui.NewDataGridHandle(), search: ui.NewFocusHandle(),
		form: ui.NewSignal[*txDraft](nil), confirm: ui.NewSignal([]string(nil)),
		accountDlg: ui.NewSignal[*ledger.Account](nil), importing: ui.NewSignal(false),
		palette: ui.NewSignal(false), theme: opts.Theme}
}

// Root returns the window's render function.
func (a *App) Root() func(*ui.Context) ui.Element {
	return func(ctx *ui.Context) ui.Element {
		a.root = ctx
		t := ctx.Theme()
		return ui.Div().Column().Size(ui.Full, ui.Full).Background(t.Background).TextColor(t.Text).FontSize(t.FontSize).
			OnAction(NewTransaction, a.newTransaction).
			OnAction(ImportCSV, func() { a.importing.Set(true) }).
			OnAction(Undo, a.s.Undo).
			OnAction(Redo, a.s.Redo).
			OnAction(PrevMonth, func() { a.month.Set(a.month.Peek().Add(-1)) }).
			OnAction(NextMonth, func() { a.month.Set(a.month.Peek().Add(1)) }).
			OnAction(ThisMonth, func() { a.month.Set(ledger.MonthOf(a.opts.Now())) }).
			OnAction(ShowOverview, func() { a.page.Set(pageOverview) }).
			OnAction(ShowTxns, func() { a.page.Set(pageTransactions) }).
			OnAction(ShowBudget, func() { a.page.Set(pageBudget) }).
			OnAction(ShowAccounts, func() { a.page.Set(pageAccounts) }).
			OnAction(Find, func() { a.page.Set(pageTransactions); a.search.Focus() }).
			OnAction(ToggleTheme, func() { a.toggleTheme(ctx) }).
			OnAction(Palette, func() { a.palette.Set(true) }).
			Child(ui.Component(a.titleBar)).
			Child(ui.Div().Row().Grow(1).MinHeight(ui.Px(0)).
				Child(ui.Component(a.sidebar)).
				Child(ui.Div().Column().Grow(1).MinWidth(ui.Px(0)).Child(ui.Component(a.pageView)))).
			Child(ui.Component(a.txFormDialog)).
			Child(ui.Component(a.confirmDialog)).
			Child(ui.Component(a.accountDialog)).
			Child(ui.Component(a.importDialog)).
			Child(ui.CommandPalette(a.palette).Placeholder("Type a command…")).
			Child(ui.Toaster().Key("toaster"))
	}
}

func (a *App) pageView(ctx *ui.Context) ui.Element {
	switch a.page.Get() {
	case pageTransactions:
		return a.transactionsPage(ctx)
	case pageBudget:
		return a.budgetPage(ctx)
	case pageAccounts:
		return a.accountsPage(ctx)
	}
	return a.overviewPage(ctx)
}

// sidebar is the navigation, with the count of the month's transactions that need a
// category on Transactions.
func (a *App) sidebar(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	d := a.s.Data.Get()
	uncat := len(d.NeedCategory(a.month.Get()))
	return ui.Div().Column().Width(ui.Px(220)).Shrink(0).Background(t.Panel).
		BorderEdges(ui.Edges{Right: 1}, t.Border).
		Child(ui.Sidebar(a.page,
			ui.SidebarHeading("Budget"),
			ui.SidebarItem{Key: pageOverview, Label: "Overview", Icon: icons.LayoutDashboard},
			ui.SidebarItem{Key: pageTransactions, Label: "Transactions", Icon: icons.Receipt, Badge: uncat},
			ui.SidebarItem{Key: pageBudget, Label: "Budget", Icon: icons.PiggyBank},
			ui.SidebarItem{Key: pageAccounts, Label: "Accounts", Icon: icons.Landmark},
		).Key("nav").Label("Pages")).
		Child(ui.Spacer()).
		Child(a.saveState(t))
}

// saveState says when the ledger could not be saved, at the foot of the sidebar.
func (a *App) saveState(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		err := a.s.SaveErr.Get()
		if err == "" {
			return ui.Div()
		}
		return ui.Div().Padding(12).Child(ui.Alert("Not saved").Danger().Description(err))
	})
}

// toast shows a toast in the window.
func (a *App) toast(v ui.ToastVariant, text string) *ui.Toast { return a.root.Toast(v, text) }

func (a *App) toggleTheme(ctx *ui.Context) {
	if ctx.Theme().Dark {
		a.theme = "light"
		ctx.SetTheme(ui.LightTheme())
	} else {
		a.theme = "dark"
		ctx.SetTheme(ui.DarkTheme())
	}
	if a.opts.OnTheme != nil {
		a.opts.OnTheme(a.theme)
	}
}

// pageHeader is a page's title, a line under it, and its actions at the end.
func pageHeader(t *ui.Theme, title, sub string, actions ...ui.Element) ui.Element {
	return ui.Div().Row().Align(ui.Center).Gap(12).PaddingX(24).PaddingTop(20).PaddingBottom(12).
		Child(ui.Div().Column().Gap(2).Grow(1).MinWidth(ui.Px(0)).
			Child(ui.Text(title).FontSize(22).FontWeight(ui.FontSemibold)).
			Child(ui.Text(sub).FontSize(12.5).TextColor(t.TextMuted))).
		Children(actions...)
}
