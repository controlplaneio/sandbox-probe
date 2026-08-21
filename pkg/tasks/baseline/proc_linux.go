package tasks

import (
	"bufio"
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
)

var isChroot = isChrootImpl

// isChrootImpl reports whether the process root differs from init's root (/proc/1/root) — the
// signature of a chroot. Best-effort: it needs /proc mounted and /proc/1/root readable, and chroot
// is by design hard to detect. Returns false on any error (not chrooted / can't tell).
func isChrootImpl() bool {
	var root, initRoot unix.Stat_t
	if err := unix.Stat("/", &root); err != nil {
		return false
	}
	if err := unix.Stat("/proc/1/root", &initRoot); err != nil {
		return false
	}
	return root.Dev != initRoot.Dev || root.Ino != initRoot.Ino
}

func isProcSelfSetNoNewPrivs() bool {
	r1, _, errno := unix.Syscall6(
		unix.SYS_PRCTL,
		uintptr(unix.PR_GET_NO_NEW_PRIVS),
		0, 0, 0, 0, 0,
	)

	if errno != 0 {
		log.Warn().Msgf("prctl PR_GET_NO_NEW_PRIVS failed: %v", errno)
		return false
	}

	enabled := r1 == 1
	log.Info().Msgf("NoNewPrivs is %v", enabled)
	return enabled
}

// isUserNamespaceWithUIDMap returns true when /proc/self/uid_map is anything other than the init
// namespace's identity map of the whole uid range — a shifted range, a truncated one, or one that
// keeps the invoking uid all mean a restricted user namespace. Whether the inner user was remapped
// to root is an invocation detail of the tool (bwrap is frequently invoked without remapping) and
// not a property of the namespace. An absent or unreadable map means no user namespace.
func isUserNamespaceWithUIDMap() bool {
	data, err := readFile("/proc/self/uid_map")
	if err != nil {
		return false
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.Fields(scanner.Text())
		// uid_map columns: inside-uid  outside-uid  count
		if len(line) < 3 {
			continue
		}
		insideUID, outsideUID, count := line[0], line[1], line[2]
		if insideUID == "0" && outsideUID == "0" && count == "4294967295" {
			continue // the init namespace's identity map
		}
		log.Info().Msgf("user namespace detected: uid %s inside maps to uid %s outside (count %s)", insideUID, outsideUID, count)
		return true
	}
	return false
}
