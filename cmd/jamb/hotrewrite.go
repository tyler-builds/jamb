package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The rewrite that keeps state across restarts (see devtools/hot). Each
// ui.NewSignal call in the app's own module becomes, in a copy that go build
// reads through its overlay, a call of hot.Signal with a key named after the
// source, and each ui.Component call a call of hot.Component that names the
// component by its call. The copies keep every line where it was, so compiler errors, stack
// traces and the inspector's source locations still point at the app's files.

const (
	uiImport  = "github.com/tyler-builds/jamb/ui"
	hotImport = "github.com/tyler-builds/jamb/devtools/hot"
	hotName   = "jambhot__"
)

// rewriteSignals rewrites the .go files of dirs that are in the module at root,
// into work. It returns the overlay's replacements: each rewritten file's path,
// and its copy's.
func rewriteSignals(root string, dirs []string, work string) (map[string]string, error) {
	out := filepath.Join(work, "hot")
	os.RemoveAll(out)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	replace := map[string]string{}
	for _, dir := range dirs {
		rel, err := filepath.Rel(root, dir)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue // another module, such as jamb itself in a workspace
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			src, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			got, changed, err := rewriteFile(src, filepath.ToSlash(rel))
			if err != nil || !changed {
				continue // a file that does not parse is left for the compiler to report
			}
			copyPath := filepath.Join(out, strconv.Itoa(len(replace))+"_"+name)
			if err := os.WriteFile(copyPath, got, 0o644); err != nil {
				return nil, err
			}
			replace[path] = copyPath
		}
	}
	return replace, nil
}

// insert is text to put at an offset of the source.
type insert struct {
	at   int
	text string
}

// rewriteFile rewrites the ui.NewSignal calls of one file. pkg names the
// package's directory in the module ("." for its root), which starts each key.
func rewriteFile(src []byte, pkg string) ([]byte, bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, false, err
	}
	ui := ""
	for _, im := range f.Imports {
		if p, _ := strconv.Unquote(im.Path.Value); p == uiImport {
			ui = "ui"
			if im.Name != nil {
				ui = im.Name.Name
			}
		}
	}
	if ui == "" || ui == "_" || ui == "." {
		return nil, false, nil
	}
	off := func(p token.Pos) int { return fset.Position(p).Offset }

	var edits []insert
	used := map[string]int{}    // keys given in this file, to tell repeats apart
	ordinal := map[string]int{} // unnamed calls per function
	var stack []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn := enclosingFunc(stack)
		if isUICall(call.Fun, ui, "Component") {
			// A component is named by its call: the function and the
			// component calls before it there (see ui's sitePath).
			ordinal["component:"+fn]++
			site := pkg + ":" + fn + "/c" + strconv.Itoa(ordinal["component:"+fn])
			edits = append(edits, insert{off(call.Pos()), hotName + ".Component("}, insert{off(call.End()), ", " + strconv.Quote(site) + ")"})
			return true
		}
		if !isUICall(call.Fun, ui, "NewSignal") {
			return true
		}
		target := targetName(stack)
		if target == "" {
			ordinal[fn]++
			target = "#" + strconv.Itoa(ordinal[fn])
		}
		key := pkg + ":" + target
		if fn != "" {
			key = pkg + ":" + fn + "/" + target
		}
		if used[key]++; used[key] > 1 {
			key += "~" + strconv.Itoa(used[key])
		}
		edits = append(edits, insert{off(call.Pos()), hotName + ".Signal("}, insert{off(call.End()), ", " + strconv.Quote(key) + ")"})
		return true
	})
	if len(edits) == 0 {
		return nil, false, nil
	}
	// The import goes on the package clause's line, so no line moves.
	edits = append(edits, insert{off(f.Name.End()), "; import " + hotName + " " + strconv.Quote(hotImport)})
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].at > edits[j].at })
	out := bytes.Clone(src)
	for _, e := range edits {
		out = append(out[:e.at], append([]byte(e.text), out[e.at:]...)...)
	}
	return out, true, nil
}

// isUICall reports whether fun is ui.<name>, or ui.<name>[T].
func isUICall(fun ast.Expr, ui, name string) bool {
	switch e := fun.(type) {
	case *ast.IndexExpr:
		fun = e.X
	case *ast.IndexListExpr:
		fun = e.X
	}
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == ui
}

// enclosingFunc names the function around the top of the stack: Name, or
// Type.Name for a method; "" at package level.
func enclosingFunc(stack []ast.Node) string {
	for i := len(stack) - 1; i >= 0; i-- {
		fd, ok := stack[i].(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fd.Recv != nil && len(fd.Recv.List) == 1 {
			if t := typeName(fd.Recv.List[0].Type); t != "" {
				return t + "." + fd.Name.Name
			}
		}
		return fd.Name.Name
	}
	return ""
}

// targetName names what the call at the top of the stack is assigned to: a
// variable, or a field of a composite literal (Type.Field); "" when it is
// nothing named, such as an argument.
func targetName(stack []ast.Node) string {
	call := stack[len(stack)-1].(ast.Expr)
	if len(stack) < 2 {
		return ""
	}
	switch p := stack[len(stack)-2].(type) {
	case *ast.AssignStmt:
		if len(p.Lhs) == len(p.Rhs) {
			for i, r := range p.Rhs {
				if r == call {
					return exprName(p.Lhs[i])
				}
			}
		}
	case *ast.ValueSpec:
		if len(p.Names) == len(p.Values) {
			for i, v := range p.Values {
				if v == call {
					return p.Names[i].Name
				}
			}
		}
	case *ast.KeyValueExpr:
		if p.Value != call {
			return ""
		}
		field := exprName(p.Key)
		if field == "" {
			return ""
		}
		if len(stack) >= 3 {
			if cl, ok := stack[len(stack)-3].(*ast.CompositeLit); ok {
				if t := typeName(cl.Type); t != "" {
					return t + "." + field
				}
			}
		}
		return field
	}
	return ""
}

// exprName names an assignment's target: x, or the field of s.x.
func exprName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		if e.Name == "_" {
			return ""
		}
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}

// typeName names a type expression: T, pkg.T, *T, T[...].
func typeName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.StarExpr:
		return typeName(e.X)
	case *ast.IndexExpr:
		return typeName(e.X)
	case *ast.IndexListExpr:
		return typeName(e.X)
	}
	return ""
}

// hotOverlay writes the go build -overlay file: the devtools link, and the
// rewritten files.
func hotOverlay(work, linkPath, linkSrc string, replace map[string]string) (string, error) {
	all := map[string]string{linkPath: linkSrc}
	for k, v := range replace {
		all[k] = v
	}
	b := []byte(`{"Replace":{`)
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			b = append(b, ',')
		}
		b = fmt.Appendf(b, "%s:%s", strconv.Quote(k), strconv.Quote(all[k]))
	}
	b = append(b, "}}"...)
	path := filepath.Join(work, "overlay.json")
	return path, os.WriteFile(path, b, 0o644)
}

// moduleRoot returns the directory of the module of the package in dir, or ""
// when the go tool cannot tell.
func moduleRoot(dir string) string {
	cmd := exec.Command("go", "env", "GOMOD")
	cmd.Dir = dir
	out, err := cmd.Output()
	mod := strings.TrimSpace(string(out))
	if err != nil || mod == "" || mod == os.DevNull {
		return ""
	}
	return filepath.Dir(mod)
}

// rewrite rewrites the app's signals for the next build, and points the
// overlay at the copies. A failure leaves the build without them.
func (r *runner) rewrite() {
	link := filepath.Join(r.work, devtoolsLinkName)
	var replace map[string]string
	if r.modRoot != "" {
		var err error
		if replace, err = rewriteSignals(r.modRoot, r.watched, r.work); err != nil {
			logf("warning: the state will not survive this restart: %v", err)
			replace = nil
		}
	}
	overlay, err := hotOverlay(r.work, filepath.Join(r.dir, devtoolsLinkName), link, replace)
	if err != nil {
		logf("warning: %v", err)
		return
	}
	r.overlay = overlay
}

// checkpoint asks the running app to write its state to r.state over its
// control channel, and reports whether it did.
func (r *runner) checkpoint(pid int) bool {
	socket := filepath.Join(ctlDir(), strconv.Itoa(pid)+".sock")
	conn, err := net.DialTimeout("unix", socket, 500*time.Millisecond)
	if err != nil {
		return false // an app from before the control channel, or one that hangs
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(time.Second))
	req, _ := json.Marshal(map[string]any{"op": "checkpoint", "path": r.state, "freeze": true})
	if _, err := conn.Write(append(req, '\n')); err != nil {
		return false
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		logf("the app did not write its state in time; it restarts without it")
		return false
	}
	var resp struct {
		Result struct{ Values int } `json:"result"`
		Error  *struct{ Message string }
	}
	if json.Unmarshal(line, &resp) != nil || resp.Error != nil {
		if resp.Error != nil {
			logf("the state was not kept: %s", resp.Error.Message)
		}
		return false
	}
	return true
}

// swap replaces the running app with the build exe: it takes the old app's
// state, starts the new one beside it, and stops the old one once the new one
// has drawn and settled, so the window is never gone between the two.
func (r *runner) swap(exe string, changedAt time.Time) {
	old, oldExited := r.child, r.exited
	if old == nil {
		r.start(exe, changedAt)
		return
	}
	r.stateOK = r.checkpoint(old.Process.Pid)
	r.start(exe, changedAt)
	if r.child != nil && !r.waitReady(r.child.Process.Pid, r.exited) {
		logf("the new build did not settle in time; it replaces the old one all the same")
	}
	old.Process.Kill()
	<-oldExited
	for _, ext := range []string{".sock", ".json"} {
		os.Remove(filepath.Join(ctlDir(), strconv.Itoa(old.Process.Pid)+ext))
	}
}

// waitReady waits up to 10 seconds for the app pid to open its control channel
// and settle. It reports whether it did; false also when the app ended.
func (r *runner) waitReady(pid int, exited <-chan struct{}) bool {
	socket := filepath.Join(ctlDir(), strconv.Itoa(pid)+".sock")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-exited:
			return false
		case <-time.After(50 * time.Millisecond):
		}
		conn, err := net.DialTimeout("unix", socket, 200*time.Millisecond)
		if err != nil {
			continue
		}
		conn.SetDeadline(deadline)
		req, _ := json.Marshal(map[string]any{"op": "wait", "for": "idle", "timeout_ms": 5000})
		conn.Write(append(req, '\n'))
		line, err := bufio.NewReader(conn).ReadBytes('\n')
		conn.Close()
		var resp struct{ Error *struct{ Code string } }
		return err == nil && json.Unmarshal(line, &resp) == nil && resp.Error == nil
	}
	return false
}
