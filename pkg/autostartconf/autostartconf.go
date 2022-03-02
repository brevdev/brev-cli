package autostartconf

const targetBin = "/usr/local/bin/brev"

type AutoStartStore interface {
	CopyBin(targetBin string) error
	WriteString(path, data string) error
}

type DaemonConfigurer interface {
	Install() error
	UnInstall() error
}
