package ui

import (
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// partRating is a rating of 2 of 3 stars, disabled or not, with the instance
// style st.
func partRating(st RatingStyle, disabled bool) Element {
	return Div().Column().Align(Start).Child(Rating("Quality", NewSignal(2)).Max(3).Key("q").Disabled(disabled).Styles(st))
}

// starNodes returns the stars of the one rating in the rig, from left to right.
func starNodes(r *accessRig) []*node {
	stars := partNodes(r.rt, "Rating.Star")
	slices.SortFunc(stars, func(a, b *node) int {
		ax, bx := r.rt.nodeRect(a).X, r.rt.nodeRect(b).X
		switch {
		case ax < bx:
			return -1
		case ax > bx:
			return 1
		}
		return 0
	})
	return stars
}

// starIcons returns, for each star from left to right, whether it is drawn filled.
func starIcons(r *accessRig) []bool {
	var out []bool
	for _, s := range starNodes(r) {
		out = append(out, len(s.children) == 1 && s.children[0].icon != nil && s.children[0].icon.icon == icons.StarFilled)
	}
	return out
}

func testRatingParts(t *testing.T, applied map[string]bool) {
	type ratingCase struct {
		part     string
		set      func(*RatingStyle, Style)
		disabled bool
		star     int // the star that draws the part, from 1; 0 is the root
		act      func(*testing.T, *accessRig)
	}
	cases := []ratingCase{
		{part: "Rating.Root", set: func(s *RatingStyle, m Style) { s.Root = m }},
		{part: "Rating.RootFocusVisible", set: func(s *RatingStyle, m Style) { s.RootFocusVisible = m },
			act: func(t *testing.T, r *accessRig) { focusNode(r, partNode(t, r.rt, "Rating.Root", "")) }},
		{part: "Rating.RootDisabled", set: func(s *RatingStyle, m Style) { s.RootDisabled = m }, disabled: true},
		{part: "Rating.Star", set: func(s *RatingStyle, m Style) { s.Star = m }, star: 3},
		{part: "Rating.StarChecked", set: func(s *RatingStyle, m Style) { s.StarChecked = m }, star: 1},
		{part: "Rating.StarHover", set: func(s *RatingStyle, m Style) { s.StarHover = m }, star: 3,
			act: func(t *testing.T, r *accessRig) { hoverNode(r, starNodes(r)[2]) }},
		{part: "Rating.StarDisabled", set: func(s *RatingStyle, m Style) { s.StarDisabled = m }, disabled: true, star: 3},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st RatingStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element { return partRating(st, c.disabled) })
			if c.act != nil {
				c.act(t, r)
			}
			if c.star == 0 {
				wantBackground(t, partNode(t, r.rt, "Rating.Root", ""), c.part, markBG)
				return
			}
			wantTextColor(t, starNodes(r)[c.star-1], c.part, markFG)
		})
	}
}

func TestRatingIsASliderWithAValueText(t *testing.T) {
	v := NewSignal(3)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Rating("Quality", v).Key("q"))
	})
	n := r.stateOf(t, access.RoleSlider, "Quality")
	accesstest.Check(t, n, access.RoleSlider, "Quality", access.StateFocusable)
	if n.Range == nil || n.Range.Value != 3 || n.Range.Min != 0 || n.Range.Max != 5 || n.Range.SmallChange != 1 {
		t.Fatalf("rating range = %+v", n.Range)
	}
	if n.Value != "3 of 5 stars" {
		t.Fatalf("value text %q, want \"3 of 5 stars\"", n.Value)
	}
	if !n.Has(access.ActionSetRangeValue) {
		t.Fatalf("the rating has no set-range-value action: %v", n.Actions)
	}
	if got := starIcons(r); !slices.Equal(got, []bool{true, true, true, false, false}) {
		t.Fatalf("filled stars %v, want the first 3", got)
	}
	r.bridge.Do(n.ID, access.Request{Action: access.ActionSetRangeValue, X: 4.4})
	r.frame()
	if v.Peek() != 4 {
		t.Fatalf("SetValue(4.4) gave %d, want 4", v.Peek())
	}
	r.bridge.Do(n.ID, access.Request{Action: access.ActionSetRangeValue, X: 50})
	r.frame()
	if v.Peek() != 5 {
		t.Fatalf("SetValue(50) gave %d, want the maximum 5", v.Peek())
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestRatingKeys(t *testing.T) {
	v := NewSignal(2)
	var changes []int
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Rating("Quality", v).Max(7).Key("q").OnChange(func(n int) {
			if n != v.Peek() {
				t.Errorf("OnChange(%d) ran before the signal was set", n)
			}
			changes = append(changes, n)
		}))
	})
	r.focusKey(t, "q")
	steps := []struct {
		key  input.Key
		want int
	}{
		{input.KeyRight, 3}, {input.KeyUp, 4}, {input.KeyLeft, 3}, {input.KeyDown, 2},
		{input.KeyEnd, 7}, {input.KeyRight, 7}, {input.KeyHome, 0}, {input.KeyLeft, 0},
		{input.Key5, 5}, {input.Key9, 5}, {input.Key0, 0},
	}
	for _, s := range steps {
		r.press(s.key, 0)
		if v.Peek() != s.want {
			t.Fatalf("key %v gave %d, want %d", s.key, v.Peek(), s.want)
		}
	}
	if want := []int{3, 4, 3, 2, 7, 0, 5, 0}; !slices.Equal(changes, want) {
		t.Fatalf("OnChange ran with %v, want %v (only for changes)", changes, want)
	}
	if got := r.stateOf(t, access.RoleSlider, "Quality").Value; got != "0 of 7 stars" {
		t.Fatalf("value text %q", got)
	}
}

func TestRatingPointerPreviewsSetsAndClears(t *testing.T) {
	v := NewSignal(1)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Align(Start).Child(Rating("Quality", v).Key("q"))
	})
	stars := starNodes(r)
	hoverNode(r, stars[3])
	if got := starIcons(r); !slices.Equal(got, []bool{true, true, true, true, false}) {
		t.Fatalf("the pointer over the 4th star shows %v, want a preview of 4", got)
	}
	if v.Peek() != 1 {
		t.Fatalf("a hover changed the value to %d", v.Peek())
	}
	rc := r.rt.nodeRect(stars[3])
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if v.Peek() != 4 {
		t.Fatalf("a click on the 4th star gave %d", v.Peek())
	}
	// A click on the star of the value clears the rating, and the stars show it.
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if v.Peek() != 0 {
		t.Fatalf("a click on the star of the value gave %d, want 0", v.Peek())
	}
	if got := starIcons(r); slices.Contains(got, true) {
		t.Fatalf("after the clear the stars show %v, want none filled", got)
	}
	// The preview comes back when the pointer moves to another star, and goes when it leaves.
	hoverNode(r, stars[1])
	if got := starIcons(r); !slices.Equal(got, []bool{true, true, false, false, false}) {
		t.Fatalf("the pointer over the 2nd star shows %v", got)
	}
	r.rt.PointerMove(490, 390, 0)
	r.frame()
	if got := starIcons(r); slices.Contains(got, true) {
		t.Fatalf("the preview stayed after the pointer left: %v", got)
	}
}

func TestRatingReadOnlyAndDisabled(t *testing.T) {
	v := NewSignal(3)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Align(Start).
			Child(Rating("Shown", v).ReadOnly().Key("ro")).
			Child(Rating("Off", v).Disabled(true).Key("off"))
	})
	for _, name := range []string{"Shown", "Off"} {
		n := r.stateOf(t, access.RoleSlider, name)
		if n.States.Has(access.StateFocusable) || n.Has(access.ActionSetRangeValue) || n.Value != "3 of 5 stars" || n.Range == nil {
			t.Fatalf("%s: states [%s], actions %v, value %q, range %+v", name, n.States, n.Actions, n.Value, n.Range)
		}
	}
	if !r.stateOf(t, access.RoleSlider, "Shown").States.Has(access.StateReadOnly) {
		t.Fatalf("a read-only rating is not reported read-only")
	}
	if !r.stateOf(t, access.RoleSlider, "Off").States.Has(access.StateDisabled) {
		t.Fatalf("a disabled rating is not reported disabled")
	}
	for _, key := range []string{"ro", "off"} {
		rc := r.rt.nodeRect(keyNode(t, r.rt, key))
		r.clickAt(rc.X+2, rc.Y+rc.H/2)
		if v.Peek() != 3 {
			t.Fatalf("a click on the %s rating changed it to %d", key, v.Peek())
		}
	}
	// A read-only rating is not dimmed; a disabled one is.
	if o := keyNode(t, r.rt, "ro").style.Opacity; o != 0 && o != 1 {
		t.Fatalf("a read-only rating has opacity %v", o)
	}
	if o := keyNode(t, r.rt, "off").style.Opacity; o != 0.5 {
		t.Fatalf("a disabled rating has opacity %v, want 0.5", o)
	}
}

func TestRatingUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Rating = RatingStyle{StarChecked: Style{TextColor: markFG}, Star: Style{TextColor: markFG}}
	v := NewSignal(2)
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return Div().Column().Align(Start).Child(Rating("Quality", v).Max(3).Key("q"))
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	stars := starNodes(r)
	if c := stars[0].style.TextColor; c != th.Accent {
		t.Errorf("high contrast: a filled star is %v, want Highlight %v", c, th.Accent)
	}
	if c := stars[2].style.TextColor; c != th.Text {
		t.Errorf("high contrast: an empty star is %v, want the text color %v", c, th.Text)
	}
	focusNode(r, partNode(t, r.rt, "Rating.Root", ""))
	if len(partNode(t, r.rt, "Rating.Root", "").style.Shadows) == 0 {
		t.Errorf("high contrast: the focused rating has no focus ring")
	}
}

// TestRatingInAField checks that a Field names the rating, gives it its focus
// handle, and checks it with Required, which rejects 0.
func TestRatingInAField(t *testing.T) {
	v := NewSignal(0)
	form := NewForm()
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(form.Field("score", "Your rating", Rating("Rating", v).Key("q").
			Validate(Required[int]())).Required())
	})
	n := r.stateOf(t, access.RoleSlider, "Your rating")
	if !n.States.Has(access.StateRequired) {
		t.Fatalf("the rating in a required field has states [%s]", n.States)
	}
	form.Submit()
	r.frame()
	if n := r.stateOf(t, access.RoleSlider, "Your rating"); !n.States.Has(access.StateInvalid) {
		t.Fatalf("a rating of 0 is not invalid after submit: [%s]", n.States)
	}
	if r.focusedKey() != "q" {
		t.Fatalf("focus is on %q after a failed submit, want the rating", r.focusedKey())
	}
	r.press(input.KeyRight, 0)
	if v.Peek() != 1 {
		t.Fatalf("Right gave %d, want 1", v.Peek())
	}
	if n := r.stateOf(t, access.RoleSlider, "Your rating"); n.States.Has(access.StateInvalid) {
		t.Fatalf("the rating is still invalid with a value: [%s]", n.States)
	}
}

func TestRatingValueTextIsOneStar(t *testing.T) {
	s := englishStrings()
	if got := s.RatingValue(1, 1); got != "1 of 1 star" {
		t.Errorf("RatingValue(1, 1) = %q", got)
	}
	if got := s.RatingValue(3, 5); got != "3 of 5 stars" {
		t.Errorf("RatingValue(3, 5) = %q", got)
	}
}
