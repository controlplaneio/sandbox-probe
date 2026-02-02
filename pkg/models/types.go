package models

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

type UserIdentity struct {
	UID    int
	GID    int
	EUID   int
	EGID   int
	Groups []int
}
