package views

import (
	"cmp"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/budget/ledger"
)

// budgetRow is a row of the budget: a category, or a group of them with their sums.
type budgetRow struct {
	key, name       string
	budgeted, spent ledger.Money
	category        string // "" for a group
	children        []budgetRow
	group           bool
}

func (r budgetRow) left() ledger.Money { return r.budgeted - r.spent }

// The keys of the budget's columns.
const (
	colBudgetName = "name"
	colBudgeted   = "budgeted"
	colSpent      = "spent"
	colLeft       = "left"
	colProgress   = "progress"
)

// budgetRows groups the spending categories of the month m by their group, in the
// order the groups first appear, with each group's sums.
func budgetRows(d ledger.Data, m ledger.Month) []budgetRow {
	var groups []budgetRow
	at := map[string]int{}
	for _, c := range d.Categories {
		if c.Income {
			continue
		}
		i, ok := at[c.Group]
		if !ok {
			i = len(groups)
			at[c.Group] = i
			groups = append(groups, budgetRow{key: "group:" + c.Group, name: c.Group, group: true})
		}
		r := budgetRow{key: c.ID, name: c.Name, category: c.ID, budgeted: d.Budgeted(m, c.ID), spent: d.Spent(m, c.ID)}
		g := &groups[i]
		g.children = append(g.children, r)
		g.budgeted += r.budgeted
		g.spent += r.spent
	}
	return groups
}

// budgetPage is the month's budget: what the month brought in, what is budgeted and
// what is left to budget, then a table of the categories by group, whose budgeted
// amounts edit in place. The groups keep their order as the table sorts.
func (a *App) budgetPage(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	hc := ctx.HighContrast()
	d := a.s.Data.Get()
	m := a.month.Get()
	tot := d.MonthTotals(m)
	rows := budgetRows(d, m)
	open := ui.Local(ctx, "open", groupKeys(rows))
	sort := ui.Local(ctx, "sort", ui.SortState{})
	prev := m.Add(-1)
	canCopy := len(d.Budgets[prev.Key()]) > 0

	value := func(f func(budgetRow) ledger.Money) func(budgetRow) string {
		return func(r budgetRow) string { return money(f(r)) }
	}
	grid := ui.DataGrid(rows, func(r budgetRow) string { return r.key },
		ui.DataColumn[budgetRow]{Key: colBudgetName, Header: "Category",
			Text: func(r budgetRow) string { return r.name },
			Cell: func(r budgetRow) ui.Element {
				if r.group {
					return ui.Text(r.name).FontWeight(ui.FontSemibold)
				}
				return ui.Text(r.name)
			},
			Compare: ui.SortByText(func(r budgetRow) string { return r.name }),
			Width:   220, Grow: 1, Resizable: true},
		ui.DataColumn[budgetRow]{Key: colBudgeted, Header: "Budgeted", Text: value(func(r budgetRow) ledger.Money { return r.budgeted }),
			Value:   func(r budgetRow) string { return ui.FormatNumber(r.budgeted.Float(), 2) },
			Compare: ui.SortBy(func(r budgetRow) ledger.Money { return r.budgeted }),
			Edit: func(r budgetRow, text string) error {
				if r.group {
					return errText("A group's budget is the sum of its categories'. Edit a category.")
				}
				v, ok := ui.ParseNumber(text)
				if text == "" {
					v, ok = 0, true
				}
				if !ok || v < 0 {
					return errText("Enter an amount of zero or more.")
				}
				a.s.SetBudget(m, r.category, ledger.FromFloat(v))
				return nil
			},
			Width: 130, Align: ui.End, Resizable: true},
		ui.DataColumn[budgetRow]{Key: colSpent, Header: "Spent", Text: value(func(r budgetRow) ledger.Money { return r.spent }),
			Compare: ui.SortBy(func(r budgetRow) ledger.Money { return r.spent }),
			Width:   120, Align: ui.End, Resizable: true},
		ui.DataColumn[budgetRow]{Key: colLeft, Header: "Left", Text: value(budgetRow.left),
			Cell: func(r budgetRow) ui.Element {
				c := t.Text
				if r.left() < 0 && !hc {
					c = t.Danger
				}
				return ui.Text(money(r.left())).TextColor(c)
			},
			Compare: ui.SortBy(budgetRow.left),
			Width:   120, Align: ui.End, Resizable: true},
		ui.DataColumn[budgetRow]{Key: colProgress, Header: "Used",
			Text: func(r budgetRow) string { return usedText(r) },
			Cell: func(r budgetRow) ui.Element { return usedBar(t, hc, r) },
			Compare: ui.SortBy(func(r budgetRow) float64 {
				if r.budgeted == 0 {
					return float64(cmp.Compare(r.spent, 0))
				}
				return float64(r.spent) / float64(r.budgeted)
			}),
			Width: 200, Resizable: true},
	).Key("budget").Label("Budget of " + ui.FormatMonthYear(m.Start())).
		Children(func(r budgetRow) []budgetRow { return r.children }).
		Expanded(open).SortChildrenOnly().Sort(sort).
		Empty(ui.Empty("No spending categories").Icon(icons.PiggyBank))

	toBudget := tot.Income - tot.Budgeted
	return ui.Div().Column().Size(ui.Full, ui.Full).
		Child(pageHeader(t, "Budget", ui.FormatMonthYear(m.Start()),
			ui.Button("Copy "+ui.FormatMonthYear(prev.Start())).Key("copy-budget").Icon(icons.Copy).Secondary().
				Disabled(!canCopy).Tooltip("Set this month's budget to last month's").
				OnClick(func() {
					a.s.CopyBudget(prev, m)
					a.toast(ui.ToastSuccess, "Copied the budget of "+ui.FormatMonthYear(prev.Start())).Action("Undo", a.s.Undo)
				}))).
		Child(ui.Div().Row().Gap(12).PaddingX(24).PaddingBottom(12).Key("budget-tiles").
			Child(tile(t, "Income", money(tot.Income), "came in this month")).
			Child(tile(t, "Budgeted", money(tot.Budgeted), "across the categories")).
			Child(tile(t, "Spent", money(tot.Spent), "of the budget")).
			Child(tile(t, leftLabel(toBudget), money(abs(toBudget)), "of the income"))).
		Child(ui.Div().Grow(1).MinHeight(ui.Px(0)).PaddingX(24).PaddingBottom(16).Child(grid))
}

func leftLabel(m ledger.Money) string {
	if m < 0 {
		return "Budgeted over income"
	}
	return "Left to budget"
}

func abs(m ledger.Money) ledger.Money {
	if m < 0 {
		return -m
	}
	return m
}

// usedText says how much of a budget is used, for assistive technology.
func usedText(r budgetRow) string {
	switch {
	case r.budgeted == 0 && r.spent > 0:
		return "No budget, " + money(r.spent) + " spent"
	case r.budgeted == 0:
		return "No budget"
	}
	return ui.FormatNumber(float64(r.spent)/float64(r.budgeted)*100, 0) + "% used"
}

// usedBar is how much of a budget is used, as a bar that turns the warning color past
// 90 percent and the danger color over the budget. Under high contrast it keeps the
// system's colors and the text says it.
func usedBar(t *ui.Theme, hc bool, r budgetRow) ui.Element {
	frac := 0.0
	if r.budgeted > 0 {
		frac = float64(r.spent) / float64(r.budgeted)
	} else if r.spent > 0 {
		frac = 1.01
	}
	bar := ui.ProgressBar(min(max(frac, 0), 1)).Label(usedText(r)).Width(ui.Px(110))
	switch {
	case hc:
	case frac > 1:
		bar.Color(t.Danger)
	case frac > 0.9:
		bar.Color(t.Warning)
	}
	return ui.Div().Row().Align(ui.Center).Gap(8).Size(ui.Full, ui.Full).
		Child(bar).Child(ui.Text(usedText(r)).FontSize(11.5).TextColor(t.TextMuted).Truncate())
}

func groupKeys(rows []budgetRow) []string {
	var keys []string
	for _, r := range rows {
		if r.group {
			keys = append(keys, r.key)
		}
	}
	return keys
}

// tile is a figure of a page's summary: its name, its value and a line under it.
func tile(t *ui.Theme, label, value, sub string) ui.Element {
	return ui.Div().Column().Gap(2).Grow(1).Basis(ui.Px(0)).MinWidth(ui.Px(0)).PaddingX(14).PaddingY(12).
		Radius(10).Background(t.Surface).Border(1, t.Border).
		Child(ui.Text(label).FontSize(12).TextColor(t.TextMuted)).
		Child(ui.Text(value).FontSize(20).FontWeight(ui.FontSemibold)).
		Child(ui.Text(sub).FontSize(11.5).TextColor(t.TextSubtle))
}
