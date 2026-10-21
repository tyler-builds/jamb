package codeedit

import (
	"context"
	"strings"
)

// A CodeActionKind is the kind of a code action, in the hierarchy of the Language Server Protocol:
// "refactor.extract" is a kind of "refactor", and the menu groups the actions by their kind.
type CodeActionKind string

// The kinds of code actions. A provider may use any kind; these are the ones the menu groups.
const (
	CodeActionQuickFix              CodeActionKind = "quickfix"
	CodeActionRefactor              CodeActionKind = "refactor"
	CodeActionRefactorExtract       CodeActionKind = "refactor.extract"
	CodeActionRefactorInline        CodeActionKind = "refactor.inline"
	CodeActionRefactorRewrite       CodeActionKind = "refactor.rewrite"
	CodeActionSource                CodeActionKind = "source"
	CodeActionSourceOrganizeImports CodeActionKind = "source.organizeImports"
	CodeActionSourceFixAll          CodeActionKind = "source.fixAll"
)

// Within reports whether k is parent or a kind below it, so that "refactor.extract" is within
// "refactor", and "refactorExtra" is not.
func (k CodeActionKind) Within(parent CodeActionKind) bool {
	return k == parent || strings.HasPrefix(string(k), string(parent)+".")
}

// A CodeActionTrigger says why the editor asks for code actions.
type CodeActionTrigger uint8

const (
	CodeActionInvoked   CodeActionTrigger = iota + 1 // Ctrl+., a click on the lightbulb, or the hover's quick fix
	CodeActionAutomatic                              // the probe that decides whether the lightbulb shows
)

// A CodeActionQuery is a Query about the primary selection, with the diagnostics the actions
// may fix and why the editor asks.
type CodeActionQuery struct {
	// Query is the text and the offset of the primary caret.
	Query
	// From and To are the primary selection's range. From equals To for a caret.
	From, To int
	// Diagnostics are the diagnostics that overlap the range, their ends included. With none, they
	// are the diagnostics on the caret's row.
	Diagnostics []Diagnostic
	// Trigger is why the editor asks.
	Trigger CodeActionTrigger
}

// A CodeAction is one action the provider offers for the query: a quick fix, a refactoring or a
// source action. The editor applies its Edits itself and then calls its Run.
type CodeAction struct {
	// Title is the text the menu shows and announces.
	Title string
	// Kind is the kind of the action, which groups it in the menu.
	Kind CodeActionKind
	// Diagnostics are the diagnostics the action fixes. They are informational.
	Diagnostics []Diagnostic
	// Preferred sorts the action first in its group, and highlights it when the menu opens.
	Preferred bool
	// Disabled, when not empty, shows the action dimmed with this reason. It is not applied.
	Disabled string
	// Edits are the changes to this document, in the byte offsets of the query's text. They are
	// made as one step of the undo history.
	Edits []Change
	// Run runs on the UI thread after Edits: a command, a change in another file, or a workspace
	// edit. It may capture state that has since changed, so it should check what it acts on.
	Run func()
}

// A CodeActionProvider offers the code actions for a query, as a language server's code action
// request does. CodeActions runs on a goroutine of its own, with a context that is canceled when
// the answer is no longer wanted.
type CodeActionProvider interface {
	CodeActions(ctx context.Context, q CodeActionQuery) ([]CodeAction, error)
}

// CodeActionFunc is a function that is a CodeActionProvider.
type CodeActionFunc func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error)

// CodeActions calls f.
func (f CodeActionFunc) CodeActions(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
	return f(ctx, q)
}

// CodeActions sets the provider of the code actions. Ctrl+. and a click on the lightbulb that
// shows beside a line with a diagnostic open the menu of the actions at the caret; the editor asks
// the provider about each open, and the provider answers with the actions for the query.
func (e *EditorElement) CodeActions(p CodeActionProvider) *EditorElement { e.actionsP = p; return e }
