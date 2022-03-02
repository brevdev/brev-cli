package autostartconf

const targetBin = "/usr/local/bin/brev"

type DaemonConfigurer interface {
	Install() error
	UnInstall() error
}
