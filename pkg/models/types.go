package models

// ProxyConfig contains all the fields with information about the proxy
type ProxyConfig struct {
	HTTPProxy  string
	HTTPSProxy string
	ALLProxy   string
	NoProxy    string
	SOCKSProxy string
	PACURL     string
}

// EnvFinding represents a secret found in an environment variable
type EnvFinding struct {
	EnvKey      string
	EnvValue    string
	Description string
}

// UserIdentity contains all the fields with information about user and its groups
type UserIdentity struct {
	UID    int
	GID    int
	EUID   int
	EGID   int
	Groups []int
}

// HostEnvironment describes the host/kernel the probe ran on, for longitudinal comparison of
// reports across kernel and OS upgrades (seccomp/landlock/userns/systrap behaviour tracks these).
type HostEnvironment struct {
	KernelRelease string // uname -r, e.g. "6.17.0-1018-azure"
	KernelVersion string // uname -v / /proc/version — build flags and date
	OSRelease     string // e.g. "Ubuntu 24.04.3 LTS" or "macOS 15.5"
}

// Process contains all the fields relevant to a process
type Process struct {
	Command    string
	PID        int
	PPID       int
	Namespaces []*Namespace
}

// Namespace contains all the fields relevant to a namespace
type Namespace struct {
	Type  string
	Inode uint32
}
