package store

import "time"

// seedNotes are written on first run.
func seedNotes() []fileNote {
	now := time.Now()
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	return []fileNote{
		{
			Title:   "Welcome to Jamb Notes",
			Created: ago(time.Hour), Updated: ago(2 * time.Minute), Pinned: true,
			Tags: []string{"jamb"},
			Body: `A small note-taking app built on jamb: native GPU rendering, written in Go.

Everything saves automatically a moment after you stop typing.

Shortcuts
- Ctrl+N: new note
- Ctrl+F: search notes
- Alt+Up / Alt+Down: previous or next note
- Ctrl+Shift+P: command palette
- Ctrl+K Ctrl+T: toggle theme
- Ctrl+= / Ctrl+- / Ctrl+0: zoom in, out, reset
- Ctrl+Z / Ctrl+Y: undo and redo
- Ctrl+A, Ctrl+C, Ctrl+X, Ctrl+V: select all, copy, cut, paste
- Ctrl+Left / Ctrl+Right: move by word
- Double-click selects a word; triple-click selects a paragraph
- keymap.json next to your notes file overrides shortcuts (see the README)

Text is shaped with HarfBuzz (via go-text/typesetting), so ligatures, right-to-left scripts and complex scripts render correctly. See the "Languages" note.`,
		},
		{
			Title:   "Languages",
			Created: ago(3 * time.Hour), Updated: ago(40 * time.Minute),
			Tags: []string{"jamb", "text"},
			Body: `English with ligatures: office, efficient, final, fluffy
Arabic (right to left): مرحبا بالعالم — النص العربي يتصل
Hebrew: שלום עולם
Hindi: नमस्ते दुनिया
Thai: สวัสดีชาวโลก
Japanese: こんにちは世界
Chinese: 你好，世界
Korean: 안녕하세요 세계
Greek and Cyrillic: Γειά σου Κόσμε · Привет, мир
Emoji: 👋 🌍 ✨ 👍🏽`,
		},
		{
			Title:   "Groceries",
			Created: ago(26 * time.Hour), Updated: ago(5 * time.Hour),
			Tags: []string{"home"},
			Body: "- oat milk\n- coffee beans\n- lemons\n- sourdough\n- basil",
		},
		{
			Title:   "Ideas for the framework",
			Created: ago(50 * time.Hour), Updated: ago(30 * time.Hour),
			Tags: []string{"jamb", "work"},
			Body: `macOS adapter: Cocoa window + Metal backend behind render.Renderer.
Skip off-screen subtrees in paint and hit testing.
Rich text: bold and italic spans inside one paragraph.`,
		},
	}
}
