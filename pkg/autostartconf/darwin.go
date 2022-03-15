package autostartconf

type DarwinPlistConfigurer struct {
	Store           AutoStartStore
	ValueConfigFile string
	DestConfigFile  string
	ServiceName     string
	ServiceType     string
}

func (dpc DarwinPlistConfigurer) UnInstall() error { return nil }
func (dpc DarwinPlistConfigurer) Install() error   { return nil }

