package ui

import (
	"fmt"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// carouselMotion is how long a slide takes to come in, and carouselShift how far it
// comes from, in logical pixels.
const (
	carouselMotion = 220 * time.Millisecond
	carouselShift  = 24
)

// CarouselStyle is the look of each part of a carousel. A part's style merges over the
// built-in defaults, then over the theme's Carousel, then over the Styles of the
// instance; see docs/styling.md. Root is the whole carousel, a column of the viewport
// and the controls. Viewport is the box that shows the current slide and clips it while
// it comes in, and Slide the box around the slide. Controls is the row under the
// viewport, NavButton the box around each of the previous and next buttons, PlayPause
// the box around the button that stops and starts AutoPlay (all three are IconButtons,
// styled by the theme's Button), and Dots the row of dots. Each Dot is a
// button that goes to its slide: DotCurrent is the dot of the slide shown, drawn wider
// so that it is not told by its color alone, DotHover a dot under the pointer,
// DotFocusVisible a dot with keyboard focus, and DotDisabled and RootDisabled the dots
// and the carousel when it is disabled.
type CarouselStyle struct {
	Root            Style // the carousel: a column of the viewport and the controls
	RootDisabled    Style // the carousel when it is disabled
	Viewport        Style // the box that shows the current slide, and clips it
	Slide           Style // the box around the slide shown
	Controls        Style // the row of the buttons and the dots
	NavButton       Style // the box around the previous and the next button
	PlayPause       Style // the box around the button that stops and starts AutoPlay
	Dots            Style // the row of dots
	Dot             Style // each dot: a button that goes to its slide
	DotCurrent      Style // the dot of the slide shown
	DotHover        Style // a dot under the pointer
	DotFocusVisible Style // the dot that has keyboard focus
	DotDisabled     Style // each dot when the carousel is disabled
}

// merge returns s with the set fields of o applied on top, part by part.
func (s CarouselStyle) merge(o CarouselStyle) CarouselStyle {
	return CarouselStyle{
		Root:            s.Root.Merge(o.Root),
		RootDisabled:    s.RootDisabled.Merge(o.RootDisabled),
		Viewport:        s.Viewport.Merge(o.Viewport),
		Slide:           s.Slide.Merge(o.Slide),
		Controls:        s.Controls.Merge(o.Controls),
		NavButton:       s.NavButton.Merge(o.NavButton),
		PlayPause:       s.PlayPause.Merge(o.PlayPause),
		Dots:            s.Dots.Merge(o.Dots),
		Dot:             s.Dot.Merge(o.Dot),
		DotCurrent:      s.DotCurrent.Merge(o.DotCurrent),
		DotHover:        s.DotHover.Merge(o.DotHover),
		DotFocusVisible: s.DotFocusVisible.Merge(o.DotFocusVisible),
		DotDisabled:     s.DotDisabled.Merge(o.DotDisabled),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s CarouselStyle) plain() CarouselStyle {
	return CarouselStyle{
		Root:            plainStyle(s.Root),
		RootDisabled:    plainStyle(s.RootDisabled),
		Viewport:        plainStyle(s.Viewport),
		Slide:           plainStyle(s.Slide),
		Controls:        plainStyle(s.Controls),
		NavButton:       plainStyle(s.NavButton),
		PlayPause:       plainStyle(s.PlayPause),
		Dots:            plainStyle(s.Dots),
		Dot:             plainStyle(s.Dot),
		DotCurrent:      plainStyle(s.DotCurrent),
		DotHover:        plainStyle(s.DotHover),
		DotFocusVisible: plainStyle(s.DotFocusVisible),
		DotDisabled:     plainStyle(s.DotDisabled),
	}
}

// carouselDefaults returns the built-in look of a carousel, from the colors of t. A dot
// is a 10 px circle in TextSubtle, 14 px from the next, so that the dots keep the spacing
// of a 24 px target; the current one is a pill in the accent color. Under high contrast
// the dots are the text color and the current one Highlight, a disabled carousel is not
// dimmed, and its dots are GrayText.
func carouselDefaults(t *Theme) CarouselStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	d := CarouselStyle{
		Root:         Style{Direction: Column, Gap: 8},
		RootDisabled: Style{Opacity: 0.5},
		Viewport: Style{Direction: Column, Grow: 1, Radius: t.Radius,
			OverflowX: OverflowHidden, OverflowY: OverflowHidden},
		Slide:     Style{Direction: Column, Grow: 1},
		Controls:  Style{Direction: Row, Align: Center, Justify: Center, Gap: 8},
		NavButton: Style{NoShrink: true},
		PlayPause: Style{NoShrink: true},
		Dots:      Style{Direction: Row, Align: Center, Gap: 14},
		Dot: Style{Width: Px(10), Height: Px(10), NoShrink: true, Radius: 5, Background: t.TextSubtle,
			Transition: 160 * time.Millisecond},
		DotCurrent:      Style{Width: Px(24), Background: t.Accent},
		DotHover:        Style{Background: t.TextMuted},
		DotFocusVisible: Style{Shadows: []Shadow{ring}},
	}
	if t.HighContrast {
		d.RootDisabled = Style{Opacity: 1}
		d.Dot.Background = t.Text
		d.DotHover = Style{Background: t.Accent}
		d.DotDisabled = Style{Background: t.Disabled}
	}
	return d
}

// resolveCarousel returns the look of a carousel: the defaults, merged with the theme's
// Carousel and then the instance's Styles. Under high contrast the overrides are plain
// (see plainStyle).
func resolveCarousel(ctx *Context, inst CarouselStyle) CarouselStyle {
	t := ctx.Theme()
	return layered(ctx, carouselDefaults(t), CarouselStyle.merge, CarouselStyle.plain,
		themeLayer(t.Carousel), instanceLayer(inst))
}

// CarouselElement shows slides one at a time, bound to the index of the slide shown, from
// 0. Under the slides are a previous and a next button and a row of dots, one for each
// slide, that go to that slide. Without Loop the buttons are disabled at the first and the
// last slide; with it they wrap around. Left and Right go to the previous and the next
// slide while focus is on the buttons or the dots. A slide comes in with a short slide and
// fade, which reduced motion turns off.
//
// Only the slide shown is built, as only a tab's selected panel is: a slide that keeps
// state of its own keeps it in signals outside the carousel.
//
// For assistive technology the carousel is a group named by its label and described as a
// carousel (Strings.Carousel), as the WAI-ARIA carousel pattern describes. The slide shown
// is a group named by its position, such as "Slide 2 of 5" (Strings.SlideOf), and the other
// slides are not in the tree. Each dot is a button named "Go to slide 3"
// (Strings.GoToSlide), in a group of the dots in which the current one is selected. When
// the user changes the slide, its position is announced politely; AutoPlay changes it
// without an announcement.
//
// With AutoPlay the controls start with a button that stops and starts it, as WCAG's
// Pause, Stop, Hide asks: a button named "Pause slides" while the slides move and "Play
// slides" while they are stopped (Strings.PauseSlides and Strings.PlaySlides). Its name
// says what it does, so it reports no pressed state as well, as the WAI-ARIA carousel
// pattern asks. It is the first of the controls in the Tab order, where that pattern
// places its rotation control.
type CarouselElement struct {
	index       *Signal[int]
	slides      []Element
	loop        bool
	autoPlay    time.Duration
	name        string // the accessible name, see Label
	key         string
	disabled    bool
	onChange    func(int)
	focusHandle *FocusHandle // see FocusHandle
	tooltip     string       // see Tooltip
	size        sizing
	styles      CarouselStyle
}

// Carousel creates a carousel of slides that reads and writes index, the slide shown,
// from 0. An index out of range shows the nearest slide.
func Carousel(index *Signal[int], slides ...Element) *CarouselElement {
	return &CarouselElement{index: index, slides: slides}
}

func (*CarouselElement) isElement()           {}
func (c *CarouselElement) elementKey() string { return c.key }

// Key identifies the carousel among its siblings (see DivElement.Key).
func (c *CarouselElement) Key(k string) *CarouselElement { c.key = k; return c }

// Label names the carousel for assistive technology, such as "Featured products". Without
// a label it is named by Strings.Carousel.
func (c *CarouselElement) Label(name string) *CarouselElement { c.name = name; return c }

// Loop makes the previous and the next button, the keys and AutoPlay wrap around: after
// the last slide comes the first.
func (c *CarouselElement) Loop() *CarouselElement { c.loop = true; return c }

// AutoPlay moves to the next slide every d, on the window's clock (see Context.After), and
// adds a button that stops and starts it. It waits while the pointer is over the carousel
// or focus is inside it, and stops at the last slide unless the carousel loops. While the
// user asks for reduced motion it starts stopped, and the button shows Play, so that the
// user can still start it. It does not run while the carousel is disabled.
func (c *CarouselElement) AutoPlay(d time.Duration) *CarouselElement { c.autoPlay = d; return c }

// Disabled shows the carousel dimmed, with its slide, and does not let the slide change.
func (c *CarouselElement) Disabled(on bool) *CarouselElement { c.disabled = on; return c }

// OnChange runs fn with the new index after the carousel changes the slide: by the
// buttons, the dots, the keys or AutoPlay. The signal is set first. It does not run when
// the index is set from outside.
func (c *CarouselElement) OnChange(fn func(int)) *CarouselElement { c.onChange = fn; return c }

// FocusHandle lets code focus the carousel: its focus goes to the dot of the slide shown
// (see NewFocusHandle).
func (c *CarouselElement) FocusHandle(h *FocusHandle) *CarouselElement { c.focusHandle = h; return c }

// Tooltip shows s below the carousel after the pointer rests on it. Assistive technology
// reads it as the carousel's description, in place of Strings.Carousel.
func (c *CarouselElement) Tooltip(s string) *CarouselElement { c.tooltip = s; return c }

// The sizing methods give the carousel a size. A carousel fills the width of a column, and
// is as high as its slide and its controls by default; with a Height, the viewport takes
// the room the controls leave. See Sizing in docs/styling.md.
func (c *CarouselElement) Width(l Length) *CarouselElement { c.size.setWidth(l); return c }

// Height sets the height of the carousel.
func (c *CarouselElement) Height(l Length) *CarouselElement { c.size.setHeight(l); return c }

// MinWidth sets the smallest width of the carousel.
func (c *CarouselElement) MinWidth(l Length) *CarouselElement { c.size.setMinWidth(l); return c }

// MinHeight sets the smallest height of the carousel.
func (c *CarouselElement) MinHeight(l Length) *CarouselElement { c.size.setMinHeight(l); return c }

// MaxWidth sets the largest width of the carousel.
func (c *CarouselElement) MaxWidth(l Length) *CarouselElement { c.size.setMaxWidth(l); return c }

// MaxHeight sets the largest height of the carousel.
func (c *CarouselElement) MaxHeight(l Length) *CarouselElement { c.size.setMaxHeight(l); return c }

// Grow makes the carousel take a share of the free space on its parent's main axis.
func (c *CarouselElement) Grow(f float32) *CarouselElement { c.size.setGrow(f); return c }

// GridSpan sets how many columns and rows the carousel covers in its parent's grid.
func (c *CarouselElement) GridSpan(cols, rows int) *CarouselElement {
	c.size.setSpan(cols, rows)
	return c
}

// Styles sets the look of the parts of the carousel, for this instance only. Set fields
// win over the theme's Carousel and the defaults; see CarouselStyle and docs/styling.md.
// Calling Styles again merges with the earlier values.
func (c *CarouselElement) Styles(st CarouselStyle) *CarouselElement {
	c.styles = c.styles.merge(st)
	return c
}

// carouselRun is what a carousel keeps between its renders, outside the signals, so that
// changing it does not render the carousel again.
type carouselRun struct {
	last     int         // the slide shown by the last render; -1 before the first
	dir      float32     // the side the next slide comes in from: 1 from the right, -1 from the left
	dirSet   bool        // dir was set by the change that the render shows
	cancel   func()      // stops the pending step of AutoPlay; nil when none is pending
	step     func()      // the step of AutoPlay, from the latest render
	playing  func() bool // whether AutoPlay plays now: the button and reduced motion
	disposed bool        // the carousel left the tree
	watched  bool        // OnDispose was registered
}

// The choices of a carousel's play button: none yet, stopped, or started.
const (
	carouselFollow = iota
	carouselPaused
	carouselPlaying
)

// stepOf returns the slide after cur (by 1) or before it (by -1) in a carousel of n
// slides, and whether there is one: at the ends it wraps around when loop is set.
func stepOf(cur, by, n int, loop bool) (int, bool) {
	to := cur + by
	switch {
	case to >= 0 && to < n:
		return to, true
	case loop && n > 1:
		return (to + n) % n, true
	}
	return cur, false
}

func (c *CarouselElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		rt := ctx.rt
		s := ctx.Strings()
		st := resolveCarousel(ctx, c.styles)
		run := Local(ctx, "run", &carouselRun{last: -1}).Peek()
		dot := Local(ctx, "dot", NewFocusHandle()).Peek()
		if c.focusHandle != nil {
			dot = c.focusHandle
		}
		n := len(c.slides)
		idx := min(max(c.index.Get(), 0), max(n-1, 0))
		// play is the user's choice of the play button: carouselFollow until it is pressed,
		// then carouselPaused or carouselPlaying. Until then AutoPlay plays unless the user
		// asks for reduced motion.
		play := Local(ctx, "play", carouselFollow)
		run.playing = func() bool {
			switch play.Peek() {
			case carouselPaused:
				return false
			case carouselPlaying:
				return true
			}
			return !rt.sys.ReducedMotion
		}

		name := c.name
		if name == "" {
			name = s.Carousel
		}
		root := Div().asPart("Carousel.Root").Style(st.Root).Role(access.RoleGroup).Label(name).Tooltip(c.tooltip)
		if c.key != "" {
			root.Key(c.key)
		}
		// The carousel is described as one, as aria-roledescription does on the web; a
		// carousel named by that word already says it.
		if c.name != "" && c.tooltip == "" {
			root.Description(s.Carousel)
		}
		if c.disabled {
			root.Style(st.RootDisabled)
		}
		if !run.watched {
			run.watched = true
			ctx.OnDispose(func() {
				run.disposed = true
				if run.cancel != nil {
					run.cancel()
					run.cancel = nil
				}
			})
		}
		if n == 0 {
			return c.size.apply(root)
		}

		// show makes slide i the one shown, coming in from the side dir. A change the user
		// makes is announced; one that AutoPlay makes is not, so that a screen reader is not
		// interrupted while it reads.
		show := func(i int, dir float32, user bool) {
			cur := min(max(c.index.Peek(), 0), n-1)
			if c.disabled || i == cur || i < 0 || i >= n {
				return
			}
			run.dir, run.dirSet = dir, true
			c.index.Set(i)
			if user {
				Announce(s.SlideOf(i+1, n), access.Polite)
			}
			if c.onChange != nil {
				c.onChange(i)
			}
		}
		// move goes to the slide before (by -1) or after (by 1) the one shown, and keeps
		// the focus on the dots when it was there.
		move := func(by int) {
			cur := min(max(c.index.Peek(), 0), n-1)
			to, ok := stepOf(cur, by, n, c.loop)
			if !ok {
				return
			}
			onDot := dot.Focused()
			show(to, float32(by), true)
			if onDot {
				dot.Focus()
			}
		}

		// A change made from outside comes in from the side of the new slide.
		if run.last >= 0 && idx != run.last && !run.dirSet {
			run.dir = 1
			if idx < run.last {
				run.dir = -1
			}
		}
		entering := run.last >= 0
		run.last, run.dirSet = idx, false

		// Only the slide shown is built, so the others are not drawn and not in the
		// accessibility tree. A new key for each slide makes it a new element, which
		// enters; reduced motion turns the entrance off.
		slide := Div().asPart("Carousel.Slide").Key(fmt.Sprintf("slide:%d", idx)).Style(st.Slide).
			Role(access.RoleGroup).Label(s.SlideOf(idx+1, n)).Child(c.slides[idx])
		if entering {
			slide.Enter(Fade().Slide(carouselShift*run.dir, 0), Animation{Duration: carouselMotion})
		}
		root.Child(Div().asPart("Carousel.Viewport").Style(st.Viewport).Child(slide))

		_, canPrev := stepOf(idx, -1, n, c.loop)
		_, canNext := stepOf(idx, 1, n, c.loop)
		nav := func(ic *icons.Icon, label string, on bool, by int) *DivElement {
			return Div().asPart("Carousel.NavButton").Style(st.NavButton).
				Child(IconButton(ic).Label(label).Disabled(c.disabled || !on).OnClick(func() { move(by) }))
		}
		// The dots are a group in which the dot of the slide shown is selected. Only that
		// dot is a Tab stop; the keys move the slide, and the focus with it.
		dots := Div().asPart("Carousel.Dots").Style(st.Dots).Role(access.RoleGroup).Label(s.Slides).SelectionContainer()
		for i := range n {
			d := Div().asPart("Carousel.Dot").Style(st.Dot).Role(access.RoleButton).Label(s.GoToSlide(i + 1)).
				AccessSelected(i == idx)
			if i == idx {
				d.Style(st.DotCurrent)
			}
			switch {
			case c.disabled:
				d.Style(st.DotDisabled)
				d.access().disabled = true
			case i == idx:
				d.FocusVisible(st.DotFocusVisible).FocusHandle(dot).OnClick(func() {})
			default:
				dir := float32(1)
				if i < idx {
					dir = -1
				}
				d.Hover(st.DotHover).FocusVisible(st.DotFocusVisible).TabStop(false).Cursor(CursorPointer).
					OnClick(func() { show(i, dir, true) })
			}
			dots.Child(d)
		}
		controls := Div().asPart("Carousel.Controls").Style(st.Controls)
		if c.autoPlay > 0 && n > 1 {
			// The play button reads the choice, so that a press renders the carousel again.
			play.Get()
			playing := run.playing()
			ic, label, next := icons.Pause, s.PauseSlides, carouselPaused
			if !playing {
				ic, label, next = icons.Play, s.PlaySlides, carouselPlaying
			}
			controls.Child(Div().asPart("Carousel.PlayPause").Style(st.PlayPause).
				Child(IconButton(ic).Label(label).Disabled(c.disabled).
					OnClick(func() { play.Set(next) })))
		}
		controls.
			Child(nav(icons.ChevronLeft, s.PreviousSlide, canPrev, -1)).
			Child(dots).
			Child(nav(icons.ChevronRight, s.NextSlide, canNext, 1))
		if !c.disabled {
			controls.OnKeyDown(func(e *KeyEvent) {
				if e.Mods != 0 {
					return
				}
				switch e.Key {
				case input.KeyLeft:
					move(-1)
				case input.KeyRight:
					move(1)
				default:
					return
				}
				e.StopPropagation()
			})
		}
		root.Child(controls)

		// AutoPlay takes a step every autoPlay, on the window's clock. A step waits while
		// the pointer is over the carousel or focus is inside it, and the timer is not set
		// while the play button or reduced motion stops it. The step reads the latest
		// render's settings.
		self := expandedID(ctx.id)
		run.step = func() {
			cur := min(max(c.index.Peek(), 0), n-1)
			if to, ok := stepOf(cur, 1, n, c.loop); ok {
				show(to, 1, false)
			}
		}
		ctx.ReducedMotion() // a change of the setting renders the carousel again
		auto := c.autoPlay > 0 && !c.disabled && run.playing() && n > 1
		switch {
		case !auto && run.cancel != nil:
			run.cancel()
			run.cancel = nil
		case auto && run.cancel == nil && !run.disposed:
			every := c.autoPlay
			var tick func()
			tick = func() {
				run.cancel = nil
				if run.disposed || !run.playing() {
					return
				}
				if rt.isHovered(self) || rt.focusInside(self) {
					run.cancel = rt.at(rt.now().Add(every), tick).cancel
					return
				}
				run.step()
			}
			run.cancel = rt.at(rt.now().Add(every), tick).cancel
		}
		return c.size.apply(root)
	})
}
