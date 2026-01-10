package spark

// Host represents a Spark target discovered from the NVIDIA Sync ssh_config.
// Options holds any additional ssh config options we should pass through
// (e.g., ProxyJump, StrictHostKeyChecking).
type Host struct {
	Alias        string
	Hostname     string
	User         string
	Port         int
	IdentityFile string
	Options      map[string]string
}

// HostResolver discovers Spark hosts available for connection.
type HostResolver interface {
	ResolveHosts() ([]Host, error)
}

// Prompter surfaces interactive selection to the user.
type Prompter interface {
	Select(label string, options []string) (string, error)
}

// Executor runs the ssh command (or other Spark-related commands later).
type Executor interface {
	Run(argv []string) error
}
