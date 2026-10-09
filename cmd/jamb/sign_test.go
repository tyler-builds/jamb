package main

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

func TestTestCertificateIsCodeSigningAndReused(t *testing.T) {
	cfg := validConfig()
	dist := t.TempDir()

	first := &signer{}
	if err := first.useTestCertificate(dist, &cfg); err != nil {
		t.Fatal(err)
	}
	second := &signer{}
	if err := second.useTestCertificate(dist, &cfg); err != nil {
		t.Fatal(err)
	}
	if first.thumbprint == "" || first.thumbprint != second.thumbprint {
		t.Errorf("thumbprints %q and %q; want the same certificate reused", first.thumbprint, second.thumbprint)
	}
	if !first.testCert || first.pfx != filepath.Join(dist, "jamb-demo-test.pfx") || first.cer != filepath.Join(dist, "jamb-demo-test.cer") {
		t.Errorf("test certificate paths: pfx %q, cer %q, testCert %v", first.pfx, first.cer, first.testCert)
	}

	cert, err := readTestCertificate(first.pfx)
	if err != nil {
		t.Fatal(err)
	}
	if cert.Subject.CommonName != "Jamb Examples" {
		t.Errorf("subject CN = %q", cert.Subject.CommonName)
	}
	if !hasCodeSigning(cert) {
		t.Error("test certificate lacks the code-signing extended key usage")
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	if _, err := cert.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}}); err != nil {
		t.Errorf("test certificate does not verify as a code-signing root: %v", err)
	}
	der, err := os.ReadFile(first.cer)
	if err != nil {
		t.Fatal(err)
	}
	if string(der) != string(cert.Raw) {
		t.Error(".cer file does not match the PFX certificate")
	}
}

func TestTestCertificateReplacedForOtherSubject(t *testing.T) {
	cfg := validConfig()
	dist := t.TempDir()
	first := &signer{}
	if err := first.useTestCertificate(dist, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.PublisherSubject = "CN=Another Publisher"
	second := &signer{}
	if err := second.useTestCertificate(dist, &cfg); err != nil {
		t.Fatal(err)
	}
	cert, err := readTestCertificate(second.pfx)
	if err != nil {
		t.Fatal(err)
	}
	if cert.Subject.CommonName != "Another Publisher" || second.thumbprint == first.thumbprint {
		t.Errorf("certificate for the new subject was not created: CN %q", cert.Subject.CommonName)
	}
}

func TestTestCertificateRenewedNearExpiry(t *testing.T) {
	cfg := validConfig()
	dist := t.TempDir()
	base := filepath.Join(dist, cfg.Name+"-test")
	// A certificate with less than testCertRenewBefore left is replaced.
	old, err := createTestCertificate("Jamb Examples", base+".pfx", base+".cer", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	s := &signer{}
	if err := s.useTestCertificate(dist, &cfg); err != nil {
		t.Fatal(err)
	}
	fresh, err := readTestCertificate(s.pfx)
	if err != nil {
		t.Fatal(err)
	}
	if string(fresh.Raw) == string(old.Raw) || time.Until(fresh.NotAfter) < testCertRenewBefore {
		t.Error("a certificate near expiry was not renewed")
	}
}

func TestTestCertificateRejectsMoreThanCommonName(t *testing.T) {
	cfg := validConfig()
	cfg.PublisherSubject = "CN=Jamb Examples, O=Jamb, C=US"
	s := &signer{}
	if err := s.useTestCertificate(t.TempDir(), &cfg); err == nil || !strings.Contains(err.Error(), "only a common name") {
		t.Errorf("err = %v; want a message that --test-cert needs a common name only", err)
	}
}

func TestNewSignerUnlocksCertPFX(t *testing.T) {
	const password = "p4ss-Phrase-7731"
	dir := t.TempDir()
	base := filepath.Join(dir, "source")
	cert, err := createTestCertificate("Jamb Examples", base+".pfx", base+".cer", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	// Re-encrypt the PFX under the password, as a commercial certificate is.
	data, err := os.ReadFile(base + ".pfx")
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := pkcs12.Decode(data, "")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := pkcs12.Modern.Encode(key, cert, nil, password)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "cert.pfx")
	if err := os.WriteFile(certPath, encrypted, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(passwordEnv, password)

	cfg := validConfig()
	s, err := newSigner(packageOptions{cert: certPath}, &cfg, t.TempDir(), "signtool.exe")
	if err != nil {
		t.Fatalf("newSigner: %v", err)
	}
	if s.testCert {
		t.Error("--cert signer reports a test certificate")
	}
	unlocked, err := os.ReadFile(s.pfx)
	if err != nil {
		t.Fatalf("temporary PFX: %v", err)
	}
	_, got, err := pkcs12.Decode(unlocked, "")
	if err != nil || string(got.Raw) != string(cert.Raw) {
		t.Fatalf("temporary PFX without a password does not hold the certificate: %v", err)
	}
	tmp := filepath.Dir(s.pfx)
	s.close()
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("temporary signing directory %s survives close", tmp)
	}
}

func TestNewSignerRejectsWrongPasswordWithoutEchoingIt(t *testing.T) {
	const password = "p4ss-Phrase-7731"
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pfx")
	if _, err := createTestCertificate("Jamb Examples", certPath, filepath.Join(dir, "cert.cer"), time.Hour); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	key, leaf, err := pkcs12.Decode(data, "")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := pkcs12.Modern.Encode(key, leaf, nil, password)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, encrypted, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(passwordEnv, "wrong-"+password)

	cfg := validConfig()
	_, err = newSigner(packageOptions{cert: certPath}, &cfg, t.TempDir(), "signtool.exe")
	if err == nil {
		t.Fatal("wrong password accepted")
	}
	if !strings.Contains(err.Error(), passwordEnv) {
		t.Errorf("error %q does not name %s", err, passwordEnv)
	}
	if strings.Contains(err.Error(), password) {
		t.Errorf("error %q echoes the password", err)
	}
}

// hasCodeSigning reports whether cert allows code signing.
func hasCodeSigning(cert *x509.Certificate) bool {
	for _, u := range cert.ExtKeyUsage {
		if u == x509.ExtKeyUsageCodeSigning {
			return true
		}
	}
	return false
}
