// jamb.js is the host script of a page that runs a jamb app: a TinyGo build
// (see build.sh). A page loads it before the app, and gives it the app's
// instance before the app runs:
//
//	jambHost.attach(result.instance);
//	await go.run(result.instance);
//
// The app then has the script deliver the display's frames, the pointer, wheel
// and key events of its canvas, and the edits and the selection of its hidden
// textarea, or on iOS its editable element, to which the script writes the
// focused text field's text, and lays it out (see layout). The script calls functions that the app exports, with numbers, and then runs the
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
  // The element that takes text input is a textarea, or, on iOS, an editable
  // element (contenteditable, plain text only) that lies over the focused
  // field and lays its text out as the field does, from the field's runs (see
  // layout, and internal/platform/web/editable.go). Its text is that of its
  // text nodes, in order; its spans hold nothing else, and the <br> a browser
  // may put in an empty one is not text. Offsets are in UTF-16 code units of
  // that text, as a textarea's are.
  let editable = false;
  let content = null; // the editable element's box of spans
  let families = []; // the font-family list and variations of each face of the runs
  let placed = "position:absolute;left:0;top:0;white-space:pre"; // the box's style, from place
  let pending = null; // a selection to make when the element takes the focus
  // contentOf returns the box of spans, made again, where place put it, if an
  // edit took it out.
  const contentOf = () => {
    if (!content || content.parentNode !== area) {
      content = document.createElement("div");
      content.style.cssText = placed;
      area.textContent = "";
      area.appendChild(content);
    }
    return content;
  };
  // readText returns the text the element holds.
  const readText = () => {
    if (!editable) return area.value;
    const r = document.createRange();
    r.selectNodeContents(area);
    return r.toString();
  };
  // offsetOf returns the offset in readText of a position in the element.
  const offsetOf = (node, offset) => {
    const r = document.createRange();
    r.selectNodeContents(area);
    r.setEnd(node, offset);
    return r.toString().length;
  };
  // trailingBreak gives the box a <br> after its text when the text ends with
  // a line break, as an editor's must: a line break at the end of a box makes
  // no line of its own, and a browser moves a caret after it back before it,
  // where the next character typed would go. The <br> is no text.
  const trailingBreak = () => {
    const box = contentOf(), last = box.lastChild;
    const want = readText().endsWith("\n");
    const has = last && last.nodeName === "BR" && last.dataset.end;
    if (want && !has) {
      const br = document.createElement("br");
      br.dataset.end = "1";
      box.appendChild(br);
    } else if (!want && has) last.remove();
  };
  // positionAt returns the position in the element of the offset u of its
  // text: in the text node it falls in, at the end of a span of text rather
  // than at the start of the next, unless that span holds a tab, a line break
  // or hidden text; after a line break that ends the text, before the <br>
  // that follows it (see trailingBreak).
  const positionAt = (u) => {
    const box = contentOf(), br = box.lastChild;
    if (br && br.nodeName === "BR" && br.dataset.end && u >= readText().length) {
      return [box, box.childNodes.length - 1];
    }
    const walk = document.createTreeWalker(area, NodeFilter.SHOW_TEXT);
    let at = 0, last = null;
    for (let n = walk.nextNode(); n; n = walk.nextNode()) {
      const len = n.data.length;
      if (u < at + len || u === at + len && !(n.parentNode.dataset && n.parentNode.dataset.k > 0)) {
        return [n, u - at];
      }
      at += len;
      last = n;
    }
    return last ? [last, last.data.length] : [contentOf(), 0];
  };
  // selection returns the element's selection: its start, its end, and 1 when
  // its anchor is its end.
  const selection = () => {
    if (!editable) return [area.selectionStart, area.selectionEnd, area.selectionDirection === "backward" ? 1 : 0];
    const sel = getSelection();
    if (!sel.rangeCount || !area.contains(sel.anchorNode) || !area.contains(sel.focusNode)) {
      const n = readText().length;
      return [n, n, 0];
    }
    const a = offsetOf(sel.anchorNode, sel.anchorOffset), f = offsetOf(sel.focusNode, sel.focusOffset);
    return [Math.min(a, f), Math.max(a, f), f < a ? 1 : 0];
  };
  // select selects from start to end, with the anchor at end when backward is
  // 1. The editable element's selection is the page's, which a browser gives
  // the focus with: it is made when the element has the focus (see focused).
  const select = (start, end, backward) => {
    if (!editable) {
      area.setSelectionRange(start, end, backward ? "backward" : "forward");
      return;
    }
    if (document.activeElement !== area) {
      pending = [start, end, backward];
      return;
    }
    pending = null;
    const [an, ao] = positionAt(backward ? end : start), [fn, fo] = positionAt(backward ? start : end);
    getSelection().setBaseAndExtent(an, ao, fn, fo);
  };
  // focused makes the selection the app gave the editable element while it
  // did not have the focus, before the browser reports where it put the caret.
  const focused = () => {
    if (pending) select(...pending);
  };
  // clear empties the element.
  const clear = () => {
    if (editable) contentOf().textContent = "";
    else area.value = "";
  };
  // styleOf returns the style of a span for a run of the kind k (0 text, 1 a
  // tab, 2 a line break, 3 nothing), in the face f, with the spacing sp and
  // the width w.
  const styleOf = (k, f, sp, w) => {
    let st = "vertical-align:top";
    if (k === 0 && families[f]) st += ";font-family:" + families[f][0] + ";font-variation-settings:" + families[f][1];
    if (k === 0 && sp) st += ";letter-spacing:" + sp + "px";
    if (k === 1) st += ";display:inline-block;overflow:hidden;width:" + w + "px;tab-size:" + w + "px";
    if (k === 3) st += ";font-size:0";
    return st;
  };
  // layout splits the element's text into the spans of the runs that desc
  // describes (see layoutDesc in internal/platform/web/editable.go). The
  // spans whose text is as it should be stay, with their text nodes, so that
  // an edit the browser made in one stands; the others are made anew, and
  // the selection is kept.
  const layout = (desc) => {
    const lines = desc.split("\n");
    const nf = Number(lines[0]);
    families = [];
    for (let i = 1; i <= nf; i++) families.push(lines[i].split("\t"));
    const runs = [];
    for (let i = nf + 1; i < lines.length; i++) {
      const [k, len, f, sp, w] = lines[i].split(" ").map(Number);
      runs.push({ k, len, st: styleOf(k, f, sp, w) });
    }
    const text = readText();
    if (runs.reduce((n, r) => n + r.len, 0) !== text.length) return; // the app's text comes first
    const box = contentOf();
    const end = box.lastChild;
    if (end && end.nodeName === "BR" && end.dataset.end) end.remove();
    let clean = area.childNodes.length === 1;
    const spans = [];
    for (const n of box.childNodes) {
      if (n.nodeName !== "SPAN" || !n.dataset.k || n.childNodes.length > 1 ||
        n.firstChild && n.firstChild.nodeType !== Node.TEXT_NODE) clean = false;
      spans.push(n);
    }
    const want = [];
    let at = 0;
    for (const r of runs) {
      want.push(text.slice(at, at + r.len));
      at += r.len;
    }
    let from = 0, to = 0; // the spans kept at the start and at the end
    if (clean) {
      while (from < spans.length && from < want.length && spans[from].textContent === want[from] &&
        spans[from].dataset.k === String(runs[from].k)) from++;
      while (to < spans.length - from && to < want.length - from &&
        spans[spans.length - 1 - to].textContent === want[want.length - 1 - to] &&
        spans[spans.length - 1 - to].dataset.k === String(runs[runs.length - 1 - to].k)) to++;
    }
    const remake = !clean || from + to < Math.max(spans.length, want.length);
    const sel = remake && document.activeElement === area ? selection() : null;
    if (!clean) {
      for (const n of [...area.childNodes]) if (n !== box) n.remove();
      box.textContent = "";
    }
    const keep = clean ? spans : [];
    const after = clean ? keep[keep.length - to] || null : null;
    for (let i = clean ? from : 0; i < keep.length - to; i++) keep[i].remove();
    for (let i = clean ? from : 0; i < want.length - (clean ? to : 0); i++) {
      const sp = document.createElement("span");
      sp.dataset.k = String(runs[i].k);
      sp.textContent = want[i];
      box.insertBefore(sp, after);
    }
    const all = box.childNodes;
    for (let i = 0; i < runs.length; i++) {
      if (all[i].getAttribute("style") !== runs[i].st) all[i].setAttribute("style", runs[i].st);
    }
    trailingBreak();
    if (sel) select(sel[0], sel[1], sel[2]);
  };
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
    const n = send(readText());
    const [start, end, back] = selection();
    if (app.jamb_input(n, start, end, back, ended)) clear();
    app.go_scheduler();
  };
  const input = () => edited(0);
  // composition delivers a step of an input method's composition, as the app
  // numbers them: its start, an update and its end.
  const composition = (kind) => (e) => {
    if (app.jamb_composition(kind, write(e.data || "").written)) clear();
    app.go_scheduler();
    if (kind === 2) edited(1);
  };
  // selected delivers a change of the textarea's selection, as a phone
  // keyboard's cursor movement makes.
  const selected = () => {
    if (document.activeElement !== area) return;
    const [start, end, back] = selection();
    app.jamb_select(start, end, back);
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
      editable = element.isContentEditable || element.getAttribute("contenteditable") !== null;
      if (editable) on(element, "focus", focused);
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
    // place moves the textarea, or the editable element, to a rectangle of
    // the canvas, in CSS pixels. With font 0 the rectangle is the caret, where
    // an input method puts its window. Otherwise it is where a text field
    // shows its text, on iOS, whose keyboard's trackpad moves the caret by
    // where the text is: the editable element lays its text out as the field
    // does (see layout), with text of the size font, lines line apart,
    // wrapped at the width wrap (0 for none), the field's text scrolled by
    // scrollX and scrollY and the element's first line top below the top of
    // the field's text, and it takes the touches there. The element's own
    // text size stays 16 px, under which iOS zooms the page in when it takes
    // the focus.
    place(x, y, width, height, font, line, scrollX, scrollY, wrap, top) {
      const r = canvas.getBoundingClientRect();
      const s = area.style;
      s.left = px(r.left + x);
      s.top = px(r.top + y);
      s.width = px(width);
      s.height = px(height);
      s.pointerEvents = font ? "auto" : "none";
      if (!editable) return;
      placed = "position:absolute;left:0;top:0;white-space:pre;font-size:16px";
      if (font) {
        placed = "position:absolute;left:" + px(-scrollX) + ";top:" + px(top - scrollY) +
          ";width:" + (wrap ? wrap.toFixed(3) + "px" : "auto") + ";white-space:" + (wrap ? "pre-wrap" : "pre") +
          ";font-size:" + font.toFixed(4) + "px;line-height:" + line.toFixed(4) + "px";
      }
      contentOf().style.cssText = placed;
      area.scrollLeft = area.scrollTop = 0;
    },
    // layout lays the editable element's text out as the string the app
    // gives it, in pieces as setText's, describes (see layout above).
    layout(n) {
      const desc = incoming + decoder.decode(buffer().subarray(0, n));
      incoming = "";
      if (editable) layout(desc);
    },
    // clear empties the element, of what was typed into a field that shows
    // input methods no text.
    clear() { clear(); },
    // textPart and setText write a string of the app's to the textarea: the
    // app gives its pieces to textPart, and its last piece, n bytes at the start
    // of its buffer, to setText, which writes the string when value is 1, and
    // selects from start to end, in UTF-16 code units, with the anchor at end
    // when backward is 1.
    textPart(n) { incoming += decoder.decode(buffer().subarray(0, n)); },
    setText(n, start, end, backward, value) {
      if (value) {
        const s = incoming + decoder.decode(buffer().subarray(0, n));
        if (readText() !== s) {
          if (editable) {
            contentOf().textContent = s; // split into its runs by the layout that follows
            trailingBreak();
          } else area.value = s;
        }
      }
      incoming = "";
      select(start, end, backward);
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
