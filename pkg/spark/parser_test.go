package spark

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type staticLocator struct {
	path string
}

func (s staticLocator) ConfigPath() (string, error) {
	return s.path, nil
}

func TestResolveHostsParsesSparkEntries(t *testing.T) {
	fs := afero.NewMemMapFs()
	configPath := "/tmp/ssh_config"
	config := `
Host spark-one spark-alt
  Hostname spark-one.local
  User ubuntu
  Port 2222
  IdentityFile ~/.ssh/spark_one
  ProxyJump jump.example.com
  StrictHostKeyChecking no

Host notspark
  Hostname ignored.local
  User nobody

Host spark-two
  Hostname 10.0.0.2
  User ec2-user
  IdentityFile /keys/two.pem
`
	err := afero.WriteFile(fs, configPath, []byte(config), 0o600)
	require.NoError(t, err)

	resolver := NewSyncConfigResolver(fs, staticLocator{path: configPath}, func() (string, error) { return "/home/tester", nil })
	hosts, err := resolver.ResolveHosts()
	require.NoError(t, err)
	require.Len(t, hosts, 3)

	require.Equal(t, "spark-one", hosts[0].Alias)
	require.Equal(t, "spark-alt", hosts[1].Alias)
	require.Equal(t, "spark-two", hosts[2].Alias)

	require.Equal(t, "/home/tester/.ssh/spark_one", hosts[0].IdentityFile)
	require.Equal(t, "spark-one.local", hosts[0].Hostname)
	require.Equal(t, 2222, hosts[0].Port)
	require.Equal(t, "ubuntu", hosts[0].User)
	require.Equal(t, map[string]string{"ProxyJump": "jump.example.com", "StrictHostKeyChecking": "no"}, hosts[0].Options)

	require.Equal(t, "/keys/two.pem", hosts[2].IdentityFile)
	require.Equal(t, 22, hosts[2].Port) // default
}

func TestResolveHostsMissingFile(t *testing.T) {
	fs := afero.NewMemMapFs()
	resolver := NewSyncConfigResolver(fs, staticLocator{path: "/tmp/missing"}, func() (string, error) { return "/home/tester", nil })

	_, err := resolver.ResolveHosts()
	require.Error(t, err)
}

func TestResolveHostsMissingFields(t *testing.T) {
	fs := afero.NewMemMapFs()
	configPath := "/tmp/ssh_config"
	config := `
Host spark-one
  User ubuntu
  IdentityFile ~/.ssh/spark_one
`
	err := afero.WriteFile(fs, configPath, []byte(config), 0o600)
	require.NoError(t, err)

	resolver := NewSyncConfigResolver(fs, staticLocator{path: configPath}, func() (string, error) { return "/home/tester", nil })
	_, err = resolver.ResolveHosts()
	require.Error(t, err)
	require.ErrorContains(t, err, "Hostname")
}
