### Changed

- A `js` build (a web page, or Node) runs the code editor's highlighter in the JavaScript host's
  WebAssembly instead of wazero, so it no longer links wazero, and TinyGo can build apps that import
  `codeedit` or `markdown/mdcode`. Native builds still use wazero.
