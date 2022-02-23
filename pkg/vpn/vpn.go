package vpn

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/vpn/tailscaled"
	"github.com/hpcloud/tail"
	"github.com/spf13/afero"
	"tailscale.com/cmd/tailscale/cli"
)

type VPN interface {
	ApplyConfig(hostName string, loginServerURL string) error
	Start() error
}

type VPNStore interface {
	RegisterNode(publicKey string) error
	GetOrCreateFile(path string) (afero.File, error)
}

type Tailscale struct {
	store               VPNStore
	userspaceNetworking bool
	socksProxyPort      int
}

func NewTailscale(store VPNStore) *Tailscale {
	return &Tailscale{store: store}
}

func (t *Tailscale) WithUserspaceNetworking(shouldUseUserspaceNetworking bool) *Tailscale {
	t.userspaceNetworking = shouldUseUserspaceNetworking
	return t
}

func (t *Tailscale) WithSockProxyPort(sockProxyPort int) *Tailscale {
	t.socksProxyPort = sockProxyPort
	return t
}

var _ VPN = Tailscale{} // tailscale is a vpn

func (t Tailscale) ApplyConfig(hostName string, loginServerURL string) error {
	outfilePath, err := files.GetTailScaleOutFilePath()
	if err != nil {
		panic(err)
	}
	outfile, err := t.store.GetOrCreateFile(*outfilePath)
	if err != nil {
		panic(err)
	}
	origStderr := os.Stderr

	go func() {
		err = doOnFileTailLine(*outfilePath, t.handleTailscaleOutput)
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
	args := []string{"tailscaled"}
	if t.userspaceNetworking {
		args = append(args, "--tun=userspace-networking")
	}
	if t.socksProxyPort != 0 {
		args = append(args, fmt.Sprintf("--socks5-server=localhost:%d", t.socksProxyPort))
	}
	os.Args = args

	if runtime.GOOS == "darwin" {
		out, err := exec.Command("networksetup", "-getdnsservers", "Wi-Fi").Output()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		fmt.Println(out)
		// cmd = exec.Command("networksetup", "-setdnsservers", "Wi-Fi", "100.100.100.100", "1.1.1.1")
		// err = cmd.Run()
		// if err != nil {
		// 	return breverrors.WrapAndTrace(err)
		// }
		// cmd = exec.Command("networksetup", "-setsearchdomains", "Wi-Fi", "100.100.100.100")
		// err = cmd.Run()
		// if err != nil {
		// 	return breverrors.WrapAndTrace(err)
		// }
		// fmt.Println("Hello from mac")
	}
	tailscaled.Run()
	return nil
}
