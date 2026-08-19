package sshcert

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
	"golang.org/x/crypto/ssh"
)

func mintTestCert(t *testing.T, validBefore time.Time) string {
	t.Helper()
	_, privCA, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ca: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privCA)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate user key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("new public key: %v", err)
	}
	cert := &ssh.Certificate{
		Key:             sshPub,
		Serial:          1,
		CertType:        ssh.UserCert,
		KeyId:           "test:user",
		ValidPrincipals: []string{"brev:v1:vm:test-env:login:ubuntu"},
		ValidAfter:      uint64(time.Now().Add(-time.Minute).Unix()),
		ValidBefore:     uint64(validBefore.Unix()),
		Permissions:     ssh.Permissions{Extensions: map[string]string{"permit-pty": ""}},
	}
	if err := cert.SignCert(rand.Reader, signer); err != nil {
		t.Fatalf("sign cert: %v", err)
	}
	return strings.TrimRight(string(ssh.MarshalAuthorizedKey(cert)), "\n")
}

func TestParseCertificate(t *testing.T) {
	cert, err := ParseCertificate(mintTestCert(t, time.Now().Add(10*time.Minute)))
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	if cert.CertType != ssh.UserCert {
		t.Errorf("expected user cert, got type %d", cert.CertType)
	}
	if len(cert.ValidPrincipals) != 1 || cert.ValidPrincipals[0] != "brev:v1:vm:test-env:login:ubuntu" {
		t.Errorf("unexpected principals: %v", cert.ValidPrincipals)
	}
	for _, bad := range []string{"", "not a cert"} {
		if _, err := ParseCertificate(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestCertValidAt(t *testing.T) {
	now := time.Now()
	valid := &ssh.Certificate{
		ValidAfter:  uint64(now.Add(-time.Hour).Unix()),
		ValidBefore: uint64(now.Add(10 * time.Minute).Unix()),
	}
	if !CertValidAt(valid, now, time.Minute) {
		t.Error("cert with 10m left should be valid with 1m margin")
	}
	if CertValidAt(valid, now, 10*time.Minute) {
		t.Error("cert with 10m left should need renewal with 10m margin")
	}
	expired := &ssh.Certificate{ValidAfter: uint64(now.Add(-time.Hour).Unix()), ValidBefore: uint64(now.Add(-time.Minute).Unix())}
	if CertValidAt(expired, now, time.Minute) {
		t.Error("expired cert should not be valid")
	}
	zeroExpiry := &ssh.Certificate{ValidAfter: uint64(now.Add(-time.Hour).Unix()), ValidBefore: 0}
	if CertValidAt(zeroExpiry, now, time.Minute) {
		t.Error("zero-expiry cert should not be valid")
	}
	if CertValidAt(nil, now, time.Minute) {
		t.Error("nil cert should not be valid")
	}
}

func TestHasValidCertAt(t *testing.T) {
	fs := afero.NewMemMapFs()
	certPath := CertPath("/home/u/.brev", "env-1")

	if ok, err := HasValidCertAt(fs, certPath, time.Now(), DefaultRenewalMargin); ok || err != nil {
		t.Fatalf("missing cert: ok=%v err=%v", ok, err)
	}
	privPEM, _ := mustGen(t)
	if err := WriteFiles(fs, KeyPath("/home/u/.brev", "env-1"), certPath, privPEM, mintTestCert(t, time.Now().Add(10*time.Minute))); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}
	if ok, _ := HasValidCertAt(fs, certPath, time.Now(), DefaultRenewalMargin); !ok {
		t.Error("expected valid after write")
	}
	if ok, _ := HasValidCertAt(fs, CertPath("/home/u/.brev", "env-2"), time.Now(), DefaultRenewalMargin); ok {
		t.Error("env-2 should have no cert")
	}
	// Corrupt -> not valid, no error (mint fresh).
	if err := afero.WriteFile(fs, certPath, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, err := HasValidCertAt(fs, certPath, time.Now(), DefaultRenewalMargin); ok || err != nil {
		t.Errorf("corrupt cert: ok=%v err=%v (want false,nil)", ok, err)
	}
}

func TestWriteFiles_NoLeftoverTemp(t *testing.T) {
	fs := afero.NewMemMapFs()
	privPEM, _ := mustGen(t)
	if err := WriteFiles(fs, KeyPath("/h/.brev", "x"), CertPath("/h/.brev", "x"), privPEM, mintTestCert(t, time.Now().Add(5*time.Minute))); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}
	entries, _ := afero.ReadDir(fs, Dir("/h/.brev"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".brev-cert-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
	b, _ := afero.ReadFile(fs, CertPath("/h/.brev", "x"))
	if !strings.HasSuffix(string(b), "\n") {
		t.Error("cert file should end with newline")
	}
}

func mustGen(t *testing.T) ([]byte, string) {
	t.Helper()
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	return priv, pub
}
