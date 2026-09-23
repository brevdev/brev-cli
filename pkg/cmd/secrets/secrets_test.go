package secrets

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type fakeSecretStore struct {
	org                   *entity.Organization
	allSecrets            []*nodev1.ManagedSecret
	secretsByID           map[string]*nodev1.ManagedSecret
	secretsByName         map[string]*nodev1.ManagedSecret
	versionsBySecretID    map[string][]*nodev1.ManagedSecretVersion
	valuesBySecretVersion map[string]string

	createdOrgID      string
	createdName       string
	createdValue      string
	setSecretID       string
	setValue          string
	deletedID         string
	gotValueSecretID  string
	gotValueVersionID string
}

func newFakeSecretStore(secrets ...*nodev1.ManagedSecret) *fakeSecretStore {
	f := &fakeSecretStore{
		org:                   &entity.Organization{ID: "org-1", Name: "test-org"},
		secretsByID:           map[string]*nodev1.ManagedSecret{},
		secretsByName:         map[string]*nodev1.ManagedSecret{},
		versionsBySecretID:    map[string][]*nodev1.ManagedSecretVersion{},
		valuesBySecretVersion: map[string]string{},
	}
	for _, s := range secrets {
		f.allSecrets = append(f.allSecrets, s)
		f.secretsByID[s.GetSecretId()] = s
		f.secretsByName[s.GetName()] = s
	}
	return f
}

func (f *fakeSecretStore) withVersions(secretID string, versions ...*nodev1.ManagedSecretVersion) *fakeSecretStore { //nolint:unparam // test ok
	f.versionsBySecretID[secretID] = versions
	return f
}

func (f *fakeSecretStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return f.org, nil
}

func (f *fakeSecretStore) CreateManagedSecret(orgID string, name string, value string) (*nodev1.ManagedSecret, error) {
	f.createdOrgID = orgID
	f.createdName = name
	f.createdValue = value
	return &nodev1.ManagedSecret{Name: name, LatestVersionNumber: 1}, nil
}

func (f *fakeSecretStore) ListManagedSecrets(_ string) ([]*nodev1.ManagedSecret, error) {
	return f.allSecrets, nil
}

func (f *fakeSecretStore) GetManagedSecret(id string) (*nodev1.ManagedSecret, error) {
	if s, ok := f.secretsByID[id]; ok {
		return s, nil
	}
	return nil, breverrors.NewValidationError(fmt.Sprintf("secret %q not found", id))
}

func (f *fakeSecretStore) GetManagedSecretByName(_ string, name string) (*nodev1.ManagedSecret, error) {
	if s, ok := f.secretsByName[name]; ok {
		return s, nil
	}
	return nil, breverrors.NewValidationError(fmt.Sprintf("secret %q not found", name))
}

func (f *fakeSecretStore) ListManagedSecretVersions(secretID string) ([]*nodev1.ManagedSecretVersion, error) {
	return f.versionsBySecretID[secretID], nil
}

func (f *fakeSecretStore) GetManagedSecretValue(secretID string, versionID string) (string, error) {
	f.gotValueSecretID = secretID
	f.gotValueVersionID = versionID
	return f.valuesBySecretVersion[secretID+"/"+versionID], nil
}

func (f *fakeSecretStore) SetManagedSecretValue(secretID string, value string) (*nodev1.ManagedSecretVersion, error) {
	f.setSecretID = secretID
	f.setValue = value
	return &nodev1.ManagedSecretVersion{VersionId: "msecv-new", VersionNumber: 2}, nil
}

func (f *fakeSecretStore) DeleteManagedSecret(secretID string) error {
	f.deletedID = secretID
	return nil
}

func testSecret(id string, name string) *nodev1.ManagedSecret {
	return &nodev1.ManagedSecret{
		SecretId:   id,
		Name:       name,
		CreateTime: timestamppb.New(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)),
	}
}

func testVersion(id string, number int64) *nodev1.ManagedSecretVersion {
	return &nodev1.ManagedSecretVersion{
		VersionId:     id,
		VersionNumber: number,
		CreateTime:    timestamppb.New(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)),
	}
}

// captureStdout swaps os.Stdout for a pipe so both the table writers and the
// Terminal (which captures os.Stdout at construction) can be asserted on.
func captureStdout(t *testing.T) func() string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() { _ = r.Close() })
	return func() string {
		require.NoError(t, w.Close())
		os.Stdout = old
		out, err := io.ReadAll(r)
		require.NoError(t, err)
		return string(out)
	}
}

func executeSecretsCmd(t *testing.T, store *fakeSecretStore, args ...string) (string, error) {
	t.Helper()
	readOutput := captureStdout(t)
	cmd := NewCmdSecrets(terminal.New(), store)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	return readOutput(), err
}

func assertValidationError(t *testing.T, err error, messageContains string) {
	t.Helper()
	require.Error(t, err)
	var ve breverrors.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Message, messageContains)
}

func TestResolveSecret(t *testing.T) {
	t.Run("by id", func(t *testing.T) {
		store := newFakeSecretStore(testSecret("msec-1", "my-secret"))

		got, err := resolveSecret(store, "msec-1", "")

		require.NoError(t, err)
		assert.Equal(t, "msec-1", got.GetSecretId())
	})

	t.Run("by name", func(t *testing.T) {
		store := newFakeSecretStore(testSecret("msec-1", "my-secret"))

		got, err := resolveSecret(store, "", "my-secret")

		require.NoError(t, err)
		assert.Equal(t, "my-secret", got.GetName())
	})

	t.Run("by name with spaces", func(t *testing.T) {
		store := newFakeSecretStore(testSecret("msec-2", "foo bar baz"))

		got, err := resolveSecret(store, "", "foo bar baz")

		require.NoError(t, err)
		assert.Equal(t, "msec-2", got.GetSecretId())
	})

	t.Run("both id and name errors", func(t *testing.T) {
		store := newFakeSecretStore(testSecret("msec-1", "my-secret"))

		_, err := resolveSecret(store, "msec-1", "my-secret")

		assertValidationError(t, err, "provide either --id or --name, not both")
	})

	t.Run("neither id nor name errors", func(t *testing.T) {
		store := newFakeSecretStore(testSecret("msec-1", "my-secret"))

		_, err := resolveSecret(store, "", "")

		assertValidationError(t, err, "provide --id or --name")
	})
}

func TestFindVersionID(t *testing.T) {
	store := newFakeSecretStore(testSecret("msec-1", "my-secret")).
		withVersions("msec-1", testVersion("msecv-1", 1), testVersion("msecv-2", 2))

	id, err := findVersionID(store, "msec-1", 2)

	require.NoError(t, err)
	assert.Equal(t, "msecv-2", id)

	_, err = findVersionID(store, "msec-1", 9)
	assertValidationError(t, err, "version 9 not found")
}

func TestFindLatestVersionID(t *testing.T) {
	store := newFakeSecretStore(testSecret("msec-1", "my-secret")).
		withVersions("msec-1", testVersion("msecv-1", 1), testVersion("msecv-3", 3), testVersion("msecv-2", 2))

	id, err := findLatestVersionID(store, "msec-1")

	require.NoError(t, err)
	assert.Equal(t, "msecv-3", id)

	empty := newFakeSecretStore(testSecret("msec-1", "my-secret"))
	_, err = findLatestVersionID(empty, "msec-1")
	assertValidationError(t, err, "secret has no versions")
}

func TestFormatTimestamp(t *testing.T) {
	assert.Empty(t, formatTimestamp(nil))

	ts := timestamppb.New(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	assert.Equal(t, "2026-01-02T03:04:05Z", formatTimestamp(ts))
}

func TestReadSecretValue(t *testing.T) {
	t.Run("from flag", func(t *testing.T) {
		value, err := readSecretValue("flag-value")

		require.NoError(t, err)
		assert.Equal(t, "flag-value", value)
	})

	t.Run("from piped stdin", func(t *testing.T) {
		oldStdin := os.Stdin
		r, w, err := os.Pipe()
		require.NoError(t, err)
		os.Stdin = r
		t.Cleanup(func() {
			os.Stdin = oldStdin
			_ = r.Close()
		})

		_, err = w.WriteString("piped-value")
		require.NoError(t, err)
		require.NoError(t, w.Close())

		value, err := readSecretValue("")

		require.NoError(t, err)
		assert.Equal(t, "piped-value", value)
	})

	t.Run("missing value errors", func(t *testing.T) {
		devNull, err := os.Open(os.DevNull)
		require.NoError(t, err)
		oldStdin := os.Stdin
		os.Stdin = devNull
		t.Cleanup(func() {
			os.Stdin = oldStdin
			_ = devNull.Close()
		})

		_, err = readSecretValue("")

		assertValidationError(t, err, "secret value required")
	})
}

func TestGetValueCommand(t *testing.T) {
	secret := testSecret("msec-1", "my-secret")

	t.Run("version number resolves to version id", func(t *testing.T) {
		store := newFakeSecretStore(secret).
			withVersions("msec-1", testVersion("msecv-1", 1), testVersion("msecv-2", 2))
		store.valuesBySecretVersion["msec-1/msecv-2"] = "v2-value"

		out, err := executeSecretsCmd(t, store, "get-value", "--id", "msec-1", "--version", "2")

		require.NoError(t, err)
		assert.Equal(t, "v2-value", out) // value as-is, no trailing newline
		assert.Equal(t, "msec-1", store.gotValueSecretID)
		assert.Equal(t, "msecv-2", store.gotValueVersionID)
	})

	t.Run("version id passes through", func(t *testing.T) {
		store := newFakeSecretStore(secret).
			withVersions("msec-1", testVersion("msecv-1", 1))
		store.valuesBySecretVersion["msec-1/msecv-9"] = "raw-id-value"

		out, err := executeSecretsCmd(t, store, "get-value", "--id", "msec-1", "--version", "msecv-9")

		require.NoError(t, err)
		assert.Equal(t, "raw-id-value", out)
		assert.Equal(t, "msecv-9", store.gotValueVersionID)
	})

	t.Run("defaults to latest version", func(t *testing.T) {
		store := newFakeSecretStore(secret).
			withVersions("msec-1", testVersion("msecv-1", 1), testVersion("msecv-2", 2))
		store.valuesBySecretVersion["msec-1/msecv-2"] = "latest"

		out, err := executeSecretsCmd(t, store, "get-value", "--id", "msec-1")

		require.NoError(t, err)
		assert.Equal(t, "latest", out)
		assert.Equal(t, "msecv-2", store.gotValueVersionID)
	})

	t.Run("by name", func(t *testing.T) {
		store := newFakeSecretStore(secret).
			withVersions("msec-1", testVersion("msecv-1", 1))
		store.valuesBySecretVersion["msec-1/msecv-1"] = "named"

		out, err := executeSecretsCmd(t, store, "get-value", "--name", "my-secret")

		require.NoError(t, err)
		assert.Equal(t, "named", out)
	})

	t.Run("unknown version number errors", func(t *testing.T) {
		store := newFakeSecretStore(secret).
			withVersions("msec-1", testVersion("msecv-1", 1))

		_, err := executeSecretsCmd(t, store, "get-value", "--id", "msec-1", "--version", "9")

		assertValidationError(t, err, "version 9 not found")
	})

	t.Run("missing id and name errors", func(t *testing.T) {
		store := newFakeSecretStore(secret)

		_, err := executeSecretsCmd(t, store, "get-value")

		assertValidationError(t, err, "provide --id or --name")
	})

	t.Run("both id and name errors", func(t *testing.T) {
		store := newFakeSecretStore(secret)

		_, err := executeSecretsCmd(t, store, "get-value", "--id", "msec-1", "--name", "my-secret")

		assertValidationError(t, err, "provide either --id or --name, not both")
	})
}

func TestCreateCommand(t *testing.T) {
	t.Run("creates secret with name and value", func(t *testing.T) {
		store := newFakeSecretStore()

		out, err := executeSecretsCmd(t, store, "create", "--name", "new secret", "--value", "v4l")

		require.NoError(t, err)
		assert.Equal(t, "org-1", store.createdOrgID)
		assert.Equal(t, "new secret", store.createdName)
		assert.Equal(t, "v4l", store.createdValue)
		assert.Contains(t, out, "Created secret \"new secret\" (version 1)")
	})

	t.Run("requires name", func(t *testing.T) {
		store := newFakeSecretStore()

		_, err := executeSecretsCmd(t, store, "create", "--value", "v4l")

		assertValidationError(t, err, "secret name required")
	})
}

func TestSetValueCommand(t *testing.T) {
	store := newFakeSecretStore(testSecret("msec-1", "my-secret"))

	out, err := executeSecretsCmd(t, store, "set-value", "--id", "msec-1", "--value", "rotated")

	require.NoError(t, err)
	assert.Equal(t, "msec-1", store.setSecretID)
	assert.Equal(t, "rotated", store.setValue)
	assert.Contains(t, out, "Updated secret \"my-secret\" (version 2)")
}

func TestDeleteCommand(t *testing.T) {
	store := newFakeSecretStore(testSecret("msec-1", "my-secret"))

	out, err := executeSecretsCmd(t, store, "delete", "--id", "msec-1")

	require.NoError(t, err)
	assert.Equal(t, "msec-1", store.deletedID)
	assert.Contains(t, out, "Deleted secret my-secret")
}

func TestGetCommandShowsMetadataAndVersions(t *testing.T) {
	store := newFakeSecretStore(testSecret("msec-1", "my-secret")).
		withVersions("msec-1", testVersion("msecv-1", 1), testVersion("msecv-2", 2))

	out, err := executeSecretsCmd(t, store, "get", "--id", "msec-1")

	require.NoError(t, err)
	assert.Contains(t, out, "my-secret")
	assert.Contains(t, out, "msec-1")
	assert.Contains(t, out, "2026-01-02T03:04:05Z")
	assert.Contains(t, out, "msecv-2")
}

func TestLsCommandListsSecrets(t *testing.T) {
	store := newFakeSecretStore(
		testSecret("msec-1", "my-secret"),
		testSecret("msec-2", "foo bar baz"),
	)

	out, err := executeSecretsCmd(t, store, "ls")

	require.NoError(t, err)
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "CREATED")
	assert.Contains(t, out, "msec-1")
	assert.Contains(t, out, "my-secret")
	assert.Contains(t, out, "msec-2")
	assert.Contains(t, out, "foo bar baz")
}
