package main

import "strings"

// activationScheme returns the URI scheme that a click on the app's notifications starts
// the app with, such as "jamb-company.product": the scheme the app registers (see
// internal/notifyscheme in the jamb module, which this copies, since cmd/jamb is a module
// of its own and does not depend on jamb). The scheme is derived from the AppID, whose
// characters it keeps, lower-cased; a '_' becomes '-', and so does any other character
// that a URI scheme does not allow. TestActivationScheme checks the copy against the same
// table as the original's test.
func activationScheme(appID string) string {
	var b strings.Builder
	b.WriteString("jamb-")
	for _, c := range strings.ToLower(appID) {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '.', c == '-':
			b.WriteRune(c)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}
