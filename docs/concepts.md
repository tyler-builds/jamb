# Concepts

The element tree, the builder API, state and keys, work on the UI thread, and errors.

## A view

```go
func (v *ProjectsView) Render(ctx *ui.Context) ui.Element {
    t := ctx.Theme()
    return ui.Div().Row().Height(ui.Px(52)).Align(ui.Center).Justify(ui.SpaceBetween).PaddingX(16).
        Child(ui.Text("Projects").FontSize(22).Bold()).
        Child(ui.Button("New Project").Icon(icons.Plus).OnClick(v.createProject)).
        Background(t.Surface)
}
```

## API guide

Everything is plain, typed Go. Chained methods configure elements.

```go
ui.Div()                       // flex container; children stack vertically by default
  .Row() / .Column()           // main axis
  .Wrap()                      // flow children onto more lines (chips, tags, galleries)
  .Gap(8).Padding(16).PaddingX(12).PaddingY(6).Margin(4).MarginTop(8)
  .Width(ui.Px(240)).Height(ui.Full).Size(w, h).MinWidth(...).MaxWidth(ui.Px(1100))
  .Grow(1).Shrink(0).Basis(ui.Px(0))          // Basis(0)+Grow(1) = equal columns
  .Align(ui.Center).Justify(ui.SpaceBetween).Center().AlignSelf(ui.End)
  .Absolute().Top(8).Right(8)  /  .Inset(0)   // positioned relative to the parent
  .Display(false)              // hidden, state kept: no space, not drawn, not focusable (see below)
  .ZIndex(1).ClipContent().ScrollY().ScrollX()  // overflowing containers get scrollbars:
                               // drag the thumb, click the track to page by 90%;
                               // horizontal bars too; presses never click content below
                               // with a Radius, content is clipped to the rounded corners
  .Background(c).Border(1, c).Radius(8).RadiusCorners(ui.Corners{...})
  .BorderEdges(ui.Edges{Bottom: 2}, c)     // a width per side (top, right, bottom, left); see Borders
  .BackgroundGradient(ui.LinearGradient(135, from, to))   // replaces Background; see Gradients
  .Shadow(ui.ShadowMd).Opacity(0.5).Cursor(ui.CursorPointer)
  // Shadow{Spread: 2, Color: c} (no blur, no offset) is a crisp ring around the box
  .TextColor(c).FontSize(13).FontWeight(ui.FontMedium)   // inherited by descendants
  .Hover(ui.Style{...}).Pressed(ui.Style{...}).Focused(ui.Style{...})
  .FocusVisible(ui.Style{...}).FocusWithin(ui.Style{...})  // focus rings; see Focus and the keyboard
  .TabIndex(2).TabStop(false)                            // place in the Tab order
  .Transition(120 * time.Millisecond)                    // ease colors, Opacity, Offset and Scale
  .Offset(0, -4).Scale(1.02)                             // paint-only: moved or scaled after layout
  .Enter(ui.Fade().Slide(0, -6), ui.Animation{Duration: 140 * time.Millisecond}) // on first appearance
  .Exit(ui.Fade(), ui.Animation{Duration: 90 * time.Millisecond})               // ghost while leaving
  .When(cond, func(d *ui.DivElement) { ... })            // conditional styling
  .OnClick(fn).OnDoubleClick(fn).OnMouseEnter(fn).OnMouseLeave(fn)
  .OnMouseDown(func(e *ui.MouseEvent){}).OnScroll(...).OnPinch(...).OnKeyDown(...).OnTextInput(...)
  .Overlay(ui.BelowStart).OverlayGap(4)    // Below|Above Start|Center|End, Right|Left Start|Center|End, OverCenter;
                               // float next to the parent: never clipped, drawn on top,
                               // hit first; flips to the other side if it doesn't fit
  .OnClickOutside(fn)          // any button pressed outside (an Overlay's parent counts as inside);
                               // on an Overlay, Escape also runs fn (see Actions and key bindings)
  .Tooltip("Save")             // label below after 500 ms of hover; every control has .Tooltip too
  .Key("stable-id").Child(el).Children(els...)

ui.Text("hi").FontSize(15).Semibold().Italic().TextColor(c).Truncate().MaxLines(2).Align(ui.Center)
ui.Textf("Count: %d", n)
ui.Text("Total").Font("Inter")                    // a family registered with app.New().Font (below); .Mono() for code
ui.Icon(icons.Folder).Size(16).Color(c)          // color defaults to the inherited text color
                                                 // icons.All lists every icon; see the icon list below
ui.Image(src).Size(ui.Px(34), ui.Px(34)).Radius(17).Fit(ui.FitCover)
                                                 // src := ui.MustImage(data): PNG, JPEG, WebP or GIF; an animated
                                                 // GIF plays while drawn, and stays on its first frame under reduced motion
ui.Button("Save").Icon(icons.Check).Secondary().Small().OnClick(fn)   // Primary|Secondary|Ghost|Danger; Small|Large
ui.IconButton(icons.Bell)
ui.TextInput(signal).Placeholder("Search…").Icon(icons.Search).OnSubmit(fn).OnChange(fn)
ui.TextArea(signal).Placeholder("Start writing…").MinRows(10)    // multi-line editor
  // both: .Borderless() .AutoFocus() .FocusHandle(h) .Style(ui.Style{FontSize: 28})
  // both: .ReadOnly(true) (focusable, selectable, copyable; not editable) and .Disabled(true)
  // TextInput also: .Small() (28 px, like Button.Small) and .Large() (40 px, 16 px text); so do
  // Select, Combobox, NumberInput, DatePicker and TimePicker, so a row of them lines up
  // Style on a field sizes and decorates its frame (Width, Height, Padding,
  // Radius, Background, Border...); font fields style the text inside.
  // Styles(ui.FieldStyle{...}) sets the frame's parts (Frame, FrameFocusWithin,
  // Text, Placeholder, Selection, Leading, Trailing), and Leading(el) and
  // Trailing(el) put an element inside the frame; see the text frame in Styling.
ui.Divider(), ui.Spacer(), ui.If(cond, el), ui.Map(items, fn)

// Focus from code (shortcuts, focusing a field you just created):
h := ui.NewFocusHandle()
ui.TextInput(query).FocusHandle(h) // every interactive control takes one: Button, Select, Tabs, DataGrid...
h.Focus()    // takes effect on the next frame, after the change that created the field
saved := ctx.FocusedHandle()  // whatever has focus now (or nothing), to restore later
saved.FocusNow()              // focuses it at once; does nothing and returns false if its element is gone
ui.Div().FocusWithin(ui.Style{BorderColor: c})  // style while focus is inside
ui.Div().CapturePointer()                       // keep getting moves while dragging
```

`Display(false)` hides an element as CSS `display: none` does, but keeps it in the
tree. Its component instances, Local state, signals, text and selection, scroll
positions and lists survive, and `Display(true)` shows it again as it was, without
a rebuild unless its inputs changed. While it is hidden, the element and its subtree
take no space in layout, are not painted or hit, are skipped by Tab and cannot take
focus, are absent from the accessibility tree, and run no repeating animation or
frame. A focused element that becomes hidden hands focus to its nearest focusable
ancestor, or to nothing when there is none. `AccessHidden()` is a different thing: it
only leaves the element out of the accessibility tree. The element is still drawn,
laid out and reachable with the mouse and keyboard. Use `Display` for content that
should not be seen or reached, and `Hidden` for content that is seen but not
announced.

## State and re-rendering

State and re-render boundaries:

```go
count := ui.NewSignal(0)
ui.Component(func(ctx *ui.Context) ui.Element {      // re-runs only when count changes
    return ui.Button(fmt.Sprintf("Count: %d", count.Get())).
        OnClick(func() { count.Set(count.Peek() + 1) })
})

// State that belongs to one component (a draft, an open/closed flag):
ui.Component(func(ctx *ui.Context) ui.Element {
    open := ui.Local(ctx, "open", false)            // *ui.Signal[bool], lives with the component
    ...
})
```

`ui.Local` finds state by name, not by call order, so it is safe inside
conditions and loops. It lives as long as the component and starts fresh if
the component is removed and added again.

A value derived from signals is a `ui.Computed`. It remembers its result and
runs its function again only when it is read after a signal it read changed;
`Get` subscribes the rendering component, as a signal's does, and `Peek` does
not. Create it once, like a signal, and read it on the UI thread (in a render,
an event handler or a `Post` function):

```go
total := ui.NewComputed(func() int {           // *ui.Computed[int]
    sum := 0
    for _, item := range cart.Get() {
        sum += item.Price
    }
    return sum
})
ui.Textf("Total: %d", total.Get())             // re-renders when cart changes
```

Treat a signal's value as immutable. To change a slice or map, build a new
one and `Set` it. Changing it in place notifies nobody.

## Keys and identity

Identity: every element's state (hover, focus, scroll position, the text
input's caret, a component's subtree) is tied to its position in the tree.
That position is the element's key if it has one, otherwise its index among
its siblings, combined with its element type.

- Give items in dynamic lists a `.Key(...)` derived from a stable ID. Then
  reordering, inserting or removing items keeps each item's state with that
  item.
- Without keys, state stays at the position. Inserting at the front shifts
  state to the next item.
- Replacing an element with one of a different type always starts fresh.
- Duplicate keys among siblings print a warning and are kept apart.

`ui/incremental_test.go` pins all of this down.

## Work on the UI thread

`rt.Post(fn)` runs fn on the UI thread at the next frame; call it from
background goroutines to hand them their results. A component that holds its
`ui.Context` can call `ctx.Post(fn)` for the same thing.

`ctx.After(d, fn)` runs fn on the UI thread once d has passed on the window's
clock, and returns a function that cancels it. A test moves that clock with
uitest's `Advance` (see [Testing your app](testing.md#time)). Cancel and start it
again on each change to wait for a pause, such as in typing.

## Errors

Errors explain what failed and how to fix it. For example, `ui.Hex("#12")`
panics with `ui.Hex("#12"): expected #rgb, #rrggbb or #rrggbbaa`, and a
missing Root says to set `app.Window.Root`.
