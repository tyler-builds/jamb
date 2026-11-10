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

// hostString returns the n bytes at the start of hostText as a string.
func hostString(n int32) string {
	if n < 0 || int(n) > len(hostText) {
		return ""
	}
	return string(hostText[:n])
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

// hostInput is text that the textarea's input event typed, the first n bytes
// of hostText. It returns 1 when the textarea is to be emptied.
//
//export jamb_input
func hostInput(n int32) int32 {
	w := hostWindow()
	if w == nil {
		return 0
	}
	hostEntry = true
	empty := w.textInput(hostString(n))
	hostEntry = false
	return flag(empty)
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
