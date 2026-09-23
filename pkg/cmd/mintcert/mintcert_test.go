package mintcert

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/spf13/afero"
	"golang.org/x/crypto/ssh"

	"github.com/brevdev/brev-cli/pkg/sshcert"
)

type fakeStore struct {
	token string
	err   error
}

func (f fakeStore) GetAccessToken() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

type certIssuerFunc struct {
	fn     func(context.Context, certIssueRequest) (string, error)
	nodeFn func(context.Context, nodeCertIssueRequest) (string, error)
}

func (c *certIssuerFunc) Issue(ctx context.Context, req certIssueRequest) (string, error) {
	return c.fn(ctx, req)
}

func (c *certIssuerFunc) IssueNode(ctx context.Context, req nodeCertIssueRequest) (string, error) {
	if c.nodeFn != nil {
		return c.nodeFn(ctx, req)
	}
	return "", fmt.Errorf("certIssuerFunc: IssueNode not configured")
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

type fakeNodeCertClient struct {
	resp *devplanev1.IssueExternalNodeSSHCertificateResponse
	err  error
	got  *devplanev1.IssueExternalNodeSSHCertificateRequest
}

func (f *fakeNodeCertClient) IssueExternalNodeSSHCertificate(_ context.Context, req *connect.Request[devplanev1.IssueExternalNodeSSHCertificateRequest]) (*connect.Response[devplanev1.IssueExternalNodeSSHCertificateResponse], error) {
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

func TestRunMintCert_MintsAndWrites(t *testing.T) {
	fs := afero.NewMemMapFs()
	outKey := "/home/u/.brev/ssh-certs/env-1"
	issuer := &certIssuerFunc{fn: func(_ context.Context, req certIssueRequest) (string, error) {
		return mintCertForTest(t, req.PublicKey), nil
	}}
	if err := runMintCertWith(fakeStore{token: "tok"}, fs, issuer, mintCertRequest{
		EnvironmentID: "env-1", PortID: "port-1", LinuxUser: "ubuntu", OutKey: outKey,
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
	if err := runMintCertWith(fakeStore{token: "tok"}, fs, issuer, mintCertRequest{
		EnvironmentID: "env-1", PortID: "port-1", LinuxUser: "ubuntu", OutKey: outKey,
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
	if err := runMintCertWith(fakeStore{token: "tok"}, fs, issuer, mintCertRequest{
		EnvironmentID: "env-1", PortID: "port-1", LinuxUser: "ubuntu", OutKey: outKey,
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
	err := runMintCertWith(fakeStore{token: "tok"}, fs, issuer, mintCertRequest{
		EnvironmentID: "env-1", PortID: "port-1", LinuxUser: "ubuntu",
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
		EnvironmentID: "env-1", PortID: "port-1", LinuxUser: "ubuntu",
		OutKey: "/home/u/.brev/ssh-certs/env-1",
	}); err == nil {
		t.Fatal("expected error on auth failure")
	}
	// Empty token (noLoginCmdStore returns "") -> auth failure, NOT a prompt.
	if err := runMintCertWith(fakeStore{token: ""}, fs, issuer, mintCertRequest{
		EnvironmentID: "env-1", PortID: "port-1", LinuxUser: "ubuntu",
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

func TestRpcCertIssuer_NodeMapsRequestAndResponse(t *testing.T) {
	client := &fakeNodeCertClient{resp: &devplanev1.IssueExternalNodeSSHCertificateResponse{
		Certificate: "ssh-ed25519-cert-v01@openssh.com AAAA node cert",
		Principal:   "brev:v1:node:node-1:login:ubuntu",
	}}
	issuer := rpcCertIssuer{nodeClient: client}
	cert, err := issuer.IssueNode(context.Background(), nodeCertIssueRequest{
		ExternalNodeID: "node-1", PortID: "port-1", LinuxUser: "ubuntu", PublicKey: "ssh-ed25519 AAAA pub",
	})
	if err != nil {
		t.Fatalf("IssueNode: %v", err)
	}
	if cert != "ssh-ed25519-cert-v01@openssh.com AAAA node cert" {
		t.Errorf("unexpected certificate: %s", cert)
	}
	if client.got.GetExternalNodeId() != "node-1" || client.got.GetPortId() != "port-1" || client.got.GetLinuxUser() != "ubuntu" || client.got.GetPublicKey() != "ssh-ed25519 AAAA pub" {
		t.Errorf("request fields wrong: %+v", client.got)
	}
}

func TestRunMintCert_NodeRoutesToIssueNode(t *testing.T) {
	fs := afero.NewMemMapFs()
	outKey := "/home/u/.brev/ssh-certs/node-node-1"
	var gotReq nodeCertIssueRequest
	issuer := &certIssuerFunc{
		fn: func(_ context.Context, _ certIssueRequest) (string, error) {
			t.Error("Issue should not be called for --node")
			return "", nil
		},
		nodeFn: func(_ context.Context, req nodeCertIssueRequest) (string, error) {
			gotReq = req
			return mintCertForTest(t, req.PublicKey), nil
		},
	}
	if err := runMintCertWith(fakeStore{token: "tok"}, fs, issuer, mintCertRequest{
		NodeID: "node-1", PortID: "port-1", LinuxUser: "ubuntu", OutKey: outKey,
	}); err != nil {
		t.Fatalf("runMintCertWith: %v", err)
	}
	if gotReq.ExternalNodeID != "node-1" {
		t.Errorf("ExternalNodeID: got %s, want node-1", gotReq.ExternalNodeID)
	}
	if gotReq.PortID != "port-1" {
		t.Errorf("PortID: got %s, want port-1", gotReq.PortID)
	}
	if gotReq.LinuxUser != "ubuntu" {
		t.Errorf("LinuxUser: got %s, want ubuntu", gotReq.LinuxUser)
	}
}

func TestNewCmdMintCert_EnvAndNodeMutuallyExclusive(t *testing.T) {
	cmd := NewCmdMintCert(fakeStore{token: "tok"})
	cmd.SetArgs([]string{"--env", "e1", "--node", "n1", "--port", "p1", "--linux-user", "u", "--out-key", "/k"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when both --env and --node are set")
	}
}

func TestNewCmdMintCert_RequiresEnvOrNode(t *testing.T) {
	cmd := NewCmdMintCert(fakeStore{token: "tok"})
	cmd.SetArgs([]string{"--port", "p1", "--linux-user", "u", "--out-key", "/k"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when neither --env nor --node is set")
	}
}
