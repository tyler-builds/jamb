// jamb.js is the host script of a page that runs a jamb app: a TinyGo build
// (see build.sh). A page loads it before the app, and gives it the app's
// instance before the app runs:
//
//	jambHost.attach(result.instance);
//	await go.run(result.instance);
//
// The app then has the script deliver the display's frames, the pointer, wheel
// and key events of its canvas, and the text typed in its hidden textarea. The
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
  // The text typed in the textarea, which the app takes and the textarea then
  // loses. Text longer than the app's buffer goes in pieces.
  const input = (e) => {
    if (e.isComposing) return;
    let s = area.value, empty = 0;
    do {
      const { read, written } = write(s);
      empty = app.jamb_input(written);
      s = s.slice(read);
    } while (s !== "" && empty);
    if (empty) area.value = "";
    app.go_scheduler();
  };
  // composition delivers a step of an input method's composition, as the app
  // numbers them: its start, an update and its end.
  const composition = (kind) => (e) => {
    if (app.jamb_composition(kind, write(e.data || "").written)) area.value = "";
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
    version: 1,
    // attach gives the script the app's instance. A build that does not export
    // what the script calls, such as one made with Go, listens for itself.
    attach(instance) {
      const e = instance.exports;
      app = e.jamb_frame && e.jamb_pointer && e.jamb_wheel && e.jamb_key && e.jamb_before_input && e.jamb_input &&
        e.jamb_composition && e.go_scheduler && e.memory ? e : null;
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
      on(c, "pointerleave", pointer(4));
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
    },
    // place moves the textarea to a rectangle of the canvas, in CSS pixels:
    // the caret, where an input method puts its window.
    place(x, y, width, height) {
      const r = canvas.getBoundingClientRect();
      const s = area.style;
      s.left = px(r.left + x);
      s.top = px(r.top + y);
      s.width = px(width);
      s.height = px(height);
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
