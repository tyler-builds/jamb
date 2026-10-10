package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// me is the account the demo runs as. Its own messages are drawn on the right.
const me = "Alex Rivera"

// replyDelay is how long a teammate takes to answer a comment. Tests shorten it.
var replyDelay = 900 * time.Millisecond

// teammate answers the comments sent from the project window.
const teammate = "Sam Chen"

var replies = []string{
	"Thanks, I will take a look after the build finishes.",
	"Agreed. I will update the task list this afternoon.",
	"That matches what I saw in the last profile run.",
	"Good catch. Can you add a note to the description as well?",
	"I am on it. The fix should land before the review.",
}

// Message is one comment in a project's thread.
type Message struct {
	ID     int
	Author string
	Body   string
	At     time.Time
}

// commentLines are the sentences that seeded comments are built from.
var commentLines = []string{
	"The scene graph change looks right, so I will run the frame benchmark tonight.",
	"Measure caching cut the second pass by about a third on the sample page.",
	"Can we keep the atlas size fixed until the subpixel work is merged?",
	"The Windows build passes now, but the DPI test still needs a 150 percent display.",
	"I left a comment on the layout change with two cases that need coverage.",
	"Shaping is fine for Latin text. The right-to-left path still needs a review.",
	"Deploying the nightly build to the test machine took under three minutes.",
	"Tasks for this week are updated. The renderer work is the critical path.",
	"Please check the input handling on a touch screen before we close this out.",
	"Two of the flaky tests were timing dependent and are now fixed.",
	"The release notes need a line about the new focus behavior.",
	"Window geometry is saved on close, so the layout checks are repeatable.",
	"I would like a second reviewer on the glyph cache before we merge.",
	"Nothing blocking here. The remaining work is documentation and examples.",
}

// seedThreads gives each project a thread of 150 to 300 messages, built from
// the project's ID so that every run shows the same threads. The messages vary
// in length (one to four sentences), so the rows vary in height. It returns the
// threads and the number of messages, which is the last message ID.
func seedThreads(ps []Project, now time.Time) (map[int][]Message, int) {
	threads := map[int][]Message{}
	next := 0
	for _, p := range ps {
		n := 150 + (p.ID*53)%151
		msgs := make([]Message, 0, n)
		for i := range n {
			next++
			msgs = append(msgs, Message{
				ID:     next,
				Author: people[(i*3+p.ID)%len(people)],
				Body:   messageBody(p.ID, i),
				At:     now.Add(-time.Duration(n-i) * 47 * time.Minute),
			})
		}
		threads[p.ID] = msgs
	}
	return threads, next
}

// messageBody is one to four sentences of seeded text.
func messageBody(project, i int) string {
	k := 1 + (project*7+i*5)%4
	parts := make([]string, k)
	for j := range parts {
		parts[j] = commentLines[(i*5+j*3+project)%len(commentLines)] // distinct for j < 4
	}
	return strings.Join(parts, " ")
}

// Comments is the project's thread in chat style. The list sits at the bottom
// of its space, follows new messages while it is at the end, and does not move
// when the user has scrolled up. Then a button jumps back to the end. Enter
// sends a comment, and a teammate answers a moment later from a goroutine.
func Comments(s *Store, id int, h *ui.ListHandle) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		atEnd := ui.Local(ctx, "atEnd", false)
		draft := ui.Local(ctx, "draft", "")
		opened := ui.Local(ctx, "opened", false)
		thread := s.Threads.Get()[id]
		// The thread opens at its latest message. Only the first render scrolls
		// there, so later renders leave the position alone.
		if !opened.Peek() {
			opened.Set(true)
			h.ScrollToEnd()
		}

		// Enter sends. The reply comes back through ctx.Post, which runs it on
		// the UI thread on the next frame.
		send := func(text string) {
			text = strings.TrimSpace(text)
			if text == "" {
				return
			}
			draft.Set("")
			s.Comment(id, me, text)
			h.ScrollToEnd()
			reply := replies[s.nextMsg%len(replies)]
			s.Typing.Set(id) // the teammate is typing until the reply lands
			go func() {
				time.Sleep(replyDelay)
				ctx.Post(func() {
					s.Typing.Set(0)
					s.Comment(id, teammate, reply)
				})
			}()
		}

		list := ui.ListOf(thread, func(m Message) string { return fmt.Sprint(m.ID) }, func(m Message) ui.Element {
			return messageRow(t, m)
		}).Key("thread").Style(ui.Style{Grow: 1}).Bottom().Handle(h).OnAtEnd(atEnd.Set)

		body := ui.Div().Grow(1).MinHeight(ui.Px(0)).Child(list)
		if !atEnd.Get() {
			// Absolute, so showing it does not resize the list.
			body.Child(ui.Div().Absolute().Left(0).Right(0).Bottom(12).Row().Justify(ui.Center).
				Child(ui.Button("Jump to latest").Icon(icons.ChevronDown).Secondary().Small().Key("jump-latest").
					OnClick(h.ScrollToEnd)))
		}

		column := ui.Div().Grow(1).MinHeight(ui.Px(0)).Gap(10).
			Child(ui.Div().Row().Align(ui.Center).Gap(8).
				Child(ui.Icon(icons.Inbox).Size(15).Color(t.TextMuted)).
				Child(SectionTitle(t, "Comments")).
				Child(ui.Text(fmt.Sprintf("%d", len(thread))).Size(12).Color(t.TextSubtle))).
			Child(Card(t).Padding(12).Grow(1).MinHeight(ui.Px(0)).Gap(0).Child(body))
		if s.Typing.Get() == id {
			// A busy ring, not a counter: the reply has no known time.
			column.Child(ui.Div().Row().Align(ui.Center).Gap(8).
				Child(ui.ProgressRing().Size(ui.RingSmall).Label(teammate + " is typing")).
				Child(ui.Text(teammate + " is typing…").Size(12).Color(t.TextMuted)))
		}
		return column.Child(ui.TextInput(draft).Label("Comment").Placeholder("Write a comment and press Enter").
			OnSubmit(send).Key("composer"))
	})
}

// messageRow is one comment: a bubble with the author, the time and the text.
// Messages from me sit on the right, in the accent tint.
func messageRow(t *ui.Theme, m Message) ui.Element {
	mine := m.Author == me
	bubble := ui.Div().Gap(4).PaddingX(12).PaddingY(8).Radius(12).MaxWidth(ui.Px(460)).
		Background(t.SurfaceHover).Border(1, t.Border)
	if mine {
		edge := t.AccentSoft
		if t.HighContrast {
			edge = t.Accent // the tint is the window color there, so the edge marks the message
		}
		bubble.Background(t.AccentSoft).Border(1, edge)
	}
	bubble.Child(ui.Div().Row().Align(ui.Center).Gap(8).
		Child(ui.Text(m.Author).Size(12).Semibold().Color(t.TextMuted)).
		Child(ui.Text(ago(m.At)).Size(11).Color(t.TextSubtle))).
		Child(ui.Text(m.Body).Size(13))
	row := ui.Div().Row().PaddingY(5)
	if mine {
		row.Justify(ui.End)
	}
	return row.Child(bubble)
}
