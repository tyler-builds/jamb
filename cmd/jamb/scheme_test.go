package main

import "testing"

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

func TestActivationScheme(t *testing.T) {
	for in, want := range schemeVectors {
		if got := activationScheme(in); got != want {
			t.Errorf("activationScheme(%q) = %q, want %q", in, got, want)
		}
	}
}
