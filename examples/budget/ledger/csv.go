package ledger

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"
)

// None is the column of a field the file does not have.
const None = -1

// Columns says which column of a CSV file holds each field of a transaction. A bank
// writes the amount in one column, signed, or in two: money out and money in.
type Columns struct {
	Date, Payee, Amount, Outflow, Inflow, Category, Memo int
}

// ReadCSV reads a CSV file: its header row and its rows. The separator is a comma,
// or a semicolon when the header has more of them, as files written in regions that
// use the comma as the decimal point have. Rows may have fewer cells than the header.
func ReadCSV(r io.Reader) (header []string, rows [][]string, err error) {
	br := bufio.NewReader(r)
	first, _ := br.Peek(4096)
	line, _, _ := strings.Cut(string(first), "\n")
	cr := csv.NewReader(br)
	if strings.Count(line, ";") > strings.Count(line, ",") {
		cr.Comma = ';'
	}
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	all, err := cr.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(all) == 0 {
		return nil, nil, fmt.Errorf("the file is empty")
	}
	header = all[0]
	for i, h := range header {
		header[i] = strings.TrimSpace(strings.TrimPrefix(h, string(rune(0xFEFF)))) // a byte order mark leads some files
	}
	return header, all[1:], nil
}

// GuessColumns picks the columns of a file by the names in its header, as banks name
// them: "Date" or "Posted", "Payee", "Description" or "Merchant", "Amount", "Debit" or
// "Withdrawal" for money out, "Credit" or "Deposit" for money in, "Category", and
// "Memo" or "Notes". A column serves one field, so a "Debit amount" column is the
// money out and not the amount. A field with no match is None.
func GuessColumns(header []string) Columns {
	c := Columns{None, None, None, None, None, None, None}
	used := map[int]bool{}
	find := func(dst *int, names ...string) {
		for i, h := range header {
			h = strings.ToLower(h)
			for _, n := range names {
				if *dst == None && !used[i] && strings.Contains(h, n) {
					*dst, used[i] = i, true
				}
			}
		}
	}
	find(&c.Date, "date", "posted")
	find(&c.Payee, "payee", "description", "merchant", "name")
	find(&c.Outflow, "debit", "withdrawal", "outflow", "money out")
	find(&c.Inflow, "credit", "deposit", "inflow", "money in")
	find(&c.Amount, "amount")
	find(&c.Category, "category")
	find(&c.Memo, "memo", "note")
	return c
}

// Problem is a row of a file that was not imported, and why.
type Problem struct {
	Row    int // from 1, after the header
	Reason string
}

// Imported is what a file gives: the transactions to add, the rows it skipped as
// already in the account, and the rows it could not read.
type Imported struct {
	Transactions []Transaction
	Duplicates   int
	Problems     []Problem
}

// Convert turns the rows of a file into transactions of the account, with the columns
// cols. parseDate and parseNumber read the cells, in the user's locale (ui.ParseDate
// and ui.ParseNumber). A category is matched by its name, ignoring case; a name the
// ledger does not have leaves the transaction uncategorized. A row like a transaction
// the account has, on the same day, to the same payee, for the same amount, is a
// duplicate and is skipped, so a file imported twice adds nothing the second time.
func Convert(d Data, rows [][]string, cols Columns, account string,
	parseDate func(string) (time.Time, bool), parseNumber func(string) (float64, bool)) Imported {
	type key struct {
		day    time.Time
		payee  string
		amount Money
	}
	have := map[key]int{}
	for _, t := range d.Transactions {
		if t.Account == account {
			have[key{t.Date, strings.ToLower(t.Payee), t.Amount}]++
		}
	}
	byName := map[string]string{}
	for _, c := range d.Categories {
		byName[strings.ToLower(c.Name)] = c.ID
	}
	cell := func(row []string, i int) string {
		if i < 0 || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	var out Imported
	for n, row := range rows {
		if strings.TrimSpace(strings.Join(row, "")) == "" {
			continue // a blank line
		}
		bad := func(reason string) { out.Problems = append(out.Problems, Problem{Row: n + 1, Reason: reason}) }
		day, ok := parseDate(cell(row, cols.Date))
		if !ok {
			bad(fmt.Sprintf("the date %q cannot be read", cell(row, cols.Date)))
			continue
		}
		var amount Money
		switch {
		case cols.Amount != None:
			v, ok := parseNumber(cell(row, cols.Amount))
			if !ok {
				bad(fmt.Sprintf("the amount %q cannot be read", cell(row, cols.Amount)))
				continue
			}
			amount = FromFloat(v)
		case cols.Outflow != None || cols.Inflow != None:
			unread := ""
			for _, c := range []struct {
				col  int
				sign Money
			}{{cols.Outflow, -1}, {cols.Inflow, 1}} {
				text := cell(row, c.col)
				if text == "" {
					continue
				}
				v, ok := parseNumber(text)
				if !ok {
					unread = text
					break
				}
				// A bank may sign the outflow column or not; its money goes out either way.
				amount += c.sign * FromFloat(max(v, -v))
			}
			if unread != "" {
				bad(fmt.Sprintf("the amount %q cannot be read", unread))
				continue
			}
		default:
			bad("no column holds the amount")
			continue
		}
		if amount == 0 {
			bad("the amount is zero")
			continue
		}
		payee := cell(row, cols.Payee)
		k := key{day, strings.ToLower(payee), amount}
		if have[k] > 0 {
			have[k]--
			out.Duplicates++
			continue
		}
		out.Transactions = append(out.Transactions, Transaction{Date: day, Payee: payee,
			Category: byName[strings.ToLower(cell(row, cols.Category))], Account: account, Amount: amount,
			Memo: cell(row, cols.Memo)})
	}
	return out
}
