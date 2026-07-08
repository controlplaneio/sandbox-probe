package tasks

import (
	"bufio"
	"os"
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

// isUserNamespaceWithUIDMap returns true when /proc/self/uid_map shows that
// uid 0 inside the current namespace maps to a non-zero uid in the parent
// namespace — the primary indicator of a bwrap user namespace.
func isUserNamespaceWithUIDMap() bool {
	f, err := os.Open("/proc/self/uid_map")
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.Fields(scanner.Text())
		// uid_map columns: inside-uid  outside-uid  count
		if len(line) < 3 {
			continue
		}
		insideUID := line[0]
		outsideUID := line[1]
		if insideUID == "0" && outsideUID != "0" {
			log.Info().Msgf("user namespace detected: uid 0 inside maps to uid %s outside", outsideUID)
			return true
		}
	}
	return false
}
