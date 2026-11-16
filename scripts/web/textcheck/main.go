// Command textcheck compares where a page's textarea, lying over a text field
// as it does on iOS, puts the boundaries between characters with where jamb
// lays them out, and fails when they differ by more than a pixel: iOS's
// keyboard trackpad finds the caret's place in the textarea's text, at the
// point under the finger, and must find the character jamb draws there.
// browser.cjs scans the textarea as the trackpad hit-tests it
// (caretPositionFromPoint along each line) and writes the scans; browser.sh
// runs textcheck on them under Node, as a js build with the Go fonts compiled
// in, so that jamb's layout is a page's, with a page's fonts:
//
//	GOOS=js GOARCH=wasm go run -tags=jamb_embedfonts ./scripts/web/textcheck SCANS.json
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"unicode/utf16"

	"github.com/tyler-builds/jamb/internal/text"
)

// scans is what browser.cjs writes: the field's text size, the width of its
// box, whether it wraps, all in CSS pixels, and a scan of each text. A scan has
// a list of runs for each line: from the x of each run on, the textarea's
// caretPositionFromPoint at the middle of the line gives the offset, in UTF-16
// code units, or -1 for a point outside its text.
type scans struct {
	Size, Width float64
	Wrap        bool
	Texts       []struct {
		Text  string
		Lines [][][2]float64
	}
}

// maxOff is the largest difference allowed between the browser's boundary
// between two characters and the middle of jamb's two carets: the scan steps
// by a quarter of a pixel.
const maxOff = 1.0

func main() {
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fail(err.Error())
	}
	var sc scans
	if err := json.Unmarshal(data, &sc); err != nil {
		fail(err.Error())
	}
	sys := text.NewSystem()
	st := text.Style{Size: float32(sc.Size), NoWrap: !sc.Wrap}
	width := float32(math.Inf(1))
	if sc.Wrap {
		width = float32(sc.Width) + 0.01
	}
	failed := false
	for _, t := range sc.Texts {
		l := sys.Layout(t.Text, st, width)
		worst, where, moved := 0.0, "", 0
		for li, runs := range t.Lines {
			if li >= len(l.Lines) {
				break
			}
			ls, le := l.LineBounds(li)
			lo, hi := units(t.Text, ls), units(t.Text, le)
			for i := 1; i < len(runs); i++ {
				xb, prev, cur := runs[i][0], int(runs[i-1][1]), int(runs[i][1])
				if prev < 0 || cur < 0 || cur <= prev {
					continue
				}
				if prev < lo || cur > hi {
					moved++ // the browser has the boundary on another line
					continue
				}
				a, b := bytes(t.Text, prev), bytes(t.Text, cur)
				mid := float64(l.LineCaretX(li, a)+l.LineCaretX(li, b)) / 2
				if d := math.Abs(xb - mid); d > worst {
					worst, where = d, fmt.Sprintf("line %d, between %q and %q: %.2f in the browser, %.2f in jamb",
						li+1, t.Text[:a], t.Text[a:b], xb, mid)
				}
			}
		}
		ok := worst <= maxOff && moved == 0
		failed = failed || !ok
		status := "ok"
		if !ok {
			status = "FAIL"
		}
		fmt.Printf("%s: %q: the boundaries differ by %.2f px at most", status, t.Text, worst)
		if worst > maxOff {
			fmt.Printf(" (%s)", where)
		}
		if moved > 0 {
			fmt.Printf("; %d are on another line", moved)
		}
		fmt.Println()
	}
	if failed {
		fail("the textarea over a text field lays its text out unlike the field")
	}
}

// units returns the byte offset i of s in UTF-16 code units.
func units(s string, i int) int { return len(utf16.Encode([]rune(s[:i]))) }

// bytes returns the offset u of s, in UTF-16 code units, in bytes.
func bytes(s string, u int) int {
	n := 0
	for i, r := range s {
		if n >= u {
			return i
		}
		n++
		if r >= 0x10000 {
			n++
		}
	}
	return len(s)
}

func fail(msg string) {
	fmt.Println("FAIL:", msg)
	os.Exit(1)
}
