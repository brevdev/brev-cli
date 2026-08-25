package mintcert

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/spf13/afero"
	"golang.org/x/crypto/ssh"

	"github.com/brevdev/brev-cli/pkg/sshcert"
)

type fakeStore struct {
	token      string
	org        *entity.Organization
	err        error
	workspaces []entity.Workspace
	user       *entity.User
}

func (f fakeStore) GetAccessToken() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

func (f fakeStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return f.org, nil
}

func (f fakeStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return nil, nil
}

func (f fakeStore) GetCurrentUser() (*entity.User, error) {
	return f.user, nil
}

func (f fakeStore) GetWorkspaceByNameOrID(_ string, _ string) ([]entity.Workspace, error) {
	return f.workspaces, nil
}

type certIssuerFunc struct {
	fn func(context.Context, certIssueRequest) (string, error)
}

func (c *certIssuerFunc) Issue(ctx context.Context, req certIssueRequest) (string, error) {
	return c.fn(ctx, req)
}

type fakeEnvCertClient struct {
	resp *devplanev1.IssueEnvironmentSSHCertificateResponse
	err  error
	got  *devplanev1.IssueEnvironmentSSHCertificateRequest
}

func (f *fakeEnvCertClient) IssueEnvironmentSSHCertificate(_ context.Context, req *connect.Request[devplanev1.IssueEnvironmentSSHCertificateRequest]) (*connect.Response[devplanev1.IssueEnvironmentSSHCertificateResponse], error) {
	f.got = req.Msg
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.resp), nil
}

func mintCertForTest(t *testing.T, pubKeyOpenSSH string) string {
	t.Helper()
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKeyOpenSSH))
	if err != nil {
		t.Fatalf("parse pub key: %v", err)
	}
	_, privCA, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ca: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privCA)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	cert := &ssh.Certificate{
		Key:             pubKey,
		Serial:          42,
		CertType:        ssh.UserCert,
		KeyId:           "brev:v1:user:test",
		ValidPrincipals: []string{"brev:v1:vm:test-env:login:ubuntu"},
		ValidAfter:      uint64(1),
		ValidBefore:     uint64(1<<63 - 1), // far future for cache tests
		Permissions:     ssh.Permissions{Extensions: map[string]string{"permit-pty": ""}},
	}
	if err := cert.SignCert(rand.Reader, signer); err != nil {
		t.Fatalf("sign cert: %v", err)
	}
	return strings.TrimRight(string(ssh.MarshalAuthorizedKey(cert)), "\n")
}

func testStore(token string) fakeStore {
	return fakeStore{
		token: token,
		user:  &entity.User{ID: "user-1"},
		org:   &entity.Organization{ID: "org-1"},
		workspaces: []entity.Workspace{
			{ID: "env-1", CreatedByUserID: "user-1"},
		},
	}
}

func TestRunMintCert_MintsAndWrites(t *testing.T) {
	fs := afero.NewMemMapFs()
	outKey := "/home/u/.brev/ssh-certs/env-1"
	issuer := &certIssuerFunc{fn: func(_ context.Context, req certIssueRequest) (string, error) {
		return mintCertForTest(t, req.PublicKey), nil
	}}
	if err := runMintCertWith(testStore("tok"), fs, issuer, mintCertRequest{
		NameOrID: "env-1", PortID: "port-1", LinuxUser: "ubuntu", OutKey: outKey,
	}); err != nil {
		t.Fatalf("runMintCertWith: %v", err)
	}
	for _, p := range []string{outKey, outKey + "-cert.pub"} {
		if ok, _ := afero.Exists(fs, p); !ok {
			t.Errorf("not written: %s", p)
		}
	}
	if ok, err := sshcert.HasValidCertAuth(fs, outKey, outKey+"-cert.pub", time.Now(), 0); err != nil || !ok {
		t.Errorf("written cert not valid: ok=%v err=%v", ok, err)
	}
	privBytes, err := afero.ReadFile(fs, outKey)
	if err != nil {
		t.Fatalf("read private key: %v", err)
	}
	_, err = ssh.ParsePrivateKey(privBytes)
	if err != nil {
		t.Fatalf("written private key not ssh-loadable: %v", err)
	}
}

func TestRunMintCert_ReusesCachedCert(t *testing.T) {
	fs := afero.NewMemMapFs()
	outKey := "/home/u/.brev/ssh-certs/env-1"
	// Seed a matching keypair: the private key must correspond to the public
	// key bound into the certificate.
	privPEM, pub, err := sshcert.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	if err := sshcert.WriteFiles(fs, outKey, outKey+"-cert.pub", privPEM, mintCertForTest(t, pub)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	issuer := &certIssuerFunc{fn: func(_ context.Context, _ certIssueRequest) (string, error) {
		t.Error("issuer should not be called when cache is valid")
		return "", nil
	}}
	if err := runMintCertWith(testStore("tok"), fs, issuer, mintCertRequest{
		NameOrID: "env-1", PortID: "port-1", LinuxUser: "ubuntu", OutKey: outKey,
	}); err != nil {
		t.Fatalf("expected reuse, got err: %v", err)
	}
}

func TestRunMintCert_RemintsOnMismatchedKey(t *testing.T) {
	// A cert paired with a wrong private key must be detected and re-minted,
	// not silently reused.
	fs := afero.NewMemMapFs()
	outKey := "/home/u/.brev/ssh-certs/env-1"
	_, pub, _ := sshcert.GenerateKeyPair()
	if err := sshcert.WriteFiles(fs, outKey, outKey+"-cert.pub", []byte("priv"), mintCertForTest(t, pub)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	minted := false
	issuer := &certIssuerFunc{fn: func(_ context.Context, req certIssueRequest) (string, error) {
		minted = true
		if req.EnvironmentID != "env-1" {
			t.Errorf("unexpected EnvironmentID: %s", req.EnvironmentID)
		}
		return mintCertForTest(t, req.PublicKey), nil
	}}
	if err := runMintCertWith(testStore("tok"), fs, issuer, mintCertRequest{
		NameOrID: "env-1", PortID: "port-1", LinuxUser: "ubuntu", OutKey: outKey,
	}); err != nil {
		t.Fatalf("runMintCertWith: %v", err)
	}
	if !minted {
		t.Fatal("expected re-mint when key/cert mismatch")
	}
}

func TestRunMintCert_FallsBackOnIssueError(t *testing.T) {
	fs := afero.NewMemMapFs()
	issuer := &certIssuerFunc{fn: func(_ context.Context, _ certIssueRequest) (string, error) {
		return "", errors.New("CA unavailable")
	}}
	err := runMintCertWith(testStore("tok"), fs, issuer, mintCertRequest{
		NameOrID: "env-1", PortID: "port-1", LinuxUser: "ubuntu",
		OutKey: "/home/u/.brev/ssh-certs/env-1",
	})
	if err == nil {
		t.Fatal("expected error on issue failure")
	}
	if ok, _ := afero.Exists(fs, "/home/u/.brev/ssh-certs/env-1"); ok {
		t.Error("private key should not be written on issue failure")
	}
}

func TestRunMintCert_FallsBackOnAuthError(t *testing.T) {
	fs := afero.NewMemMapFs()
	issuer := &certIssuerFunc{fn: func(_ context.Context, _ certIssueRequest) (string, error) {
		t.Error("issuer should not be called when not authenticated")
		return "", nil
	}}
	// GetAccessToken error -> auth failure (no prompt, fall back to brev.pem).
	if err := runMintCertWith(fakeStore{err: errors.New("no token")}, fs, issuer, mintCertRequest{
		NameOrID: "env-1", PortID: "port-1", LinuxUser: "ubuntu",
		OutKey: "/home/u/.brev/ssh-certs/env-1",
	}); err == nil {
		t.Fatal("expected error on auth failure")
	}
	// Empty token (noLoginCmdStore returns "") -> auth failure, NOT a prompt.
	if err := runMintCertWith(testStore(""), fs, issuer, mintCertRequest{
		NameOrID: "env-1", PortID: "port-1", LinuxUser: "ubuntu",
		OutKey: "/home/u/.brev/ssh-certs/env-1",
	}); err == nil {
		t.Fatal("expected error on empty token (must not prompt)")
	}
}

func TestRpcCertIssuer_MapsRequestAndResponse(t *testing.T) {
	client := &fakeEnvCertClient{resp: &devplanev1.IssueEnvironmentSSHCertificateResponse{
		Certificate: "ssh-ed25519-cert-v01@openssh.com AAAA cert",
		Principal:   "brev:v1:vm:env-1:login:ubuntu",
	}}
	issuer := rpcCertIssuer{client: client}
	cert, err := issuer.Issue(context.Background(), certIssueRequest{
		EnvironmentID: "env-1", PortID: "port-1", LinuxUser: "ubuntu", PublicKey: "ssh-ed25519 AAAA pub",
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if cert != "ssh-ed25519-cert-v01@openssh.com AAAA cert" {
		t.Errorf("unexpected certificate: %s", cert)
	}
	if client.got.GetEnvironmentId() != "env-1" || client.got.GetPortId() != "port-1" || client.got.GetLinuxUser() != "ubuntu" || client.got.GetPublicKey() != "ssh-ed25519 AAAA pub" {
		t.Errorf("request fields wrong: %+v", client.got)
	}
}
