package views

import (
	"errors"
	"regexp"

	"github.com/tyler-builds/jamb-grammars/bash"
	"github.com/tyler-builds/jamb-grammars/golang"
	"github.com/tyler-builds/jamb-grammars/javascript"
	"github.com/tyler-builds/jamb-grammars/json"
	"github.com/tyler-builds/jamb-grammars/python"
	"github.com/tyler-builds/jamb-grammars/rust"
	"github.com/tyler-builds/jamb/markdown"
	"github.com/tyler-builds/jamb/markdown/mdcode"
	"github.com/tyler-builds/jamb/ui"
)

// markdownSample is the document of the Markdown page: one of each element.
const markdownSample = "# Markdown\n\n" +
	"A `markdown.View` shows this text as jamb elements. Each block is *selectable* text, " +
	"with **strong**, *emphasis*, ***both***, ~~strikethrough~~ and `inline code`.  \n" +
	"A line ending in two spaces breaks here, and [links](https://github.com/tyler-builds/jamb) " +
	"open with the system, as autolinks do: <https://go.dev> and www.example.com.\n\n" +
	"## Lists\n\n" +
	"- A bullet list\n- with a nested one:\n  - nested item\n  - another\n- and a third item\n\n" +
	"3. An ordered list\n4. that starts at three\n\n" +
	"- [x] A task that is done\n- [ ] A task to do (click its box)\n- [ ] Another\n\n" +
	"## Quotes and rules\n\n" +
	"> A block quote.\n>\n> > And a quote inside it.\n\n---\n\n" +
	"## Code\n\n" +
	"Fenced blocks are highlighted by the grammar their fence names:\n\n" +
	"```go\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello, world\") // a comment\n}\n```\n\n" +
	"```python\ndef greet(name: str) -> str:\n    return f\"hello, {name}\"\n```\n\n" +
	"```json\n{\"name\": \"jamb\", \"version\": \"0.2.0\", \"pure_go\": true}\n```\n\n" +
	"    An indented block is code too, without colors.\n\n" +
	"## Tables\n\n" +
	"| Element | Accessible as | Aligned |\n|:--|:-:|--:|\n| Heading | heading, with its level | left |\n" +
	"| List | list of items | center |\n| Task | check box | right |\n\n" +
	"## Images\n\n" +
	"![A banner of colored shapes](banner.png)\n\n" +
	"An image the loader refuses shows its alternative text: ![Remote logo](https://example.com/logo.png)\n\n" +
	"## Footnotes and anchors\n\n" +
	"A claim with a footnote.[^1] [Back to the top](#markdown).\n\n" +
	"HTML is shown as it is written: <kbd>Ctrl</kbd>.\n\n" +
	"[^1]: The footnote, listed at the end of the document.\n"

// demoCode colors the code blocks of the Markdown page. One highlighter serves every
// render: it keeps the colors of the blocks it has parsed.
var demoCode = mdcode.Highlight(golang.Grammar, python.Grammar, javascript.Grammar, rust.Grammar, bash.Grammar, json.Grammar)

// Markdown is the page that shows a Markdown document with every element, its code
// highlighted. Its task list is live: a click on a task's box changes the text.
func Markdown() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		src := ui.Local(ctx, "markdown.src", markdownSample)
		view := markdown.View(src.Get()).Code(demoCode).
			Images(func(name string) (*ui.ImageSource, error) {
				if name == "banner.png" {
					return banner, nil // the Dashboard's banner
				}
				return nil, errors.New("the demo loads its own images only")
			}).
			OnTaskToggle(func(i int, done bool) { src.Set(toggleTask(src.Peek(), i, done)) }).
			MaxWidth(ui.Px(860))
		return ui.Div().Grow(1).ScrollY().
			Child(ui.Div().PaddingX(32).PaddingY(28).Child(view))
	})
}

// taskMarker finds the check boxes of task list items: "[ ]" or "[x]" after a list
// marker at the start of a line.
var taskMarker = regexp.MustCompile(`(?m)^[ \t]*(?:[-*+]|\d+[.)])[ \t]+\[([ xX])\]`)

// toggleTask returns src with the i-th task (from 0) marked done or not.
func toggleTask(src string, i int, done bool) string {
	m := taskMarker.FindAllStringSubmatchIndex(src, -1)
	if i < 0 || i >= len(m) {
		return src
	}
	mark := " "
	if done {
		mark = "x"
	}
	return src[:m[i][2]] + mark + src[m[i][3]:]
}
