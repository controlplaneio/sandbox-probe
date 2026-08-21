package tasks

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/controlplaneio/sandbox-probe/v6/pkg/models"
	"github.com/rs/zerolog/log"
)

func DnsQuery(name string) ([]net.IP, error) {
	r := &net.Resolver{
		PreferGo: false,
	}

	return r.LookupIP(context.TODO(), "ip", name)
}

func GetProxyFromEnv() (*models.ProxyConfig, error) {
	httpProxy := os.Getenv("HTTP_PROXY")
	if httpProxy == "" {
		httpProxy = os.Getenv("http_proxy")
	}
	httpsProxy := os.Getenv("HTTPS_PROXY")
	if httpsProxy == "" {
		httpsProxy = os.Getenv("https_proxy")
	}
	allProxy := os.Getenv("ALL_PROXY")
	if allProxy == "" {
		allProxy = os.Getenv("all_proxy")
	}
	noProxy := os.Getenv("NO_PROXY")
	if noProxy == "" {
		noProxy = os.Getenv("no_proxy")
	}

	return &models.ProxyConfig{
		HTTPProxy:  httpProxy,
		HTTPSProxy: httpsProxy,
		ALLProxy:   allProxy,
		NoProxy:    noProxy,
	}, nil
}

func GetProxy() (*models.ProxyConfig, error) {
	return GetProxyFromEnv()
}

func networkAddress(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func GetSockets(startPath string, fast bool) ([]string, error) {
	var mu sync.Mutex
	results := []string{}
	err := Walk(startPath, fast, func(path string, typ os.FileMode) error {
		if typ.IsDir() {
			return nil
		}

		if typ&os.ModeSocket != 0 {
			log.Info().Msgf("Socket %s found", path)
			mu.Lock()
			results = append(results, path)
			mu.Unlock()
		}
		return nil
	})
	if err != nil {
		return []string{}, err
	}
	return results, nil
}

// DefaultSocketRoots are the runtime dirs where Unix domain sockets live: FHS runtime dirs, the
// macOS /private real-paths, and per-user runtime dirs ($XDG_RUNTIME_DIR, $TMPDIR). Scanning these
// — not the whole root filesystem — finds every socket a sandbox can expose in well under a second.
// /var/lib is omitted on purpose (its sockets are mirrored under /run; its docker/overlay2 tree
// would re-introduce the multi-million-file walk).
func DefaultSocketRoots() []string {
	return socketRootsForHome(homeOrRoot())
}

// socketRootsForHome is the testable core: it takes the home directory instead
// of calling os.UserHomeDir().
func socketRootsForHome(home string) []string {
	roots := []string{
		"/run", "/var/run", "/dev",
		"/tmp", "/var/tmp",
		"/private/var/run", "/private/tmp", "/private/var/tmp",
		// Home-scoped runtime dirs the socket catalogue lives in: code-server's
		// IPC socket (Linux) and Docker Desktop's user socket (macOS) both sit
		// outside every FHS runtime dir, so a decoy planted at either would
		// otherwise never be found again. Both are small, unlike the
		// application-support trees deliberately left unscanned.
		filepath.Join(home, ".local", "share", "code-server"),
		filepath.Join(home, ".docker", "run"),
	}
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		roots = append(roots, d)
	}
	if d := os.Getenv("TMPDIR"); d != "" {
		roots = append(roots, d)
	}
	return roots
}

// ScanSocketRoots walks each root for Unix domain sockets, returning the deduped set. Roots are
// symlink-resolved and any nested inside another is dropped (so /var/run→/run, /tmp→/private/tmp
// aren't walked twice); a missing or unreadable root is skipped, not fatal.
func ScanSocketRoots(roots []string, fast bool) ([]string, error) {
	// Resolve + dedup + drop nested roots.
	var resolved []string
	seenRoot := map[string]bool{}
	for _, r := range roots {
		real, err := filepath.EvalSymlinks(r)
		if err != nil {
			continue // doesn't exist / unreadable
		}
		if seenRoot[real] {
			continue
		}
		seenRoot[real] = true
		resolved = append(resolved, real)
	}
	var kept []string
	for _, r := range resolved {
		nested := false
		for _, other := range resolved {
			if other != r && strings.HasPrefix(r, other+string(os.PathSeparator)) {
				nested = true
				break
			}
		}
		if !nested {
			kept = append(kept, r)
		}
	}

	seen := map[string]bool{}
	results := []string{}
	for _, root := range kept {
		socks, err := GetSockets(root, fast)
		if err != nil {
			log.Warn().Err(err).Str("root", root).Msg("Socket scan of root failed; continuing")
			continue
		}
		for _, s := range socks {
			if !seen[s] {
				seen[s] = true
				results = append(results, s)
			}
		}
	}
	return results, nil
}
