package app

import (
	"os/exec"
	"strings"
	"testing"
)

// The core packages link no crypto package. Importing one, even for a hash that needs no
// cryptographic strength, links Go's FIPS module, whose random generator reserves 32 MB in
// every app (see BENCHMARKS.md). Use hash/fnv or hash/maphash, and math/rand/v2, instead.
// The test asks the go command for the packages' dependencies, as the build sees them.
func TestCorePackagesLinkNoCrypto(t *testing.T) {
	pkgs := []string{"github.com/tyler-builds/jamb/app", "github.com/tyler-builds/jamb/ui"}
	out, err := exec.Command("go", append([]string{"list", "-deps"}, pkgs...)...).Output()
	if err != nil {
		t.Skipf("go list is not available: %v", err)
	}
	for _, p := range strings.Fields(string(out)) {
		if p == "crypto" || strings.HasPrefix(p, "crypto/") {
			t.Errorf("app or ui depends on %s", p)
		}
	}
}
