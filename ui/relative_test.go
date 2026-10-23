package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// relNow is the time the relative tests treat as now: October 23, 2026 at noon.
var relNow = time.Date(2026, time.October, 23, 12, 0, 0, 0, time.Local)

// relCases are the times the relative tests format, each with its text in English,
// Italian, Simplified Chinese, Traditional Chinese (Taiwan) and Hong Kong. The English
// and Italian forms are CLDR's, with the count in digits.
var relCases = []struct {
	name string
	t    time.Time
	want [5]string // en, it, zh-CN, zh-TW, zh-HK
}{
	{"under a minute ago", relNow.Add(-30 * time.Second), [5]string{"now", "ora", "现在", "現在", "現在"}},
	{"under a minute ahead", relNow.Add(30 * time.Second), [5]string{"now", "ora", "现在", "現在", "現在"}},
	{"a minute ago", relNow.Add(-time.Minute), [5]string{"1 minute ago", "1 minuto fa", "1分钟前", "1 分鐘前", "1 分鐘前"}},
	{"45 minutes ago", relNow.Add(-45 * time.Minute), [5]string{"45 minutes ago", "45 minuti fa", "45分钟前", "45 分鐘前", "45 分鐘前"}},
	{"an hour ago", relNow.Add(-time.Hour), [5]string{"1 hour ago", "1 ora fa", "1小时前", "1 小時前", "1 小時前"}},
	{"5 hours ago", relNow.Add(-5 * time.Hour), [5]string{"5 hours ago", "5 ore fa", "5小时前", "5 小時前", "5 小時前"}},
	{"yesterday", time.Date(2026, time.October, 22, 9, 0, 0, 0, time.Local), [5]string{"yesterday", "ieri", "昨天", "昨天", "昨日"}},
	{"3 days ago", time.Date(2026, time.October, 20, 12, 0, 0, 0, time.Local), [5]string{"3 days ago", "3 giorni fa", "3天前", "3 天前", "3 日前"}},
	{"2 weeks ago", time.Date(2026, time.October, 9, 12, 0, 0, 0, time.Local), [5]string{"2 weeks ago", "2 settimane fa", "2周前", "2 週前", "2 星期前"}},
	{"a month ago", time.Date(2026, time.September, 23, 12, 0, 0, 0, time.Local), [5]string{"1 month ago", "1 mese fa", "1个月前", "1 個月前", "1 個月前"}},
	{"a year ago", time.Date(2025, time.October, 23, 12, 0, 0, 0, time.Local), [5]string{"1 year ago", "1 anno fa", "1年前", "1 年前", "1 年前"}},
	{"tomorrow", time.Date(2026, time.October, 24, 9, 0, 0, 0, time.Local), [5]string{"tomorrow", "domani", "明天", "明天", "明日"}},
	{"in 2 minutes", relNow.Add(2 * time.Minute), [5]string{"in 2 minutes", "tra 2 minuti", "2分钟后", "2 分鐘後", "2 分鐘後"}},
	{"in 3 hours", relNow.Add(3 * time.Hour), [5]string{"in 3 hours", "tra 3 ore", "3小时后", "3 小時後", "3 小時後"}},
}

// Each language forms the text of a time from its unit, count and direction, with the
// words of CLDR's long forms.
func TestFormatRelativeInEachLanguage(t *testing.T) {
	packs := []struct {
		name string
		s    *Strings
	}{
		{"en", EnglishStrings()},
		{"it", StringsFor("it")},
		{"zh-CN", StringsFor("zh-CN")},
		{"zh-TW", StringsFor("zh-TW")},
		{"zh-HK", StringsFor("zh-HK")},
	}
	for _, c := range relCases {
		for i, p := range packs {
			if got := formatRelative(c.t, relNow, p.s); got != c.want[i] {
				t.Errorf("%s in %s = %q, want %q", c.name, p.name, got, c.want[i])
			}
		}
	}
}

// Past the year's day counts the unit by its size: a week from 7 days, a month from 30,
// a year from 365, so that 29 days is 4 weeks and 364 days is 12 months.
func TestFormatRelativeUnitBoundaries(t *testing.T) {
	en := EnglishStrings()
	for _, c := range []struct {
		days int
		want string
	}{
		{6, "6 days ago"},
		{7, "1 week ago"},
		{29, "4 weeks ago"},
		{30, "1 month ago"},
		{364, "12 months ago"},
		{365, "1 year ago"},
	} {
		at := relNow.AddDate(0, 0, -c.days)
		if got := formatRelative(at, relNow, en); got != c.want {
			t.Errorf("%d days ago = %q, want %q", c.days, got, c.want)
		}
	}
}

// A zero time has no relative text, as it has no date.
func TestFormatRelativeZeroTimeIsEmpty(t *testing.T) {
	if got := formatRelative(time.Time{}, relNow, EnglishStrings()); got != "" {
		t.Errorf("a zero time formats as %q, want empty", got)
	}
}

// FormatRelative uses the language of the system, as SystemStrings does.
func TestFormatRelativeUsesTheSystemLanguage(t *testing.T) {
	at := relNow.Add(-3 * time.Hour)
	if got, want := FormatRelative(at, relNow), formatRelative(at, relNow, SystemStrings()); got != want {
		t.Errorf("FormatRelative = %q, want %q", got, want)
	}
}

// relRigStart is the clock of the rig, which newAccessRig starts at.
var relRigStart = time.Unix(1_000_000, 0)

// relRig is a runtime with a TimeAgo that is shown while show is set, under a wrapper
// that visible hides with Display(false).
type relRig struct {
	*accessRig
	show, visible *Signal[bool]
	at            time.Time
}

// newRelRig returns a rig whose TimeAgo shows the time at, shown and on screen.
func newRelRig(at time.Time) *relRig {
	r := &relRig{at: at, show: NewSignal(true), visible: NewSignal(true)}
	r.accessRig = newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(
			If(r.show.Get(), Div().Key("wrap").Display(r.visible.Get()).Child(TimeAgo(r.at).Key("ago"))),
		)
	})
	return r
}

// advance moves the clock by d and runs a frame.
func (r *relRig) advance(d time.Duration) {
	r.now = r.now.Add(d)
	r.rt.Frame(r.now)
}

// A TimeAgo shows the text of its time, then renders again as the minutes pass, and it
// keeps one timer while it is shown.
func TestTimeAgoRendersAgainAsTimePasses(t *testing.T) {
	r := newRelRig(relRigStart.Add(-10 * time.Minute))
	r.advance(time.Millisecond)
	accesstest.Find(t, r.tree(), access.RoleStaticText, "10 minutes ago")
	if n := len(r.rt.timers); n != 1 {
		t.Fatalf("a shown TimeAgo keeps %d timers, want 1", n)
	}
	r.advance(time.Minute)
	accesstest.Find(t, r.tree(), access.RoleStaticText, "11 minutes ago")
}

// The full date and time of a TimeAgo is its accessible description, so that assistive
// technology reads the exact time after the relative text.
func TestTimeAgoDescribesTheFullTime(t *testing.T) {
	r := newRelRig(relRigStart.Add(-10 * time.Minute))
	r.advance(time.Millisecond)
	want := FormatDate(relRigStart.Add(-10*time.Minute)) + " " + FormatTime(relRigStart.Add(-10*time.Minute))
	found := false
	r.tree().Walk(func(n *access.Node) bool {
		found = found || n.Description == want
		return true
	})
	if !found {
		t.Errorf("no node has the description %q", want)
	}
	if len(r.warnings) != 0 {
		t.Errorf("accessibility warnings: %v", r.warnings)
	}
}

// An off-screen TimeAgo keeps no timer, and a removed one keeps none either.
func TestTimeAgoKeepsNoTimerOffScreenOrDisposed(t *testing.T) {
	r := newRelRig(relRigStart.Add(-10 * time.Minute))
	r.visible.Set(false)
	// The tick that finds the element hidden is the one after the frame that hid it: the
	// frame's timers run before its paint, so the first tick still sees the old paint.
	r.advance(time.Minute)
	r.advance(time.Minute)
	if n := len(r.rt.timers); n != 0 {
		t.Errorf("an off-screen TimeAgo keeps %d timers, want 0", n)
	}
	r.visible.Set(true)
	r.advance(time.Millisecond)
	if n := len(r.rt.timers); n != 1 {
		t.Errorf("a TimeAgo shown again keeps %d timers, want 1", n)
	}
	r.show.Set(false)
	r.advance(time.Millisecond)
	if n := len(r.rt.timers); n != 0 {
		t.Errorf("a removed TimeAgo keeps %d timers, want 0", n)
	}
}

// The refresh is every minute under an hour, every hour under a day, and daily after.
func TestTimeAgoPeriod(t *testing.T) {
	for _, c := range []struct {
		age  time.Duration
		want time.Duration
	}{
		{5 * time.Minute, time.Minute},
		{-59 * time.Minute, time.Minute},
		{2 * time.Hour, time.Hour},
		{23 * time.Hour, time.Hour},
		{3 * 24 * time.Hour, 24 * time.Hour},
	} {
		if got := timeAgoPeriod(c.age); got != c.want {
			t.Errorf("timeAgoPeriod(%v) = %v, want %v", c.age, got, c.want)
		}
	}
}
