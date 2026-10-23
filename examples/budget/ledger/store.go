package ledger

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// undoDepth is how many changes Undo can take back.
const undoDepth = 100

// step is a change the store can take back: the data before it, and its name.
type step struct {
	before Data
	label  string
}

// Store holds the ledger in a signal. Each change is a new Data, and the one before
// it goes on the undo stack. Changes run on the UI thread.
type Store struct {
	Data *ui.Signal[Data]
	// UndoLabel and RedoLabel name the change that Undo and Redo would take back or
	// make again, such as "Delete 3 transactions", or "" when there is none.
	UndoLabel, RedoLabel *ui.Signal[string]
	// SaveErr is the last failure to save, or "".
	SaveErr *ui.Signal[string]

	undo, redo []step
	path       string
	mu         sync.Mutex // guards the save timer
	timer      *time.Timer
}

// New returns a store of d, saved to path ("" saves nothing, as in tests).
func New(d Data, path string) *Store {
	if d.Budgets == nil {
		d.Budgets = map[string]map[string]Money{}
	}
	return &Store{Data: ui.NewSignal(d), UndoLabel: ui.NewSignal(""), RedoLabel: ui.NewSignal(""),
		SaveErr: ui.NewSignal(""), path: path}
}

// Apply makes the change fn, which edits a copy of the data, and records it as one
// step named label for Undo.
func (s *Store) Apply(label string, fn func(d *Data)) {
	before := s.Data.Peek()
	next := before.clone()
	fn(&next)
	s.undo = append(s.undo, step{before, label})
	if len(s.undo) > undoDepth {
		s.undo = slices.Delete(s.undo, 0, len(s.undo)-undoDepth)
	}
	s.redo = nil
	s.set(next)
}

// Undo takes back the last change.
func (s *Store) Undo() {
	if len(s.undo) == 0 {
		return
	}
	st := s.undo[len(s.undo)-1]
	s.undo = s.undo[:len(s.undo)-1]
	s.redo = append(s.redo, step{s.Data.Peek(), st.label})
	s.set(st.before)
}

// Redo makes the last change that Undo took back again.
func (s *Store) Redo() {
	if len(s.redo) == 0 {
		return
	}
	st := s.redo[len(s.redo)-1]
	s.redo = s.redo[:len(s.redo)-1]
	s.undo = append(s.undo, step{s.Data.Peek(), st.label})
	s.set(st.before)
}

func (s *Store) set(d Data) {
	s.Data.Set(d)
	s.UndoLabel.Set(lastLabel(s.undo))
	s.RedoLabel.Set(lastLabel(s.redo))
	s.scheduleSave()
}

func lastLabel(steps []step) string {
	if len(steps) == 0 {
		return ""
	}
	return steps[len(steps)-1].label
}

// AddTransaction adds t, with a new ID, and returns the ID.
func (s *Store) AddTransaction(t Transaction) string {
	var id string
	s.Apply("Add transaction", func(d *Data) {
		t.ID = d.newID("t")
		id = t.ID
		d.Transactions = append(d.Transactions, t)
	})
	return id
}

// UpdateTransaction replaces the transaction with t's ID by t, as one step named label.
func (s *Store) UpdateTransaction(label string, t Transaction) {
	s.Apply(label, func(d *Data) {
		if i := slices.IndexFunc(d.Transactions, func(x Transaction) bool { return x.ID == t.ID }); i >= 0 {
			d.Transactions[i] = t
		}
	})
}

// DeleteTransactions removes the transactions with the IDs.
func (s *Store) DeleteTransactions(ids []string) {
	label := "Delete transaction"
	if len(ids) > 1 {
		label = "Delete " + strconv.Itoa(len(ids)) + " transactions"
	}
	s.Apply(label, func(d *Data) {
		d.Transactions = slices.DeleteFunc(d.Transactions, func(t Transaction) bool { return slices.Contains(ids, t.ID) })
	})
}

// SetBudget sets the amount budgeted for the category in the month.
func (s *Store) SetBudget(m Month, category string, amount Money) {
	s.Apply("Change budget", func(d *Data) {
		k := m.Key()
		if d.Budgets[k] == nil {
			d.Budgets[k] = map[string]Money{}
		}
		if amount == 0 {
			delete(d.Budgets[k], category)
		} else {
			d.Budgets[k][category] = amount
		}
	})
}

// CopyBudget sets the budgets of month to those of from, the month before it.
func (s *Store) CopyBudget(from, month Month) {
	s.Apply("Copy last month's budget", func(d *Data) {
		src := d.Budgets[from.Key()]
		dst := make(map[string]Money, len(src))
		for c, v := range src {
			dst[c] = v
		}
		d.Budgets[month.Key()] = dst
	})
}

// SaveAccount adds a (when its ID is empty) or replaces the account with its ID.
func (s *Store) SaveAccount(a Account) string {
	label := "Edit account"
	if a.ID == "" {
		label = "Add account"
	}
	var id string
	s.Apply(label, func(d *Data) {
		if a.ID == "" {
			a.ID = d.newID("a")
			d.Accounts = append(d.Accounts, a)
		} else if i := slices.IndexFunc(d.Accounts, func(x Account) bool { return x.ID == a.ID }); i >= 0 {
			d.Accounts[i] = a
		}
		id = a.ID
	})
	return id
}

// ErrAccountInUse is the error of DeleteAccount for an account that has transactions.
var ErrAccountInUse = errors.New("the account has transactions; move or delete them first")

// DeleteAccount removes the account with id, which must have no transactions.
func (s *Store) DeleteAccount(id string) error {
	if slices.ContainsFunc(s.Data.Peek().Transactions, func(t Transaction) bool { return t.Account == id }) {
		return ErrAccountInUse
	}
	s.Apply("Delete account", func(d *Data) {
		d.Accounts = slices.DeleteFunc(d.Accounts, func(a Account) bool { return a.ID == id })
	})
	return nil
}

// Import adds the transactions, each with a new ID, as one step.
func (s *Store) Import(ts []Transaction) {
	s.Apply("Import "+strconv.Itoa(len(ts))+" transactions", func(d *Data) {
		for _, t := range ts {
			t.ID = d.newID("t")
			d.Transactions = append(d.Transactions, t)
		}
	})
}

// ---- saving ----

// DefaultPath is where the ledger is kept: jamb-budget\budget.json in the user's
// configuration folder, or the file JAMB_BUDGET_FILE names.
func DefaultPath() string {
	if p := os.Getenv("JAMB_BUDGET_FILE"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "jamb-budget", "budget.json")
}

// Load reads the ledger at path. A missing file gives false, so the app can start
// from the sample.
func Load(path string) (Data, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Data{}, false, nil
	}
	if err != nil {
		return Data{}, false, err
	}
	var d Data
	if err := json.Unmarshal(b, &d); err != nil {
		return Data{}, false, err
	}
	return d, true, nil
}

// scheduleSave saves 400 ms after the last change, so a run of edits is one write.
func (s *Store) scheduleSave() {
	if s.path == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
	}
	d := s.Data.Peek()
	s.timer = time.AfterFunc(400*time.Millisecond, func() { s.write(d) })
}

// Flush saves now, as the window closes.
func (s *Store) Flush() error {
	if s.path == "" {
		return nil
	}
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.mu.Unlock()
	return s.write(s.Data.Peek())
}

// write saves d to a temporary file and renames it over the ledger, so a failure
// part way leaves the last saved ledger whole.
func (s *Store) write(d Data) error {
	err := func() error {
		b, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
			return err
		}
		tmp := s.path + ".tmp"
		if err := os.WriteFile(tmp, b, 0o644); err != nil {
			return err
		}
		return os.Rename(tmp, s.path)
	}()
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	if s.SaveErr.Peek() != msg {
		s.SaveErr.Set(msg)
	}
	return err
}
