package spark

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kevinburke/ssh_config"
	"github.com/spf13/afero"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type SyncConfigResolver struct {
	fs      afero.Fs
	locator ConfigLocator
	homeDir func() (string, error)
}

func NewDefaultSyncConfigResolver(fs afero.Fs, locator ConfigLocator) SyncConfigResolver {
	return NewSyncConfigResolver(fs, locator, os.UserHomeDir)
}

func NewSyncConfigResolver(fs afero.Fs, locator ConfigLocator, home func() (string, error)) SyncConfigResolver {
	return SyncConfigResolver{
		fs:      fs,
		locator: locator,
		homeDir: home,
	}
}

func (r SyncConfigResolver) ResolveHosts() ([]Host, error) {
	configPath, err := r.locator.ConfigPath()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	exists, err := afero.Exists(r.fs, configPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if !exists {
		return nil, breverrors.WrapAndTrace(fmt.Errorf("Sync ssh_config not found at %s. Launch NVIDIA Sync then rerun.", configPath))
	}

	data, err := afero.ReadFile(r.fs, configPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	cfg, err := ssh_config.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, breverrors.WrapAndTrace(fmt.Errorf("failed to parse Sync ssh_config at %s", configPath))
	}

	home, err := r.homeDir()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var hosts []Host
	seen := map[string]bool{}
	for _, hostBlock := range cfg.Hosts {
		aliases := sparkAliases(hostBlock.Patterns)
		if len(aliases) == 0 {
			continue
		}

		kvs := collectKVs(hostBlock.Nodes)
		for _, alias := range aliases {
			if seen[alias] {
				continue
			}
			h, err := buildHost(alias, kvs, home)
			if err != nil {
				return nil, breverrors.WrapAndTrace(err)
			}
			seen[alias] = true
			hosts = append(hosts, h)
		}
	}

	if len(hosts) == 0 {
		return nil, breverrors.WrapAndTrace(fmt.Errorf("no Spark hosts found in %s. Launch NVIDIA Sync then rerun.", configPath))
	}

	return hosts, nil
}

func buildHost(alias string, kvs map[string]string, home string) (Host, error) {
	hostname := kvs["Hostname"]
	if hostname == "" {
		return Host{}, fmt.Errorf("missing Hostname for %s", alias)
	}

	user := kvs["User"]
	if user == "" {
		return Host{}, fmt.Errorf("missing User for %s", alias)
	}

	port := 22
	if portStr := kvs["Port"]; portStr != "" {
		parsed, err := strconv.Atoi(portStr)
		if err != nil {
			return Host{}, fmt.Errorf("invalid Port for %s: %s", alias, portStr)
		}
		port = parsed
	}

	identityFile := kvs["IdentityFile"]
	if identityFile == "" {
		return Host{}, fmt.Errorf("missing IdentityFile for %s", alias)
	}
	identityFile = expandPath(identityFile, home)

	options := map[string]string{}
	for k, v := range kvs {
		if isCoreField(k) {
			continue
		}
		options[k] = v
	}

	return Host{
		Alias:        alias,
		Hostname:     hostname,
		User:         user,
		Port:         port,
		IdentityFile: identityFile,
		Options:      options,
	}, nil
}

func sparkAliases(patterns []*ssh_config.Pattern) []string {
	var aliases []string
	for _, p := range patterns {
		name := strings.TrimSpace(p.String())
		// Include all explicitly named hosts; skip wildcards.
		if name == "" || name == "*" {
			continue
		}
		aliases = append(aliases, name)
	}
	return aliases
}

func collectKVs(nodes []ssh_config.Node) map[string]string {
	result := map[string]string{}
	for _, node := range nodes {
		kv, ok := node.(*ssh_config.KV)
		if !ok {
			continue
		}
		key := strings.TrimSpace(kv.Key)
		value := strings.TrimSpace(kv.Value)
		if key == "" {
			continue
		}
		result[key] = value
	}
	return result
}

func expandPath(path string, home string) string {
	if path == "" {
		return path
	}

	if strings.HasPrefix(path, "~") {
		trimmed := strings.TrimPrefix(path, "~")
		return filepath.Join(home, strings.TrimPrefix(trimmed, string(filepath.Separator)))
	}

	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(home, path)
}

func isCoreField(key string) bool {
	switch strings.ToLower(key) {
	case "hostname", "user", "port", "identityfile":
		return true
	default:
		return false
	}
}
