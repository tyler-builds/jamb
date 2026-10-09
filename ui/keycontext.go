package ui

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Key contexts and context expressions follow GPUI (keymap/context.rs). An
// element names its context with KeyContext; a contextual binding's
// expression (BindIn) is matched against the contexts on the focus path. The
// deepest match decides which binding runs first.

// ctxEntry is one entry of a KeyContext: an identifier (value is ""), or a
// key with a value.
type ctxEntry struct{ key, value string }

// keyCtx is a parsed KeyContext.
type keyCtx []ctxEntry

// contains reports whether key is an entry of kc, with or without a value.
func (kc keyCtx) contains(key string) bool {
	for _, e := range kc {
		if e.key == key {
			return true
		}
	}
	return false
}

// get returns the value of key, if key has one.
func (kc keyCtx) get(key string) (string, bool) {
	for _, e := range kc {
		if e.key == key {
			return e.value, e.value != ""
		}
	}
	return "", false
}

// parseKeyCtx parses a KeyContext such as "Editor mode=full": identifiers and
// key=value pairs separated by whitespace. A key that is already present keeps
// its first entry.
func parseKeyCtx(src string) (keyCtx, error) {
	var kc keyCtx
	src = skipSpace(src)
	for src != "" {
		key := ctxIdent(src)
		if key == "" {
			r, _ := utf8.DecodeRuneInString(src)
			return nil, fmt.Errorf("unexpected %q; entries are identifiers or key=value", r)
		}
		src = skipSpace(src[len(key):])
		rest, ok := strings.CutPrefix(src, "=")
		if !ok {
			if !kc.contains(key) {
				kc = append(kc, ctxEntry{key: key})
			}
			continue
		}
		src = skipSpace(rest)
		value := ctxIdent(src)
		if value == "" {
			return nil, fmt.Errorf("%q has no value", key+"=")
		}
		src = skipSpace(src[len(value):])
		if !kc.contains(key) {
			kc = append(kc, ctxEntry{key, value})
		}
	}
	return kc, nil
}

// contextStack returns the key contexts of the elements on the focus path
// that have one, root first.
func contextStack(path []*node) []keyCtx {
	var stack []keyCtx
	for i := len(path) - 1; i >= 0; i-- {
		if n := path[i]; n.div != nil && len(n.div.keyCtx) > 0 {
			stack = append(stack, n.div.keyCtx)
		}
	}
	return stack
}

// predOp is the kind of a contextPredicate node.
type predOp uint8

const (
	predIdent      predOp = iota // name
	predEq                       // name == value
	predNeq                      // name != value
	predDescendant               // a > b: b below a
	predNot                      // !a
	predAnd                      // a && b
	predOr                       // a || b
)

// Binding precedences, lowest first, as in GPUI.
const (
	precDescendant = 1
	precOr         = 2
	precAnd        = 3
	precEq         = 4
	precNot        = 5
)

// contextPredicate is a parsed BindIn expression.
type contextPredicate struct {
	op    predOp
	name  string // the identifier, or the key of == and !=
	value string // the value of == and !=
	a, b  *contextPredicate
}

// predOps are the binary operators, in the order the parser tries them.
var predOps = []struct {
	text string
	prec int
}{
	{">", precDescendant}, {"&&", precAnd}, {"||", precOr}, {"==", precEq}, {"!=", precEq},
}

// parseContextPredicate parses a BindIn expression: identifiers, key == value
// and key != value (identifiers on both sides), ! (not), && and ||, the >
// descendant operator, and parentheses. Operators bind, from loosest to
// tightest: >, ||, &&, == and !=, !.
func parseContextPredicate(src string) (*contextPredicate, error) {
	p, rest, err := parsePredicate(skipSpace(src), 0)
	if err != nil {
		return nil, err
	}
	if rest != "" {
		r, _ := utf8.DecodeRuneInString(rest)
		return nil, fmt.Errorf("unexpected character %q", r)
	}
	return p, nil
}

// parsePredicate parses operators that bind at least as tightly as minPrec.
func parsePredicate(src string, minPrec int) (*contextPredicate, string, error) {
	p, src, err := parsePredicatePrimary(src)
	if err != nil {
		return nil, "", err
	}
	for {
		var op string
		var prec int
		for _, o := range predOps {
			if strings.HasPrefix(src, o.text) && o.prec >= minPrec {
				op, prec = o.text, o.prec
				break
			}
		}
		if op == "" {
			return p, src, nil
		}
		right, rest, err := parsePredicate(skipSpace(src[len(op):]), prec+1)
		if err != nil {
			return nil, "", err
		}
		if p, err = newPredicate(op, p, right); err != nil {
			return nil, "", err
		}
		src = rest
	}
}

// parsePredicatePrimary parses an identifier, a !, or a parenthesized group.
func parsePredicatePrimary(src string) (*contextPredicate, string, error) {
	if src == "" {
		return nil, "", errors.New("unexpected end of expression")
	}
	next, _ := utf8.DecodeRuneInString(src)
	switch {
	case next == '(':
		p, rest, err := parsePredicate(skipSpace(src[1:]), 0)
		if err != nil {
			return nil, "", err
		}
		rest, ok := strings.CutPrefix(rest, ")")
		if !ok {
			return nil, "", errors.New("expected a ')'")
		}
		return p, skipSpace(rest), nil
	case next == '!':
		p, rest, err := parsePredicate(skipSpace(src[1:]), precNot)
		if err != nil {
			return nil, "", err
		}
		return &contextPredicate{op: predNot, a: p}, rest, nil
	case isCtxIdentChar(next):
		name := ctxIdent(src)
		return &contextPredicate{op: predIdent, name: name}, skipSpace(src[len(name):]), nil
	}
	return nil, "", fmt.Errorf("unexpected character %q", next)
}

// newPredicate joins two operands with op.
func newPredicate(op string, a, b *contextPredicate) (*contextPredicate, error) {
	switch op {
	case ">":
		return &contextPredicate{op: predDescendant, a: a, b: b}, nil
	case "&&":
		return &contextPredicate{op: predAnd, a: a, b: b}, nil
	case "||":
		return &contextPredicate{op: predOr, a: a, b: b}, nil
	}
	if a.op != predIdent || b.op != predIdent {
		return nil, fmt.Errorf("operands of %s must be identifiers", op)
	}
	kind := predEq
	if op == "!=" {
		kind = predNeq
	}
	return &contextPredicate{op: kind, name: a.name, value: b.name}, nil
}

// depthOf returns the deepest depth at which p matches stack, where depth d
// means the first d contexts, and reports whether p matches at all.
func (p *contextPredicate) depthOf(stack []keyCtx) (int, bool) {
	for depth := len(stack); depth >= 0; depth-- {
		if p.eval(stack[:depth], stack) {
			return depth, true
		}
	}
	return 0, false
}

// eval matches p against contexts, the stack up to some depth, ordered from
// lowest to highest. all is the whole stack; ! checks every prefix of it.
func (p *contextPredicate) eval(contexts, all []keyCtx) bool {
	if len(contexts) == 0 {
		return false
	}
	ctx := contexts[len(contexts)-1]
	switch p.op {
	case predIdent:
		return ctx.contains(p.name)
	case predEq:
		v, ok := ctx.get(p.name)
		return ok && v == p.value
	case predNeq:
		v, ok := ctx.get(p.name)
		return !ok || v != p.value
	case predNot:
		for i := range all {
			if p.a.eval(all[:i+1], all) {
				return false
			}
		}
		return true
	case predDescendant:
		// Workspace > Pane > Editor: the first match of a that has a
		// descendant matching b decides.
		for i := 0; i < len(contexts)-1; i++ {
			if p.a.eval(contexts[:i+1], all) {
				return p.b.eval(contexts[i+1:], contexts[i+1:])
			}
		}
		return false
	case predAnd:
		return p.a.eval(contexts, all) && p.b.eval(contexts, all)
	case predOr:
		return p.a.eval(contexts, all) || p.b.eval(contexts, all)
	}
	return false
}

// isCtxIdentChar reports whether r may appear in an identifier: letters,
// digits, _ and -.
func isCtxIdentChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

// ctxIdent returns the identifier at the start of s, or "".
func ctxIdent(s string) string {
	end := strings.IndexFunc(s, func(r rune) bool { return !isCtxIdentChar(r) })
	if end < 0 {
		return s
	}
	return s[:end]
}

// skipSpace drops leading whitespace.
func skipSpace(s string) string { return strings.TrimLeftFunc(s, unicode.IsSpace) }
