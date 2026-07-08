package tasks

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/controlplaneio/sandbox-probe/pkg/models"
	"github.com/prometheus/procfs"
	"github.com/rs/zerolog/log"
	"github.com/zricethezav/gitleaks/v8/detect"
)

// ContainerRuntime represents the container runtime type
type ContainerRuntime int

type Mount struct {
	Source string
	Target string
	FSType string
}

const (
	RuntimeNotFound ContainerRuntime = iota
	RuntimeDocker
	RuntimePodman
	RuntimeLXC
	RuntimeOpenVZ
	RuntimeGVisor
	RuntimeWSL
	RuntimeFirejail
	RuntimeSeatbelt
	RuntimeLandlock
	RuntimeBubblewrap
	RuntimeNspawn
	RuntimeAppArmor
	RuntimeChroot
	// add other runtimes as needed
	RuntimeUnknown
)

var readFile = os.ReadFile

// readProcAttr reads a /proc/*/attr file. os.ReadFile issues a follow-up read to detect EOF, which
// the AppArmor/SELinux attr interfaces reject with EINVAL — so it fails even though the first read
// returns the profile. Read in a loop, keeping whatever the first read yields and stopping on any
// error or short read. Returns "" on open error / no data.
var readProcAttr = func(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	var out []byte
	buf := make([]byte, 128)
	for len(out) < 4096 {
		n, err := f.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil || n < len(buf) {
			break
		}
	}
	return strings.TrimSpace(string(out))
}

// detectSensitiveEnvVars scans environment variables for secrets
func detectSensitiveEnvVars() ([]models.EnvFinding, error) {
	// TODO: Make it configurable
	detector, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		return nil, err
	}

	var results []models.EnvFinding

	for _, env := range os.Environ() {
		// env is "KEY=value"
		key, value, ok := splitEnv(env)
		if !ok {
			continue
		}

		// Scan VALUE
		findings := detector.DetectString(value)

		for _, f := range findings {
			results = append(results, models.EnvFinding{
				EnvKey:      key,
				EnvValue:    value,
				Description: f.Description,
			})
		}
	}

	return results, nil
}

func splitEnv(env string) (key, value string, ok bool) {
	for i := 0; i < len(env); i++ {
		if env[i] == '=' {
			return env[:i], env[i+1:], true
		}
	}
	return "", "", false
}

func GetUserGroupInfo() (*models.UserIdentity, error) {
	if runtime.GOOS == "windows" {
		return nil, errors.New("GetUserGroupInfo is not supported on Windows")
	}

	groups, err := os.Getgroups()
	if err != nil {
		return nil, err
	}

	return &models.UserIdentity{
		UID:    os.Getuid(),
		GID:    os.Getgid(),
		EUID:   os.Geteuid(),
		EGID:   os.Getegid(),
		Groups: groups,
	}, nil
}

func GetHostName() (string, error) {
	return os.Hostname()
}

// GetContainerRuntime detects the container runtime for a given process
// note it will only work for Linux based systems
func GetContainerRuntime(tgid, pid int) ContainerRuntime {
	// most detections are for linux so check for seatbelt for
	// an early exit on darwin
	if isSeatbelt() {
		return RuntimeSeatbelt
	}

	var cgroupFile string
	if pid > 0 {
		if tgid > 0 {
			cgroupFile = fmt.Sprintf("/proc/%d/task/%d/cgroup", tgid, pid)
		} else {
			cgroupFile = fmt.Sprintf("/proc/%d/cgroup", pid)
		}
	} else {
		cgroupFile = "/proc/self/cgroup"
	}

	// hold details on the runtime we think we've identified
	// start with NotFound, can progress to Unknown in the case where there
	// is some sandboxing but we still don't know which containerization type
	identifiedRuntime := RuntimeNotFound

	// Try reading cgroup file
	cgroups, err := readFile(cgroupFile)
	if err == nil {
		runtime := stringToContainerRuntime(string(cgroups))
		// found runtime, return early
		if runtime != RuntimeUnknown {
			return runtime
		}
		identifiedRuntime = runtime
	}

	if fileExistsFunc("/.dockerenv") {
		return RuntimeDocker
	}

	// OpenVZ detection
	if fileExistsFunc("/proc/vz") && !fileExistsFunc("/proc/bc") {
		return RuntimeOpenVZ
	}

	// gVisor detection: the container-mode marker, or its synthetic /proc/version (present under
	// `runsc run`, which does not create /__runsc_containers__).
	if fileExistsFunc("/__runsc_containers__") {
		return RuntimeGVisor
	}
	if data, err := readFile("/proc/version"); err == nil && strings.Contains(strings.ToLower(string(data)), "gvisor") {
		return RuntimeGVisor
	}

	// PID cmdline
	proc, err := getRunningProcessCommandLinux(pid)
	if err == nil && proc != nil {
		runtime := stringToContainerRuntime(proc.Command)
		if runtime != RuntimeNotFound {
			return runtime
		}
	}

	// Parent commands, maximum 10 iterations
	iter := 0
	for pid > 1 && iter < 10 {
		proc, err := getRunningParentProcessLinux(pid)
		if err != nil || proc == nil {
			break
		}
		log.Info().Msgf("found parent process with pid: %d and command %v", proc.PID, proc.Command)
		runtime := stringToContainerRuntime(proc.Command)
		if runtime != RuntimeNotFound {
			return runtime
		}

		pid = proc.PPID
		iter++
	}

	// WSL detection
	versionSignatureFile := "/proc/version_signature"
	if data, err := readFile(versionSignatureFile); err == nil {
		if strings.HasPrefix(string(data), "Microsoft") {
			return RuntimeWSL
		}
	}

	// /run/systemd/container names the container manager (e.g. "systemd-nspawn"); only trust it when
	// it maps to a known runtime, otherwise a bare/unknown marker would mask later detections.
	systemdContainerFile := "/run/systemd/container"
	if data, err := readFile(systemdContainerFile); err == nil {
		if runtime := stringToContainerRuntime(string(data)); runtime != RuntimeUnknown {
			return runtime
		}
	}

	// container env variable (same: only when it identifies a known runtime)
	if containerEnv := os.Getenv("container"); containerEnv != "" {
		if runtime := stringToContainerRuntime(containerEnv); runtime != RuntimeUnknown {
			return runtime
		}
	}

	// AppArmor: the current profile ("unconfined" when none applies; a named profile means confined).
	// Newer kernels (6.x) expose it at the LSM-specific path; older ones at the legacy attr path.
	for _, p := range []string{"/proc/self/attr/apparmor/current", "/proc/self/attr/current"} {
		if v := readProcAttr(p); v != "" && !strings.HasPrefix(v, "unconfined") {
			return RuntimeAppArmor
		}
	}

	landlock, err := probeForLandlock()
	if err != nil {
		log.Error().Err(err).Msg("failed to check for landlock")
	}
	if landlock {
		return RuntimeLandlock
	}

	// bwrap creates a user namespace mapping uid 0 inside to the real uid outside;
	// this is detectable even when the ancestor /proc entries are hidden by PID namespace.
	if isUserNamespaceWithUIDMap() {
		return RuntimeBubblewrap
	}

	// chroot: a differing root vs init's. Checked after the container/LSM detections above so a real
	// container (whose root also differs) is named first; this catches a bare chroot.
	if isChroot() {
		return RuntimeChroot
	}

	// no-new-privs is set by bwrap and by some other sandboxes (landlock, firejail);
	// treat it as a signal that some sandboxing is present even if we can't name it.
	if isProcSelfSetNoNewPrivs() {
		return RuntimeUnknown
	}

	return identifiedRuntime
}

// stringToContainerRuntime parses a string for known runtime keywords
func stringToContainerRuntime(s string) ContainerRuntime {
	switch {
	case strings.TrimSpace(s) == "":
		return RuntimeNotFound
	case strings.Contains(s, "docker"):
		return RuntimeDocker
	case strings.Contains(s, "podman"):
		return RuntimePodman
	case strings.Contains(s, "lxc"):
		return RuntimeLXC
	case strings.Contains(s, "nspawn"):
		return RuntimeNspawn
	case strings.Contains(s, "firejail"):
		return RuntimeFirejail
	case strings.Contains(s, "seatbelt"):
		return RuntimeSeatbelt
	case strings.Contains(s, "landlock"):
		return RuntimeLandlock
	case strings.Contains(s, "bwrap"):
		return RuntimeBubblewrap
	}
	return RuntimeUnknown
}

// GetBubbleWrap tries to detect if the system is running in bwrap
func GetBubbleWrap(pid int) (bool, error) {
	proc, err := getRunningParentProcessLinux(pid)
	if err != nil || proc == nil || proc.PID < 0 {
		return false, nil
	}
	log.Info().Msgf("found this parent command %s", proc.Command)
	if strings.Contains(proc.Command, "bwrap") {
		return true, nil
	}

	is_bwrap, err := GetBubbleWrap(proc.PID)
	if err != nil {
		return false, nil
	}

	return is_bwrap, nil
}

// ActiveMechanisms returns the set of kernel enforcement mechanisms currently
// active for this process. This is mechanism-level detection (landlock,
// seccomp-filter, seccomp-notify, no-new-privs) rather than wrapper-level
// detection (docker, bwrap, nono). Multiple mechanisms may be active
// simultaneously — e.g. a nono session may show landlock + seccomp-notify +
// no-new-privs without any namespace isolation.
func ActiveMechanisms() []string {
	var mechanisms []string

	// /proc/self/status fields (all kernels that have the feature export it here)
	if s, err := readProcSelfStatus(); err == nil {
		// Seccomp field: 0=none, 1=strict, 2=filter/notify
		if v, ok := s["Seccomp"]; ok {
			switch v {
			case "1":
				mechanisms = append(mechanisms, "seccomp-strict")
			case "2":
				// Mode 2 covers both BPF filter and seccomp-notify supervisor.
				// Distinguish by filter count: notify-only supervisors attach zero
				// filters to the child, BPF filter sandboxes attach ≥1.
				if fc, ok2 := s["Seccomp_filters"]; ok2 && fc != "0" {
					mechanisms = append(mechanisms, "seccomp-filter")
				} else {
					mechanisms = append(mechanisms, "seccomp-notify")
				}
			}
		}

		// Landlock field: present and non-zero on kernel ≥ 6.6 when restricted.
		// Format: "Landlock:\t<ruleset-count>" — zero means kernel supports it
		// but no rulesets have been applied to this process.
		if v, ok := s["Landlock"]; ok && v != "0" && v != "" {
			mechanisms = append(mechanisms, "landlock")
		}

		// NoNewPrivs: set by bwrap, nono, firejail, and other sandboxes.
		// Not a sandbox on its own but a strong signal that some enforcement
		// is present.
		if v, ok := s["NoNewPrivs"]; ok && v == "1" {
			mechanisms = append(mechanisms, "no-new-privs")
		}
	}

	return mechanisms
}

// readProcSelfStatus parses /proc/self/status into a key→value map.
// Keys are the field names without the trailing colon; values are trimmed.
func readProcSelfStatus() (map[string]string, error) {
	data, err := readFile("/proc/self/status")
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.Index(line, ":"); i > 0 {
			key := strings.TrimSpace(line[:i])
			val := strings.TrimSpace(line[i+1:])
			result[key] = val
		}
	}
	return result, nil
}

func GetHostMounts() ([]Mount, error) {
	fs, err := procfs.NewFS("/proc")
	if err != nil {
		return []Mount{}, err
	}

	mounts, err := fs.GetMounts()
	if err != nil {
		return []Mount{}, err
	}

	var res []Mount

	for _, m := range mounts {
		if isLikelyHostMount(m) {
			res = append(res, Mount{
				Source: m.Source,
				Target: m.MountPoint,
				FSType: m.FSType,
			})
		}
	}

	return res, nil
}

func isLikelyHostMount(m *procfs.MountInfo) bool {
	// Ignore virtual/container-internal filesystems
	switch m.FSType {
	case "overlay", "tmpfs", "proc", "sysfs", "cgroup", "cgroup2", "devpts":
		return false
	}

	// Heuristics for host/VM-mounted paths
	if strings.HasPrefix(m.Source, "/") {
		return true
	}
	if strings.Contains(m.FSType, "fuse") {
		return true // macOS Docker Desktop mounts
	}

	return false
}
