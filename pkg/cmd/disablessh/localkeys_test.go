package disablessh

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParsePasswd_EnumeratesAndDeduplicatesHomes(t *testing.T) {
	data, err := os.ReadFile("testdata/passwd.txt")
	if err != nil {
		t.Fatal(err)
	}

	got, err := parsePasswd(data)
	if err != nil {
		t.Fatalf("parsePasswd: %v", err)
	}
	want := []localAccount{
		{Username: "root", HomeDir: "/root"},
		{Username: "alice", HomeDir: "/home/alice"},
		{Username: "svc-agent", HomeDir: "/var/lib/svc-agent"},
		{Username: "bob", HomeDir: "/home/shared"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePasswd() = %#v, want %#v", got, want)
	}
}

func TestParsePasswd_RejectsMalformedRecord(t *testing.T) {
	_, err := parsePasswd([]byte("alice:x:1000:1000:Alice:/home/alice\n"))
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("parsePasswd() error = %v, want line context", err)
	}
}

func TestParsePasswd_RejectsRelativeHome(t *testing.T) {
	_, err := parsePasswd([]byte("alice:x:1000:1000:Alice:home/alice:/bin/bash\n"))
	if err == nil || !strings.Contains(err.Error(), "line 1") || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("parsePasswd() error = %v, want absolute-home error with line context", err)
	}
}

func TestStripBrevManagedAuthorizedKeyLines_PreservesUnrelatedBytes(t *testing.T) {
	before, err := os.ReadFile("testdata/authorized_keys.before")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/authorized_keys.after")
	if err != nil {
		t.Fatal(err)
	}

	got, removed := stripBrevManagedAuthorizedKeyLines(before)
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("cleaned bytes = %q, want %q", got, want)
	}
}

func TestStripBrevManagedAuthorizedKeyLines_NoMarkersReturnsOriginalBytes(t *testing.T) {
	data := []byte("ssh-ed25519 AAAA_KEEP keep@example.com\r\n\nssh-rsa AAAA_FINAL")
	got, removed := stripBrevManagedAuthorizedKeyLines(data)
	if removed != 0 {
		t.Fatalf("removed = %d, want 0", removed)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("cleaned bytes = %q, want original %q", got, data)
	}
}

func TestSystemLocalKeyCleaner_AttemptsEveryAccountAndJoinsErrors(t *testing.T) {
	accounts := []localAccount{
		{Username: "alice", HomeDir: "/home/alice"},
		{Username: "bob", HomeDir: "/home/bob"},
		{Username: "carol", HomeDir: "/home/carol"},
	}
	var cleaned []localAccount
	cleaner := systemLocalKeyCleaner{
		listAccounts: func(context.Context) ([]localAccount, error) { return accounts, nil },
		cleanAccount: func(account localAccount) (int, error) {
			cleaned = append(cleaned, account)
			switch account.Username {
			case "alice":
				return 0, errors.New("first failure")
			case "bob":
				return 2, nil
			case "carol":
				return 0, errors.New("third failure")
			default:
				return 0, nil
			}
		},
	}

	got, err := cleaner.RemoveBrevKeys(context.Background())
	if !reflect.DeepEqual(cleaned, accounts) {
		t.Fatalf("cleaned accounts = %#v, want every account %#v", cleaned, accounts)
	}
	want := KeyCleanupResult{AccountsScanned: 1, AccountsChanged: 1, KeysRemoved: 2}
	if got != want {
		t.Fatalf("result = %#v, want %#v", got, want)
	}
	if err == nil {
		t.Fatal("RemoveBrevKeys() error = nil, want joined account failures")
	}
	for _, text := range []string{"alice", "/home/alice", "first failure", "carol", "/home/carol", "third failure"} {
		if !strings.Contains(err.Error(), text) {
			t.Errorf("error %q does not contain %q", err, text)
		}
	}
}

type fakeLocalKeyCleaner struct {
	result KeyCleanupResult
	err    error
	calls  int
}

func (f *fakeLocalKeyCleaner) RemoveBrevKeys(context.Context) (KeyCleanupResult, error) {
	f.calls++
	return f.result, f.err
}

type privilegedCommandCall struct {
	name string
	args []string
}

type fakePrivilegedCommandRunner struct {
	output []byte
	err    error
	calls  []privilegedCommandCall
}

func (f *fakePrivilegedCommandRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, privilegedCommandCall{name: name, args: append([]string(nil), args...)})
	return append([]byte(nil), f.output...), f.err
}

func TestPrivilegedLocalKeyCleaner_RootRunsDirectly(t *testing.T) {
	direct := &fakeLocalKeyCleaner{result: KeyCleanupResult{AccountsScanned: 3, KeysRemoved: 2}}
	runner := &fakePrivilegedCommandRunner{}
	cleaner := privilegedLocalKeyCleaner{
		geteuid:    func() int { return 0 },
		executable: func() (string, error) { t.Fatal("executable lookup called as root"); return "", nil },
		runner:     runner,
		direct:     direct,
	}

	got, err := cleaner.RemoveBrevKeys(context.Background())
	if err != nil {
		t.Fatalf("RemoveBrevKeys: %v", err)
	}
	if got != direct.result || direct.calls != 1 || len(runner.calls) != 0 {
		t.Fatalf("got %#v, direct calls %d, runner calls %#v", got, direct.calls, runner.calls)
	}
}

func TestPrivilegedLocalKeyCleaner_UsesFixedSudoCommandWhenNotRoot(t *testing.T) {
	runner := &fakePrivilegedCommandRunner{output: []byte(`{"accounts_scanned":4,"accounts_changed":2,"keys_removed":3}`)}
	cleaner := privilegedLocalKeyCleaner{
		geteuid:    func() int { return 501 },
		executable: func() (string, error) { return "/opt/brev/bin/brev", nil },
		runner:     runner,
		direct:     &fakeLocalKeyCleaner{},
	}

	got, err := cleaner.RemoveBrevKeys(context.Background())
	if err != nil {
		t.Fatalf("RemoveBrevKeys: %v", err)
	}
	wantResult := KeyCleanupResult{AccountsScanned: 4, AccountsChanged: 2, KeysRemoved: 3}
	if got != wantResult {
		t.Fatalf("result = %#v, want %#v", got, wantResult)
	}
	wantCalls := []privilegedCommandCall{{
		name: "sudo",
		args: []string{"-n", "/opt/brev/bin/brev", "__brev-disable-ssh-cleanup"},
	}}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("runner calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestPrivilegedLocalKeyCleaner_RejectsInvalidJSON(t *testing.T) {
	cleaner := privilegedLocalKeyCleaner{
		geteuid:    func() int { return 501 },
		executable: func() (string, error) { return "/opt/brev/bin/brev", nil },
		runner:     &fakePrivilegedCommandRunner{output: []byte("not json")},
		direct:     &fakeLocalKeyCleaner{},
	}

	_, err := cleaner.RemoveBrevKeys(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode privileged Brev key cleanup result") {
		t.Fatalf("RemoveBrevKeys() error = %v, want invalid JSON context", err)
	}
}

func TestExecPrivilegedCommandRunner_IncludesStderrOnFailure(t *testing.T) {
	_, err := (execPrivilegedCommandRunner{}).Output(
		context.Background(),
		"/bin/sh",
		"-c",
		"printf 'sudo denied' >&2; exit 7",
	)
	if err == nil || !strings.Contains(err.Error(), "sudo denied") {
		t.Fatalf("Output() error = %v, want captured stderr", err)
	}
}

func TestRunLocalKeyCleanupHelper_IgnoresNormalCLIArguments(t *testing.T) {
	cleaner := &fakeLocalKeyCleaner{}
	var stdout bytes.Buffer
	handled, err := runLocalKeyCleanupHelper(context.Background(), []string{"join", "--approve"}, &stdout, "linux", func() int { return 0 }, cleaner)
	if err != nil || handled {
		t.Fatalf("handled, err = %v, %v; want false, nil", handled, err)
	}
	if cleaner.calls != 0 || stdout.Len() != 0 {
		t.Fatalf("cleaner calls = %d, stdout = %q; want no side effects", cleaner.calls, stdout.String())
	}
}

func TestRunLocalKeyCleanupHelper_RequiresExactToken(t *testing.T) {
	cleaner := &fakeLocalKeyCleaner{}
	var stdout bytes.Buffer
	handled, err := runLocalKeyCleanupHelper(context.Background(), []string{"__brev-disable-ssh-cleanup-extra"}, &stdout, "linux", func() int { return 0 }, cleaner)
	if err != nil || handled {
		t.Fatalf("handled, err = %v, %v; want false, nil", handled, err)
	}
	if cleaner.calls != 0 || stdout.Len() != 0 {
		t.Fatalf("cleaner calls = %d, stdout = %q; want no side effects", cleaner.calls, stdout.String())
	}
}

func TestRunLocalKeyCleanupHelper_RejectsExtraArguments(t *testing.T) {
	handled, err := runLocalKeyCleanupHelper(context.Background(), []string{"__brev-disable-ssh-cleanup", "/home/alice"}, &bytes.Buffer{}, "linux", func() int { return 0 }, &fakeLocalKeyCleaner{})
	if !handled || err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("handled, err = %v, %v; want selected argument-count error", handled, err)
	}
}

func TestRunLocalKeyCleanupHelper_RejectsNonRoot(t *testing.T) {
	handled, err := runLocalKeyCleanupHelper(context.Background(), []string{"__brev-disable-ssh-cleanup"}, &bytes.Buffer{}, "linux", func() int { return 1000 }, &fakeLocalKeyCleaner{})
	if !handled || err == nil || !strings.Contains(err.Error(), "root") {
		t.Fatalf("handled, err = %v, %v; want selected root error", handled, err)
	}
}

func TestRunLocalKeyCleanupHelper_RejectsNonLinux(t *testing.T) {
	cleaner := &fakeLocalKeyCleaner{}
	var stdout bytes.Buffer
	handled, err := runLocalKeyCleanupHelper(context.Background(), []string{"__brev-disable-ssh-cleanup"}, &stdout, "darwin", func() int { return 0 }, cleaner)
	if !handled || err == nil || !strings.Contains(err.Error(), "only supported on Linux") {
		t.Fatalf("handled, err = %v, %v; want selected Linux-only error", handled, err)
	}
	if cleaner.calls != 0 || stdout.Len() != 0 {
		t.Fatalf("cleaner calls = %d, stdout = %q; want no side effects", cleaner.calls, stdout.String())
	}
}

func TestRunLocalKeyCleanupHelper_EmitsJSON(t *testing.T) {
	cleaner := &fakeLocalKeyCleaner{result: KeyCleanupResult{AccountsScanned: 4, AccountsChanged: 2, KeysRemoved: 3}}
	var stdout bytes.Buffer
	handled, err := runLocalKeyCleanupHelper(context.Background(), []string{"__brev-disable-ssh-cleanup"}, &stdout, "linux", func() int { return 0 }, cleaner)
	if err != nil || !handled {
		t.Fatalf("handled, err = %v, %v; want true, nil", handled, err)
	}
	if want := "{\"accounts_scanned\":4,\"accounts_changed\":2,\"keys_removed\":3}\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want JSON only %q", stdout.String(), want)
	}
}
