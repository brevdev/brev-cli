package spark

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakePrompter struct {
	selection string
	err       error
}

func (f fakePrompter) Select(_ string, _ []string) (string, error) {
	return f.selection, f.err
}

func TestSelectHostWithAlias(t *testing.T) {
	host := Host{Alias: "spark-one"}
	selected, err := SelectHost([]Host{host}, "spark-one", fakePrompter{})
	require.NoError(t, err)
	require.Equal(t, host, selected)
}

func TestSelectHostAutoPickSingle(t *testing.T) {
	host := Host{Alias: "spark-one"}
	selected, err := SelectHost([]Host{host}, "", fakePrompter{})
	require.NoError(t, err)
	require.Equal(t, host, selected)
}

func TestSelectHostPromptsWhenMultiple(t *testing.T) {
	hosts := []Host{
		{Alias: "spark-one", User: "u1", Hostname: "h1", Port: 22},
		{Alias: "spark-two", User: "u2", Hostname: "h2", Port: 22},
	}

	choice := fmt.Sprintf("%s (%s@%s:%d)", hosts[1].Alias, hosts[1].User, hosts[1].Hostname, hosts[1].Port)
	selected, err := SelectHost(hosts, "", fakePrompter{selection: choice})
	require.NoError(t, err)
	require.Equal(t, hosts[1], selected)
}

func TestSelectHostMissingAlias(t *testing.T) {
	hosts := []Host{{Alias: "spark-one"}}
	_, err := SelectHost(hosts, "spark-missing", fakePrompter{})
	require.Error(t, err)
}
