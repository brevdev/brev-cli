package autostartconf

const (
	targetBin = "/usr/local/bin/brev"
)

type DaemonConfigurer interface {
	WriteString(path, data string) error
	Install() error
	UnInstall() error
}
