package sshcert

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
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

// mintTestCertWithKey generates a fresh keypair and returns the private key PEM
// alongside a certificate bound to that key's public key.
func mintTestCertWithKey(t *testing.T, validBefore time.Time) ([]byte, string) {
	t.Helper()
	_, privCA, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ca: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privCA)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
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
	block, err := ssh.MarshalPrivateKey(priv, "brev")
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	return pem.EncodeToMemory(block), strings.TrimRight(string(ssh.MarshalAuthorizedKey(cert)), "\n")
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

func TestHasValidSSHAuth(t *testing.T) { //nolint:gocyclo // test
	fs := afero.NewMemMapFs()
	keyPath := KeyPath("/home/u/.brev", "env-1")
	certPath := CertPath("/home/u/.brev", "env-1")

	// No files -> not valid.
	if ok, err := HasValidCertAuth(fs, keyPath, certPath, time.Now(), DefaultRenewalMargin); ok || err != nil {
		t.Fatalf("empty: ok=%v err=%v", ok, err)
	}

	// Valid matching keypair -> valid.
	privPEM, cert := mintTestCertWithKey(t, time.Now().Add(10*time.Minute))
	if err := WriteFiles(fs, keyPath, certPath, privPEM, cert); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}
	if ok, err := HasValidCertAuth(fs, keyPath, certPath, time.Now(), DefaultRenewalMargin); !ok || err != nil {
		t.Fatalf("valid pair: ok=%v err=%v", ok, err)
	}

	// Missing private key -> not valid (cert alone is insufficient).
	_ = fs.Remove(keyPath)
	if ok, err := HasValidCertAuth(fs, keyPath, certPath, time.Now(), DefaultRenewalMargin); ok || err != nil {
		t.Errorf("missing key: ok=%v err=%v (want false,nil)", ok, err)
	}

	// Restore key, corrupt it -> not valid.
	if err := afero.WriteFile(fs, keyPath, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ok, err := HasValidCertAuth(fs, keyPath, certPath, time.Now(), DefaultRenewalMargin); ok || err != nil {
		t.Errorf("corrupt key: ok=%v err=%v (want false,nil)", ok, err)
	}

	// Mismatched key (valid but wrong key) -> not valid.
	otherPriv, _ := mustGen(t)
	if err := afero.WriteFile(fs, keyPath, otherPriv, 0o600); err != nil {
		t.Fatal(err)
	}
	if ok, err := HasValidCertAuth(fs, keyPath, certPath, time.Now(), DefaultRenewalMargin); ok || err != nil {
		t.Errorf("mismatched key: ok=%v err=%v (want false,nil)", ok, err)
	}

	// Corrupt cert -> not valid (mint fresh).
	if err := afero.WriteFile(fs, certPath, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, err := HasValidCertAuth(fs, keyPath, certPath, time.Now(), DefaultRenewalMargin); ok || err != nil {
		t.Errorf("corrupt cert: ok=%v err=%v (want false,nil)", ok, err)
	}

	// Expired cert -> not valid.
	expiredPriv, expiredCert := mintTestCertWithKey(t, time.Now().Add(-time.Minute))
	if err := WriteFiles(fs, keyPath, certPath, expiredPriv, expiredCert); err != nil {
		t.Fatalf("WriteFiles expired: %v", err)
	}
	if ok, err := HasValidCertAuth(fs, keyPath, certPath, time.Now(), DefaultRenewalMargin); ok || err != nil {
		t.Errorf("expired: ok=%v err=%v (want false,nil)", ok, err)
	}
}

func TestPublicKeyMatches(t *testing.T) {
	_, pub1 := mustGen(t)
	_, pub2 := mustGen(t)
	k1, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pub1))
	if err != nil {
		t.Fatal(err)
	}
	k2, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pub2))
	if err != nil {
		t.Fatal(err)
	}
	if !PublicKeyMatches(k1, k1) {
		t.Error("same key should match")
	}
	if PublicKeyMatches(k1, k2) {
		t.Error("different keys should not match")
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
