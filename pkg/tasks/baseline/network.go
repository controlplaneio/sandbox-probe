package tasks

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/controlplaneio/sandbox-probe/pkg/models"
	"github.com/rs/zerolog/log"
)

const (
	startPort = 1
	endPort   = 65535

	timeout = 3000 * time.Millisecond
	workers = 66535
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

func ScanTCP(host string) []int {
	ports := make(chan int, workers)
	results := make(chan int, 1024)

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for port := range ports {
				address := fmt.Sprintf("%s:%d", host, port)
				conn, err := net.DialTimeout("tcp", address, timeout)
				if err == nil {
					results <- port
					conn.Close()
				}
			}
		}()
	}

	go func() {
		for port := startPort; port <= endPort; port++ {
			ports <- port
		}
		close(ports)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var openPorts []int
	for port := range results {
		openPorts = append(openPorts, port)
	}

	return openPorts
}

// TODO: this method is not the best to get UDP ports exposed
// it fails for some ports that don't repond to empty queries
// develop OS specific method in the future
func ScanUDP(host string) []int {
	// TODO: fix usage in darwin
	// it reports all the ports because they all timeout
	if runtime.GOOS == "darwin" {
		return []int{}
	}
	ports := make(chan int, workers)
	results := make(chan int, 1024)

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for port := range ports {
				address := fmt.Sprintf("%s:%d", host, port)

				conn, err := net.DialTimeout("udp", address, timeout)
				if err != nil {
					continue
				}

				_, err = conn.Write([]byte{})
				if err == nil {
					_ = conn.SetReadDeadline(time.Now().Add(timeout))
					buf := make([]byte, 1)
					_, err = conn.Read(buf)

					// responded OR silent → open|filtered
					if err == nil || netErrTimeout(err) {
						results <- port
					}
				}
				conn.Close()
			}
		}()
	}

	go func() {
		for port := startPort; port <= endPort; port++ {
			ports <- port
		}
		close(ports)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var openPorts []int
	for port := range results {
		openPorts = append(openPorts, port)
	}

	return openPorts
}

func netErrTimeout(err error) bool {
	if err == nil {
		return false
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return true
	}
	return false
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
	roots := []string{
		"/run", "/var/run", "/dev",
		"/tmp", "/var/tmp",
		"/private/var/run", "/private/tmp", "/private/var/tmp",
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
