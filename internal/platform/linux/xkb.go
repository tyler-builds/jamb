//go:build linux

package linux

import (
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/tyler-builds/jamb/input"
)

// libxkbcommon turns the compositor's keymap and the key codes of wl_keyboard
// into key symbols and text. It is loaded at run time with purego, so the
// adapter needs no cgo; every desktop that runs Wayland has it.
var xkb struct {
	once sync.Once
	err  error

	contextNew        func(flags int32) uintptr
	contextUnref      func(ctx uintptr)
	keymapNewFromBuf  func(ctx uintptr, buf unsafe.Pointer, size uintptr, format, flags int32) uintptr
	keymapUnref       func(km uintptr)
	keymapKeyRepeats  func(km uintptr, key uint32) int32
	keymapSymsByLevel func(km uintptr, key, layout, level uint32, syms *unsafe.Pointer) int32
	stateNew          func(km uintptr) uintptr
	stateUnref        func(st uintptr)
	stateUpdateMask   func(st uintptr, depressed, latched, locked, dl, dlatched, group uint32) int32
	stateKeyGetOneSym func(st uintptr, key uint32) uint32
	stateKeyGetUTF8   func(st uintptr, key uint32, buf *byte, size uintptr) int32
	stateKeyGetLayout func(st uintptr, key uint32) uint32
	stateModNameIs    func(st uintptr, name string, kind int32) int32
	keysymToUTF8      func(sym uint32, buf *byte, size uintptr) int32

	// Compose (dead keys and the Compose key). Optional: older builds lack it.
	composeTableNew func(ctx uintptr, locale string, flags int32) uintptr
	composeStateNew func(table uintptr, flags int32) uintptr
	composeFeed     func(st uintptr, sym uint32) int32
	composeReset    func(st uintptr)
	composeStatus   func(st uintptr) int32
	composeUTF8     func(st uintptr, buf *byte, size uintptr) int32
	composeOneSym   func(st uintptr) uint32
}

func loadXKB() error {
	xkb.once.Do(func() {
		lib, err := purego.Dlopen("libxkbcommon.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			xkb.err = fmt.Errorf("platform/linux: loading libxkbcommon: %w", err)
			return
		}
		reg := func(fn any, name string) {
			if xkb.err == nil {
				defer func() {
					if r := recover(); r != nil {
						xkb.err = fmt.Errorf("platform/linux: libxkbcommon has no %s", name)
					}
				}()
				purego.RegisterLibFunc(fn, lib, name)
			}
		}
		reg(&xkb.contextNew, "xkb_context_new")
		reg(&xkb.contextUnref, "xkb_context_unref")
		reg(&xkb.keymapNewFromBuf, "xkb_keymap_new_from_buffer")
		reg(&xkb.keymapUnref, "xkb_keymap_unref")
		reg(&xkb.keymapKeyRepeats, "xkb_keymap_key_repeats")
		reg(&xkb.keymapSymsByLevel, "xkb_keymap_key_get_syms_by_level")
		reg(&xkb.stateNew, "xkb_state_new")
		reg(&xkb.stateUnref, "xkb_state_unref")
		reg(&xkb.stateUpdateMask, "xkb_state_update_mask")
		reg(&xkb.stateKeyGetOneSym, "xkb_state_key_get_one_sym")
		reg(&xkb.stateKeyGetUTF8, "xkb_state_key_get_utf8")
		reg(&xkb.stateKeyGetLayout, "xkb_state_key_get_layout")
		reg(&xkb.stateModNameIs, "xkb_state_mod_name_is_active")
		reg(&xkb.keysymToUTF8, "xkb_keysym_to_utf8")
		if xkb.err != nil {
			return
		}
		// Compose is optional: without it, dead keys type nothing.
		var composeErr error
		for _, f := range []struct {
			fn   any
			name string
		}{
			{&xkb.composeTableNew, "xkb_compose_table_new_from_locale"},
			{&xkb.composeStateNew, "xkb_compose_state_new"},
			{&xkb.composeFeed, "xkb_compose_state_feed"},
			{&xkb.composeReset, "xkb_compose_state_reset"},
			{&xkb.composeStatus, "xkb_compose_state_get_status"},
			{&xkb.composeUTF8, "xkb_compose_state_get_utf8"},
			{&xkb.composeOneSym, "xkb_compose_state_get_one_sym"},
		} {
			func() {
				defer func() {
					if recover() != nil {
						composeErr = fmt.Errorf("no %s", f.name)
					}
				}()
				if composeErr == nil {
					purego.RegisterLibFunc(f.fn, lib, f.name)
				}
			}()
		}
		if composeErr != nil {
			xkb.composeTableNew = nil
		}
	})
	return xkb.err
}

const (
	xkbKeymapFormatTextV1 = 1
	xkbStateModsEffective = 1 << 3
	xkbComposeFeedAccept  = 1
	xkbComposeComposing   = 1
	xkbComposeComposed    = 2
	xkbComposeCancelled   = 3
)

// keyboard is the state of the seat's keyboard: its keymap, the modifiers and
// layout the compositor reports, and the compose sequence being typed.
type keyboard struct {
	ctx     uintptr
	keymap  uintptr
	state   uintptr
	compose uintptr // 0 without a compose table
	mods    input.Modifiers
}

func newKeyboard() (*keyboard, error) {
	if err := loadXKB(); err != nil {
		return nil, err
	}
	k := &keyboard{ctx: xkb.contextNew(0)}
	if k.ctx == 0 {
		return nil, fmt.Errorf("platform/linux: xkb_context_new failed")
	}
	if xkb.composeTableNew != nil {
		locale := firstEnv("LC_ALL", "LC_CTYPE", "LANG")
		if locale == "" {
			locale = "C"
		}
		if table := xkb.composeTableNew(k.ctx, locale, 0); table != 0 {
			k.compose = xkb.composeStateNew(table, 0)
		}
	}
	return k, nil
}

func firstEnv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}

// setKeymap replaces the keymap with the compositor's text keymap.
func (k *keyboard) setKeymap(text []byte) error {
	if len(text) == 0 {
		return fmt.Errorf("platform/linux: empty keymap")
	}
	km := xkb.keymapNewFromBuf(k.ctx, unsafe.Pointer(&text[0]), uintptr(len(text)), xkbKeymapFormatTextV1, 0)
	if km == 0 {
		return fmt.Errorf("platform/linux: the compositor's keymap does not compile")
	}
	st := xkb.stateNew(km)
	if st == 0 {
		xkb.keymapUnref(km)
		return fmt.Errorf("platform/linux: xkb_state_new failed")
	}
	if k.state != 0 {
		xkb.stateUnref(k.state)
		xkb.keymapUnref(k.keymap)
	}
	k.keymap, k.state = km, st
	return nil
}

// updateMods applies wl_keyboard.modifiers.
func (k *keyboard) updateMods(depressed, latched, locked, group uint32) {
	if k.state == 0 {
		return
	}
	xkb.stateUpdateMask(k.state, depressed, latched, locked, 0, 0, group)
	var m input.Modifiers
	if xkb.stateModNameIs(k.state, "Shift", xkbStateModsEffective) > 0 {
		m |= input.ModShift
	}
	if xkb.stateModNameIs(k.state, "Control", xkbStateModsEffective) > 0 {
		m |= input.ModCtrl
	}
	if xkb.stateModNameIs(k.state, "Mod1", xkbStateModsEffective) > 0 {
		m |= input.ModAlt
	}
	if xkb.stateModNameIs(k.state, "Mod4", xkbStateModsEffective) > 0 {
		m |= input.ModSuper
	}
	k.mods = m
}

// repeats reports whether holding the key repeats it.
func (k *keyboard) repeats(code uint32) bool {
	return k.keymap != 0 && xkb.keymapKeyRepeats(k.keymap, code+8) > 0
}

// key returns the input.Key of an evdev key code. Letters follow the layout,
// as Windows' virtual keys do, so Ctrl+Z is the key that types z; digits and
// punctuation are named by their position (see input.Key).
func (k *keyboard) key(code uint32) input.Key {
	if k.state != 0 {
		if sym := k.baseSym(code); sym >= 'a' && sym <= 'z' {
			return input.KeyA + input.Key(sym-'a')
		} else if sym >= 'A' && sym <= 'Z' {
			return input.KeyA + input.Key(sym-'A')
		}
		if key := keyOfSym(xkb.stateKeyGetOneSym(k.state, code+8)); key != input.KeyUnknown {
			return key
		}
	}
	return keyOfCode(code)
}

// baseSym returns the key's symbol at the first level of the active layout.
func (k *keyboard) baseSym(code uint32) uint32 {
	layout := xkb.stateKeyGetLayout(k.state, code+8)
	var syms unsafe.Pointer
	if n := xkb.keymapSymsByLevel(k.keymap, code+8, layout, 0, &syms); n > 0 && syms != nil {
		return *(*uint32)(syms)
	}
	return 0
}

// text returns what a key press types, after compose: "" for a key that types
// nothing, a control character, or a key that is part of an unfinished
// compose sequence.
func (k *keyboard) text(code uint32) string {
	if k.state == 0 {
		return ""
	}
	sym := xkb.stateKeyGetOneSym(k.state, code+8)
	if k.compose != 0 && sym != 0 && xkb.composeFeed(k.compose, sym) == xkbComposeFeedAccept {
		switch xkb.composeStatus(k.compose) {
		case xkbComposeComposing:
			return ""
		case xkbComposeComposed:
			s := cString(func(b *byte, n uintptr) int32 { return xkb.composeUTF8(k.compose, b, n) })
			xkb.composeReset(k.compose)
			return printable(s)
		case xkbComposeCancelled:
			xkb.composeReset(k.compose)
			return ""
		}
	}
	return printable(cString(func(b *byte, n uintptr) int32 { return xkb.stateKeyGetUTF8(k.state, code+8, b, n) }))
}

// cString calls fn with a buffer and returns the string it wrote.
func cString(fn func(*byte, uintptr) int32) string {
	var buf [64]byte
	n := fn(&buf[0], uintptr(len(buf)))
	if n <= 0 || int(n) >= len(buf) {
		return ""
	}
	return string(buf[:n])
}

// printable drops control characters, as the Windows adapter's WM_CHAR does:
// Ctrl+letter types a control character that is a shortcut, not text.
func printable(s string) string {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	return s
}

// keyOfSym maps the keysyms of the keys that are not letters, digits or
// punctuation.
func keyOfSym(sym uint32) input.Key {
	switch {
	case sym >= 0xffbe && sym <= 0xffc9: // XK_F1..XK_F12
		return input.KeyF1 + input.Key(sym-0xffbe)
	}
	switch sym {
	case 0xff0d, 0xff8d: // Return, KP_Enter
		return input.KeyEnter
	case 0xff1b:
		return input.KeyEscape
	case 0xff08:
		return input.KeyBackspace
	case 0xffff, 0xff9f: // Delete, KP_Delete
		return input.KeyDelete
	case 0xff09, 0xfe20: // Tab, ISO_Left_Tab (Shift+Tab)
		return input.KeyTab
	case 0x20:
		return input.KeySpace
	case 0xff51, 0xff96:
		return input.KeyLeft
	case 0xff53, 0xff98:
		return input.KeyRight
	case 0xff52, 0xff97:
		return input.KeyUp
	case 0xff54, 0xff99:
		return input.KeyDown
	case 0xff50, 0xff95:
		return input.KeyHome
	case 0xff57, 0xff9c:
		return input.KeyEnd
	case 0xff55, 0xff9a:
		return input.KeyPageUp
	case 0xff56, 0xff9b:
		return input.KeyPageDown
	case 0xff63, 0xff9e:
		return input.KeyInsert
	case 0xff67:
		return input.KeyMenu
	case 0xffab: // KP_Add
		return input.KeyEqual
	case 0xffad: // KP_Subtract
		return input.KeyMinus
	}
	if sym >= 0xffb0 && sym <= 0xffb9 { // KP_0..KP_9
		return input.Key0 + input.Key(sym-0xffb0)
	}
	return input.KeyUnknown
}

// keyOfCode maps evdev key codes by position: the digits and punctuation, and
// the letters of a layout that has no Latin letters.
func keyOfCode(code uint32) input.Key {
	if code >= 2 && code <= 10 { // KEY_1..KEY_9
		return input.Key1 + input.Key(code-2)
	}
	switch code {
	case 11:
		return input.Key0
	case 12:
		return input.KeyMinus
	case 13:
		return input.KeyEqual
	case 26:
		return input.KeyLeftBracket
	case 27:
		return input.KeyRightBracket
	case 39:
		return input.KeySemicolon
	case 40:
		return input.KeyApostrophe
	case 41:
		return input.KeyBacktick
	case 43:
		return input.KeyBackslash
	case 51:
		return input.KeyComma
	case 52:
		return input.KeyPeriod
	case 53:
		return input.KeySlash
	case 57:
		return input.KeySpace
	}
	if k, ok := letterCodes[code]; ok {
		return input.KeyA + input.Key(k-'a')
	}
	return input.KeyUnknown
}

// letterCodes are the evdev codes of the letters on a US keyboard.
var letterCodes = map[uint32]byte{
	16: 'q', 17: 'w', 18: 'e', 19: 'r', 20: 't', 21: 'y', 22: 'u', 23: 'i', 24: 'o', 25: 'p',
	30: 'a', 31: 's', 32: 'd', 33: 'f', 34: 'g', 35: 'h', 36: 'j', 37: 'k', 38: 'l',
	44: 'z', 45: 'x', 46: 'c', 47: 'v', 48: 'b', 49: 'n', 50: 'm',
}
