package svg

import "strings"

// ParsePlain reads an SVG image that is written in the plain subset of XML
// that the icons package writes its own icons in, without the XML decoder, so
// a program that draws only those icons does not link the decoder. root is the
// start tag of the root element and inside is what the root holds. An element
// is a name and attributes whose values are in double quotes, and it closes
// itself (<path d="…"/>) or holds more elements (<g fill="none">…</g>). One
// space separates attributes, and nothing separates elements. The subset has
// no text, comments, entities, names with a prefix, or repeated attributes.
//
// It returns the document that Parse returns for root, inside and the root's
// end tag, and reports false for a source outside the subset.
func ParsePlain(root, inside string) (*Doc, bool) {
	p := plainParser{s: root}
	r, open := p.startTag()
	if r == nil || !open || p.s != "" || r.name != "svg" {
		return nil, false
	}
	p.s = inside
	for p.s != "" {
		c := p.element()
		if c == nil {
			return nil, false
		}
		r.children = append(r.children, c)
	}
	return newDoc(r, nil), true
}

// plainParser reads elements from the start of s.
type plainParser struct{ s string }

// element reads one element with the elements it holds, or returns nil.
func (p *plainParser) element() *node {
	n, open := p.startTag()
	if n == nil || !open {
		return n
	}
	for !strings.HasPrefix(p.s, "</") {
		c := p.element()
		if c == nil {
			return nil
		}
		n.children = append(n.children, c)
	}
	end := "</" + n.name + ">"
	if !strings.HasPrefix(p.s, end) {
		return nil
	}
	p.s = p.s[len(end):]
	return n
}

// startTag reads a start tag, and reports whether the element is open: its tag
// ends with > and not with />. It returns nil when s does not start with a tag
// of the subset.
func (p *plainParser) startTag() (n *node, open bool) {
	if !strings.HasPrefix(p.s, "<") {
		return nil, false
	}
	p.s = p.s[1:]
	name := p.name()
	if name == "" {
		return nil, false
	}
	n = &node{name: name, attrs: map[string]string{}}
	for {
		switch {
		case strings.HasPrefix(p.s, "/>"):
			p.s = p.s[2:]
			return n, false
		case strings.HasPrefix(p.s, ">"):
			p.s = p.s[1:]
			return n, true
		case !strings.HasPrefix(p.s, " "):
			return nil, false
		}
		p.s = p.s[1:]
		k := p.name()
		if k == "" || !strings.HasPrefix(p.s, `="`) {
			return nil, false
		}
		p.s = p.s[2:]
		end := strings.IndexByte(p.s, '"')
		if end < 0 {
			return nil, false
		}
		v := p.s[:end]
		p.s = p.s[end+1:]
		if _, repeated := n.attrs[k]; repeated || strings.ContainsAny(v, "&<") {
			return nil, false
		}
		n.attrs[k] = v
	}
}

// name reads a name of letters, digits and hyphens, which may be empty.
func (p *plainParser) name() string {
	i := 0
	for i < len(p.s) {
		c := p.s[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' {
			break
		}
		i++
	}
	name := p.s[:i]
	p.s = p.s[i:]
	return name
}
