// Package sshcert manages short-lived, per-environment SSH certificates and
// their backing ephemeral keypairs on disk for use by the OpenSSH client.
package sshcert

import (
	"bytes"
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

func HasValidCertAuth(fs afero.Fs, keyPath, certPath string, now time.Time, margin time.Duration) (bool, error) {
	certBytes, err := afero.ReadFile(fs, certPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, breverrors.WrapAndTrace(err)
	}
	cert, err := ParseCertificate(string(certBytes))
	if err != nil {
		return false, nil
	}
	if !CertValidAt(cert, now, margin) {
		return false, nil
	}

	keyBytes, err := afero.ReadFile(fs, keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, breverrors.WrapAndTrace(err)
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return false, nil
	}

	if !PublicKeyMatches(signer.PublicKey(), cert.Key) {
		return false, nil
	}
	return true, nil
}

func PublicKeyMatches(a, b ssh.PublicKey) bool {
	return bytes.Equal(a.Marshal(), b.Marshal())
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

// CertAuthorityPrincipal returns the SSH certificate principal for a node and
// Linux user
func CertAuthorityPrincipal(nodeID, linuxUser string) string {
	return fmt.Sprintf("brev:v1:vm:%s:login:%s", nodeID, linuxUser)
}

// RemoveCertAuthorityLine removes the cert-authority line for the given node
// and Linux user from ~/.ssh/authorized_keys. Returns true if a line was
// removed. Missing file is treated as nothing-to-remove.
func RemoveCertAuthorityLine(homeDir, nodeID, linuxUser string) (bool, error) {
	prefix := fmt.Sprintf("cert-authority,principals=%q ", CertAuthorityPrincipal(nodeID, linuxUser))

	authKeysPath := filepath.Join(homeDir, ".ssh", "authorized_keys")

	existing, err := os.ReadFile(authKeysPath) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading authorized_keys: %w", err)
	}

	var kept []string
	var removed bool
	for line := range strings.SplitSeq(string(existing), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) && strings.Contains(trimmed, "cert-authority") {
			removed = true
			continue
		}
		kept = append(kept, line)
	}

	if !removed {
		return false, nil
	}

	result := strings.Join(kept, "\n")
	if err := os.WriteFile(authKeysPath, []byte(result), 0o600); err != nil {
		return false, fmt.Errorf("writing authorized_keys: %w", err)
	}

	return true, nil
}

// HasCertAuthorityLine reports whether authorized_keys contains a Brev
// cert-authority line for the given node (any Linux user).
func HasCertAuthorityLine(homeDir, nodeID string) bool {
	data, err := os.ReadFile(filepath.Join(homeDir, ".ssh", "authorized_keys")) // #nosec G304
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "brev:v1:vm:"+nodeID+":")
}
