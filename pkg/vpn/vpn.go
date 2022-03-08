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
	Start() error
}

type VPNConfigurer interface {
	VPN
	ApplyConfig() error
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

func (t Tailscale) Start() error {
	args := []string{"tailscaled"}
	if t.userspaceNetworking {
		args = append(args, "--tun=userspace-networking")
	}
	if t.socksProxyPort != 0 {
		args = append(args, fmt.Sprintf("--socks5-server=localhost:%d", t.socksProxyPort))
	}
	os.Args = args

	done := func() error { return nil }
	var err error
	if runtime.GOOS == "darwin" {

		tailscaleDNSIP := "100.100.100.100"
		done, err = UseDarwinDNS(tailscaleDNSIP, "")
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	defer done() //nolint:errcheck // using to handle in case of panics
	tailscaled.Run()

	return nil
}

func UseDarwinDNS(tailscaleDNSIP string, searchDomain string) (func() error, error) {
	out, err := exec.Command("networksetup", "-getdnsservers", "Wi-Fi").Output()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	prevDNS := strings.Split(strings.TrimSpace(string(out)), "\n")

	args := []string{"-setdnsservers", "Wi-Fi", tailscaleDNSIP}
	args = append(args, prevDNS...)
	out, err = exec.Command("networksetup", args...).Output() //nolint:gosec // static strings
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	fmt.Printf("%s", out)

	out, err = exec.Command("networksetup", "-getsearchdomains", "Wi-Fi").Output()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	prevSearchDomains := strings.Split(strings.TrimSpace(string(out)), "\n")

	args = []string{"-setsearchdomains", "Wi-Fi", searchDomain}
	args = append(args, prevSearchDomains...)
	out, err = exec.Command("networksetup", args...).Output() //nolint:gosec // static strings
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	fmt.Printf("%s", out)

	return func() error {
		args := []string{"-setdnsservers", "Wi-Fi"}
		args = append(args, prevDNS...)
		out, err = exec.Command("networksetup", args...).Output()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		fmt.Printf("%s", out)

		args = []string{"-setsearchdomains", "Wi-Fi"}
		args = append(args, prevSearchDomains...)
		out, err = exec.Command("networksetup", args...).Output()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		return nil
	}, nil
}

func (t *Tailscale) WithConfigurerOptions(hostname string, loginServerURL string) *TailscaleConfigurer {
	return &TailscaleConfigurer{*t, hostname, loginServerURL, false}
}

type TailscaleConfigurer struct {
	Tailscale
	HostName          string
	LoginServerURL    string
	ShouldForceReauth bool
}

var _ VPNConfigurer = TailscaleConfigurer{}

func (t *TailscaleConfigurer) WithForceReauth(shouldForceReauth bool) *TailscaleConfigurer {
	t.ShouldForceReauth = shouldForceReauth
	return t
}

func (t TailscaleConfigurer) ApplyConfig() error {
	outfilePath, err := files.GetTailScaleOutFilePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	outfile, err := t.store.GetOrCreateFile(*outfilePath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
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
	args := []string{"up", fmt.Sprintf("--hostname=%s", t.HostName), fmt.Sprintf("--login-server=%s", t.LoginServerURL)}
	if t.ShouldForceReauth {
		args = append(args, "--force-reauth")
	}

	err = cli.Run(args)
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

func (t *TailscaleConfigurer) WithAuthKey(authKey string) *TailscaleConfigurerAuthKey {
	return &TailscaleConfigurerAuthKey{*t, authKey}
}

type TailscaleConfigurerAuthKey struct {
	TailscaleConfigurer
	AuthKey string
}

var _ VPNConfigurer = TailscaleConfigurerAuthKey{}

func (t TailscaleConfigurerAuthKey) ApplyConfig() error {
	args := []string{"up", fmt.Sprintf("--hostname=%s", t.HostName), fmt.Sprintf("--login-server=%s", t.LoginServerURL), fmt.Sprintf("--authkey=%s", t.AuthKey)}
	if t.ShouldForceReauth {
		args = append(args, "--force-reauth")
	}
	err := cli.Run(args)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
