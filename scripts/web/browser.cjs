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
//     app's own edits and caret come back to the textarea.
//
// Usage: node browser.cjs DIR X Y, where DIR is the page and X, Y is a point of
// a text field in its first view, in CSS pixels of a 1280×800 page.
"use strict";

const http = require("http");
const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");

const [dir, fieldX, fieldY] = [process.argv[2], Number(process.argv[3]), Number(process.argv[4])];
const types = { ".html": "text/html", ".js": "text/javascript", ".wasm": "application/wasm" };

// The largest difference allowed between the renderers, per channel, and how
// many pixels may differ by more than 2.
const maxDelta = 8;
const maxOff = 0.001;

function serve() {
  const server = http.createServer((req, res) => {
    const name = path.join(dir, decodeURIComponent(new URL(req.url, "http://x").pathname));
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

    // The caret blinks again when the page is shown again, as when a phone
    // comes back to the browser, though it had stopped blinking after 15
    // seconds without input (caretBlinkStop in ui/editor.go). A headless page
    // is never hidden, so its visibility is set by hand.
    await gl.evaluate(() => {
      globalThis.jambVisibility = "hidden";
      Object.defineProperty(document, "visibilityState", { configurable: true, get: () => globalThis.jambVisibility });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await gl.waitForTimeout(16000);
    await gl.evaluate(() => {
      globalThis.jambVisibility = "visible";
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await gl.waitForTimeout(100);
    const shown = await gl.screenshot({ clip });
    let again = false;
    for (let i = 0; i < 8 && !again; i++) {
      await gl.waitForTimeout(150);
      again = !shown.equals(await gl.screenshot({ clip }));
    }
    console.log("caret blinks again when the page is shown again:", again);
    if (!again) {
      console.log("FAIL: the caret does not blink when the page is shown again");
      failed = true;
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
