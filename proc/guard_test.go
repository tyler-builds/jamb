package proc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// allowed are the files that may start a program without this package, and why. A
// file not listed starts programs through Command, CommandContext or Hide, so a GUI
// app on Windows flashes no console window for them.
var allowed = map[string]string{
	"proc/proc.go":             "it is the helper",
	"cmd/jamb/":                "a console tool: what it runs (go build, a signing tool that may ask for a PIN, the app in development) shares its console on purpose; the one GUI app it runs in the background is hidden by hideProcess",
	"examples/term/session.go": "the shell runs in a pseudo terminal the example draws, not in a console window",
}

// TestProgramsStartHidden fails when code outside allowed starts a program with os/exec,
// os.StartProcess or syscall directly. Tests are not checked: they run from a console.
func TestProgramsStartHidden(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") || name == "testdata" || name == "third_party" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		for prefix := range allowed {
			if rel == prefix || (strings.HasSuffix(prefix, "/") && strings.HasPrefix(rel, prefix)) {
				return nil
			}
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		checked++
		for _, pos := range directStarts(f) {
			p := fset.Position(pos)
			t.Errorf("%s:%d starts a program directly; use proc.Command, proc.CommandContext or proc.Hide so it opens no console window on Windows (or list the file in allowed, with why)", rel, p.Line)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 10 {
		t.Fatalf("checked only %d files under %s", checked, root)
	}
}

// directStarts finds the calls in f that start a program: exec.Command,
// exec.CommandContext, an exec.Cmd literal, os.StartProcess, syscall.StartProcess and
// syscall.ForkExec. A Cmd that Hide is called on in the same function is fine.
func directStarts(f *ast.File) []token.Pos {
	names := map[string]string{} // local name → import path
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		local := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			local = imp.Name.Name
		}
		names[local] = path
	}
	banned := map[string]map[string]bool{
		"os/exec": {"Command": true, "CommandContext": true},
		"os":      {"StartProcess": true},
		"syscall": {"StartProcess": true, "ForkExec": true},
	}
	var out []token.Pos
	for _, decl := range f.Decls {
		var found []token.Pos
		hidden := false
		ast.Inspect(decl, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok && sel.Sel.Name == "Hide" && strings.HasSuffix(names[id.Name], "/proc") {
						hidden = true
					}
				}
			case *ast.SelectorExpr:
				if id, ok := n.X.(*ast.Ident); ok && banned[names[id.Name]][n.Sel.Name] {
					found = append(found, n.Pos())
				}
			case *ast.CompositeLit:
				if sel, ok := n.Type.(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok && names[id.Name] == "os/exec" && sel.Sel.Name == "Cmd" {
						found = append(found, n.Pos())
					}
				}
			}
			return true
		})
		if !hidden {
			out = append(out, found...)
		}
	}
	return out
}
