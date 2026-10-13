// Package svg draws static SVG images with internal/vector: shapes, paths,
// groups and use, transforms, fills and strokes with colors and linear and
// radial gradients, opacity, clip paths, and the presentation attributes,
// style attributes and simple style sheets that design tools write. Text,
// filters, masks, patterns, markers, embedded images, animation and scripts
// are left out: those elements draw nothing.
package svg

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// node is an element of the document, with its attributes after style sheets
// and style attributes were applied, each property under its own name.
type node struct {
	name     string
	attrs    map[string]string
	children []*node
}

func (n *node) attr(k string) string { return n.attrs[k] }

// Doc is a parsed SVG image.
type Doc struct {
	root *node
	ids  map[string]*node
	// W and H are the image's size: its width and height, else its viewBox's.
	W, H float32
	// viewBox maps the image's coordinates to its size.
	vbX, vbY, vbW, vbH float32
	hasViewBox         bool
	align              string // preserveAspectRatio's alignment, such as xMidYMid, or none
	slice              bool
}

// Parse reads an SVG document.
func Parse(data []byte) (*Doc, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.Entity = xml.HTMLEntity
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	var stack []*node
	var root *node
	var sheets []string
	inStyle := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("svg: %v", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &node{name: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				k := a.Name.Local
				if a.Name.Space != "" && k == "href" {
					k = "href" // xlink:href, as href
				} else if a.Name.Space != "" && a.Name.Space != "http://www.w3.org/2000/svg" {
					continue
				}
				n.attrs[k] = a.Value
			}
			if len(stack) == 0 {
				if n.name != "svg" {
					return nil, errors.New("svg: the root element is not svg")
				}
				root = n
			} else {
				p := stack[len(stack)-1]
				p.children = append(p.children, n)
			}
			stack = append(stack, n)
			inStyle = n.name == "style"
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			inStyle = false
		case xml.CharData:
			if inStyle {
				sheets = append(sheets, string(t))
			}
		}
	}
	if root == nil {
		return nil, errors.New("svg: no svg element")
	}
	d := &Doc{root: root, ids: map[string]*node{}}
	rules := parseSheet(strings.Join(sheets, "\n"))
	var walk func(n *node)
	walk = func(n *node) {
		applyStyles(n, rules)
		if id := n.attrs["id"]; id != "" {
			d.ids[id] = n
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(root)
	d.size()
	return d, nil
}

// size reads the image's size and viewBox.
func (d *Doc) size() {
	r := d.root
	if vb := r.attr("viewBox"); vb != "" {
		n := &numbers{s: vb}
		x, ok1 := n.next()
		y, ok2 := n.next()
		w, ok3 := n.next()
		h, ok4 := n.next()
		if ok1 && ok2 && ok3 && ok4 && w > 0 && h > 0 {
			d.vbX, d.vbY, d.vbW, d.vbH, d.hasViewBox = x, y, w, h, true
		}
	}
	w, wok := length(r.attr("width"), 0)
	h, hok := length(r.attr("height"), 0)
	switch {
	case wok && hok:
	case d.hasViewBox && wok:
		h = w * d.vbH / d.vbW
	case d.hasViewBox && hok:
		w = h * d.vbW / d.vbH
	case d.hasViewBox:
		w, h = d.vbW, d.vbH
	default:
		w, h = 300, 150 // the size browsers give an SVG with neither
	}
	d.W, d.H = w, h
	d.align = "xMidYMid"
	par := strings.Fields(r.attr("preserveAspectRatio"))
	if len(par) > 0 {
		d.align = par[0]
		d.slice = len(par) > 1 && par[1] == "slice"
	}
}

// length reads a length in pixels: a number with px, pt, pc, mm, cm, in or em
// (16 pixels). A percentage is of ref; with ref 0 it is not a length here.
func length(s string, ref float32) (float32, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	unit := float32(1)
	for _, u := range []struct {
		s string
		k float32
	}{{"px", 1}, {"pt", 4.0 / 3}, {"pc", 16}, {"mm", 96 / 25.4}, {"cm", 96 / 2.54}, {"in", 96}, {"em", 16}, {"rem", 16}, {"%", -1}} {
		if strings.HasSuffix(s, u.s) {
			s, unit = strings.TrimSpace(s[:len(s)-len(u.s)]), u.k
			break
		}
	}
	n := &numbers{s: s}
	v, ok := n.next()
	if !ok || n.more() {
		return 0, false
	}
	if unit < 0 {
		if ref == 0 {
			return 0, false
		}
		return v * ref / 100, true
	}
	return v * unit, true
}

// ---- style sheets ----

// rule is one selector of a style sheet with its declarations.
type rule struct {
	tag, class, id string // empty: any
	decls          [][2]string
	specificity    int
	order          int
}

// parseSheet reads the rules of a style sheet whose selectors are a type, a
// class, an id, or a type with a class or id (such as path.st0). Rules with
// other selectors, and at-rules, are skipped.
func parseSheet(css string) []rule {
	// Comments.
	for {
		i := strings.Index(css, "/*")
		if i < 0 {
			break
		}
		j := strings.Index(css[i+2:], "*/")
		if j < 0 {
			css = css[:i]
			break
		}
		css = css[:i] + css[i+2+j+2:]
	}
	var rules []rule
	for len(css) > 0 {
		open := strings.IndexByte(css, '{')
		if open < 0 {
			break
		}
		closeAt := strings.IndexByte(css[open:], '}')
		if closeAt < 0 {
			break
		}
		sel, body := strings.TrimSpace(css[:open]), css[open+1:open+closeAt]
		css = css[open+closeAt+1:]
		if strings.HasPrefix(sel, "@") {
			continue
		}
		decls := parseDecls(body)
		for _, s := range strings.Split(sel, ",") {
			if r, ok := parseSelector(strings.TrimSpace(s)); ok {
				r.decls, r.order = decls, len(rules)
				rules = append(rules, r)
			}
		}
	}
	return rules
}

func parseSelector(s string) (rule, bool) {
	if s == "" || strings.ContainsAny(s, " >+~:[") {
		return rule{}, false
	}
	var r rule
	if s == "*" {
		return r, true
	}
	if i := strings.IndexAny(s, ".#"); i != 0 {
		if i < 0 {
			r.tag = s
			r.specificity = 1
			return r, true
		}
		r.tag, s = s[:i], s[i:]
		r.specificity = 1
	}
	switch s[0] {
	case '.':
		if strings.ContainsAny(s[1:], ".#") {
			return rule{}, false
		}
		r.class = s[1:]
		r.specificity += 10
	case '#':
		r.id = s[1:]
		r.specificity += 100
	}
	return r, true
}

// parseDecls reads "fill: red; stroke: #000" into name, value pairs.
func parseDecls(s string) [][2]string {
	var out [][2]string
	for _, d := range strings.Split(s, ";") {
		k, v, ok := strings.Cut(d, ":")
		if !ok {
			continue
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "!important"))
		if k != "" && v != "" {
			out = append(out, [2]string{k, v})
		}
	}
	return out
}

// applyStyles applies the matching rules, then the style attribute, over the
// element's presentation attributes, as CSS ranks them.
func applyStyles(n *node, rules []rule) {
	var match []rule
	classes := strings.Fields(n.attrs["class"])
	for _, r := range rules {
		if (r.tag == "" || r.tag == n.name) && (r.id == "" || r.id == n.attrs["id"]) &&
			(r.class == "" || contains(classes, r.class)) {
			match = append(match, r)
		}
	}
	// Lower specificity first, then document order, so later writes win.
	for i := 1; i < len(match); i++ {
		for j := i; j > 0 && (match[j].specificity < match[j-1].specificity ||
			match[j].specificity == match[j-1].specificity && match[j].order < match[j-1].order); j-- {
			match[j], match[j-1] = match[j-1], match[j]
		}
	}
	for _, r := range match {
		for _, d := range r.decls {
			n.attrs[d[0]] = d[1]
		}
	}
	for _, d := range parseDecls(n.attrs["style"]) {
		n.attrs[d[0]] = d[1]
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
