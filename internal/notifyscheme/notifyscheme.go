// Package notifyscheme derives the URI scheme that a click on a jamb app's
// notification starts the app with. The app registers the scheme for the user, and
// the packager declares it in an MSIX manifest, so both call this one function and
// agree on the name.
package notifyscheme

import "strings"

// Activation returns the URI scheme that a click on the app's notifications starts
// the app with, such as "jamb-company.product". The scheme is derived from the AppID,
// whose characters it keeps, lower-cased; a '_' becomes '-', and so does any other
// character that a URI scheme does not allow.
func Activation(appID string) string {
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
