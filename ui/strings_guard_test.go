package ui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// The text that the user reads or hears is in Strings, so that it can be translated.
// This guard scans the non-test files of the package for string literals in the places
// where the builders take user-facing text: the arguments of the calls in
// userFacingCalls, the values of the variables in userFacingVars, and the Label and
// Title of a struct literal. A literal with no letters, such as "..." or "»", is a
// symbol and is not checked. The scan works on the syntax tree, so it does not see a
// literal that is built at run time or passed through another variable.

// userFacingCalls are the builders whose string arguments are shown or announced.
var userFacingCalls = map[string]bool{
	"Text": true, "Textf": true, "Label": true, "Placeholder": true,
	"Tooltip": true, "Announce": true, "Alt": true, "Button": true,
}

// userFacingVars are the variables that hold a name or a title that a builder shows.
// The messages that are announced are checked at their calls, in userFacingCalls.
// A variable called msg or label is not here: the accessibility warnings (access.go)
// and the dump labels (debug.go) use them for the developer.
var userFacingVars = map[string]bool{
	"name": true, "title": true, "status": true, "placeholder": true, "maximizeLabel": true,
}

// developerFiles are the files whose text is for the developer and stays English: the
// dump of the element tree (debug.go). The rest of the package is scanned.
var developerFiles = map[string]bool{
	"debug.go": true,
}

// literalAllowed are the literals with letters that are not translated, because they
// are the same in every language.
var literalAllowed = map[string]bool{
	"#RRGGBB":   true, // hex notation, a placeholder for a color
	"#RRGGBBAA": true, // hex notation, a placeholder for a color with alpha
}

// userFacingLiterals returns the literals of the source file, named file, that are
// user-facing text and are not allowed, each with its position.
func userFacingLiterals(file, src string) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		return []string{file + ": " + err.Error()}
	}
	var found []string
	check := func(e ast.Expr) {
		lit, ok := e.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return
		}
		text, err := strconv.Unquote(lit.Value)
		if err != nil || literalAllowed[text] || strings.IndexFunc(text, unicode.IsLetter) < 0 {
			return
		}
		found = append(found, fset.Position(lit.Pos()).String()+" "+strconv.Quote(text))
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			name := ""
			switch fn := n.Fun.(type) {
			case *ast.Ident:
				name = fn.Name
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			}
			if userFacingCalls[name] {
				for _, a := range n.Args {
					check(a)
				}
			}
		case *ast.AssignStmt:
			for i, l := range n.Lhs {
				if id, ok := l.(*ast.Ident); ok && userFacingVars[id.Name] && i < len(n.Rhs) {
					check(n.Rhs[i])
				}
			}
		case *ast.KeyValueExpr:
			if id, ok := n.Key.(*ast.Ident); ok && (id.Name == "Label" || id.Name == "Title") {
				check(n.Value)
			}
		}
		return true
	})
	return found
}

// TestBuiltInTextIsNotWrittenInTheCode fails on a user-facing literal in the package:
// move the text to a field of Strings, and read it from the window's Strings.
func TestBuiltInTextIsNotWrittenInTheCode(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || developerFiles[name] {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, hit := range userFacingLiterals(name, string(src)) {
			t.Errorf("user-facing text written in the code, move it to a field of Strings: %s", hit)
		}
	}
}

// The guard finds the shapes it is meant to find, so that a passing scan means
// something: the calls, the variables and the struct field, and nothing else.
func TestLiteralScanFindsUserFacingText(t *testing.T) {
	src := `package ui

func f() {
	Text("Hello")
	Text("...")
	name = "Pagination"
	name = ""
	placeholder := "#RRGGBB"
	Button(x).Label("Save")
	Announce("Saved", access.Polite)
	_ = TopMenu{Label: "File"}
	_ = Text(fmt.Sprint(n))
}
`
	got := userFacingLiterals("probe.go", src)
	if len(got) != 5 {
		t.Fatalf("the scan found %d literals, want 5 (Hello, Pagination, Save, Saved, File): %v", len(got), got)
	}
	for _, want := range []string{`"Hello"`, `"Pagination"`, `"Save"`, `"Saved"`, `"File"`} {
		found := false
		for _, g := range got {
			found = found || strings.HasSuffix(g, want)
		}
		if !found {
			t.Errorf("the scan missed %s: %v", want, got)
		}
	}
}
