package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// configFile is the packaging settings file in an application's directory.
const configFile = "jamb.json"

// config is the content of jamb.json. Every field is required except
// copyright and notificationProtocol. The Packaging section of the README describes each
// one.
type config struct {
	Name                 string `json:"name"`
	DisplayName          string `json:"displayName"`
	Description          string `json:"description"`
	Version              string `json:"version"`
	Publisher            string `json:"publisher"`
	PublisherSubject     string `json:"publisherSubject"`
	IdentityName         string `json:"identityName"`
	Copyright            string `json:"copyright"`
	Icon                 string `json:"icon"`
	NotificationProtocol bool   `json:"notificationProtocol"`
}

var (
	exeNamePattern      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
	identityNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{1,48}[A-Za-z0-9]$`)
	// uriSchemePattern is the grammar of a URI scheme in RFC 3986.
	uriSchemePattern = regexp.MustCompile(`^[a-z][a-z0-9+.-]*$`)
)

// protocolScheme returns the URI scheme that a click on the app's notifications starts
// the app with. identityName is the app's AppID, and the scheme is derived by the same
// function the app uses, so the packager and the app agree on the name. The result is
// checked against the grammar of a URI scheme.
func protocolScheme(identityName string) (string, error) {
	s := activationScheme(identityName)
	if !uriSchemePattern.MatchString(s) {
		return "", fmt.Errorf("the protocol %q derived from identityName is not a valid URI scheme", s)
	}
	return s, nil
}

// loadConfig reads and validates jamb.json in dir. Every problem is reported
// together, so one run shows them all.
func loadConfig(dir string) (*config, error) {
	path := filepath.Join(dir, configFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s not found in %s; jamb package reads the app's settings from it (see the Packaging section of the README)", configFile, dir)
	}
	if err != nil {
		return nil, err
	}
	var c config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("%s: unexpected content after the JSON object", path)
	}
	if problems := c.problems(); len(problems) > 0 {
		return nil, fmt.Errorf("%s: %d problem(s):\n  %s", path, len(problems), strings.Join(problems, "\n  "))
	}
	return &c, nil
}

// problems lists what is wrong with the settings, one message per problem.
func (c *config) problems() []string {
	var p []string
	required := func(field, value string) {
		if strings.TrimSpace(value) == "" {
			p = append(p, fmt.Sprintf("%q is required", field))
		}
	}
	required("name", c.Name)
	required("displayName", c.DisplayName)
	required("description", c.Description)
	required("version", c.Version)
	required("publisher", c.Publisher)
	required("publisherSubject", c.PublisherSubject)
	required("identityName", c.IdentityName)
	required("icon", c.Icon)

	if c.Name != "" && !exeNamePattern.MatchString(c.Name) {
		p = append(p, `"name" must start with a letter, contain only letters, digits, "-" and "_", and be at most 64 characters; it names the .exe`)
	}
	if c.Version != "" {
		if _, err := windowsVersion(c.Version); err != nil {
			p = append(p, err.Error())
		}
	}
	if c.PublisherSubject != "" && (!strings.HasPrefix(c.PublisherSubject, "CN=") || strings.TrimSpace(c.PublisherSubject[len("CN="):]) == "") {
		p = append(p, `"publisherSubject" must start with "CN=" followed by the name, such as "CN=Example"; it must match the signing certificate's subject`)
	}
	if c.IdentityName != "" && !identityNamePattern.MatchString(c.IdentityName) {
		p = append(p, `"identityName" must be 3 to 50 characters of letters, digits, "." and "-", starting and ending with a letter or digit`)
	}
	if c.Icon != "" && !filepath.IsLocal(c.Icon) {
		p = append(p, `"icon" must be a relative path inside the app's directory`)
	}
	return p
}

// windowsVersion maps a semantic version to the four numeric parts that Windows
// version resources and MSIX use: MAJOR.MINOR.PATCH.0. Pre-release and build
// suffixes have no Windows form, so they are rejected.
func windowsVersion(v string) (string, error) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf(`"version" %q must be MAJOR.MINOR.PATCH, such as "1.2.3"; pre-release and build suffixes have no Windows form`, v)
	}
	for _, p := range parts {
		if _, err := strconv.ParseUint(p, 10, 16); err != nil || (len(p) > 1 && p[0] == '0') {
			return "", fmt.Errorf(`"version" %q: each part must be a number from 0 to 65535 without leading zeros (Windows stores each part in 16 bits); pre-release and build suffixes have no Windows form`, v)
		}
	}
	return parts[0] + "." + parts[1] + "." + parts[2] + ".0", nil
}
