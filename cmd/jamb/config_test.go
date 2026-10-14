package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validConfig is a settings file that passes every check.
func validConfig() config {
	return config{
		Name:             "jamb-demo",
		DisplayName:      "Jamb Demo",
		Description:      "A demo.",
		Version:          "0.1.0",
		Publisher:        "Jamb Examples",
		PublisherSubject: "CN=Jamb Examples",
		IdentityName:     "JambExamples.Demo",
		Icon:             "icon.png",
	}
}

func TestWindowsVersion(t *testing.T) {
	for in, want := range map[string]string{
		"0.1.0":      "0.1.0.0",
		"1.2.3":      "1.2.3.0",
		"10.0.65535": "10.0.65535.0",
	} {
		got, err := windowsVersion(in)
		if err != nil || got != want {
			t.Errorf("windowsVersion(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "1.2", "1.2.3.4", "1.2.3-beta", "1.2.3+build", "v1.2.3", "1.2.65536", "01.2.3", "1.-2.3", "1.2.x"} {
		if got, err := windowsVersion(in); err == nil {
			t.Errorf("windowsVersion(%q) = %q; want an error", in, got)
		}
	}
}

func TestConfigProblems(t *testing.T) {
	valid := validConfig()
	if p := valid.problems(); len(p) != 0 {
		t.Fatalf("valid config reported problems: %v", p)
	}
	cases := []struct {
		name   string
		mutate func(c *config)
		want   string
	}{
		{"name missing", func(c *config) { c.Name = "" }, `"name" is required`},
		{"name starts with a digit", func(c *config) { c.Name = "1demo" }, `"name" must start with a letter`},
		{"name has a space", func(c *config) { c.Name = "jamb demo" }, `"name" must start with a letter`},
		{"display name missing", func(c *config) { c.DisplayName = " " }, `"displayName" is required`},
		{"description missing", func(c *config) { c.Description = "" }, `"description" is required`},
		{"version missing", func(c *config) { c.Version = "" }, `"version" is required`},
		{"version with pre-release", func(c *config) { c.Version = "1.2.3-rc1" }, `"version" "1.2.3-rc1": each part must be a number`},
		{"publisher missing", func(c *config) { c.Publisher = "" }, `"publisher" is required`},
		{"subject without CN", func(c *config) { c.PublisherSubject = "Jamb Examples" }, `"publisherSubject" must start with "CN="`},
		{"subject with empty CN", func(c *config) { c.PublisherSubject = "CN=  " }, `"publisherSubject" must start with "CN="`},
		{"identity too short", func(c *config) { c.IdentityName = "ab" }, `"identityName" must be 3 to 50 characters`},
		{"identity with a space", func(c *config) { c.IdentityName = "Jamb Demo" }, `"identityName" must be 3 to 50 characters`},
		{"identity ends with a dot", func(c *config) { c.IdentityName = "Jamb.Demo." }, `"identityName" must be 3 to 50 characters`},
		{"icon missing", func(c *config) { c.Icon = "" }, `"icon" is required`},
		{"icon outside the app", func(c *config) { c.Icon = "../icon.png" }, `"icon" must be a relative path`},
		{"icon absolute", func(c *config) { c.Icon = filepath.Join(string(filepath.Separator), "icon.png") }, `"icon" must be a relative path`},
	}
	for _, tc := range cases {
		c := validConfig()
		tc.mutate(&c)
		p := c.problems()
		if len(p) != 1 || !strings.Contains(p[0], tc.want) {
			t.Errorf("%s: problems = %q; want one containing %q", tc.name, p, tc.want)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	write := func(t *testing.T, body string) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, configFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	valid := `{"name":"jamb-demo","displayName":"Jamb Demo","description":"A demo.","version":"0.1.0",
		"publisher":"Jamb Examples","publisherSubject":"CN=Jamb Examples","identityName":"JambExamples.Demo",
		"copyright":"Copyright 2026 The jamb Authors","icon":"icon.png"}`
	c, err := loadConfig(write(t, valid))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if c.Name != "jamb-demo" || c.Copyright != "Copyright 2026 The jamb Authors" || c.Icon != "icon.png" {
		t.Errorf("loadConfig read %+v", *c)
	}

	if _, err := loadConfig(t.TempDir()); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing file: err = %v; want a not-found message", err)
	}
	if _, err := loadConfig(write(t, `{"name":"x","colour":"red"}`)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("unknown field: err = %v; want it named", err)
	}
	if _, err := loadConfig(write(t, `{"name":`)); err == nil || !strings.Contains(err.Error(), configFile) {
		t.Errorf("syntax error: err = %v; want the file named", err)
	}
	if _, err := loadConfig(write(t, valid+` {}`)); err == nil || !strings.Contains(err.Error(), "unexpected content") {
		t.Errorf("trailing data: err = %v; want it reported", err)
	}
	_, err = loadConfig(write(t, `{"name":"1x","version":"1.2"}`))
	if err == nil || !strings.Contains(err.Error(), `"name" must start`) || !strings.Contains(err.Error(), `"version" "1.2"`) || !strings.Contains(err.Error(), `"displayName" is required`) {
		t.Errorf("all problems at once: err = %v", err)
	}
}

func TestNotificationProtocol(t *testing.T) {
	for in, want := range map[string]string{
		"JambExamples.Demo": "jamb-jambexamples.demo",
		"Example.Notes2":    "jamb-example.notes2",
	} {
		got, err := protocolScheme(in)
		if err != nil || got != want {
			t.Errorf("protocolScheme(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	// The setting is optional, and it is read from jamb.json.
	dir := t.TempDir()
	body := `{"name":"jamb-demo","displayName":"Jamb Demo","description":"A demo.","version":"0.1.0",
		"publisher":"Jamb Examples","publisherSubject":"CN=Jamb Examples","identityName":"JambExamples.Demo",
		"icon":"icon.png","notificationProtocol":true}`
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if !c.NotificationProtocol {
		t.Error("notificationProtocol is not read from jamb.json")
	}
}
