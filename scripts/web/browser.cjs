// browser.cjs checks a page that build.sh made, in Chromium (see browser.sh):
//
//   - the app draws with WebGL 2, and its frame matches the CPU renderer's,
//     which the page asks for with jambEnv (see web.Getenv), within a small
//     tolerance for rounding;
//   - the page's host script delivers the app's frames and input;
//   - a click in a text field gives the hidden textarea the focus, typing
//     there reaches the field, and the field's caret blinks;
//   - the textarea holds the field's text and caret, so a phone's keyboard can
//     correct and complete words: the edits it makes there, as iOS Safari
//     sends them, and an input method's composition, reach the field, and the
//     app's own edits and caret come back to the textarea;
//   - on an iPhone (its user agent), an editable element lies over the focused
//     field and lays its text out as the field does: in the page of fields
//     (internal/webcheck/fields), each case of overlayCases is typed into a
//     field, and the element is scanned as iOS's keyboard trackpad hit-tests
//     it, along each line, against where the field puts its caret at every
//     character boundary, which the page asks the app for
//     (jambEnv.JAMB_TEXT_LAYOUT).
//
// Usage: node browser.cjs DIR X Y FIELDS, where DIR is the page and X, Y is a
// point of a text field in its first view, in CSS pixels of a 1280×800 page,
// and FIELDS is the page of fields.
"use strict";

const http = require("http");
const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");

const [dir, fieldX, fieldY, fieldsDir] = [process.argv[2], Number(process.argv[3]), Number(process.argv[4]), process.argv[5]];
const types = { ".html": "text/html", ".js": "text/javascript", ".wasm": "application/wasm" };

// The largest difference allowed between the renderers, per channel, and how
// many pixels may differ by more than 2.
const maxDelta = 8;
const maxOff = 0.001;

function serve(root = dir) {
  const server = http.createServer((req, res) => {
    const name = path.join(root, decodeURIComponent(new URL(req.url, "http://x").pathname));
    const file = name.endsWith(path.sep) ? path.join(name, "index.html") : name;
    fs.readFile(file, (err, data) => {
      if (err) {
        res.writeHead(404).end();
        return;
      }
      res.writeHead(200, { "Content-Type": types[path.extname(file)] || "application/octet-stream" }).end(data);
    });
  });
  return new Promise((resolve) => server.listen(0, "127.0.0.1", () => resolve(server)));
}

// load opens the page, with the CPU renderer when cpu is set, and returns it
// once the app has drawn and the page is still.
async function load(browser, url, cpu) {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));
  page.on("console", (m) => m.type() === "error" && errors.push(m.text()));
  if (cpu) await page.addInitScript(() => { window.jambEnv = { JAMB_RENDERER: "cpu" }; });
  await page.goto(url);
  await page.waitForSelector("canvas", { timeout: 60000 });
  await page.waitForTimeout(2000); // the first frames, and the fonts' glyphs
  if (errors.length) throw new Error("the page reported errors:\n" + errors.join("\n"));
  return page;
}

// compare returns the largest channel difference of two PNG screenshots and the
// share of pixels that differ by more than 2, decoded in the page.
function compare(page, a, b) {
  return page.evaluate(async ([a, b]) => {
    const decode = async (b64) => {
      const img = await createImageBitmap(await (await fetch("data:image/png;base64," + b64)).blob());
      const c = new OffscreenCanvas(img.width, img.height);
      const ctx = c.getContext("2d");
      ctx.drawImage(img, 0, 0);
      return ctx.getImageData(0, 0, img.width, img.height).data;
    };
    const [pa, pb] = [await decode(a), await decode(b)];
    if (pa.length !== pb.length) return { max: 255, off: 1 };
    let max = 0, off = 0;
    for (let i = 0; i < pa.length; i += 4) {
      let d = 0;
      for (let k = 0; k < 3; k++) d = Math.max(d, Math.abs(pa[i + k] - pb[i + k]));
      max = Math.max(max, d);
      if (d > 2) off++;
    }
    return { max, off: off / (pa.length / 4) };
  }, [a.toString("base64"), b.toString("base64")]);
}

// cdp returns a DevTools session of the page, for an input method's
// composition, which Playwright's keyboard does not make.
const cdp = (page) => page.context().newCDPSession(page);

// mirrors checks the focused field's textarea after "jamb" was typed in it,
// and reports whether it passed.
async function mirrors(page, sessionPromise) {
  const session = await sessionPromise;
  await page.evaluate(() => {
    // Count the writes of the app's text to the textarea.
    const host = globalThis.jambHost, set = host.setText;
    globalThis.jambWrites = 0;
    host.setText = function (n, start, end, backward, value) {
      const area = document.activeElement, before = area.value;
      set.call(this, n, start, end, backward, value);
      if (area.value !== before) globalThis.jambWrites++;
    };
  });
  // iosEdit replaces the last back characters of the textarea's text with
  // text, as iOS Safari does for a correction, a double space or a
  // suggestion: a beforeinput event, the edit, and an input event.
  const iosEdit = (back, text, inputType) => page.evaluate(([back, text, inputType]) => {
    const a = document.activeElement, e = a.value.length, s = e - back;
    const opts = { inputType, data: text, bubbles: true, cancelable: true };
    if (!a.dispatchEvent(new InputEvent("beforeinput", opts))) return;
    a.value = a.value.slice(0, s) + text;
    a.setSelectionRange(s + text.length, s + text.length);
    a.dispatchEvent(new InputEvent("input", opts));
  }, [back, text, inputType]);
  const state = () => page.evaluate(() => {
    const a = document.activeElement;
    return { tag: a.tagName, value: a.value, caret: a.selectionEnd, writes: globalThis.jambWrites,
      autocorrect: a.getAttribute("autocorrect"), autocapitalize: a.getAttribute("autocapitalize") };
  });
  let ok = true;
  const expect = async (what, value, writes) => {
    await page.waitForTimeout(150);
    const st = await state();
    const pass = st.tag === "TEXTAREA" && st.value === value && st.caret === value.length && st.writes === writes;
    console.log(`${pass ? "ok" : "FAIL"}: ${what}: the textarea holds ${JSON.stringify(st.value)}, caret ${st.caret}, ` +
      `${st.writes} writes${pass ? "" : `; want ${JSON.stringify(value)}, caret at its end, ${writes} writes`}`);
    ok = ok && pass;
  };
  const st = await state();
  console.log("textarea attributes:", st.autocorrect, st.autocapitalize);
  if (st.autocorrect !== "on" || st.autocapitalize !== "sentences") {
    console.log("FAIL: the textarea of a prose field does not correct and capitalize");
    ok = false;
  }
  await expect("typed on the keyboard", "jamb", 0);
  await page.keyboard.type(" teh");
  await iosEdit(3, "the", "insertReplacementText");
  await expect("an autocorrection", "jamb the", 0);
  await page.keyboard.type(" ");
  await iosEdit(1, ". ", "insertText");
  await expect("a double space's period", "jamb the. ", 0);
  await iosEdit(0, "Wor", "insertText");
  await iosEdit(3, "World ", "insertReplacementText");
  await expect("a suggestion that completes a word", "jamb the. World ", 0);
  await iosEdit(6, "", "deleteWordBackward");
  await expect("a word deleted", "jamb the. ", 0);
  // Backspace on a keyboard is the app's: the field changes, and the textarea
  // follows it.
  await page.keyboard.press("Backspace");
  await expect("Backspace, which the app handles", "jamb the.", 1);
  // The keyboard's cursor movement moves the field's caret: Backspace there
  // deletes the character before it.
  await page.evaluate(() => {
    document.activeElement.setSelectionRange(4, 4);
    document.dispatchEvent(new Event("selectionchange"));
  });
  await page.keyboard.press("Backspace");
  await page.keyboard.press("End");
  await expect("the caret moved by the keyboard", "jam the.", 2);
  // An input method's composition.
  await session.send("Input.imeSetComposition", { text: "n", selectionStart: 1, selectionEnd: 1 });
  await session.send("Input.imeSetComposition", { text: "ni", selectionStart: 2, selectionEnd: 2 });
  await session.send("Input.insertText", { text: "你" });
  await expect("a composition", "jam the.你", 2);
  return ok;
}

// overlayCases are the texts typed into the fields of the page of fields, by
// the fields they go in (0 a text input, 1 a text area, 2 one with Markdown's
// marks, 3 one with text of 18 pixels), whose layout in the editable element
// is compared with the field's: plain text, which drifts from go-text's
// rounding of advances; tabs; characters no font of the page has, which the
// field draws as boxes; lines that wrap; paragraphs; and Markdown, whose code
// is monospace, whose syntax hides away from the caret, and whose headings
// are bold.
const overlayCases = [
  [[0, 1, 3], "The quick brown fox jumps over the lazy dog. To wove fox."],
  [[0, 1], "a\tb\tc  two  spaces\tend"],
  [[1], "Thanks 😀 that works 👍🏽 ok 🇺🇸 done 日本語のテキスト"],
  [[1, 3], "When the session expires the client retries the request once with a fresh token, and if that fails it signs the user out and shows why."],
  [[1], "first line\nsecond line is a bit longer than the first one\n\nafter a blank line"],
  [[2], "Use `ctx.Done()` and **bold** then *italic* and ~~gone~~ plain [link](https://x.y) end"],
  [[2], "# A heading line\n## Smaller heading\nplain after"],
  [[2], "```go\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n```\nafter the fence"],
  [[2], "- item one\n- item **two** here\n> quoted *text* here"],
  [[2], "A long paragraph with `inline code` and **bold words** and *italic ones* that wraps over several lines, with a [link](https://example.com) and `code` at the end."],
];

// overlayMax is the largest difference allowed, in CSS pixels, between where
// the element puts the boundary between two characters, the point at which
// caretPositionFromPoint goes from the one's offset to the other's, and the
// middle of the field's two carets there. The scan steps by a quarter pixel.
const overlayMax = 0.5;

// overlayScan scans the focused editable element along each line of the
// field's carets (from its data-jamb-carets) and returns the largest
// difference from them, where, and how many boundaries the element has on
// another line than the field.
function overlayScan() {
  const a = document.activeElement, st = a.style, saved = [st.height, st.width];
  const raw = a.getAttribute("data-jamb-carets");
  if (!raw) return null;
  const r0 = a.getBoundingClientRect();
  st.height = st.width = "3000px"; // the scan reaches lines out of the field's view
  const r = a.getBoundingClientRect();
  const off = (node, o) => {
    if (!a.contains(node)) return -1;
    const g = document.createRange();
    g.selectNodeContents(a);
    g.setEnd(node, o);
    return g.toString().length;
  };
  const lines = new Map();
  for (const t of raw.split(",")) {
    const [o, x, y] = t.split(" ").map(Number);
    const k = (r0.top + y).toFixed(2);
    if (!lines.has(k)) lines.set(k, []);
    lines.get(k).push([o, r0.left + x]);
  }
  const keys = [...lines.keys()].sort((p, q) => p - q);
  const lineOf = new Map();
  keys.forEach((k, i) => { for (const [o] of lines.get(k)) lineOf.set(o, i); });
  let worst = 0, where = "", moved = 0;
  keys.forEach((k, li) => {
    const cs = lines.get(k), xs = new Map(cs);
    const lo = Math.min(...cs.map((c) => c[1])) - 4, hi = Math.max(...cs.map((c) => c[1])) + 4;
    let prev = null;
    const runs = [];
    for (let x = lo; x <= hi; x += 0.25) {
      const cp = document.caretPositionFromPoint(x, Number(k));
      const o = cp ? off(cp.offsetNode, cp.offset) : -1;
      if (o !== prev) {
        runs.push([x, o]);
        prev = o;
      }
    }
    for (let i = 1; i < runs.length; i++) {
      const [xb, b] = runs[i], p = runs[i - 1][1];
      if (p < 0 || b <= p) continue;
      if (!xs.has(p) || !xs.has(b)) {
        // The offset of a soft wrap is on the end of one line and the start of the next.
        const lp = lineOf.get(p), lb = lineOf.get(b);
        if (!((lp === li || lp === li - 1) && (lb === li || lb === li + 1))) moved++;
        continue;
      }
      if (Math.abs(xs.get(p) - xs.get(b)) < 0.01) continue; // text of no width between, as hidden syntax
      const d = Math.abs(xb - (xs.get(p) + xs.get(b)) / 2);
      if (d > worst) {
        worst = d;
        where = `line ${li + 1}, between the offsets ${p} and ${b}`;
      }
    }
  });
  st.height = saved[0];
  st.width = saved[1];
  const g = document.createRange();
  g.selectNodeContents(a);
  return { worst, where, moved, text: g.toString() };
}

// overlay opens the page of fields as an iPhone's browser, types each case of
// overlayCases into its fields, and checks that the editable element over the
// field lays it out as the field does (see overlayScan). It reports whether
// every case passed.
async function overlay(browser, url) {
  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 3, hasTouch: true,
    isMobile: true, userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1" });
  await ctx.addInitScript(() => { window.jambEnv = Object.assign(window.jambEnv || {}, { JAMB_TEXT_LAYOUT: "1" }); });
  const page = await ctx.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));
  await page.goto(url);
  await page.waitForSelector("canvas", { timeout: 60000 });
  await page.waitForTimeout(2000);
  let ok = true;
  for (const [fields, text] of overlayCases) {
    for (const f of fields) {
      await page.touchscreen.tap(250, 16 + f * 120 + 12);
      await page.waitForTimeout(300);
      await page.evaluate(() => getSelection().selectAllChildren(document.activeElement));
      const lines = text.split("\n");
      for (let i = 0; i < lines.length; i++) {
        if (i) {
          await page.keyboard.press("Enter");
          await page.waitForTimeout(150);
        }
        if (lines[i]) await page.keyboard.insertText(lines[i]);
        else if (i === 0) await page.keyboard.press("Backspace");
      }
      // Selecting all of the field is the browser's change of the element's
      // selection, after which the element keeps its text where it is for
      // half a second (see browserSelSettle), as during the keyboard's trackpad.
      await page.waitForTimeout(700);
      await page.evaluate(() => document.fonts.ready);
      const r = await page.evaluate(overlayScan);
      const pass = r && r.worst <= overlayMax && r.moved === 0 && r.text === text;
      ok = ok && pass;
      const name = JSON.stringify(text.length > 40 ? text.slice(0, 40) + "…" : text);
      if (!r) console.log(`FAIL: field ${f}, ${name}: the element is not laid out from the field (no data-jamb-carets)`);
      else console.log(`${pass ? "ok" : "FAIL"}: field ${f}, ${name}: boundaries within ${r.worst.toFixed(2)} px` +
        (r.worst > overlayMax ? ` (${r.where})` : "") + (r.moved ? `, ${r.moved} on another line` : "") +
        (r.text !== text ? `, and the field holds ${JSON.stringify(r.text)}` : ""));
    }
  }
  if (errors.length) {
    console.log("FAIL: the page of fields reported errors:\n" + errors.join("\n"));
    ok = false;
  }
  await ctx.close();
  return ok;
}

async function main() {
  const server = await serve();
  const url = `http://127.0.0.1:${server.address().port}/`;
  // Without a GPU, Chromium draws WebGL with SwiftShader, on the CPU.
  const browser = await chromium.launch({ args: ["--enable-unsafe-swiftshader", "--ignore-gpu-blocklist"] });
  let failed = false;
  try {
    const gl = await load(browser, url, false);
    const context = await gl.evaluate(() => {
      const c = document.querySelector("canvas");
      return c.getContext("webgl2") ? "webgl2" : c.getContext("2d") ? "2d" : "none";
    });
    console.log("renderer:", context);
    if (context !== "webgl2") {
      console.log("FAIL: the page does not draw with WebGL 2");
      failed = true;
    }
    // The host script delivers the app's frames and input (see jamb.js);
    // without it the app pauses to collect memory as the pointer moves.
    if (!(await gl.evaluate(() => Boolean(globalThis.jambHost && globalThis.jambHost.ready)))) {
      console.log("FAIL: the page's host script, jamb.js, did not find the app's exports");
      failed = true;
    }
    const shotGL = await gl.screenshot();
    const cpu = await load(browser, url, true);
    const shotCPU = await cpu.screenshot();
    const { max, off } = await compare(cpu, shotGL, shotCPU);
    console.log(`WebGL 2 against the CPU: largest difference ${max}, ${(off * 100).toFixed(3)}% of pixels off by more than 2`);
    if (max > maxDelta || off > maxOff) {
      fs.writeFileSync("webgl.png", shotGL);
      fs.writeFileSync("cpu.png", shotCPU);
      console.log("FAIL: WebGL 2 draws different pixels from the CPU renderer (see webgl.png and cpu.png)");
      failed = true;
    }

    // A click in the text field moves the page's focus to the textarea, and
    // the text typed there reaches the field, which draws it.
    const before = await gl.screenshot({ clip: { x: fieldX - 100, y: fieldY - 16, width: 240, height: 32 } });
    await gl.mouse.click(fieldX, fieldY);
    await gl.waitForTimeout(300);
    const focused = await gl.evaluate(() => document.activeElement.tagName);
    await gl.keyboard.type("jamb");
    await gl.waitForTimeout(300);
    const after = await gl.screenshot({ clip: { x: fieldX - 100, y: fieldY - 16, width: 240, height: 32 } });
    console.log("focus after a click in the text field:", focused);
    if (focused !== "TEXTAREA") {
      console.log("FAIL: a click in a text field does not give the textarea the focus");
      failed = true;
    } else if (before.equals(after)) {
      console.log("FAIL: the text typed does not show in the field");
      failed = true;
    }

    // The textarea mirrors the field (see internal/platform/web/mirror.go).
    // Each step edits the textarea, or the field through the app, and the
    // textarea must then hold what the field holds: the app writes its text to
    // the textarea when they differ, which is counted.
    if (!(await mirrors(gl, cdp(gl)))) failed = true;

    // The focused field's caret blinks, on and then off for 530 ms each: the
    // field draws differently within a blink. The page's focus moving to the
    // textarea once made the app take its window as inactive, which hides
    // the caret.
    const clip = { x: fieldX - 100, y: fieldY - 16, width: 240, height: 32 };
    const first = await gl.screenshot({ clip });
    let blinks = false;
    for (let i = 0; i < 8 && !blinks; i++) {
      await gl.waitForTimeout(150);
      blinks = !first.equals(await gl.screenshot({ clip }));
    }
    console.log("caret blinks in the focused text field:", blinks);
    if (!blinks) {
      console.log("FAIL: the focused text field's caret does not blink");
      failed = true;
    }

    // On an iPhone the editable element lies over the field, laid out as the
    // field lays its text out (see overlay).
    const fieldsServer = await serve(fieldsDir);
    try {
      if (!(await overlay(browser, `http://127.0.0.1:${fieldsServer.address().port}/`))) failed = true;
    } finally {
      fieldsServer.close();
    }
  } finally {
    await browser.close();
    server.close();
  }
  process.exit(failed ? 1 : 0);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
