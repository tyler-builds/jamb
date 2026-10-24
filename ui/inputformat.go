package ui

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A format makes a TextInput show its text in a fixed shape as the user types it:
// a phone number as (555) 123-4567, or a number with the group separators of the
// user's locale. The signal holds the raw value, which is what the user typed
// without the shape: the slot characters of a template ("5551234567"), or a number
// in a canonical form (ASCII digits, "." for the decimal point and a leading "-",
// without groups), which strconv parses. The field shows the raw value presented
// (see presented); the caret, the selection and the undo history stay in the raw
// value, and are mapped to the text shown where it is drawn, hit and read.
//
// This file holds the formats themselves, without a Runtime: the editor (editor.go)
// asks them how to show the raw value and how an edit changes it.

// InputFormat is the shape a TextInput shows its text in (see TextInputElement.Format).
// Make one with TemplateFormat or NumberFormat; the zero InputFormat shows the text as
// it is.
type InputFormat struct {
	tokens []formatToken // the template, in order; nil for a number
	slots  []int         // the index in tokens of each slot of the template
	number bool
	opts   NumberOptions
}

// formatToken is one character of a template: a slot, which takes one character of
// its class, or a literal, which the field shows between the slots.
type formatToken struct {
	class byte // '9', 'A', '#' or '*' for a slot; 0 for a literal
	lit   rune
}

// NumberOptions say which numbers a NumberFormat takes.
type NumberOptions struct {
	Decimals   int  // the most digits after the decimal point; 0 takes whole numbers only
	Signed     bool // allow a leading minus
	NoGrouping bool // show no group separators
	// Currency makes the number an amount of money in the currency of the user's
	// region settings: it takes the decimal point, the group separator and the
	// grouping of amounts of money, and a zero Decimals is the currency's digits. A
	// TextInput with the format shows the currency's symbol before or after the
	// number, where the settings put it (see CurrencySymbol).
	Currency bool
}

// TemplateFormat returns the format of the template spec, in which each character is
// a slot or a literal:
//
//	9   a digit (full-width digits are taken as ASCII)
//	A   a letter
//	#   a letter or a digit
//	*   any character but a control character
//	\x  the literal x
//
// and any other character is a literal. "(999) 999-9999" is a phone number of the
// US. The raw value is the slot characters only, and a literal shows once the slot
// before it is filled. It panics when spec has no slots.
func TemplateFormat(spec string) InputFormat {
	var f InputFormat
	escaped := false
	for _, r := range spec {
		switch {
		case escaped:
			escaped = false
			f.tokens = append(f.tokens, formatToken{lit: r})
		case r == '\\':
			escaped = true
		case r == '9' || r == 'A' || r == '#' || r == '*':
			f.slots = append(f.slots, len(f.tokens))
			f.tokens = append(f.tokens, formatToken{class: byte(r)})
		default:
			f.tokens = append(f.tokens, formatToken{lit: r})
		}
	}
	if escaped {
		f.tokens = append(f.tokens, formatToken{lit: '\\'}) // a backslash at the end is itself
	}
	if len(f.slots) == 0 {
		panic("ui: TemplateFormat " + strconv.Quote(spec) + " has no slots")
	}
	return f
}

// NumberFormat returns the format of a number, shown with the decimal point, the
// group separators and the minus sign of the user's locale, such as 1,234.5 in
// en-US or 1.234,5 in de-DE. The raw value is the number in ASCII digits with "."
// and a leading "-", such as "-1234.5", which strconv.ParseFloat parses. While the
// user types, it can be "-", "12." or "007"; leaving the field makes it "", "12"
// and "7".
func NumberFormat(o NumberOptions) InputFormat {
	o.Decimals = max(o.Decimals, 0)
	if o.Currency && o.Decimals == 0 {
		o.Decimals = currentLocale().currDigits
	}
	return InputFormat{number: true, opts: o}
}

// locale returns the locale a number is shown and read in: the user's, with the
// decimal point and the group separator of amounts of money for a currency.
func (f *InputFormat) locale() *dateLocale {
	if f.opts.Currency {
		return currentLocale().money()
	}
	return currentLocale()
}

// zero reports whether f is the zero InputFormat, which shows the text as it is.
func (f *InputFormat) zero() bool { return !f.number && len(f.slots) == 0 }

// Display returns the raw value as the field shows it, such as "(555) 123-4567" for
// "5551234567", for showing the value outside the field: in a grid cell or a label.
// A value that does not fit the format is returned as it is.
func (f InputFormat) Display(raw string) string {
	if f.zero() {
		return raw
	}
	return f.present(raw).shown
}

// Complete returns a validator that rejects, with the message msg, a raw value that
// is not complete for f: a template with a slot not filled, a number that does not
// parse (such as "-" or "."), or a value that does not fit f. The empty value
// passes, so pair it with Required when the field must be filled in.
func (f InputFormat) Complete(msg string) Validator[string] {
	err := &messageError{text: func(*Strings) string { return msg }}
	return func(s string) error {
		if s == "" || f.zero() || f.complete(s) {
			return nil
		}
		return err
	}
}

// complete reports whether the raw value is a complete value of f.
func (f *InputFormat) complete(raw string) bool {
	if f.number {
		_, err := strconv.ParseFloat(raw, 64)
		return f.numberFits(raw) && err == nil
	}
	return f.fits(raw) && utf8.RuneCountInString(raw) == len(f.slots)
}

// ---- presentation ----

// presented is what a field shows for its raw value, and how the byte offsets of the
// two map to each other. raw holds the offset of each character of the raw value and
// then its length, and at the offset in shown where each of them is drawn, and then
// len(shown). Without them the text is shown as it is.
type presented struct {
	shown   string
	raw, at []int
}

// toShown returns the offset in the shown text of the raw offset r: 0 for 0, the
// end for the end, and the start of its character otherwise, so that a caret sits
// just before the slot it is at, after the literals before it.
func (p presented) toShown(r int) int {
	if p.raw == nil {
		return r
	}
	if r <= 0 {
		return 0
	}
	i := len(p.raw) - 1
	for i > 0 && p.raw[i] > r {
		i--
	}
	return p.at[i]
}

// endShown returns the offset in the shown text of the raw offset r as the end of a
// selection: just after the character before it, so that a selection ends before
// the literals after it, as "123" of "(555) 123-4567" does. The end is the end.
func (p presented) endShown(r int) int {
	if p.raw == nil {
		return r
	}
	i := len(p.raw) - 1
	if r >= p.raw[i] {
		return len(p.shown)
	}
	for i > 0 && p.raw[i] > r {
		i--
	}
	if i == 0 {
		return 0
	}
	_, size := utf8.DecodeRuneInString(p.shown[p.at[i-1]:])
	return p.at[i-1] + size
}

// fromShown returns the raw offset of the offset q in the shown text: the offset
// after the raw characters drawn before q.
func (p presented) fromShown(q int) int {
	if p.raw == nil {
		return q
	}
	k := 0
	for k < len(p.at)-1 && p.at[k] < q {
		k++
	}
	return p.raw[k]
}

// present returns how the field shows the raw value. A value that does not fit f is
// shown as it is.
func (f *InputFormat) present(raw string) presented {
	if raw == "" {
		return presented{}
	}
	if f.number {
		return f.presentNumber(raw, f.locale())
	}
	if !f.fits(raw) {
		return presented{shown: raw}
	}
	var b strings.Builder
	var p presented
	i := 0
	for _, t := range f.tokens {
		if t.class == 0 {
			b.WriteRune(t.lit)
			continue
		}
		if i == len(raw) {
			break // the literals after the last filled slot show; the rest does not
		}
		r, size := utf8.DecodeRuneInString(raw[i:])
		p.raw, p.at = append(p.raw, i), append(p.at, b.Len())
		b.WriteRune(r)
		i += size
	}
	p.shown = b.String()
	p.raw, p.at = append(p.raw, len(raw)), append(p.at, len(p.shown))
	return p
}

// promptChar is what a template draws for a slot not filled yet.
const promptChar = '_'

// prompt returns the rest of the template after what the raw value fills, with a
// promptChar for each slot: "_) ___-____" after "(55". The field draws it after the
// text; it is not part of the value. A number, a value that does not fit and a full
// template have none.
func (f *InputFormat) prompt(raw string) string {
	if f.number || !f.fits(raw) {
		return ""
	}
	n := utf8.RuneCountInString(raw)
	if n == len(f.slots) {
		return ""
	}
	from := 0
	if n > 0 {
		// What present shows ends with the literals after the last filled slot.
		from = f.slots[n]
	}
	var b strings.Builder
	for _, t := range f.tokens[from:] {
		if t.class == 0 {
			b.WriteRune(t.lit)
		} else {
			b.WriteRune(promptChar)
		}
	}
	return b.String()
}

// ---- templates ----

// asciiDigit returns r as an ASCII digit, for an ASCII or a full-width digit.
func asciiDigit(r rune) (rune, bool) {
	switch {
	case r >= '0' && r <= '9':
		return r, true
	case r >= '０' && r <= '９':
		return r - '０' + '0', true
	}
	return r, false
}

// takes returns the character a slot of the class holds for r, and whether it takes r.
func takes(class byte, r rune) (rune, bool) {
	if class == '9' || class == '#' {
		if d, ok := asciiDigit(r); ok {
			return d, true
		}
	}
	switch class {
	case 'A', '#':
		return r, unicode.IsLetter(r)
	case '*':
		return r, !unicode.IsControl(r)
	}
	return r, false
}

// fits reports whether the raw value fits the template: no more characters than it
// has slots, each one a character its slot holds.
func (f *InputFormat) fits(raw string) bool {
	k := 0
	for _, r := range raw {
		if k == len(f.slots) {
			return false
		}
		if c, ok := takes(f.tokens[f.slots[k]].class, r); !ok || c != r {
			return false
		}
		k++
	}
	return true
}

// nextLiteral reports whether r is a literal of the template between slot k-1 and
// slot k: one that the field shows at a caret after k slot characters.
func (f *InputFormat) nextLiteral(k int, r rune) bool {
	if k > len(f.slots) {
		return false
	}
	from, to := 0, len(f.tokens)
	if k > 0 {
		from = f.slots[k-1] + 1
	}
	if k < len(f.slots) {
		to = f.slots[k]
	}
	for _, t := range f.tokens[from:to] {
		if t.lit == r {
			return true
		}
	}
	return false
}

// fitPrefix appends to cur the characters of rest while they fit the slots after
// it, and returns the result and whether all of rest fit.
func (f *InputFormat) fitPrefix(cur []rune, rest string) ([]rune, bool) {
	for _, r := range rest {
		if len(cur) >= len(f.slots) {
			return cur, false
		}
		if c, ok := takes(f.tokens[f.slots[len(cur)]].class, r); !ok || c != r {
			return cur, false
		}
		cur = append(cur, r)
	}
	return cur, true
}

// templateFromDisplay returns the raw value of text when it is the template as the
// field shows it, or the start of it: each literal in its place and each slot
// holding a character it takes. It reports false for any other text.
func (f *InputFormat) templateFromDisplay(text string) (string, bool) {
	var b strings.Builder
	rs := []rune(text)
	i := 0
	for _, t := range f.tokens {
		if i == len(rs) {
			break
		}
		if t.class == 0 {
			if rs[i] != t.lit {
				return "", false
			}
			i++
			continue
		}
		c, ok := takes(t.class, rs[i])
		if !ok {
			return "", false
		}
		b.WriteRune(c)
		i++
	}
	return b.String(), i == len(rs) && b.Len() > 0
}

// editTemplate is edit for a template.
func (f *InputFormat) editTemplate(raw string, a, b int, ins string, paste bool) (string, int, bool) {
	if !f.fits(raw[:a]) {
		return raw, a, false // the value does not fit before the edit either (the app set it)
	}
	cur := []rune(raw[:a])
	added := 0
	if paste {
		// A paste takes the characters that fit, in order, and skips the others. Text
		// shown in the template's shape gives its slot characters.
		if s, ok := f.templateFromDisplay(ins); ok {
			ins = s
		}
		room := len(f.slots) - len(cur) - utf8.RuneCountInString(raw[b:])
		for _, r := range ins {
			if added >= room {
				break // the rest does not fit
			}
			if c, ok := takes(f.tokens[f.slots[len(cur)]].class, r); ok {
				cur = append(cur, c)
				added++
			}
		}
	} else {
		// Typing takes every character, or none: a character that does not fit its
		// slot, or that finds the template full, refuses the whole of it. The literal
		// that comes next is skipped, as the field shows it already.
		for _, r := range ins {
			if len(cur) < len(f.slots) {
				if c, ok := takes(f.tokens[f.slots[len(cur)]].class, r); ok {
					cur = append(cur, c)
					added++
					continue
				}
			}
			if f.nextLiteral(len(cur), r) {
				continue
			}
			return raw, a, false
		}
	}
	if added == 0 {
		return raw, a, false
	}
	caret := len(string(cur))
	// The characters after the edit move to the slots after it. Typing refuses an
	// edit that leaves one without a slot that takes it; a paste keeps those that fit.
	cur, ok := f.fitPrefix(cur, raw[b:])
	if !ok && !paste {
		return raw, a, false
	}
	return string(cur), caret, true
}

// ---- numbers ----

// numberFits reports whether the raw value is a number in the canonical form that
// f takes, or the start of one: an optional "-" (when f is signed), ASCII digits,
// and an optional "." (when f takes decimals) with at most f's decimals after it.
func (f *InputFormat) numberFits(raw string) bool {
	s := raw
	if strings.HasPrefix(s, "-") {
		if !f.opts.Signed {
			return false
		}
		s = s[1:]
	}
	whole, frac, dot := strings.Cut(s, ".")
	if dot && f.opts.Decimals == 0 || len(frac) > f.opts.Decimals {
		return false
	}
	return allDigits(whole) && allDigits(frac)
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// groupBreaks returns, for the n digits of a whole part, whether a group separator
// goes before each of them. sizes are the sizes of the groups from the right, the
// last one repeating; a size of 0 ends the grouping (see parseGrouping).
func groupBreaks(n int, sizes []int) []bool {
	brk := make([]bool, n)
	pos := n
	for i := 0; len(sizes) > 0; i++ {
		size := sizes[min(i, len(sizes)-1)]
		if size <= 0 {
			break
		}
		pos -= size
		if pos <= 0 {
			break
		}
		brk[pos] = true
	}
	return brk
}

// presentNumber shows the raw number in the locale: its minus sign, its group
// separators in the whole part and its decimal point.
func (f *InputFormat) presentNumber(raw string, loc *dateLocale) presented {
	if !f.numberFits(raw) {
		return presented{shown: raw}
	}
	whole := len(raw)
	if i := strings.IndexByte(raw, '.'); i >= 0 {
		whole = i
	}
	sign := 0
	if strings.HasPrefix(raw, "-") {
		sign = 1
	}
	var brk []bool
	if !f.opts.NoGrouping {
		brk = groupBreaks(whole-sign, loc.grouping)
	}
	var b strings.Builder
	var p presented
	for i := 0; i < len(raw); i++ {
		if brk != nil && i >= sign && i < whole && brk[i-sign] {
			b.WriteString(loc.group)
		}
		p.raw, p.at = append(p.raw, i), append(p.at, b.Len())
		switch raw[i] {
		case '-':
			b.WriteString(loc.minus)
		case '.':
			b.WriteString(loc.decimal)
		default:
			b.WriteByte(raw[i])
		}
	}
	p.shown = b.String()
	p.raw, p.at = append(p.raw, len(raw)), append(p.at, len(p.shown))
	return p
}

// The kinds of symbol in the text of a number (see numberSymbol).
const (
	symOther = iota
	symDigit
	symDecimal
	symGroup
	symMinus
	symPlus
)

// numberSymbol reads the symbol at the start of s, in the locale: a digit (ASCII or
// full-width, returned as ASCII), the decimal point, a group separator (the locale's,
// or a space of any width), a minus sign or a plus sign. It returns its kind, the
// digit, and its length in bytes.
func numberSymbol(s string, loc *dateLocale) (kind int, digit byte, size int) {
	r, size := utf8.DecodeRuneInString(s)
	if d, ok := asciiDigit(r); ok {
		return symDigit, byte(d), size
	}
	switch {
	case loc.decimal != "" && strings.HasPrefix(s, loc.decimal):
		return symDecimal, 0, len(loc.decimal)
	case r == '．':
		return symDecimal, 0, size
	case loc.group != "" && strings.HasPrefix(s, loc.group):
		return symGroup, 0, len(loc.group)
	case r == ' ' || r == ' ' || r == ' ':
		return symGroup, 0, size
	case loc.minus != "" && strings.HasPrefix(s, loc.minus):
		return symMinus, 0, len(loc.minus)
	case r == '-' || r == '－' || r == '−':
		return symMinus, 0, size
	case r == '+' || r == '＋':
		return symPlus, 0, size
	}
	return symOther, 0, size
}

// numberFromDisplay returns the raw value of a number written in the locale, such as
// "1 234,56" in fr-FR for "1234.56": group separators are dropped, the decimal point
// becomes ".", and a sign may lead. It reports false for text with anything else in
// it, or with no digit or decimal point.
func numberFromDisplay(text string, loc *dateLocale) (string, bool) {
	text = strings.TrimSpace(text)
	var b strings.Builder
	started := false
	for len(text) > 0 {
		kind, d, size := numberSymbol(text, loc)
		text = text[size:]
		switch {
		case kind == symDigit:
			b.WriteByte(d)
		case kind == symDecimal:
			b.WriteByte('.')
		case kind == symGroup:
			continue
		case kind == symMinus && !started:
			b.WriteByte('-')
		case kind == symPlus && !started:
		default:
			return "", false
		}
		started = true
	}
	s := b.String()
	return s, strings.ContainsAny(s, "0123456789.")
}

// editNumber is edit for a number.
func (f *InputFormat) editNumber(raw string, a, b int, ins string, paste bool) (string, int, bool) {
	loc := f.locale()
	cur, pos := raw[:a]+raw[b:], a
	signLen := func() int {
		if strings.HasPrefix(cur, "-") {
			return 1
		}
		return 0
	}
	if paste {
		// A paste is a number written in the locale; anything else refuses all of it.
		s, ok := numberFromDisplay(ins, loc)
		if !ok {
			return raw, a, false
		}
		if !strings.HasPrefix(s, "-") {
			pos = max(pos, signLen()) // digits go after the sign
		}
		cur = cur[:pos] + s + cur[pos:]
		pos += len(s)
	} else {
		changed := false
		for len(ins) > 0 {
			kind, d, size := numberSymbol(ins, loc)
			ins = ins[size:]
			switch kind {
			case symDigit, symDecimal:
				c := "."
				if kind == symDigit {
					c = string(d)
				}
				pos = max(pos, signLen())
				cur = cur[:pos] + c + cur[pos:]
				pos++
				changed = true
			case symGroup:
				// The field places the separators itself.
			case symMinus:
				// A minus anywhere toggles the sign, and the caret stays on its digit.
				if !f.opts.Signed {
					return raw, a, false
				}
				if signLen() == 1 {
					cur, pos = cur[1:], max(pos-1, 0)
				} else {
					cur, pos = "-"+cur, pos+1
				}
				changed = true
			case symPlus:
				if signLen() == 1 {
					cur, pos = cur[1:], max(pos-1, 0)
					changed = true
				}
			default:
				return raw, a, false
			}
		}
		if !changed {
			return raw, a, false
		}
	}
	if !f.numberFits(cur) {
		// A second decimal point, a digit past the decimals, or a sign the number
		// does not take.
		return raw, a, false
	}
	return cur, pos, true
}

// normalizeNumber returns the raw number in its shortest form, as the field makes it
// when the user leaves it: "-", "." and "-." become "", "12." becomes "12", "007"
// becomes "7", ".5" becomes "0.5", and a minus zero loses its sign. A value that
// does not fit is kept.
func (f *InputFormat) normalizeNumber(raw string) string {
	if !f.numberFits(raw) {
		return raw
	}
	neg := strings.HasPrefix(raw, "-")
	whole, frac, _ := strings.Cut(strings.TrimPrefix(raw, "-"), ".")
	if whole == "" && frac == "" {
		return ""
	}
	whole = strings.TrimLeft(whole, "0")
	if whole == "" {
		whole = "0"
	}
	s := whole
	if frac != "" {
		s += "." + frac
	}
	if neg && strings.Trim(s, "0.") != "" {
		s = "-" + s
	}
	return s
}

// ---- edits ----

// edit returns the raw value after an edit replaced the raw bytes a to b with ins,
// the raw offset of the caret after it, and whether f takes the edit at all. Typed
// text must fit as a whole; a paste (paste set) is read leniently: a template takes
// the characters that fit and skips the others, and a number takes the text of a
// number in the locale. A refused edit leaves the value as it is.
func (f *InputFormat) edit(raw string, a, b int, ins string, paste bool) (string, int, bool) {
	if f.number {
		return f.editNumber(raw, a, b, ins, paste)
	}
	return f.editTemplate(raw, a, b, ins, paste)
}

// del returns the raw value after the raw bytes from to to are deleted, and the
// caret after it. The characters after them move to earlier slots; in a template
// with slots of different classes, the value is cut where one no longer fits.
func (f *InputFormat) del(raw string, from, to int) (string, int) {
	s := raw[:from] + raw[to:]
	if f.number || !f.fits(raw) {
		return s, from
	}
	cur, _ := f.fitPrefix(nil, s)
	out := string(cur)
	return out, min(from, len(out))
}

// normalize returns the raw value as the field makes it when the user leaves it (see
// normalizeNumber). A template keeps it as it is.
func (f *InputFormat) normalize(raw string) string {
	if f.number {
		return f.normalizeNumber(raw)
	}
	return raw
}

// fromDisplay returns the raw value of a text shown in f's shape, such as
// "(555) 123-4567" or "1,234.5", and whether it is one.
func (f *InputFormat) fromDisplay(text string) (string, bool) {
	if f.number {
		s, ok := numberFromDisplay(text, f.locale())
		return s, ok && f.numberFits(s)
	}
	return f.templateFromDisplay(text)
}

// words returns the raw offsets where the words of the raw value start, then its
// length: the groups of slots between literals of a template, as "555", "123" and
// "4567" of a phone number. A number, and a value that does not fit, is one word.
func (f *InputFormat) words(raw string) []int {
	bs := []int{0}
	if !f.number && f.fits(raw) {
		k := 0
		for i := range raw {
			if k > 0 && f.slots[k] > f.slots[k-1]+1 {
				bs = append(bs, i)
			}
			k++
		}
	}
	return append(bs, len(raw))
}

// wordStep returns the raw offset of the word start before p (dir -1), or of the
// word start or end after it (dir 1), as Ctrl+Left and Ctrl+Right move.
func (f *InputFormat) wordStep(raw string, p, dir int) int {
	bs := f.words(raw)
	if dir < 0 {
		for i := len(bs) - 1; i >= 0; i-- {
			if bs[i] < p {
				return bs[i]
			}
		}
		return 0
	}
	for _, b := range bs {
		if b > p {
			return b
		}
	}
	return len(raw)
}

// wordAt returns the raw bounds of the word at p, as a double-click selects it.
func (f *InputFormat) wordAt(raw string, p int) (int, int) {
	bs := f.words(raw)
	for i := 0; i+1 < len(bs); i++ {
		if p < bs[i+1] || i+2 == len(bs) {
			return bs[i], bs[i+1]
		}
	}
	return 0, len(raw)
}
