package tasks

import (
	"fmt"
	"os"
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
	// add other runtimes as needed
)

var readFile = os.ReadFile

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

	// Try reading cgroup file
	cgroups, err := readFile(cgroupFile)
	if err == nil {
		runtime := stringToContainerRuntime(string(cgroups))
		if runtime != RuntimeNotFound {
			return runtime
		}
	}

	// OpenVZ detection
	if fileExistsFunc("/proc/vz") && !fileExistsFunc("/proc/bc") {
		return RuntimeOpenVZ
	}

	// gVisor detection
	if fileExistsFunc("/__runsc_containers__") {
		return RuntimeGVisor
	}

	// PID cmdline
	proc, err := getRunningProcessCommandLinux(pid)
	if err == nil {
		runtime := stringToContainerRuntime(proc.Command)
		if runtime != RuntimeNotFound {
			return runtime
		}
	}

	// Parent commands, maximum 10 iterations
	iter := 0
	for pid > 1 && iter < 10 {
		proc, err := getRunningParentProcessLinux(pid)
		if err != nil {
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

	// container env variable
	if containerEnv := os.Getenv("container"); containerEnv != "" {
		runtime := stringToContainerRuntime(containerEnv)
		if runtime != RuntimeNotFound {
			return runtime
		}
	}

	// /run/systemd/container
	systemdContainerFile := "/run/systemd/container"
	if data, err := readFile(systemdContainerFile); err == nil {
		runtime := stringToContainerRuntime(string(data))
		if runtime != RuntimeNotFound {
			return runtime
		}
	}

	return RuntimeNotFound
}

// stringToContainerRuntime parses a string for known runtime keywords
func stringToContainerRuntime(s string) ContainerRuntime {
	switch {
	case strings.Contains(s, "docker"):
		return RuntimeDocker
	case strings.Contains(s, "podman"):
		return RuntimePodman
	case strings.Contains(s, "lxc"):
		return RuntimeLXC
	case strings.Contains(s, "firejail"):
		return RuntimeFirejail
	case strings.Contains(s, "seatbelt"):
		return RuntimeSeatbelt
	}
	return RuntimeNotFound
}

// GetBubbleWrap tries to detect if the system is running in bwrap
func GetBubbleWrap(pid int) (bool, error) {
	proc, err := getRunningParentProcessLinux(pid)
	if err != nil || proc.PID < 0 {
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
