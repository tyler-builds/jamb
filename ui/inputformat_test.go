package ui

import (
	"slices"
	"testing"
)

// The formats of these tests: a US phone number, a template with slots of two
// classes, and a signed number with two decimals.
var (
	phoneFormat = TemplateFormat("(999) 999-9999")
	mixedFormat = TemplateFormat("AA-9999")
	moneyFormat = NumberFormat(NumberOptions{Decimals: 2, Signed: true})
)

// numberLocale returns the en-US locale with the symbols of numbers replaced.
func numberLocale(decimal, group string, grouping []int) *dateLocale {
	l := englishLocale()
	l.decimal, l.group, l.grouping = decimal, group, grouping
	return l
}

// useLocale makes l the locale for the rest of the test.
func useLocale(t *testing.T, l *dateLocale) {
	t.Helper()
	prev := localeOverride
	localeOverride = l
	t.Cleanup(func() { localeOverride = prev })
}

func localeDE() *dateLocale { return numberLocale(",", ".", []int{3}) }
func localeFR() *dateLocale { return numberLocale(",", " ", []int{3}) }
func localeHI() *dateLocale { return numberLocale(".", ",", []int{3, 2}) }

// Each row of the design's table of edits: the raw value, the raw selection a to b,
// what is typed or pasted (or, with del, the raw bytes a to b deleted), and the raw
// value, the caret and whether the format takes it. show is what the field shows
// after it, when set.
func TestInputFormatEdits(t *testing.T) {
	useLocale(t, testLocaleEN())
	for _, c := range []struct {
		name   string
		f      InputFormat
		loc    *dateLocale
		raw    string
		a, b   int
		ins    string
		paste  bool
		del    bool
		want   string
		caret  int
		ok     bool
		show   string
		prompt string
	}{
		{name: "type 5 into empty", f: phoneFormat, ins: "5", want: "5", caret: 1, ok: true, show: "(5", prompt: "__) ___-____"},
		{name: "type the third digit", f: phoneFormat, raw: "55", a: 2, b: 2, ins: "5", want: "555", caret: 3, ok: true, show: "(555) ", prompt: "___-____"},
		{name: "type the next literal )", f: phoneFormat, raw: "555", a: 3, b: 3, ins: ")", want: "555", caret: 3},
		{name: "type the next literal space", f: phoneFormat, raw: "555", a: 3, b: 3, ins: " ", want: "555", caret: 3},
		{name: "type the next literal -", f: phoneFormat, raw: "555123", a: 6, b: 6, ins: "-", want: "555123", caret: 6},
		{name: "type the leading literal", f: phoneFormat, ins: "(", want: "", caret: 0},
		{name: "type a literal that is not next", f: phoneFormat, raw: "555", a: 3, b: 3, ins: "-", want: "555", caret: 3},
		{name: "type a letter in a digit slot", f: phoneFormat, raw: "55", a: 2, b: 2, ins: "a", want: "55", caret: 2},
		{name: "type into a full field", f: phoneFormat, raw: "5551234567", a: 10, b: 10, ins: "1", want: "5551234567", caret: 10},
		{name: "insert in the middle", f: phoneFormat, raw: "555123", a: 1, b: 1, ins: "9", want: "5955123", caret: 2, ok: true, show: "(595) 512-3"},
		{name: "insert in the middle of a full field", f: phoneFormat, raw: "5551234567", a: 1, b: 1, ins: "9", want: "5551234567", caret: 1},
		{name: "insert that no longer fits its slot", f: mixedFormat, raw: "AB12", a: 0, b: 0, ins: "C", want: "AB12", caret: 0},
		{name: "backspace after the literals", f: phoneFormat, raw: "555", a: 2, b: 3, del: true, want: "55", caret: 2, ok: true, show: "(55"},
		{name: "delete before a literal", f: phoneFormat, raw: "5551234567", a: 3, b: 4, del: true, want: "555234567", caret: 3, ok: true, show: "(555) 234-567"},
		{name: "type over a selection", f: phoneFormat, raw: "5551234567", a: 3, b: 6, ins: "9", want: "55594567", caret: 4, ok: true, show: "(555) 945-67"},
		{name: "paste the shape", f: phoneFormat, ins: "(555) 123-4567", paste: true, want: "5551234567", caret: 10, ok: true, show: "(555) 123-4567"},
		{name: "paste with dots", f: phoneFormat, ins: "555.123.4567", paste: true, want: "5551234567", caret: 10, ok: true},
		{name: "paste that overflows", f: phoneFormat, ins: "555123456789", paste: true, want: "5551234567", caret: 10, ok: true},
		{name: "paste into the middle drops the overflow", f: phoneFormat, raw: "55512345", a: 3, b: 3, ins: "999", paste: true, want: "5559912345", caret: 5, ok: true},
		{name: "paste with nothing that fits", f: phoneFormat, raw: "55", a: 2, b: 2, ins: "abc", paste: true, want: "55", caret: 2},
		{name: "paste into a full field", f: phoneFormat, raw: "5551234567", a: 10, b: 10, ins: "1", paste: true, want: "5551234567", caret: 10},
		{name: "paste full-width digits", f: phoneFormat, ins: "５５５", paste: true, want: "555", caret: 3, ok: true},
		{name: "delete shifts a digit into a letter slot", f: mixedFormat, raw: "AB1234", a: 1, b: 2, del: true, want: "A", caret: 1, ok: true, show: "A"},
		{name: "type full-width digits", f: phoneFormat, ins: "５５５", want: "555", caret: 3, ok: true},

		{name: "number 1,234 then 5", f: moneyFormat, raw: "1234", a: 4, b: 4, ins: "5", want: "12345", caret: 5, ok: true, show: "12,345"},
		{name: "number 1,|234 backspace", f: moneyFormat, raw: "1234", a: 0, b: 1, del: true, want: "234", caret: 0, ok: true, show: "234"},
		{name: "decimal key", f: moneyFormat, raw: "12", a: 2, b: 2, ins: ".", want: "12.", caret: 3, ok: true, show: "12."},
		{name: "decimal key of de-DE", f: moneyFormat, loc: localeDE(), raw: "1234", a: 4, b: 4, ins: ",", want: "1234.", caret: 5, ok: true, show: "1.234,"},
		{name: "full-width decimal point", f: moneyFormat, raw: "1", a: 1, b: 1, ins: "．", want: "1.", caret: 2, ok: true},
		{name: "second decimal point", f: moneyFormat, raw: "1.5", a: 3, b: 3, ins: ".", want: "1.5", caret: 3},
		{name: "digit past the decimals", f: moneyFormat, raw: "1.25", a: 4, b: 4, ins: "1", want: "1.25", caret: 4},
		{name: "decimal point of a whole number", f: NumberFormat(NumberOptions{}), raw: "1", a: 1, b: 1, ins: ".", want: "1", caret: 1},
		{name: "typed group separator", f: moneyFormat, raw: "1234", a: 1, b: 1, ins: ",", want: "1234", caret: 1},
		{name: "typed group separator of de-DE", f: moneyFormat, loc: localeDE(), raw: "1234", a: 1, b: 1, ins: ".", want: "1234", caret: 1},
		{name: "minus adds the sign", f: moneyFormat, raw: "1234", a: 2, b: 2, ins: "-", want: "-1234", caret: 3, ok: true, show: "-1,234"},
		{name: "minus removes the sign", f: moneyFormat, raw: "-1234", a: 3, b: 3, ins: "-", want: "1234", caret: 2, ok: true},
		{name: "full-width minus", f: moneyFormat, raw: "5", a: 1, b: 1, ins: "－", want: "-5", caret: 2, ok: true},
		{name: "plus clears the sign", f: moneyFormat, raw: "-5", a: 2, b: 2, ins: "+", want: "5", caret: 1, ok: true},
		{name: "plus without a sign", f: moneyFormat, raw: "5", a: 1, b: 1, ins: "+", want: "5", caret: 1},
		{name: "minus of an unsigned number", f: NumberFormat(NumberOptions{}), raw: "5", a: 1, b: 1, ins: "-", want: "5", caret: 1},
		{name: "minus into empty", f: moneyFormat, ins: "-", want: "-", caret: 1, ok: true, show: "-"},
		{name: "digit before the sign goes after it", f: moneyFormat, raw: "-5", a: 0, b: 0, ins: "3", want: "-35", caret: 2, ok: true},
		{name: "letter in a number", f: moneyFormat, raw: "5", a: 1, b: 1, ins: "x", want: "5", caret: 1},
		{name: "paste fr-FR", f: moneyFormat, loc: localeFR(), ins: "1 234,56", paste: true, want: "1234.56", caret: 7, ok: true, show: "1 234,56"},
		{name: "paste en-US with groups and sign", f: moneyFormat, ins: "-1,234.5", paste: true, want: "-1234.5", caret: 7, ok: true},
		{name: "paste that is not a number", f: moneyFormat, raw: "1", a: 1, b: 1, ins: "12abc", paste: true, want: "1", caret: 1},
		{name: "paste with too many decimals", f: moneyFormat, ins: "1.234", paste: true, want: "", caret: 0},
		{name: "paste over a selection", f: moneyFormat, raw: "1234", a: 1, b: 3, ins: "99", paste: true, want: "1994", caret: 3, ok: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.loc != nil {
				useLocale(t, c.loc)
			}
			var got string
			var caret int
			ok := true
			if c.del {
				got, caret = c.f.del(c.raw, c.a, c.b)
			} else {
				got, caret, ok = c.f.edit(c.raw, c.a, c.b, c.ins, c.paste)
			}
			if got != c.want || caret != c.caret || ok != c.ok {
				t.Fatalf("got %q, caret %d, ok %v; want %q, caret %d, ok %v", got, caret, ok, c.want, c.caret, c.ok)
			}
			if c.show != "" {
				if s := c.f.Display(got); s != c.show {
					t.Errorf("the field shows %q, want %q", s, c.show)
				}
			}
			if c.prompt != "" {
				if p := c.f.prompt(got); p != c.prompt {
					t.Errorf("the prompt is %q, want %q", p, c.prompt)
				}
			}
		})
	}
}

// The caret moves one slot at a time in the raw value, and each raw offset is
// drawn just before its slot, after the literals before it: 0 at the start and
// the end at the end. A click inside a literal run goes to the slot after it.
func TestInputFormatOffsets(t *testing.T) {
	useLocale(t, testLocaleEN())
	p := phoneFormat.present("5551234567")
	if p.shown != "(555) 123-4567" {
		t.Fatalf("shown %q", p.shown)
	}
	var shown []int
	for r := 0; r <= 10; r++ {
		shown = append(shown, p.toShown(r))
	}
	if want := []int{0, 2, 3, 6, 7, 8, 10, 11, 12, 13, 14}; !slices.Equal(shown, want) {
		t.Errorf("toShown gives %v, want %v", shown, want)
	}
	// Clicks: before "(", after it, inside ") ", and between "3" and "-".
	for q, want := range map[int]int{0: 0, 1: 0, 4: 3, 5: 3, 6: 3, 9: 6, 10: 6, 14: 10} {
		if got := p.fromShown(q); got != want {
			t.Errorf("fromShown(%d) = %d, want %d", q, got, want)
		}
	}
	// A selection ends after its last slot, before the literals after it.
	for r, want := range map[int]int{0: 0, 3: 4, 6: 9, 10: 14} {
		if got := p.endShown(r); got != want {
			t.Errorf("endShown(%d) = %d, want %d", r, got, want)
		}
	}
	if got := p.shown[p.toShown(3):p.endShown(6)]; got != "123" {
		t.Errorf("the selection 3 to 6 shows %q", got)
	}
	// "(555) |" is the caret after three digits.
	if got := phoneFormat.present("555").toShown(3); got != len("(555) ") {
		t.Errorf("the caret after three digits is at %d, want after the literals", got)
	}
}

// toShown and fromShown are inverses for every raw offset, in templates, numbers
// in each locale, and values shown as they are.
func TestInputFormatRoundTrip(t *testing.T) {
	cases := []struct {
		f   InputFormat
		loc *dateLocale
		raw string
	}{
		{phoneFormat, nil, "5551234567"},
		{phoneFormat, nil, "555"},
		{phoneFormat, nil, "hello"},
		{mixedFormat, nil, "éB12"},
		{TemplateFormat("**/**"), nil, "日本語"},
		{moneyFormat, nil, "-1234567.25"},
		{moneyFormat, localeDE(), "-1234567.25"},
		{moneyFormat, localeFR(), "1234567.5"},
		{moneyFormat, localeHI(), "-12345678"},
		{moneyFormat, numberLocale(".", ",", []int{3}), "12."},
		{moneyFormat, nil, "1,234"},
	}
	for _, c := range cases {
		l := c.loc
		if l == nil {
			l = testLocaleEN()
		}
		useLocale(t, l)
		p := c.f.present(c.raw)
		for r := 0; r <= len(c.raw); r++ {
			if r < len(c.raw) && c.raw[r]&0xC0 == 0x80 {
				continue // inside a character
			}
			if got := p.fromShown(p.toShown(r)); got != r {
				t.Errorf("%q in %q: raw %d shows at %d, which maps back to %d", c.raw, p.shown, r, p.toShown(r), got)
			}
		}
		if p.toShown(len(c.raw)) != len(p.shown) {
			t.Errorf("%q: the end of the raw value is not the end of %q", c.raw, p.shown)
		}
	}
}

// Numbers are grouped as the locale says.
func TestInputFormatGrouping(t *testing.T) {
	for _, c := range []struct {
		name string
		loc  *dateLocale
		f    InputFormat
		raw  string
		want string
	}{
		{"en-US", testLocaleEN(), moneyFormat, "1234567", "1,234,567"},
		{"en-US small", testLocaleEN(), moneyFormat, "123", "123"},
		{"en-US four", testLocaleEN(), moneyFormat, "1234", "1,234"},
		{"en-US signed decimals", testLocaleEN(), moneyFormat, "-1234.5", "-1,234.5"},
		{"de-DE", localeDE(), moneyFormat, "-1234567.25", "-1.234.567,25"},
		{"fr-FR", localeFR(), moneyFormat, "1234567", "1 234 567"},
		{"hi-IN", localeHI(), moneyFormat, "12345678", "1,23,45,678"},
		{"one group", numberLocale(".", ",", []int{3, 0}), moneyFormat, "1234567", "1234,567"},
		{"no grouping", testLocaleEN(), NumberFormat(NumberOptions{NoGrouping: true}), "1234567", "1234567"},
		{"minus sign of the locale", func() *dateLocale { l := testLocaleEN(); l.minus = "−"; return l }(), moneyFormat, "-12", "−12"},
	} {
		useLocale(t, c.loc)
		if got := c.f.Display(c.raw); got != c.want {
			t.Errorf("%s: %q shows as %q, want %q", c.name, c.raw, got, c.want)
		}
	}
}

func TestParseGrouping(t *testing.T) {
	for s, want := range map[string][]int{"3;0": {3}, "3;2;0": {3, 2}, "3": {3, 0}, "0": {0}, "3;2": {3, 2, 0}} {
		if got, ok := parseGrouping(s); !ok || !slices.Equal(got, want) {
			t.Errorf("parseGrouping(%q) = %v, %v; want %v", s, got, ok, want)
		}
	}
	for _, s := range []string{"", "x", "3;-1", "3;;0"} {
		if _, ok := parseGrouping(s); ok {
			t.Errorf("parseGrouping(%q) read a grouping", s)
		}
	}
}

// Leaving the field puts a number in its shortest form, and keeps a template.
func TestInputFormatNormalize(t *testing.T) {
	for raw, want := range map[string]string{
		"-": "", ".": "", "-.": "", "12.": "12", "007": "7", ".5": "0.5", "-0": "0", "-00.0": "0.0",
		"-007.50": "-7.50", "0": "0", "1234": "1234", "": "", "1,234": "1,234",
	} {
		if got := moneyFormat.normalize(raw); got != want {
			t.Errorf("normalize(%q) = %q, want %q", raw, got, want)
		}
	}
	if got := phoneFormat.normalize("555"); got != "555" {
		t.Errorf("a template normalized %q to %q", "555", got)
	}
}

// Complete passes the empty value and complete ones, and rejects a partly filled
// template, a number that does not parse, and values that do not fit.
func TestInputFormatComplete(t *testing.T) {
	for _, c := range []struct {
		f    InputFormat
		raw  string
		pass bool
	}{
		{phoneFormat, "", true},
		{phoneFormat, "5551234567", true},
		{phoneFormat, "555", false},
		{phoneFormat, "555123456a", false},
		{phoneFormat, "55512345678", false},
		{moneyFormat, "", true},
		{moneyFormat, "-12.5", true},
		{moneyFormat, "12.", true},
		{moneyFormat, "-", false},
		{moneyFormat, ".", false},
		{moneyFormat, "-.", false},
		{moneyFormat, "1.234", false},
		{moneyFormat, "1,234", false},
		{NumberFormat(NumberOptions{}), "-1", false},
		{InputFormat{}, "anything", true},
	} {
		err := c.f.Complete("Incomplete")(c.raw)
		if (err == nil) != c.pass {
			t.Errorf("Complete(%q) = %v, want pass %v", c.raw, err, c.pass)
		}
		if err != nil && err.Error() != "Incomplete" {
			t.Errorf("Complete(%q) says %q", c.raw, err.Error())
		}
	}
}

// A value the app sets that does not fit is shown as it is, one to one, with no
// prompt; typing into it is refused, and deleting from it works.
func TestInputFormatValueThatDoesNotFit(t *testing.T) {
	useLocale(t, testLocaleEN())
	p := phoneFormat.present("hello")
	if p.shown != "hello" || p.toShown(3) != 3 || p.fromShown(2) != 2 {
		t.Errorf("a value that does not fit shows as %q, mapping 3 to %d", p.shown, p.toShown(3))
	}
	if got := phoneFormat.prompt("hello"); got != "" {
		t.Errorf("a value that does not fit has the prompt %q", got)
	}
	if _, _, ok := phoneFormat.edit("hello", 5, 5, "5", false); ok {
		t.Error("typing into a value that does not fit was taken")
	}
	if got, caret := phoneFormat.del("hello", 0, 1); got != "ello" || caret != 0 {
		t.Errorf("deleting from a value that does not fit gave %q, caret %d", got, caret)
	}
	if got := moneyFormat.Display("12abc"); got != "12abc" {
		t.Errorf("a number that does not fit shows as %q", got)
	}
}

// The prompts are the slots not filled, after the literals that show.
func TestInputFormatPrompt(t *testing.T) {
	for raw, want := range map[string]string{
		"": "(___) ___-____", "5": "__) ___-____", "555": "___-____", "5551234567": "",
	} {
		if got := phoneFormat.prompt(raw); got != want {
			t.Errorf("prompt(%q) = %q, want %q", raw, got, want)
		}
	}
	if got := moneyFormat.prompt(""); got != "" {
		t.Errorf("a number has the prompt %q", got)
	}
}

// A word is a group of slots between literals; a number is one word.
func TestInputFormatWords(t *testing.T) {
	raw := "5551234567"
	if got := phoneFormat.words(raw); !slices.Equal(got, []int{0, 3, 6, 10}) {
		t.Errorf("words %v", got)
	}
	for _, c := range []struct{ p, dir, want int }{{0, 1, 3}, {3, 1, 6}, {7, 1, 10}, {10, 1, 10}, {10, -1, 6}, {6, -1, 3}, {5, -1, 3}, {2, -1, 0}, {0, -1, 0}} {
		if got := phoneFormat.wordStep(raw, c.p, c.dir); got != c.want {
			t.Errorf("wordStep(%d, %d) = %d, want %d", c.p, c.dir, got, c.want)
		}
	}
	for p, want := range map[int][2]int{0: {0, 3}, 4: {3, 6}, 6: {6, 10}, 10: {6, 10}} {
		if a, b := phoneFormat.wordAt(raw, p); a != want[0] || b != want[1] {
			t.Errorf("wordAt(%d) = %d, %d; want %v", p, a, b, want)
		}
	}
	if got := moneyFormat.words("-1234.5"); !slices.Equal(got, []int{0, 7}) {
		t.Errorf("the words of a number are %v, want the whole text", got)
	}
}

// The template's syntax: the four slots, an escaped literal, and a template without
// slots, which panics.
func TestTemplateFormatSyntax(t *testing.T) {
	f := TemplateFormat(`\9A#*-9`)
	if len(f.slots) != 4 {
		t.Fatalf("%d slots, want 4", len(f.slots))
	}
	for _, c := range []struct {
		ins string
		ok  bool
	}{{"x1?5", true}, {"11?5", false}, {"x_?5", false}, {"x1\t5", false}, {"éé日5", true}} {
		if _, _, ok := f.edit("", 0, 0, c.ins, false); ok != c.ok {
			t.Errorf("typing %q: ok %v, want %v", c.ins, ok, c.ok)
		}
	}
	if got := f.Display("x1?5"); got != "9x1?-5" {
		t.Errorf("shows %q", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("a template without slots did not panic")
		}
	}()
	TemplateFormat("(-)")
}

// The text of a number in the locale reads back as the raw value.
func TestInputFormatFromDisplay(t *testing.T) {
	useLocale(t, localeDE())
	if got, ok := moneyFormat.fromDisplay("-1.234,5"); !ok || got != "-1234.5" {
		t.Errorf("fromDisplay de-DE = %q, %v", got, ok)
	}
	if _, ok := moneyFormat.fromDisplay("1,2,3"); ok {
		t.Error("two decimal points read as a number")
	}
	if got, ok := phoneFormat.fromDisplay("(555) 12"); !ok || got != "55512" {
		t.Errorf("fromDisplay of the start of a phone number = %q, %v", got, ok)
	}
	// A literal digit in its place is not a slot character.
	f := TemplateFormat("+1 999")
	if got, _, ok := f.edit("", 0, 0, "+1 555", true); !ok || got != "555" {
		t.Errorf("pasting the shape with a literal digit gave %q, %v", got, ok)
	}
}
