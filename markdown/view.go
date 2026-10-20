// Package markdown shows Markdown as jamb elements: a read-only view of a document in
// CommonMark, with the GitHub extensions for tables, strikethrough, task lists,
// autolinks and footnotes. The text is parsed with goldmark, and the view builds
// elements from its syntax tree; nothing is rendered to HTML.
//
//	ui.Div().ScrollY().Child(markdown.View(readme).BaseDir(dir).MaxWidth(ui.Px(820)))
//
// The view is a column of blocks and does not scroll by itself: place it in a
// scrolling container, as above. It parses its text once, and again only when the
// text changes, so a view built on each render of its component costs a parse only
// for a new text.
//
// Each block's text is a rich text, which the user can select and copy, one block at
// a time. Headings are headings for assistive technology, with their levels; lists
// are lists of items; links are links, which Tab reaches and Enter follows; a task's
// check box reports whether it is done.
//
// Links open with the system, through ui.Context.OpenURL, which opens http, https and
// mailto URLs only. A link to "#name" moves to the heading whose anchor is name, as
// on GitHub. OnLink replaces all of that with the app's own handler. Images are read
// by a loader: by default, from the local files named relative to BaseDir. The view
// downloads nothing: the default loader refuses URLs and network paths, and an image
// that cannot be shown is replaced by its alternative text. HTML in the text is shown
// as it is written, and HTML comments are left out; no HTML is interpreted.
//
// Code blocks are in the monospace font, and scroll sideways when a line is long.
// They are plain unless the view has a CodeHighlighter (see Code), such as the one of
// the mdcode package, which colors them with the theme's syntax colors.
//
// The words that the view adds, such as the name of a table or of an image without
// alternative text, are the Markdown fields of the window's ui.Strings. The parts of
// the view are styled with Style (see Styles).
package markdown

import (
	"github.com/tyler-builds/jamb/ui"
)

// ViewElement is a Markdown view, made by View and configured with the methods below,
// each of which returns the view so that they chain. It is a ui.Element, so it is
// placed in a tree as it is. It is as wide as its parent and as high as its content
// unless the sizing methods say otherwise.
type ViewElement struct {
	ui.Composite
	src     string
	label   string
	styles  Style
	onLink  func(url string)
	onTask  func(index int, checked bool)
	images  func(src string) (*ui.ImageSource, error)
	baseDir string
	code    CodeHighlighter
	size    sizes // see Width
}

// View returns a view of the Markdown text src.
func View(src string) *ViewElement { return &ViewElement{src: src} }

// Render builds the view's elements. The view is an element: place it in a tree with
// ui.Div().Child or any other container. Its parsed text and its loaded images are
// kept by the element, so give it a Key when its place in the tree may change.
func (v *ViewElement) Render(ctx *ui.Context) ui.Element {
	doc := ui.Local(ctx, "markdown.doc", &docCache{}).Peek().get(v.src)
	images := ui.Local(ctx, "markdown.images", &imageCache{}).Peek()
	// version changes when an image has loaded or code has been highlighted, which
	// happen on other goroutines; reading it renders the view again then.
	version := ui.Local(ctx, "markdown.version", 0)
	version.Get()
	// update runs fn on the UI thread, from any goroutine, and renders the view again.
	update := func(fn func()) {
		ctx.Post(func() {
			fn()
			version.Set(version.Peek() + 1)
		})
	}
	scope := ui.Local(ctx, "markdown.code", &codeScope{}).Peek()
	el := newBuilder(v, ctx, doc, images, update, scope.begin(ctx, v.src)).document()
	scope.done()
	return el
}

// Key gives the view a stable identity among its siblings, so its state (the parsed
// text, the images, the selection of its text) survives reordering.
func (v *ViewElement) Key(k string) *ViewElement { v.SetKey(k); return v }

// Label names the view for assistive technology, which then reads it as a group of
// that name, such as "Preview". Without a label the blocks are read in the window's
// order, with no group around them.
func (v *ViewElement) Label(s string) *ViewElement { v.label = s; return v }

// Styles sets the look of the parts of the view, for this view only. Set fields win
// over the theme's Style and the defaults. Calling Styles again merges with the
// earlier values.
func (v *ViewElement) Styles(st Style) *ViewElement { v.styles = v.styles.merge(st); return v }

// OnLink makes fn handle every link the user follows, with the destination as it is
// written in the text: absolute URLs, relative paths and "#name" anchors alike, and
// images inside links. It replaces the view's own handling (see the package
// documentation). fn runs on the UI thread; it decides what is safe to open.
func (v *ViewElement) OnLink(fn func(url string)) *ViewElement { v.onLink = fn; return v }

// OnTaskToggle makes the check boxes of task list items operable: a click, or Space
// while one has focus, calls fn with the task's index (0 for the first task of the
// document, in the order of the text) and the state it asks for. The view does not
// change the text: fn changes it, and the view shows the new text. Without
// OnTaskToggle the check boxes only show whether each task is done.
func (v *ViewElement) OnTaskToggle(fn func(index int, checked bool)) *ViewElement {
	v.onTask = fn
	return v
}

// Images sets the loader of the view's images, which is given each image's source
// as it is written in the text. It runs on a goroutine of its own, so it may read
// files or wait; the view shows a placeholder until it returns, and the image's
// alternative text if it returns an error. Results are kept by the view for each
// source. The default loader reads local files (see BaseDir).
func (v *ViewElement) Images(load func(src string) (*ui.ImageSource, error)) *ViewElement {
	v.images = load
	return v
}

// BaseDir sets the directory that the default image loader reads relative paths
// from. Without it, they are relative to the working directory. The default loader
// refuses URLs (http, https, data and the other schemes) and network paths
// (\\server\share), so it never reaches the network; it reads regular files of up to
// 32 MB in the formats of ui.ImageFromBytes.
func (v *ViewElement) BaseDir(dir string) *ViewElement { v.baseDir = dir; return v }

// Code sets the highlighter of the view's code blocks. Without one, code blocks are
// plain. See CodeHighlighter, and the mdcode package for one built on the code
// editor's highlighter.
func (v *ViewElement) Code(h CodeHighlighter) *ViewElement { v.code = h; return v }

// The sizing methods give the view's box a size, as they do for the widgets of the ui
// package, and win over the Document part of Styles. See Sizing in docs/styling.md.
func (v *ViewElement) Width(l ui.Length) *ViewElement { v.size.setWidth(l); return v }

// Height sets the height of the view, which is that of its content by default.
func (v *ViewElement) Height(l ui.Length) *ViewElement { v.size.setHeight(l); return v }

// MinWidth sets the smallest width of the view.
func (v *ViewElement) MinWidth(l ui.Length) *ViewElement { v.size.setMinWidth(l); return v }

// MinHeight sets the smallest height of the view.
func (v *ViewElement) MinHeight(l ui.Length) *ViewElement { v.size.setMinHeight(l); return v }

// MaxWidth sets the largest width of the view, such as a comfortable line length.
func (v *ViewElement) MaxWidth(l ui.Length) *ViewElement { v.size.setMaxWidth(l); return v }

// MaxHeight sets the largest height of the view.
func (v *ViewElement) MaxHeight(l ui.Length) *ViewElement { v.size.setMaxHeight(l); return v }

// Grow makes the view take a share of the free space on its parent's main axis.
func (v *ViewElement) Grow(f float32) *ViewElement { v.size.setGrow(f); return v }

// GridSpan sets how many columns and rows the view covers in its parent's grid.
func (v *ViewElement) GridSpan(cols, rows int) *ViewElement { v.size.setSpan(cols, rows); return v }
