package vpn

import (
	"fmt"
	"os"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/vpn/tailscaled"
	"github.com/hpcloud/tail"
	"tailscale.com/cmd/tailscale/cli"
)

type VPN interface {
	ApplyConfig(hostName string, loginServerURL string) error
	Start() error
}

type VPNStore interface {
	RegisterNode(publicKey string) error
}

type Tailscale struct {
	store VPNStore
}

func NewTailscale(store VPNStore) *Tailscale {
	return &Tailscale{store}
}

var _ VPN = Tailscale{} // tailscale is a vpn

func (t Tailscale) ApplyConfig(hostName string, loginServerURL string) error {
	outfile, err := os.Create("./out.txt") // todo choose better file and handle .brev
	if err != nil {
		panic(err)
	}
	origStderr := os.Stderr

	go func() {
		err = doOnFileTailLine("./out.txt", t.handleTailscaleOutput)
		if err != nil {
			fmt.Print(err)
		}
	}()

	cli.Stderr = outfile
	// // tailscale up --login-server https://8080-headscale-9izu-brevdev.brev.sh --hostname=me
	err = cli.Run([]string{"up", fmt.Sprintf("--hostname=%s", hostName), fmt.Sprintf("--login-server=%s", loginServerURL)})
	cli.Stderr = origStderr
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = outfile.Close()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (t Tailscale) handleTailscaleOutput(s string) error {
	if strings.Contains(s, "register?key=") {
		pubKey, err := getPublicKeyFromAuthString(s)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		err = t.store.RegisterNode(pubKey)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func doOnFileTailLine(filePath string, onLine func(string) error) error {
	t, err := tail.TailFile(filePath, tail.Config{Follow: true}) // todo choose better file
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	for line := range t.Lines {
		fmt.Println(line.Text)
		err := onLine(line.Text)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func getPublicKeyFromAuthString(authString string) (string, error) {
	//     http://127.0.0.1:8080/register?key=4d217e8083a8e1a0ca84f405a193a52a1f49616e40868899017be1787d455956
	split := strings.Split(authString, "register?key=")
	if len(split) != 2 {
		return "", fmt.Errorf("invalid string")
	}

	return strings.TrimSpace(split[1]), nil
}

func (t Tailscale) Start() error {
	// os.Args = []string{"tailscaled"}                                                                 // macos
	// logic to check if can use regular networking vs user networking -- catch failure?/detect?
	os.Args = []string{"tailscaled", "--tun=userspace-networking", "--socks5-server=localhost:1055"} // brev workspace
	tailscaled.Run()
	return nil
}
