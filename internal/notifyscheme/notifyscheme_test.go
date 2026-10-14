package notifyscheme

import (
	"regexp"
	"testing"
)

// schemeVectors are the AppIDs and the schemes they must give. The same table is in
// internal/notifyscheme's test and in cmd/jamb's test, because cmd/jamb is a module of
// its own and keeps its own copy of the function: the two copies must agree, or the
// packager would declare a scheme the app does not register.
var schemeVectors = map[string]string{
	"JambExamples.Notes": "jamb-jambexamples.notes",
	"My_App-1":           "jamb-my-app-1",
	"":                   "jamb-",
	"Café App":           "jamb-caf--app",
	"x/y:z":              "jamb-x-y-z",
	"A_B.C-D9":           "jamb-a-b.c-d9",
}

func TestActivation(t *testing.T) {
	for in, want := range schemeVectors {
		if got := Activation(in); got != want {
			t.Errorf("Activation(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestActivationIsAURIScheme checks that the derived names follow the scheme grammar
// of RFC 3986, which both Windows and the MSIX manifest need.
func TestActivationIsAURIScheme(t *testing.T) {
	scheme := regexp.MustCompile(`^[a-z][a-z0-9+.-]*$`)
	for _, in := range []string{"JambExamples.Notes", "a", "Café App", "x/y:z", "A_B.C-D9"} {
		if got := Activation(in); !scheme.MatchString(got) {
			t.Errorf("Activation(%q) = %q, which is not a URI scheme", in, got)
		}
	}
}
