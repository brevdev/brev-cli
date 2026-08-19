// Package sshcert manages short-lived, per-environment SSH certificates and
// their backing ephemeral keypairs on disk for use by the OpenSSH client.
package sshcert

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"
	"golang.org/x/crypto/ssh"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

const certSubDir = "ssh-certs"

// DefaultRenewalMargin is how long before expiry we renew, to avoid a race
// where the cert expires between mint and the subsequent ssh use.
const DefaultRenewalMargin = 60 * time.Second

// Label constants mirroring dev-plane's internal/labels package
const (
	LabelKeySSHProvider = "sshprovider"
	SSHProviderCertAuth = "certauth"
)

func EnvironmentCertEligible(labels map[string]string) bool {
	return labels[LabelKeySSHProvider] == SSHProviderCertAuth
}

func Dir(brevDir string) string {
	return filepath.Join(brevDir, certSubDir)
}

func safeFilename(envID string) string {
	s := strings.TrimSpace(envID)
	if s == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := b.String()
	if out == "" {
		return "default"
	}
	return out
}

func KeyPath(brevDir, envID string) string {
	return filepath.Join(Dir(brevDir), safeFilename(envID))
}

// CertPath follows OpenSSH's <IdentityFile>-cert.pub convention, so a single
// IdentityFile directive loads both the key and the cert.
func CertPath(brevDir, envID string) string {
	return KeyPath(brevDir, envID) + "-cert.pub"
}

func GenerateKeyPair() (privKeyPEM []byte, pubKeyOpenSSH string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", breverrors.WrapAndTrace(err)
	}
	sshPubKey, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, "", breverrors.WrapAndTrace(err)
	}
	pubKeyOpenSSH = string(ssh.MarshalAuthorizedKey(sshPubKey))
	block, err := ssh.MarshalPrivateKey(priv, "brev")
	if err != nil {
		return nil, "", breverrors.WrapAndTrace(err)
	}
	return pem.EncodeToMemory(block), pubKeyOpenSSH, nil
}

func ParseCertificate(certOpenSSH string) (*ssh.Certificate, error) {
	certOpenSSH = strings.TrimSpace(certOpenSSH)
	if certOpenSSH == "" {
		return nil, fmt.Errorf("certificate is empty")
	}
	pubKey, _, _, rest, err := ssh.ParseAuthorizedKey([]byte(certOpenSSH))
	if err != nil {
		return nil, breverrors.WrapAndTrace(fmt.Errorf("parse certificate: %w", err))
	}
	if len(strings.TrimSpace(string(rest))) != 0 {
		return nil, fmt.Errorf("certificate has trailing data; expected exactly one key")
	}
	cert, ok := pubKey.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("public key is not a certificate")
	}
	if cert.CertType != ssh.UserCert {
		return nil, fmt.Errorf("certificate is not a user certificate (type=%d)", cert.CertType)
	}
	return cert, nil
}

func CertValidAt(cert *ssh.Certificate, now time.Time, margin time.Duration) bool {
	if cert == nil {
		return false
	}
	notAfter := int64(cert.ValidBefore)
	return now.Add(margin).Unix() < notAfter
}

func HasValidCertAt(fs afero.Fs, certPath string, now time.Time, margin time.Duration) (bool, error) {
	exists, err := afero.Exists(fs, certPath)
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	if !exists {
		return false, nil
	}
	certBytes, err := afero.ReadFile(fs, certPath)
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	cert, err := ParseCertificate(string(certBytes))
	if err != nil {
		return false, nil // corrupt cert -> mint fresh
	}
	return CertValidAt(cert, now, margin), nil
}

func WriteFiles(fs afero.Fs, keyPath, certPath string, privKeyPEM []byte, certOpenSSH string) error {
	if err := fs.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if err := writeAtomic(fs, keyPath, privKeyPEM, 0o600); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !strings.HasSuffix(certOpenSSH, "\n") {
		certOpenSSH += "\n"
	}
	return writeAtomic(fs, certPath, []byte(certOpenSSH), 0o644)
}

// writeAtomic renames a temp file in the same directory into place, so a
// reader never observes a partial write.
func writeAtomic(fs afero.Fs, path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := afero.TempFile(fs, dir, ".brev-cert-*.tmp")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	tmpName := tmp.Name()
	defer func() { _ = fs.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return breverrors.WrapAndTrace(err)
	}
	if err := tmp.Close(); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if err := fs.Chmod(tmpName, mode); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return breverrors.WrapAndTrace(fs.Rename(tmpName, path))
}
