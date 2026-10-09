package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// passwordEnv names the environment variable that holds the password of the
// PFX file given to --cert. The password is never a flag or a log line.
const passwordEnv = "JAMB_CERT_PASSWORD"

// testCertValidity is how long a generated test certificate lasts. A
// certificate with less than testCertRenewBefore left is replaced.
const (
	testCertValidity    = 365 * 24 * time.Hour
	testCertRenewBefore = 30 * 24 * time.Hour
)

// signer signs files with signtool.exe, using a PFX file that has no password.
// signtool reads such a file without a password argument, so no secret reaches
// a command line.
type signer struct {
	signtool  string
	pfx       string
	timestamp string
	// testCert is set when the signer uses the self-signed test certificate.
	// cer is its DER file, and thumbprint its SHA-1, which removes its trust.
	testCert   bool
	cer        string
	thumbprint string
	cleanup    func()
}

// newSigner prepares the certificate that signs the release.
//
// With --cert, the PFX file's password is read from JAMB_CERT_PASSWORD. Its key
// and chain are copied into a temporary PFX that has no password, because
// signtool cannot be given one without a command-line argument. The copy holds
// the decrypted key until close removes it, so it lives only for one run.
//
// With --test-cert, the self-signed certificate is created in dist, or reused
// when the one there still matches the publisher subject and is not close to
// expiring. It is never added to a certificate store.
func newSigner(o packageOptions, cfg *config, dist, signtool string) (*signer, error) {
	s := &signer{signtool: signtool, timestamp: o.timestamp, cleanup: func() {}}
	if o.testCert {
		if err := s.useTestCertificate(dist, cfg); err != nil {
			return nil, err
		}
		return s, nil
	}

	data, err := os.ReadFile(o.cert)
	if err != nil {
		return nil, fmt.Errorf("--cert: %w", err)
	}
	key, leaf, chain, err := pkcs12.DecodeChain(data, os.Getenv(passwordEnv))
	if err != nil {
		return nil, fmt.Errorf("--cert %s: cannot open it with the password in %s: %v", o.cert, passwordEnv, err)
	}
	tmp, err := os.MkdirTemp("", "jamb-sign-")
	if err != nil {
		return nil, err
	}
	s.cleanup = func() { os.RemoveAll(tmp) }
	unlocked, err := pkcs12.Modern.Encode(key, leaf, chain, "")
	if err != nil {
		s.close()
		return nil, err
	}
	s.pfx = filepath.Join(tmp, "signing.pfx")
	if err := os.WriteFile(s.pfx, unlocked, 0o600); err != nil {
		s.close()
		return nil, err
	}
	return s, nil
}

// close removes the temporary PFX, if there is one.
func (s *signer) close() {
	s.cleanup()
}

// sign signs file in place with SHA-256 and, when a timestamp server is set,
// a timestamp.
func (s *signer) sign(file string) error {
	args := []string{"sign", "/fd", "SHA256", "/f", s.pfx}
	if s.timestamp != "" {
		args = append(args, "/td", "SHA256", "/tr", s.timestamp)
	}
	return runTool(s.signtool, append(args, file)...)
}

// useTestCertificate sets the signer to the test certificate in dist, creating
// or renewing it first.
func (s *signer) useTestCertificate(dist string, cfg *config) error {
	if strings.Contains(strings.TrimPrefix(cfg.PublisherSubject, "CN="), ",") {
		return errors.New("--test-cert needs a publisherSubject with only a common name, such as \"CN=Jamb Examples\"; use --cert for a certificate with more fields")
	}
	cn := strings.TrimPrefix(cfg.PublisherSubject, "CN=")
	base := filepath.Join(dist, cfg.Name+"-test")
	s.pfx, s.cer, s.testCert = base+".pfx", base+".cer", true

	cert, err := readTestCertificate(s.pfx)
	if err != nil || cert.Subject.CommonName != cn || time.Until(cert.NotAfter) < testCertRenewBefore {
		if cert, err = createTestCertificate(cn, s.pfx, s.cer, testCertValidity); err != nil {
			return err
		}
		pkgf("created the test certificate %s", s.cer)
	}
	sum := sha1.Sum(cert.Raw)
	s.thumbprint = strings.ToUpper(hex.EncodeToString(sum[:]))
	return nil
}

// readTestCertificate returns the certificate of an existing test PFX.
func readTestCertificate(pfx string) (*x509.Certificate, error) {
	data, err := os.ReadFile(pfx)
	if err != nil {
		return nil, err
	}
	_, cert, err := pkcs12.Decode(data, "")
	return cert, err
}

// createTestCertificate makes a self-signed code-signing certificate with the
// common name cn that lasts validity. It writes the certificate as a PFX with no
// password (pfxPath) and as a DER .cer file (cerPath).
func createTestCertificate(cn, pfxPath, cerPath string, validity time.Duration) (*x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	pfx, err := pkcs12.Modern.Encode(key, cert, nil, "")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(pfxPath, pfx, 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(cerPath, der, 0o644); err != nil {
		return nil, err
	}
	return cert, nil
}
