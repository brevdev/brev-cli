package writeconnectionevent

import (
	"net"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func runCMDonEnv(privateKey, host, cmd string) error {
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return breverrors.WrapAndTrace(err, "unable to parse private key")
	}
	config := &ssh.ClientConfig{
		User: "ubuntu",
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// use OpenSSH's known_hosts file if you care about host validation
			return nil
		},
		Timeout: 5 * time.Second,
	}

	// Connect to the remote server and perform the SSH handshake.
	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		return breverrors.WrapAndTrace(err, "unable to connect")
	}
	session, err := client.NewSession()
	if err != nil {
		return breverrors.WrapAndTrace(err, "unable to create session: %v")
	}
	defer session.Close() //nolint:errcheck // defer
	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return breverrors.WrapAndTrace(err, "unable to run: %v \n %v", cmd, string(out))
	}
	err = client.Close()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

type wce interface {
	GetCurrentUserKeys() (*entity.UserKeys, error)
}

func WriteWCEOnEnv(store wce, name string) error {
	keys, err := store.GetCurrentUserKeys()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = runCMDonEnv(
		keys.PrivateKey,
		name,
		"sudo brev write-connection-event",
	)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}
