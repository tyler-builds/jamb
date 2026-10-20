// Command update downloads a release of @pierre/highlights from the npm registry
// and writes its WebAssembly module, license and tables into the lexer package:
//
//	go run ./update -version 0.1.3
//
// It is run by go generate in the lexer package's directory. It checks the
// tarball against the registry's integrity hash, and writes highlights.wasm,
// LICENSE-APACHE and tables.go, which holds the version, the module's SHA-256, the
// language names and the token names.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("update: ")
	version := flag.String("version", "", "the version of @pierre/highlights to download, such as 0.1.3")
	flag.Parse()
	if *version == "" {
		log.Fatal("-version is required")
	}
	if err := run(*version); err != nil {
		log.Fatal(err)
	}
}

func run(version string) error {
	client := &http.Client{Timeout: time.Minute}
	meta, err := get(client, "https://registry.npmjs.org/@pierre/highlights/"+version)
	if err != nil {
		return err
	}
	var pkg struct {
		Version string `json:"version"`
		Dist    struct {
			Tarball   string `json:"tarball"`
			Integrity string `json:"integrity"`
		} `json:"dist"`
	}
	if err := json.Unmarshal(meta, &pkg); err != nil {
		return fmt.Errorf("registry metadata: %w", err)
	}
	if pkg.Version != version {
		return fmt.Errorf("the registry returned version %q for %q", pkg.Version, version)
	}
	tgz, err := get(client, pkg.Dist.Tarball)
	if err != nil {
		return err
	}
	want, ok := strings.CutPrefix(pkg.Dist.Integrity, "sha512-")
	if !ok {
		return fmt.Errorf("unsupported integrity %q", pkg.Dist.Integrity)
	}
	sum := sha512.Sum512(tgz)
	if base64.StdEncoding.EncodeToString(sum[:]) != want {
		return errors.New("the tarball does not match the registry's integrity hash")
	}
	files, err := untar(tgz, "package/dist/highlights.wasm", "package/dist/languages.js", "package/dist/token-types.js", "package/LICENSE.md")
	if err != nil {
		return err
	}
	langs, err := parseLanguages(files["package/dist/languages.js"])
	if err != nil {
		return err
	}
	tokens, err := parseTokens(files["package/dist/token-types.js"])
	if err != nil {
		return err
	}
	wasm := files["package/dist/highlights.wasm"]
	src, err := tables(version, wasm, langs, tokens)
	if err != nil {
		return err
	}
	for name, data := range map[string][]byte{
		"highlights.wasm": wasm,
		"LICENSE-APACHE":  files["package/LICENSE.md"],
		"tables.go":       src,
	} {
		if err := os.WriteFile(name, data, 0o644); err != nil {
			return err
		}
	}
	log.Printf("wrote @pierre/highlights %s: %d bytes of WebAssembly, %d language names, %d tokens", version, len(wasm), len(langs), len(tokens))
	return nil
}

// get returns the body of a successful GET request.
func get(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// untar returns the named files of a gzipped tarball, and an error if one is missing.
func untar(tgz []byte, names ...string) (map[string][]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(zr)
	out := map[string][]byte{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if slices.Contains(names, h.Name) {
			if out[h.Name], err = io.ReadAll(tr); err != nil {
				return nil, err
			}
		}
	}
	for _, n := range names {
		if out[n] == nil {
			return nil, fmt.Errorf("the tarball has no %s", n)
		}
	}
	return out, nil
}

// language is one name or alias of a language and its module ID.
type language struct {
	name string
	id   int
}

var languageLine = regexp.MustCompile(`(?m)^\s*(?:"([^"]+)"|([A-Za-z0-9_$]+)):\s*(\d+),?\s*$`)

// parseLanguages reads the object literal of dist/languages.js: one name: id pair
// on each line.
func parseLanguages(js []byte) ([]language, error) {
	body, err := between(js, "const languages = {", "};")
	if err != nil {
		return nil, err
	}
	var out []language
	for _, m := range languageLine.FindAllStringSubmatch(body, -1) {
		id, _ := strconv.Atoi(m[3])
		out = append(out, language{name: m[1] + m[2], id: id})
	}
	if len(out) == 0 || out[0].name != "plain" || out[0].id != 0 {
		return nil, errors.New("languages.js: the table does not start with plain: 0")
	}
	return out, nil
}

var tokenLine = regexp.MustCompile(`(?m)^\s*"([^"]+)",?\s*$`)

// parseTokens reads the array literal of dist/token-types.js: one quoted name on
// each line, in the order of the token IDs.
func parseTokens(js []byte) ([]string, error) {
	body, err := between(js, "const tokenTypes = [", "];")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, m := range tokenLine.FindAllStringSubmatch(body, -1) {
		out = append(out, m[1])
	}
	if len(out) == 0 || out[0] != "none" {
		return nil, errors.New("token-types.js: the list does not start with none")
	}
	return out, nil
}

// between returns the text of js between start and the first end after it.
func between(js []byte, start, end string) (string, error) {
	s := string(js)
	i := strings.Index(s, start)
	if i < 0 {
		return "", fmt.Errorf("no %q", start)
	}
	s = s[i+len(start):]
	j := strings.Index(s, end)
	if j < 0 {
		return "", fmt.Errorf("no %q after %q", end, start)
	}
	return s[:j], nil
}

// tables returns the source of tables.go.
func tables(version string, wasm []byte, langs []language, tokens []string) ([]byte, error) {
	sum := sha256.Sum256(wasm)
	var b bytes.Buffer
	b.WriteString("// Code generated by go run ./update; DO NOT EDIT.\n\npackage lexer\n\n")
	fmt.Fprintf(&b, "// Version is the release of @pierre/highlights that highlights.wasm comes from.\nconst Version = %q\n\n", version)
	fmt.Fprintf(&b, "// wasmSHA256 is the SHA-256 of highlights.wasm, in hexadecimal.\nconst wasmSHA256 = %q\n\n", hex.EncodeToString(sum[:]))
	b.WriteString("// languageIDs maps each language name and alias, in lower case, to its ID in the module.\nvar languageIDs = map[string]int{\n")
	for _, l := range langs {
		fmt.Fprintf(&b, "\t%q: %d,\n", l.name, l.id)
	}
	b.WriteString("}\n\n// tokenNames are the names of the module's token IDs, in order. ID 0 is none.\nvar tokenNames = []string{\n")
	for _, t := range tokens {
		fmt.Fprintf(&b, "\t%q,\n", t)
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}
