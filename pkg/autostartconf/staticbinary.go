package autostartconf

type StaticBinaryConfigurer struct {
	LinuxSystemdConfigurer
	URL  string
	Name string
}

// best effort

// download binary
