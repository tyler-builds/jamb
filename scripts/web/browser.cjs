// browser.cjs checks a page that build.sh made, in Chromium (see browser.sh):
//
//   - the app draws with WebGL 2, and its frame matches the CPU renderer's,
//     which the page asks for with jambEnv (see web.Getenv), within a small
//     tolerance for rounding;
//   - the page's host script delivers the app's frames and input;
//   - a click in a text field gives the hidden textarea the focus, and typing
//     there reaches the field.
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
