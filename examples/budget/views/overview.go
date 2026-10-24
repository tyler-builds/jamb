package views

import (
	"cmp"
	"slices"
	"strconv"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/charts"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/budget/ledger"
)

// share is a category's spending, a slice of the donut.
type share struct {
	name  string
	spent float64
}

// monthBars are a month's spending and budget, the bars of the history chart.
type monthBars struct {
	name            string
	spent, budgeted float64
}

// overviewPage sums up the month: its figures, the transactions that need a
// category, where the money went, the last six months against their budgets, the
// categories over budget, and the latest transactions.
func (a *App) overviewPage(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	hc := ctx.HighContrast()
	d := a.s.Data.Get()
	m := a.month.Get()
	tot := d.MonthTotals(m)

	var shares []share
	var over []budgetRow
	for _, g := range budgetRows(d, m) {
		for _, c := range g.children {
			if c.spent > 0 {
				shares = append(shares, share{c.name, c.spent.Float()})
			}
			if c.spent > c.budgeted {
				over = append(over, c)
			}
		}
	}
	slices.SortFunc(shares, func(x, y share) int { return cmp.Compare(y.spent, x.spent) })
	slices.SortFunc(over, func(x, y budgetRow) int { return cmp.Compare(x.left(), y.left()) })
	var hist []monthBars
	for back := 5; back >= 0; back-- {
		mm := m.Add(-back)
		mt := d.MonthTotals(mm)
		hist = append(hist, monthBars{ui.FormatMonthYear(mm.Start()), mt.Spent.Float(), mt.Budgeted.Float()})
	}
	cur := func(v float64) string { return ui.FormatCurrency(v) }

	page := ui.Div().Column().Size(ui.Full, ui.Full).ScrollY().Key("overview").
		Child(pageHeader(t, "Overview", ui.FormatMonthYear(m.Start()),
			ui.Button("Add transaction").Icon(icons.Plus).Primary().OnClick(a.newTransaction)))
	body := ui.Div().Column().Gap(16).PaddingX(24).PaddingBottom(24)
	if need := d.NeedCategory(m); len(need) > 0 {
		body.Child(ui.Alert(pluralize(len(need), "transaction") + " need a category").Key("need-category").
			Description("The budget counts a transaction only once it has a category.").
			Action(ui.Button("Review").Small().OnClick(func() {
				a.txCategory.Set(filterUncategorized)
				a.allTime.Set(false)
				a.page.Set(pageTransactions)
			})))
	}
	body.Child(ui.Div().Row().Gap(12).Key("overview-tiles").
		Child(tile(t, "Income", money(tot.Income), "came in")).
		Child(tile(t, "Spent", money(tot.Spent), "of "+money(tot.Budgeted)+" budgeted")).
		Child(tile(t, "Left in the budget", money(tot.Budgeted-tot.Spent), "to spend this month")).
		Child(tile(t, "Net worth", money(d.NetWorth()), "across "+pluralize(len(d.Accounts), "account"))))

	var donut ui.Element = ui.Empty("No spending this month").Icon(icons.Wallet)
	if len(shares) > 0 {
		donut = charts.Donut(charts.Series[share]{Name: "Spending", Data: shares,
			Category: func(s share) string { return s.name }, Y: func(s share) float64 { return s.spent }}).
			Key("spending-donut").Title("Where the money went").Format(cur).Legend(charts.LegendRight).
			Height(ui.Px(260))
	}
	bars := charts.Bar(
		charts.Series[monthBars]{Name: "Spent", Data: hist, Category: func(b monthBars) string { return b.name },
			Y: func(b monthBars) float64 { return b.spent }},
		charts.Series[monthBars]{Name: "Budgeted", Data: hist, Category: func(b monthBars) string { return b.name },
			Y: func(b monthBars) float64 { return b.budgeted }},
	).Key("history").Title("The last six months").Format(cur).YAxis(charts.Axis{Format: cur}.Min(0)).Height(ui.Px(260))
	body.Child(ui.Div().Row().Gap(16).Wrap().
		Child(card(t).Key("spending-card").Grow(1).Basis(ui.Px(360)).Child(donut)).
		Child(card(t).Key("history-card").Grow(1).Basis(ui.Px(360)).Child(bars)))

	overList := ui.Div().Column().Gap(10)
	if len(over) == 0 {
		overList.Child(ui.Text("Every category is within its budget.").TextColor(t.TextMuted))
	}
	for _, c := range over {
		overList.Child(ui.Div().Column().Gap(4).
			Child(ui.Div().Row().
				Child(ui.Text(c.name).FontWeight(ui.FontMedium)).Child(ui.Spacer()).
				Child(ui.Text(money(-c.left()) + " over").TextColor(dangerOr(t, hc)))).
			Child(usedBar(t, hc, c)))
	}
	recent := slices.Clone(d.Transactions)
	slices.SortFunc(recent, func(x, y ledger.Transaction) int { return y.Date.Compare(x.Date) })
	recentList := ui.Div().Column().Key("recent")
	for _, tx := range recent[:min(len(recent), 6)] {
		id := tx.ID
		recentList.Child(ui.Div().Row().Align(ui.Center).Gap(10).PaddingY(6).Radius(6).PaddingX(6).
			Hover(ui.Style{Background: t.SurfaceHover}).Cursor(ui.CursorPointer).
			Role(access.RoleButton).Label(tx.Payee + ", " + money(tx.Amount)).Focusable().
			OnClick(func() { a.editTransaction(id) }).
			Child(ui.Div().Column().Grow(1).MinWidth(ui.Px(0)).
				Child(ui.Text(tx.Payee).Truncate()).
				Child(ui.Text(ui.FormatDate(tx.Date) + " · " + d.CategoryName(tx.Category)).FontSize(12).TextColor(t.TextMuted))).
			Child(ui.Text(money(tx.Amount)).TextColor(amountColor(t, hc, tx.Amount))))
	}
	body.Child(ui.Div().Row().Gap(16).Wrap().
		Child(card(t).Grow(1).Basis(ui.Px(360)).Gap(12).Key("over-budget").
			Child(ui.Text("Over budget").FontSize(15).FontWeight(ui.FontSemibold)).Child(overList)).
		Child(card(t).Grow(1).Basis(ui.Px(360)).Gap(8).
			Child(ui.Div().Row().Align(ui.Center).
				Child(ui.Text("Latest transactions").FontSize(15).FontWeight(ui.FontSemibold)).Child(ui.Spacer()).
				Child(ui.Button("All " + strconv.Itoa(len(d.Transactions))).Ghost().Small().OnClick(func() {
					a.allTime.Set(true)
					a.page.Set(pageTransactions)
				}))).
			Child(recentList)))
	return page.Child(body)
}

// card is a panel of the overview.
func card(t *ui.Theme) *ui.DivElement {
	return ui.Div().Column().Padding(16).Radius(10).Background(t.Surface).Border(1, t.Border).MinWidth(ui.Px(0))
}

func dangerOr(t *ui.Theme, hc bool) ui.Color {
	if hc {
		return t.Text
	}
	return t.Danger
}
