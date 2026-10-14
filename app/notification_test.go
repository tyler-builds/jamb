package app

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"image"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// parseXML reads every token of doc, and fails the test if the XML is not well
// formed. It returns the start elements in order, with their attributes.
func parseXML(t *testing.T, doc string) []xml.StartElement {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(doc))
	var starts []xml.StartElement
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return starts
		}
		if err != nil {
			t.Fatalf("toast XML is not well formed: %v\n%s", err, doc)
		}
		if se, ok := tok.(xml.StartElement); ok {
			starts = append(starts, se)
		}
	}
}

// texts returns the character data of every <text> element, in order.
func texts(t *testing.T, doc string) []string {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(doc))
	var out []string
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatalf("toast XML is not well formed: %v", err)
		}
		switch v := tok.(type) {
		case xml.StartElement:
			inText = v.Name.Local == "text"
		case xml.CharData:
			if inText {
				out = append(out, string(v))
			}
		case xml.EndElement:
			inText = false
		}
	}
}

func attr(se xml.StartElement, name string) (string, bool) {
	for _, a := range se.Attr {
		if a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}

func TestToastXMLEscapesEveryField(t *testing.T) {
	hostile := "a <b>&amp; \"q\" 'apos' ]]> </text><action content=\"x\"/> \x01 end"
	c := toastContent{
		Title:    hostile,
		Body:     hostile,
		ImageURI: `file:///C:/Users/x/a&b "c".png`,
		Protocol: true,
		Launch:   `jamb-x://activate?d=a&b<c>"d"`,
		Actions:  []toastAction{{Label: hostile, Arguments: `jamb-x://activate?d=a&b"`}},
	}
	doc := toastXML(c)
	starts := parseXML(t, doc)

	// The hostile markup did not add elements: only the expected ones are present.
	names := map[string]int{}
	for _, se := range starts {
		names[se.Name.Local]++
	}
	want := map[string]int{"toast": 1, "visual": 1, "binding": 1, "image": 1, "text": 2, "actions": 1, "action": 1}
	for name, n := range want {
		if names[name] != n {
			t.Errorf("element %s: %d times, want %d\n%s", name, names[name], n, doc)
		}
	}
	if len(starts) != 8 {
		t.Errorf("%d elements, want 8 (hostile text added markup?)\n%s", len(starts), doc)
	}

	// The text round trips, with the characters XML cannot hold replaced.
	got := texts(t, doc)
	const repl = "\uFFFD"
	wantText := strings.ReplaceAll(hostile, "\x01", repl)
	if len(got) != 2 || got[0] != wantText || got[1] != wantText {
		t.Errorf("text round trip = %q, want two copies of %q", got, wantText)
	}

	// Every attribute round trips.
	for _, se := range starts {
		switch se.Name.Local {
		case "image":
			if v, _ := attr(se, "src"); v != c.ImageURI {
				t.Errorf("image src = %q, want %q", v, c.ImageURI)
			}
		case "toast":
			if v, _ := attr(se, "launch"); v != c.Launch {
				t.Errorf("launch = %q, want %q", v, c.Launch)
			}
		case "action":
			if v, _ := attr(se, "content"); v != strings.ReplaceAll(hostile, "\x01", repl) {
				t.Errorf("action content = %q", v)
			}
			if v, _ := attr(se, "arguments"); v != c.Actions[0].Arguments {
				t.Errorf("action arguments = %q, want %q", v, c.Actions[0].Arguments)
			}
		}
	}
}

func TestToastXMLStructure(t *testing.T) {
	c := toastContent{
		Title: "Build finished", Body: "The build has 3 warnings.", ImageURI: "file:///C:/a.png",
		Protocol: true, Launch: "jamb-x://activate?d=e",
		Actions: []toastAction{{Label: "Open", Arguments: "u1"}, {Label: "Dismiss", Arguments: "u2"}},
		Silent:  true,
	}
	doc := toastXML(c)
	parseXML(t, doc)
	for _, s := range []string{
		`<toast activationType="protocol" launch="jamb-x://activate?d=e">`,
		`<image placement="hero" src="file:///C:/a.png"/>`,
		`<action content="Open" arguments="u1" activationType="protocol"/>`,
		`<action content="Dismiss" arguments="u2" activationType="protocol"/>`,
		`<audio silent="true"/>`,
	} {
		if !strings.Contains(doc, s) {
			t.Errorf("toast XML lacks %s\n%s", s, doc)
		}
	}
	// The hero image comes before the text, and the title before the body.
	if strings.Index(doc, "<image") > strings.Index(doc, "<text>") {
		t.Errorf("hero image comes after the text\n%s", doc)
	}
	if got := texts(t, doc); len(got) != 2 || got[0] != "Build finished" {
		t.Errorf("texts = %q", got)
	}
}

func TestToastXMLPlainNotification(t *testing.T) {
	doc := toastXML(toastContent{Title: "Saved"})
	parseXML(t, doc)
	// A plain toast has the title only, and none of the optional elements.
	for _, s := range []string{"activationType", "launch", "<image", "<actions", "<audio"} {
		if strings.Contains(doc, s) {
			t.Errorf("plain toast contains %s\n%s", s, doc)
		}
	}
	if !strings.Contains(doc, "<text>Saved</text>") {
		t.Errorf("plain toast lacks its title\n%s", doc)
	}
	if got := texts(t, doc); len(got) != 1 {
		t.Errorf("plain toast has %d text elements, want 1", len(got))
	}
}

func TestNotificationCheck(t *testing.T) {
	long := strings.Repeat("t", 65)
	tests := []struct {
		name string
		n    Notification
		ok   bool
	}{
		{"valid", Notification{Title: "Done", Actions: []NotificationAction{{ID: "a", Label: "A"}}}, true},
		{"empty title", Notification{Title: "  "}, false},
		{"tag of 64", Notification{Title: "x", Tag: strings.Repeat("t", 64)}, true},
		{"tag of 65", Notification{Title: "x", Tag: long}, false},
		{"six actions", Notification{Title: "x", Actions: make([]NotificationAction, 6)}, false},
		{"action without ID", Notification{Title: "x", Actions: []NotificationAction{{Label: "A"}}}, false},
		{"action without label", Notification{Title: "x", Actions: []NotificationAction{{ID: "a"}}}, false},
		{"duplicate action IDs", Notification{Title: "x", Actions: []NotificationAction{{ID: "a", Label: "A"}, {ID: "a", Label: "B"}}}, false},
		{"empty image", Notification{Title: "x", Image: image.NewRGBA(image.Rect(0, 0, 0, 0))}, false},
	}
	for _, tt := range tests {
		err := tt.n.check()
		if (err == nil) != tt.ok {
			t.Errorf("%s: check() = %v, want ok=%v", tt.name, err, tt.ok)
		}
	}
}

func TestActivationRoundTrip(t *testing.T) {
	const scheme = "jamb-jambexamples.notes"
	cases := []activationData{
		{Action: "open", Tag: "build", Args: "build-42", Nonce: "0123456789abcdef"},
		{Tag: "", Args: "", Nonce: "fedcba9876543210"}, // a click on the body
		{Action: "dismiss", Args: "unicode é 中文 &<>\"'", Nonce: "aa"},
		{Action: "a b&c=d?e#f", Tag: "t/u", Nonce: "bb"},
	}
	for _, d := range cases {
		uri, err := encodeActivation(scheme, d)
		if err != nil {
			t.Fatalf("encodeActivation(%+v): %v", d, err)
		}
		if !strings.HasPrefix(uri, scheme+"://activate?d=") {
			t.Errorf("URI %q has the wrong shape", uri)
		}
		got, err := decodeActivation(scheme, uri)
		if err != nil {
			t.Fatalf("decodeActivation(%q): %v", uri, err)
		}
		d.V = activationVersion
		if got != d {
			t.Errorf("round trip = %+v, want %+v", got, d)
		}
	}
}

// TestDecodeActivationRefusesHostileInput checks that data which is not a click of
// this app, or is damaged, is refused rather than delivered.
func TestDecodeActivationRefusesHostileInput(t *testing.T) {
	const scheme = "jamb-x"
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	valid := `{"v":1,"a":"open","n":"0123"}`
	tests := map[string]string{
		"empty":              "",
		"not a URI":          "::::",
		"other scheme":       "jamb-y://activate?d=" + enc(valid),
		"other host":         scheme + "://launch?d=" + enc(valid),
		"no data":            scheme + "://activate",
		"bad base64":         scheme + "://activate?d=%%%",
		"empty data":         scheme + "://activate?d=",
		"bad JSON":           scheme + "://activate?d=" + enc(`{"v":1,`),
		"unknown field":      scheme + "://activate?d=" + enc(`{"v":1,"n":"1","x":"y"}`),
		"trailing text":      scheme + "://activate?d=" + enc(valid+` {}`),
		"wrong version":      scheme + "://activate?d=" + enc(`{"v":2,"n":"1"}`),
		"missing version":    scheme + "://activate?d=" + enc(`{"n":"1"}`),
		"missing nonce":      scheme + "://activate?d=" + enc(`{"v":1,"a":"open"}`),
		"long nonce":         scheme + "://activate?d=" + enc(`{"v":1,"n":"`+strings.Repeat("n", 65)+`"}`),
		"not an object":      scheme + "://activate?d=" + enc(`["v"]`),
		"URI too long":       scheme + "://activate?d=" + enc(`{"v":1,"n":"1","g":"`+strings.Repeat("g", 600)+`"}`),
		"data too long":      scheme + "://activate?d=" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte("{"), 2000)),
		"wrong JSON type":    scheme + "://activate?d=" + enc(`{"v":"1","n":"1"}`),
		"action is a number": scheme + "://activate?d=" + enc(`{"v":1,"n":"1","a":7}`),
		"null body":          scheme + "://activate?d=" + enc(`null`),
		"empty nonce":        scheme + "://activate?d=" + enc(`{"v":1,"n":""}`),
	}
	for name, raw := range tests {
		if _, err := decodeActivation(scheme, raw); err == nil {
			t.Errorf("%s: decodeActivation accepted %q", name, raw)
		}
	}
}

func TestEncodeActivationLimit(t *testing.T) {
	_, err := encodeActivation("jamb-x", activationData{Nonce: "1", Args: strings.Repeat("a", 600)})
	if err == nil || !strings.Contains(err.Error(), "limit is 512") {
		t.Fatalf("encodeActivation over the limit = %v, want the limit error", err)
	}
	if _, err := encodeActivation("jamb-x", activationData{Nonce: "1", Args: "short"}); err != nil {
		t.Fatalf("encodeActivation within the limit: %v", err)
	}
}

func TestSplitActivation(t *testing.T) {
	const scheme = "jamb-x"
	args := []string{"--flag", "jamb-x://activate?d=abc", "file.txt", "jamb-x://activate?d=second"}
	raw, rest, found := splitActivation(scheme, args)
	if !found || raw != "jamb-x://activate?d=abc" {
		t.Fatalf("splitActivation found=%v raw=%q", found, raw)
	}
	if strings.Join(rest, "|") != "--flag|file.txt" {
		t.Errorf("rest = %q, want the arguments that are not activations", rest)
	}
	if _, rest, found := splitActivation(scheme, []string{"--new"}); found || len(rest) != 1 {
		t.Errorf("no activation: found=%v rest=%q", found, rest)
	}
	// A different scheme is an ordinary argument.
	if _, _, found := splitActivation(scheme, []string{"jamb-y://activate?d=x"}); found {
		t.Error("an activation of another scheme was taken")
	}
}

func TestActivationLogDeliversOnce(t *testing.T) {
	var l activationLog
	if !l.first("a") || l.first("a") {
		t.Fatal("a nonce is delivered once")
	}
	// The log keeps the last maxRecentActivations nonces, so an old nonce is new again
	// after that many others; the recent ones are still refused.
	for i := 0; i < maxRecentActivations; i++ {
		l.first("n" + strconv.Itoa(i))
	}
	if !l.first("a") {
		t.Error("an evicted nonce was refused")
	}
	if l.first("n" + strconv.Itoa(maxRecentActivations-1)) {
		t.Error("a recent nonce was delivered again")
	}
}

func TestPlanNotifyRegistrationWritesNothing(t *testing.T) {
	programs := filepath.Join(`C:\Users\x\AppData\Roaming`, "Microsoft", "Windows", "Start Menu", "Programs")
	exe := `C:\Apps\Notes Pro.exe`

	plain := planNotifyRegistration("JambExamples.Notes", exe, programs, nil, false)
	if plain.Action != shortcutCreate || plain.Shortcut != filepath.Join(programs, "Notes Pro.lnk") || plain.AppID != "JambExamples.Notes" || plain.Target != exe {
		t.Errorf("plan = %+v", plain)
	}
	if len(plain.Values) != 0 {
		t.Errorf("without a protocol the plan writes %d values, want none", len(plain.Values))
	}
	existing := []shortcutCandidate{{Path: filepath.Join(programs, "Notes Pro.lnk"), Target: exe, Writable: true}}
	stamp := planNotifyRegistration("JambExamples.Notes", exe, programs, existing, false)
	if stamp.Action != shortcutStamp || stamp.Shortcut != existing[0].Path {
		t.Errorf("a writable shortcut to the program: plan = %+v; want it stamped", stamp)
	}

	proto := planNotifyRegistration("JambExamples.Notes", exe, programs, nil, true)
	if len(proto.Values) != 3 {
		t.Fatalf("protocol plan has %d values, want 3", len(proto.Values))
	}
	key := `Software\Classes\jamb-jambexamples.notes`
	want := []regValue{
		{Key: key, Name: "", Value: "URL:jamb-jambexamples.notes"},
		{Key: key, Name: "URL Protocol", Value: ""},
		{Key: key + `\shell\open\command`, Name: "", Value: `"C:\Apps\Notes Pro.exe" "%1"`},
	}
	for i := range want {
		if proto.Values[i] != want[i] {
			t.Errorf("value %d = %+v, want %+v", i, proto.Values[i], want[i])
		}
	}
	for _, v := range proto.Values {
		if !strings.HasPrefix(v.Key, `Software\Classes\`) {
			t.Errorf("value written outside HKCU\\Software\\Classes: %q", v.Key)
		}
	}
}

func TestShortcutName(t *testing.T) {
	tests := map[string]string{
		`C:\Apps\Notes Pro.exe`: "Notes Pro",
		`C:\Apps\a<b>c:d.exe`:   "abcd",
		`C:\Apps\..exe`:         "JambExamples.Notes", // nothing left: the AppID
		`C:\Apps\CON"name".exe`: `CONname`,
		`C:\Apps\trailing .exe`: "trailing",
	}
	for exe, want := range tests {
		if got := shortcutName(exe, "JambExamples.Notes"); got != want {
			t.Errorf("shortcutName(%q) = %q, want %q", exe, got, want)
		}
	}
}

// TestChooseShortcut checks the rules of chooseShortcut. The paths are Windows paths, as
// the registration uses them, so the test runs on Windows only.
func TestChooseShortcut(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the paths are Windows paths")
	}
	const appID = "JambExamples.Notes"
	exe := `C:\Apps\Notes Pro.exe`
	created := `C:\Users\x\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\Notes Pro.lnk`
	userLnk := created
	allLnk := `C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Notes Pro\Notes Pro.lnk`
	tests := []struct {
		name   string
		found  []shortcutCandidate
		action shortcutAction
		path   string
	}{
		{"none found creates", nil, shortcutCreate, created},
		{"found writable without the AppID is stamped",
			[]shortcutCandidate{{Path: allLnk, Target: exe, Writable: true}}, shortcutStamp, allLnk},
		{"found writable with the AppID is kept",
			[]shortcutCandidate{{Path: userLnk, Target: exe, AppID: appID, Writable: true}}, shortcutKeep, userLnk},
		{"found not writable without the AppID falls back to creating",
			[]shortcutCandidate{{Path: allLnk, Target: exe}}, shortcutCreate, created},
		{"found not writable with the AppID is kept, since nothing needs writing",
			[]shortcutCandidate{{Path: allLnk, Target: exe, AppID: appID}}, shortcutKeep, allLnk},
		{"another executable's shortcut is ignored",
			[]shortcutCandidate{{Path: userLnk, Target: `C:\Apps\Other.exe`, Writable: true}}, shortcutCreate, created},
		{"a shortcut with another AppID is stamped",
			[]shortcutCandidate{{Path: userLnk, Target: exe, AppID: "Other.App", Writable: true}}, shortcutStamp, userLnk},
		{"the first writable shortcut is stamped, per-user before all-users",
			[]shortcutCandidate{{Path: userLnk, Target: exe, Writable: true}, {Path: allLnk, Target: exe, Writable: true}}, shortcutStamp, userLnk},
		{"an unwritable shortcut is passed over for a writable one",
			[]shortcutCandidate{{Path: allLnk, Target: exe}, {Path: userLnk, Target: exe, Writable: true}}, shortcutStamp, userLnk},
		{"a kept shortcut wins over an earlier one to stamp",
			[]shortcutCandidate{{Path: userLnk, Target: exe, Writable: true}, {Path: allLnk, Target: exe, AppID: appID}}, shortcutKeep, allLnk},
		{"the target's path is normalized",
			[]shortcutCandidate{{Path: userLnk, Target: `c:/apps/NOTES PRO.exe`, AppID: appID}}, shortcutKeep, userLnk},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, path := chooseShortcut(appID, exe, created, tt.found)
			if action != tt.action || path != tt.path {
				t.Errorf("chooseShortcut = %d, %q; want %d, %q", action, path, tt.action, tt.path)
			}
		})
	}
}

func TestSamePath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the paths are Windows paths")
	}
	same := [][2]string{
		{`C:\Apps\Notes Pro.exe`, `c:\apps\notes pro.exe`},
		{`C:\Apps\Notes Pro.exe`, `C:/Apps/Notes Pro.exe`},
		{`C:\Apps\x\..\Notes Pro.exe`, `C:\Apps\Notes Pro.exe`},
	}
	for _, p := range same {
		if !samePath(p[0], p[1]) {
			t.Errorf("samePath(%q, %q) = false; want true", p[0], p[1])
		}
	}
	different := [][2]string{
		{`C:\Apps\Notes Pro.exe`, `C:\Apps\Notes.exe`},
		{`C:\Apps\a.exe`, `D:\Apps\a.exe`},
		{`C:\Apps\a.exe`, ""},
	}
	for _, p := range different {
		if samePath(p[0], p[1]) {
			t.Errorf("samePath(%q, %q) = true; want false", p[0], p[1])
		}
	}
}

func TestNotificationImagePath(t *testing.T) {
	dir := t.TempDir()
	a := notificationImagePath(dir, []byte("one"))
	b := notificationImagePath(dir, []byte("one"))
	c := notificationImagePath(dir, []byte("two"))
	if a != b || a == c {
		t.Errorf("paths: %q %q %q; the same data must give the same path, different data another", a, b, c)
	}
	if filepath.Dir(a) != dir || !strings.HasSuffix(a, ".png") {
		t.Errorf("path %q is not a PNG in %q", a, dir)
	}
}

func TestWriteNotificationImage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "jamb-notifications")
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	path, err := writeNotificationImage(dir, img)
	if err != nil {
		t.Fatalf("writeNotificationImage: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("the file is not a PNG")
	}
	// The same image is saved once: the second call returns the same file.
	again, err := writeNotificationImage(dir, img)
	if err != nil || again != path {
		t.Fatalf("second write = %q, %v; want %q", again, err, path)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("%d files in the image folder, want 1 (temporary files left behind?)", len(entries))
	}
}

func TestFileURI(t *testing.T) {
	got := fileURI(filepath.Join(`C:\`, "tmp dir", "a&b.png"))
	if !strings.HasPrefix(got, "file:///C:/tmp%20dir/") || !strings.HasSuffix(got, "a&b.png") {
		t.Errorf("fileURI = %q", got)
	}
}
