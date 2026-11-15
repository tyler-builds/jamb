//go:build js && tinygo

package web

import "unsafe"

// hostExports reports whether the build exports the functions that the host
// script calls.
const hostExports = true

// hostText is where the host script writes the strings of an event, as UTF-8,
// before it calls the event's function with their lengths.
var hostText [hostTextSize]byte

// hostTextAddress returns the address of hostText in the app's memory.
func hostTextAddress() int { return int(uintptr(unsafe.Pointer(&hostText[0]))) }

// hostParts holds the pieces of a string longer than hostText that the host
// script delivered before its last (see hostTextPart).
var hostParts []byte

// hostString returns the string that the host script delivered: the pieces
// before, and the n bytes at the start of hostText.
func hostString(n int32) string {
	if n < 0 || int(n) > len(hostText) {
		hostParts = hostParts[:0]
		return ""
	}
	if len(hostParts) == 0 {
		return string(hostText[:n])
	}
	s := string(append(hostParts, hostText[:n]...))
	hostParts = hostParts[:0]
	return s
}

// hostSetText has the host script write text, when value is set, and the
// selection from start to end, in UTF-16 code units, to the textarea (see
// writeArea). The text goes through hostText, in pieces when it is longer.
func (w *window) hostSetText(text string, start, end int, backward, value bool) {
	n := 0
	if value {
		for len(text) > len(hostText) {
			k := runeStart(text, len(hostText))
			copy(hostText[:], text[:k])
			w.host.Call("textPart", k)
			text = text[k:]
		}
		n = copy(hostText[:], text)
	}
	w.host.Call("setText", n, start, end, flag(backward), flag(value))
}

// hostWindow returns the page's window while the host script delivers its
// events.
func hostWindow() *window {
	if len(windows) == 0 || !windows[0].hosted || windows[0].destroyed {
		return nil
	}
	return windows[0]
}

func flag(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// The functions below are what the host script calls. They are exported the
// plain way (//export), which runs them on the caller's stack, in no
// goroutine: a call takes under a microsecond and leaves nothing to collect.
// A //go:wasmexport function runs in a goroutine of its own, whose stack (1 MB
// here, see scripts/web/tinygo.sh) TinyGo allocates for each call and leaves
// to the collector. So they must not block, and do not: each notes the event
// and posts its work to the UI goroutine, which the script then runs
// (go_scheduler) before the browser's event ends. The ones for input return 1
// when the event's default action is prevented.

// hostFrame is the display's frame at the time t, from requestAnimationFrame.
//
//export jamb_frame
func hostFrame(t float64) {
	if w := hostWindow(); w != nil {
		w.displayFrame(t)
	}
}

// hostPointer is a pointer event: kind is a pointerKind, x and y are in CSS
// pixels relative to the canvas, and mods are the modifier keys as bits.
//
//export jamb_pointer
func hostPointer(kind, touch, id int32, x, y float64, button, buttons, mods int32, at float64) int32 {
	w := hostWindow()
	if w == nil {
		return 0
	}
	hostEntry = true
	prevent := w.pointer(pointerEvent{kind: pointerKind(kind), touch: touch != 0, id: int(id), x: x, y: y,
		button: int(button), buttons: int(buttons), mods: pageMods(int(mods)), at: at})
	hostEntry = false
	return flag(prevent)
}

// hostWheel is a wheel event.
//
//export jamb_wheel
func hostWheel(x, y, dx, dy float64, mode, mods int32) int32 {
	w := hostWindow()
	if w == nil {
		return 0
	}
	hostEntry = true
	prevent := w.wheel(wheelEvent{x: x, y: y, dx: dx, dy: dy, mode: int(mode), mods: pageMods(int(mods))})
	hostEntry = false
	return flag(prevent)
}

// hostKey is a key press or release, whose code and key are in hostText, the
// first codeLen bytes and the keyLen bytes after them.
//
//export jamb_key
func hostKey(down, codeLen, keyLen, mods, repeat, composing int32) int32 {
	w := hostWindow()
	if w == nil || codeLen < 0 || keyLen < 0 || int(codeLen)+int(keyLen) > len(hostText) {
		return 0
	}
	hostEntry = true
	prevent := w.key(keyEvent{down: down != 0, code: string(hostText[:codeLen]),
		key: string(hostText[codeLen : codeLen+keyLen]), mods: pageMods(int(mods)), repeat: repeat != 0,
		composing: composing != 0})
	hostEntry = false
	return flag(prevent)
}

// hostBeforeInput is the textarea's beforeinput event, whose inputType is the
// first n bytes of hostText.
//
//export jamb_before_input
func hostBeforeInput(n int32) int32 {
	w := hostWindow()
	if w == nil {
		return 0
	}
	hostEntry = true
	prevent := w.beforeInput(hostString(n))
	hostEntry = false
	return flag(prevent)
}

// hostTextPart is a piece of a string longer than hostText, the first n bytes
// of hostText, which the next call's string continues.
//
//export jamb_text_part
func hostTextPart(n int32) {
	if n > 0 && int(n) <= len(hostText) {
		hostParts = append(hostParts, hostText[:n]...)
	}
}

// hostInput is the textarea's text after its input event, or after a
// composition ended (ended is 1), the first n bytes of hostText, with its
// selection from start to end in UTF-16 code units, whose anchor is end when
// backward is 1 (see areaInput). It returns 1 when the textarea is to be
// emptied.
//
//export jamb_input
func hostInput(n, start, end, backward, ended int32) int32 {
	w := hostWindow()
	s := hostString(n)
	if w == nil {
		return 0
	}
	hostEntry = true
	empty := w.areaInput(s, int(start), int(end), backward != 0, ended != 0)
	hostEntry = false
	return flag(empty)
}

// hostSelect is a change of the textarea's selection that no edit made: from
// start to end, in UTF-16 code units, whose anchor is end when backward is 1.
//
//export jamb_select
func hostSelect(start, end, backward int32) {
	w := hostWindow()
	if w == nil {
		return
	}
	hostEntry = true
	w.areaSelect(int(start), int(end), backward != 0)
	hostEntry = false
}

// hostComposition is a step of an input method's composition, a
// compositionKind, whose text is the first n bytes of hostText. It returns 1
// when the textarea is to be emptied.
//
//export jamb_composition
func hostComposition(kind, n int32) int32 {
	w := hostWindow()
	if w == nil {
		return 0
	}
	hostEntry = true
	empty := w.composition(compositionKind(kind), hostString(n))
	hostEntry = false
	return flag(empty)
}
