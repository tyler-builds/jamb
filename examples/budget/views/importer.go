package views

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/budget/ledger"
)

// importState is the CSV file being imported: its name, its header and rows, the
// columns chosen for the fields, the account it goes into, and what failed.
type importState struct {
	name    *ui.Signal[string]
	header  *ui.Signal[[]string]
	rows    *ui.Signal[[][]string]
	cols    *ui.Signal[ledger.Columns]
	account *ui.Signal[string]
	err     *ui.Signal[string]
}

func newImportState() *importState {
	return &importState{name: ui.NewSignal(""), header: ui.NewSignal([]string(nil)), rows: ui.NewSignal([][]string(nil)),
		cols: ui.NewSignal(ledger.Columns{}), account: ui.NewSignal(""), err: ui.NewSignal("")}
}

// imp returns the state of the import, made on first use.
func (a *App) imp() *importState {
	if a.importSt == nil {
		a.importSt = newImportState()
	}
	return a.importSt
}

// chooseFile asks for a CSV file with the system's dialog, and reads it.
func (a *App) chooseFile(ctx *ui.Context) {
	ctx.Window().OpenFiles(dialog.FileOptions{Title: "Import transactions",
		Filters: []dialog.Filter{{Name: "CSV files", Patterns: []string{"*.csv"}}, {Name: "All files", Patterns: []string{"*.*"}}},
	}, func(res dialog.FileResult) {
		if res.Canceled || len(res.Paths) == 0 {
			return
		}
		if res.Err != nil {
			a.imp().err.Set(res.Err.Error())
			return
		}
		f, err := os.Open(res.Paths[0])
		if err != nil {
			a.imp().err.Set(err.Error())
			return
		}
		defer f.Close()
		a.loadCSV(filepath.Base(res.Paths[0]), f)
	})
}

// loadCSV reads a CSV file named name from r, and guesses its columns.
func (a *App) loadCSV(name string, r io.Reader) {
	st := a.imp()
	header, rows, err := ledger.ReadCSV(r)
	if err != nil {
		st.err.Set(name + " cannot be read: " + err.Error())
		return
	}
	st.err.Set("")
	st.name.Set(name)
	st.header.Set(header)
	st.rows.Set(rows)
	st.cols.Set(ledger.GuessColumns(header))
	if st.account.Peek() == "" {
		if d := a.s.Data.Peek(); len(d.Accounts) > 0 {
			st.account.Set(d.Accounts[0].ID)
		}
	}
}

func (a *App) closeImport() {
	a.importing.Set(false)
	a.importSt = nil
}

// importDialog imports transactions from a CSV file, such as a bank exports: it reads
// the file, guesses which column holds each field (each can be changed), and shows
// what would be imported, what is already in the account, and the rows it cannot read,
// before it adds anything. The import is one step for Undo.
func (a *App) importDialog(ctx *ui.Context) ui.Element {
	if !a.importing.Get() {
		return ui.Div()
	}
	t := ctx.Theme()
	st := a.imp()
	d := a.s.Data.Get()
	header := st.header.Get()
	var body *ui.DivElement
	var got ledger.Imported
	if len(header) == 0 {
		body = ui.Div().Column().Gap(12).Width(ui.Px(520)).
			Child(ui.Text("Choose a CSV file of transactions, such as one your bank exports. Dates and amounts are read in your region's format, and ISO dates (2026-10-13) too.").
				TextColor(t.TextMuted)).
			Child(ui.Button("Choose a CSV file…").Key("choose-file").Icon(icons.FileUp).OnClick(func() { a.chooseFile(ctx) }))
	} else {
		cols := st.cols.Get()
		got = ledger.Convert(d, st.rows.Get(), cols, st.account.Get(), ui.ParseDate, ui.ParseNumber)
		colOpts := []ui.Option[int]{{Value: ledger.None, Label: "None"}}
		for i, h := range header {
			name := h
			if name == "" {
				name = "Column " + strconv.Itoa(i+1)
			}
			colOpts = append(colOpts, ui.Option[int]{Value: i, Label: name})
		}
		pick := func(key, label string, get func(ledger.Columns) int, set func(*ledger.Columns, int)) ui.Element {
			v := ui.NewSignal(get(cols))
			return ui.Field(label, ui.Select(v, colOpts...).Key("col-"+key).Small().Width(ui.Full).
				OnChange(func(i int) {
					c := st.cols.Peek()
					set(&c, i)
					st.cols.Set(c)
				})).Grow(1)
		}
		acctOpts := make([]ui.Option[string], 0, len(d.Accounts))
		for _, ac := range d.Accounts {
			acctOpts = append(acctOpts, ui.Option[string]{Value: ac.ID, Label: ac.Name})
		}
		preview := ui.DataGrid(got.Transactions[:min(len(got.Transactions), 200)],
			func(x ledger.Transaction) string {
				return x.Date.String() + x.Payee + strconv.Itoa(int(x.Amount)) + x.Memo
			},
			ui.DataColumn[ledger.Transaction]{Header: "Date", Text: func(x ledger.Transaction) string { return ui.FormatDate(x.Date) }, Width: 100},
			ui.DataColumn[ledger.Transaction]{Header: "Payee", Text: func(x ledger.Transaction) string { return x.Payee }, Grow: 1},
			ui.DataColumn[ledger.Transaction]{Header: "Category", Text: func(x ledger.Transaction) string { return d.CategoryName(x.Category) }, Width: 140},
			ui.DataColumn[ledger.Transaction]{Header: "Amount", Text: func(x ledger.Transaction) string { return money(x.Amount) }, Width: 110, Align: ui.End},
		).Key("import-preview").Label("Transactions to import").Density(ui.DataGridCompact).Height(ui.Px(220)).
			Empty(ui.Empty("Nothing to import").Description("Every row is already in the account, or cannot be read."))
		summary := fmt.Sprintf("%s to import from %s", pluralize(len(got.Transactions), "transaction"), st.name.Get())
		if got.Duplicates > 0 {
			summary += ", " + pluralize(got.Duplicates, "row") + " already in the account"
		}
		body = ui.Div().Column().Gap(12).Width(ui.Px(640)).
			Child(ui.Div().Row().Gap(10).
				Child(ui.Field("Into the account", ui.Select(st.account, acctOpts...).Key("import-account").Small().Width(ui.Full)).Grow(1)).
				Child(pick("date", "Date", func(c ledger.Columns) int { return c.Date }, func(c *ledger.Columns, i int) { c.Date = i })).
				Child(pick("payee", "Payee", func(c ledger.Columns) int { return c.Payee }, func(c *ledger.Columns, i int) { c.Payee = i }))).
			Child(ui.Div().Row().Gap(10).
				Child(pick("amount", "Amount", func(c ledger.Columns) int { return c.Amount }, func(c *ledger.Columns, i int) { c.Amount = i })).
				Child(pick("out", "Money out", func(c ledger.Columns) int { return c.Outflow }, func(c *ledger.Columns, i int) { c.Outflow = i })).
				Child(pick("in", "Money in", func(c ledger.Columns) int { return c.Inflow }, func(c *ledger.Columns, i int) { c.Inflow = i })).
				Child(pick("category", "Category", func(c ledger.Columns) int { return c.Category }, func(c *ledger.Columns, i int) { c.Category = i }))).
			Child(ui.Text(summary).Key("import-summary").FontWeight(ui.FontMedium)).
			Child(preview)
		if n := len(got.Problems); n > 0 {
			list := ""
			for _, p := range got.Problems[:min(n, 4)] {
				list += fmt.Sprintf("Row %d: %s. ", p.Row, p.Reason)
			}
			if n > 4 {
				list += fmt.Sprintf("And %d more.", n-4)
			}
			body.Child(ui.Alert(pluralize(n, "row") + " cannot be read").Key("import-problems").Description(list))
		}
	}
	if e := st.err.Get(); e != "" {
		body.Child(ui.Alert("The file was not read").Danger().Description(e))
	}
	n := len(got.Transactions)
	doImport := ui.Button("Import " + pluralize(n, "transaction")).Key("do-import").Primary().Disabled(n == 0).
		OnClick(func() {
			a.s.Import(got.Transactions)
			a.closeImport()
			a.txAccount.Set("")
			a.txCategory.Set("")
			a.allTime.Set(true)
			a.page.Set(pageTransactions)
			a.toast(ui.ToastSuccess, "Imported "+pluralize(n, "transaction")).Action("Undo", a.s.Undo)
		})
	return ui.Modal("Import from CSV", body).Key("import-dialog").
		Footer(ui.Button("Cancel").Secondary().OnClick(a.closeImport), doImport).
		Default(doImport).
		OnClose(a.closeImport)
}
