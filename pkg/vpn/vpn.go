package vpn

import (
	"fmt"
	"os"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/vpn/tailscaled"
	"tailscale.com/cmd/tailscale/cli"
)

type VPN interface {
	ApplyConfig(hostName string, loginServerURL string) error
	Start() error
}

type Tailscale struct{}

var _ VPN = Tailscale{} // tailscale is a vpn

func (t Tailscale) ApplyConfig(hostName string, loginServerURL string) error {
	// tailscale up --login-server https://8080-headscale-9izu-brevdev.brev.sh --hostname=me
	err := cli.Run([]string{"up", fmt.Sprintf("--hostname=%s", hostName), fmt.Sprintf("--login-server=%s", loginServerURL)})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (t Tailscale) Start() error {
	// os.Args = []string{"tailscaled"}                                                                 // macos
	os.Args = []string{"tailscaled", "--tun=userspace-networking", "--socks5-server=localhost:1055"} // brev workspace
	tailscaled.Run()
	return nil
}
