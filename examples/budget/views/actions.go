package views

import "github.com/tyler-builds/jamb/ui"

// The app's actions. Key bindings, the command palette and the menus run them, and
// App.Root handles them.
var (
	NewTransaction = ui.NewAction("budget.new", "Add transaction…")
	ImportCSV      = ui.NewAction("budget.import", "Import from CSV…")
	Undo           = ui.NewAction("budget.undo", "Undo")
	Redo           = ui.NewAction("budget.redo", "Redo")
	PrevMonth      = ui.NewAction("budget.prev-month", "Previous month")
	NextMonth      = ui.NewAction("budget.next-month", "Next month")
	ThisMonth      = ui.NewAction("budget.this-month", "Go to this month")
	ShowOverview   = ui.NewAction("budget.overview", "Show overview")
	ShowTxns       = ui.NewAction("budget.transactions", "Show transactions")
	ShowBudget     = ui.NewAction("budget.budget", "Show budget")
	ShowAccounts   = ui.NewAction("budget.accounts", "Show accounts")
	Find           = ui.NewAction("budget.find", "Find transactions")
	DeleteSelected = ui.NewAction("budget.delete", "Delete selected transactions…")
	EditSelected   = ui.NewAction("budget.edit", "Edit transaction…")
	ToggleTheme    = ui.NewAction("budget.theme", "Toggle light and dark theme")
	Palette        = ui.NewAction("budget.palette", "Command palette").Hide()
)

// Actions lists every action, for app.Window.Actions.
func Actions() []*ui.Action {
	return []*ui.Action{NewTransaction, ImportCSV, Undo, Redo, PrevMonth, NextMonth, ThisMonth, ShowOverview,
		ShowTxns, ShowBudget, ShowAccounts, Find, DeleteSelected, EditSelected, ToggleTheme, Palette}
}

// Bindings returns the window's key bindings. Ctrl+Z and Ctrl+Y undo the ledger's
// changes; a text field binds them in its own context, which outranks these, so in
// a field they undo its typing. Delete and F2 act on the transaction table only.
func Bindings() []ui.Binding {
	return []ui.Binding{
		ui.Bind("ctrl+n", NewTransaction),
		ui.Bind("ctrl+i", ImportCSV),
		ui.Bind("ctrl+z", Undo),
		ui.Bind("ctrl+y", Redo),
		ui.Bind("ctrl+shift+z", Redo),
		ui.Bind("alt+left", PrevMonth),
		ui.Bind("alt+right", NextMonth),
		ui.Bind("alt+home", ThisMonth),
		ui.Bind("ctrl+1", ShowOverview),
		ui.Bind("ctrl+2", ShowTxns),
		ui.Bind("ctrl+3", ShowBudget),
		ui.Bind("ctrl+4", ShowAccounts),
		ui.Bind("ctrl+f", Find),
		ui.Bind("ctrl+shift+l", ToggleTheme),
		ui.Bind("ctrl+shift+p", Palette),
		ui.Bind("ctrl+k", Palette),
		ui.BindIn("Transactions", "delete", DeleteSelected),
	}
}
