// jamb.js is the host script of a page that runs a jamb app: a TinyGo build
// (see build.sh). A page loads it before the app, and gives it the app's
// instance before the app runs:
//
//	jambHost.attach(result.instance);
//	await go.run(result.instance);
//
// The app then has the script deliver the display's frames, the pointer, wheel
// and key events of its canvas, and the edits and the selection of its hidden
// textarea, to which the script writes the focused text field's text. The
// script calls functions that the app exports, with numbers, and then runs the
// app's scheduler, so the app handles each event, and draws each frame, before
// the browser's callback ends.
//
// Without the script the app listens for itself, through Go's syscall/js, where
// every call of a listener makes references to JavaScript values that TinyGo's
// collector counts: it collects, for a time in proportion to the app's memory,
// every few frames of a moving pointer (see internal/platform/web/host.go).
(() => {
  "use strict";
  let app = null; // the app's exports
  let canvas = null;
  let area = null; // the app's hidden textarea
  let text = 0; // the address and the size of the app's buffer for an event's strings
  let textSize = 0;
  let closed = true;
  let removers = [];
  const encoder = new TextEncoder();
  const decoder = new TextDecoder();
  let incoming = ""; // the pieces so far of a string the app writes (see textPart)

  const mods = (e) => (e.shiftKey ? 1 : 0) | (e.ctrlKey ? 2 : 0) | (e.altKey ? 4 : 0) | (e.metaKey ? 8 : 0);
  const on = (target, type, fn, options) => {
    target.addEventListener(type, fn, options);
    removers.push(() => target.removeEventListener(type, fn, options));
  };
  // buffer returns the app's buffer. The memory's own buffer is another object
  // after the memory grew, so the view is made for each event.
  const buffer = () => new Uint8Array(app.memory.buffer, text, textSize);
  // write puts the start of s in the app's buffer, and returns the length it
  // wrote, in bytes, and how much of s that was.
  const write = (s) => encoder.encodeInto(s, buffer());
  // send puts s in the app's buffer, the pieces that do not fit first, each
  // given to the app as part of the string that the next call delivers, and
  // returns the length of the last piece, in bytes.
  const send = (s) => {
    for (;;) {
      const { read, written } = write(s);
      if (read >= s.length) return written;
      app.jamb_text_part(written);
      s = s.slice(read);
    }
  };
  // backward returns 1 when the anchor of the textarea's selection is its end.
  const backward = () => (area.selectionDirection === "backward" ? 1 : 0);
  // handled runs the app until it waits, so that it handles the event now,
  // and prevents the event's default action when the app asked for that.
  const handled = (e, prevent) => {
    app.go_scheduler();
    if (prevent) e.preventDefault();
  };

  // pointer delivers a pointer event of the kind as the app numbers them.
  const pointer = (kind) => (e) => {
    const r = canvas.getBoundingClientRect();
    handled(e, app.jamb_pointer(kind, e.pointerType === "touch" ? 1 : 0, e.pointerId,
      e.clientX - r.left, e.clientY - r.top, e.button, e.buttons, mods(e), e.timeStamp));
  };
  const wheel = (e) => {
    const r = canvas.getBoundingClientRect();
    handled(e, app.jamb_wheel(e.clientX - r.left, e.clientY - r.top, e.deltaX, e.deltaY, e.deltaMode, mods(e)));
  };
  const key = (down) => (e) => {
    const buf = buffer();
    const code = encoder.encodeInto(e.code || "", buf.subarray(0, 64)).written;
    const name = encoder.encodeInto(e.key || "", buf.subarray(code)).written;
    handled(e, app.jamb_key(down, code, name, mods(e), e.repeat ? 1 : 0, e.isComposing || e.keyCode === 229 ? 1 : 0));
  };
  const beforeInput = (e) => handled(e, app.jamb_before_input(write(e.inputType || "").written));
  // edited delivers the textarea's text and selection after an edit: an input
  // event, or the end of a composition. The textarea holds the text field's
  // text around the caret, which the app compares with what it held, or else
  // only what was typed, which the app takes and the textarea then loses.
  const edited = (ended) => {
    const n = send(area.value);
    if (app.jamb_input(n, area.selectionStart, area.selectionEnd, backward(), ended)) area.value = "";
    app.go_scheduler();
  };
  const input = () => edited(0);
  // composition delivers a step of an input method's composition, as the app
  // numbers them: its start, an update and its end.
  const composition = (kind) => (e) => {
    if (app.jamb_composition(kind, write(e.data || "").written)) area.value = "";
    app.go_scheduler();
    if (kind === 2) edited(1);
  };
  // selected delivers a change of the textarea's selection, as a phone
  // keyboard's cursor movement makes.
  const selected = () => {
    if (document.activeElement !== area) return;
    app.jamb_select(area.selectionStart, area.selectionEnd, backward());
    app.go_scheduler();
  };
  const frame = (t) => {
    if (closed) return;
    app.jamb_frame(t);
    app.go_scheduler();
  };
  const prevent = (e) => e.preventDefault();
  const px = (v) => v.toFixed(2) + "px";

  globalThis.jambHost = {
    // The version of the calls between the script and the app, which the app
    // checks (hostVersion).
    version: 3,
    // attach gives the script the app's instance. A build that does not export
    // what the script calls, such as one made with Go, listens for itself.
    attach(instance) {
      const e = instance.exports;
      app = e.jamb_frame && e.jamb_pointer && e.jamb_wheel && e.jamb_key && e.jamb_before_input && e.jamb_input &&
        e.jamb_text_part && e.jamb_select && e.jamb_composition && e.go_scheduler && e.memory ? e : null;
    },
    get ready() { return app !== null; },

    // The app calls the rest.

    // listen delivers the pointer, wheel and key events of the canvas c. The
    // app reads an event's strings in the textSize bytes at textAddress of its
    // memory. A press starts on the canvas, and its moves and its release are
    // heard on the whole window, so a drag goes on outside the canvas.
    listen(c, textAddress, size) {
      canvas = c;
      text = textAddress;
      textSize = size;
      closed = false;
      on(c, "pointerdown", pointer(0));
      on(window, "pointermove", pointer(1));
      on(window, "pointerup", pointer(2));
      on(window, "pointercancel", pointer(3));
      const leave = pointer(4);
      // The textarea over a text field (see place) is the canvas's.
      on(c, "pointerleave", (e) => { if (!area || e.relatedTarget !== area) leave(e); });
      // Not passive, so that the app can keep the page from scrolling and zooming.
      on(c, "wheel", wheel, { passive: false });
      on(c, "keydown", key(1));
      on(c, "keyup", key(0));
      on(c, "contextmenu", prevent);
      // The mouse events that a browser makes from a touch would move the focus
      // to the canvas, away from the textarea, which hides a phone's keyboard.
      on(c, "mousedown", prevent);
    },
    // text delivers the key events of the app's hidden textarea, and the text
    // typed in it.
    text(element) {
      area = element;
      on(element, "keydown", key(1));
      on(element, "keyup", key(0));
      on(element, "beforeinput", beforeInput);
      on(element, "input", input);
      on(element, "compositionstart", composition(0));
      on(element, "compositionupdate", composition(1));
      on(element, "compositionend", composition(2));
      on(document, "selectionchange", selected);
      // Over a text field, on iOS, the textarea takes the touches and clicks
      // there, which are the canvas's (see place).
      on(element, "pointerdown", pointer(0));
      on(element, "wheel", wheel, { passive: false });
      on(element, "mousedown", prevent);
      on(element, "contextmenu", prevent);
    },
    // place moves the textarea to a rectangle of the canvas, in CSS pixels.
    // With font 0 the rectangle is the caret, where an input method puts its
    // window. Otherwise it is where a text field shows its text, on iOS, whose
    // keyboard's trackpad moves the caret by where the text is: the textarea
    // lays its text out as the field does, with text of the size font, lines
    // line apart, wrapped when wrap is 1, and scrolled by scrollX and scrollY,
    // and it takes the touches there. Text smaller than 16 px, at which iOS
    // zooms the page in when the textarea takes the focus, is 16 px, and the
    // textarea is scaled down.
    place(x, y, width, height, font, line, scrollX, scrollY, wrap) {
      const r = canvas.getBoundingClientRect();
      const s = area.style;
      s.left = px(r.left + x);
      s.top = px(r.top + y);
      if (!font) {
        s.width = px(width);
        s.height = px(height);
        s.fontSize = "16px";
        s.lineHeight = "normal";
        s.transform = "none";
        s.whiteSpace = "pre";
        s.pointerEvents = "none";
        return;
      }
      const k = Math.min(font / 16, 1);
      s.width = px(width / k);
      s.height = px(height / k);
      s.fontSize = px(Math.max(font, 16));
      s.lineHeight = px(line / k);
      s.transform = "scale(" + k.toFixed(4) + ")";
      s.whiteSpace = wrap ? "pre-wrap" : "pre";
      s.pointerEvents = "auto";
      area.scrollLeft = scrollX / k;
      area.scrollTop = scrollY / k;
    },
    // textPart and setText write a string of the app's to the textarea: the
    // app gives its pieces to textPart, and its last piece, n bytes at the start
    // of its buffer, to setText, which writes the string when value is 1, and
    // selects from start to end, in UTF-16 code units, with the anchor at end
    // when backward is 1.
    textPart(n) { incoming += decoder.decode(buffer().subarray(0, n)); },
    setText(n, start, end, backward, value) {
      if (value) {
        const s = incoming + decoder.decode(buffer().subarray(0, n));
        if (area.value !== s) area.value = s;
      }
      incoming = "";
      area.setSelectionRange(start, end, backward ? "backward" : "forward");
    },
    // frame delivers the next display frame.
    frame() { requestAnimationFrame(frame); },
    // close stops the deliveries: the window closed.
    close() {
      closed = true;
      for (const remove of removers) remove();
      removers = [];
    },
  };
})();
