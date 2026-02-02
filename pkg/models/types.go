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
