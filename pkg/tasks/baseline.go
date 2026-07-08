package tasks

import (
	"context"
	"fmt"
	"os"

	reportv1 "github.com/controlplaneio/sandbox-probe/api/gen/proto/report/v1"
	"github.com/controlplaneio/sandbox-probe/pkg/models"
	baselineTasks "github.com/controlplaneio/sandbox-probe/pkg/tasks/baseline"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/structpb"
)

// var expectedTypes = map[string]reflect.Type{
// 	WRITEABLEPATHS:            reflect.TypeOf([]string{}),
// SENSITIVEREADABLEPATHS:    reflect.TypeOf([]string{}),

const TaskPrefix = "baseline"

// Helper function to convert *models.Process to structpb.Value
func processToInterface(p *models.Process) (*structpb.Value, error) {
	// Convert namespaces to a format that structpb can handle
	if p == nil {
		return nil, fmt.Errorf("received nil process")
	}
	var namespacesInterface []interface{}
	for _, ns := range p.Namespaces {
		nsMap := map[string]interface{}{
			"type":  ns.Type,
			"inode": ns.Inode,
		}
		namespacesInterface = append(namespacesInterface, nsMap)
	}

	// Convert Process struct to map for protobuf
	processMap := map[string]interface{}{
		"command":    p.Command,
		"pid":        p.PID,
		"ppid":       p.PPID,
		"namespaces": namespacesInterface,
	}

	return structpb.NewValue(processMap)
}

// Helper function to convert []string to []interface{} for structpb
func stringSliceToInterface(slice []string) []interface{} {
	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}

// Helper function to convert []int to []interface{} for structpb
func intSliceToInterface(slice []int) []interface{} {
	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}

type PathTask struct {
	baseTask
}

func NewPathTask() *PathTask {
	return &PathTask{
		baseTask: baseTask{
			name:        fmt.Sprintf("%s_filesystem_enumerator", TaskPrefix),
			description: "Scans filesystem for writable and sensitive readable paths",
		},
	}
}

func (t *PathTask) Run(ctx context.Context, ti Inputs) ([]*reportv1.Finding, error) {
	log.Info().Str("task", t.GetName()).Msg("Starting filesystem enumeration task")

	paths := baselineTasks.ScanTargetedPaths()
	log.Info().
		Int("writable_count", len(paths.WritablePaths)).
		Int("readable_count", len(paths.ReadablePaths)).
		Msg("Completed filesystem scan")

	// Convert []string to structpb.Value
	writableValue, err := structpb.NewValue(stringSliceToInterface(paths.WritablePaths))
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert writable paths to protobuf value")
		return nil, err
	}

	readableValue, err := structpb.NewValue(stringSliceToInterface(paths.ReadablePaths))
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert readable paths to protobuf value")
		return nil, err
	}

	log.Info().Str("task", t.GetName()).Msg("Filesystem enumeration task completed successfully")
	return []*reportv1.Finding{
		{
			FindingType: WRITEABLEPATHS,
			Task:        t.GetName(),
			Description: "Writeable paths",
			Value:       writableValue,
		},
		{
			FindingType: SENSITIVEREADABLEPATHS,
			Task:        t.GetName(),
			Description: "Readable sensitive paths",
			Value:       readableValue,
		},
	}, nil
}

// NetworkTask produces: EXTERNALHOSTDNSRESOLUTION, EXTERNALHOSTCONNECTIVITY, TCPPORTSOPEN, UDPPORTSOPEN
type NetworkTask struct {
	baseTask
	testHost     string
	testHostname string
}

func NewNetworkTask() *NetworkTask {
	return &NetworkTask{
		baseTask: baseTask{
			name:        fmt.Sprintf("%s_network_scanner", TaskPrefix),
			description: "Scans network for DNS resolution, connectivity, and open TCP/UDP ports",
		},
		testHost:     "localhost",
		testHostname: "google.com",
	}
}

func (t *NetworkTask) Run(ctx context.Context, ti Inputs) ([]*reportv1.Finding, error) {
	log.Info().Str("task", t.GetName()).Msg("Starting network scanning task")
	var findings []*reportv1.Finding

	// EXTERNALHOSTDNSRESOLUTION
	log.Info().Str("hostname", t.testHostname).Msg("Performing DNS query")
	ips, err := baselineTasks.DnsQuery(t.testHostname)
	var dnsHosts []string
	if err == nil {
		for _, ip := range ips {
			dnsHosts = append(dnsHosts, ip.String())
		}
		log.Info().Int("ip_count", len(dnsHosts)).Msg("DNS query successful")
	} else {
		log.Warn().Err(err).Msg("DNS query failed")
	}
	dnsValue, err := structpb.NewValue(stringSliceToInterface(dnsHosts))
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert DNS hosts to protobuf value")
		return nil, err
	}
	findings = append(findings, &reportv1.Finding{
		FindingType: EXTERNALHOSTDNSRESOLUTION,
		Task:        t.GetName(),
		Description: "External host DNS resolution",
		Value:       dnsValue,
	})

	// EXTERNALHOSTCONNECTIVITY
	connHosts := []string{}
	if len(dnsHosts) > 0 {
		connHosts = append(connHosts, t.testHostname)
		log.Info().Str("host", t.testHostname).Msg("External connectivity confirmed")
	}
	connValue, err := structpb.NewValue(stringSliceToInterface(connHosts))
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert connectivity hosts to protobuf value")
		return nil, err
	}
	findings = append(findings, &reportv1.Finding{
		FindingType: EXTERNALHOSTCONNECTIVITY,
		Task:        t.GetName(),
		Description: "External host connectivity",
		Value:       connValue,
	})

	// TCPPORTSOPEN
	log.Info().Str("host", t.testHost).Msg("Scanning TCP ports")
	tcpPorts := baselineTasks.ScanTCP(t.testHost)
	log.Info().Int("open_tcp_ports", len(tcpPorts)).Msg("TCP scan completed")

	if len(tcpPorts) > 0 {
		tcpValue, err := structpb.NewValue(intSliceToInterface(tcpPorts))
		if err != nil {
			log.Error().Err(err).Msg("Failed to convert TCP ports to protobuf value")
			return nil, err
		}
		findings = append(findings, &reportv1.Finding{
			FindingType: TCPPORTSOPEN,
			Task:        t.GetName(),
			Description: "Open TCP ports",
			Value:       tcpValue,
		})
	}

	// UDPPORTSOPEN
	log.Info().Str("host", t.testHost).Msg("Scanning UDP ports")
	udpPorts := baselineTasks.ScanUDP(t.testHost)
	log.Info().Int("open_udp_ports", len(udpPorts)).Msg("UDP scan completed")

	if len(udpPorts) > 0 {
		udpValue, err := structpb.NewValue(intSliceToInterface(udpPorts))
		if err != nil {
			log.Error().Err(err).Msg("Failed to convert UDP ports to protobuf value")
			return nil, err
		}
		findings = append(findings, &reportv1.Finding{
			FindingType: UDPPORTSOPEN,
			Task:        t.GetName(),
			Description: "Open UDP ports",
			Value:       udpValue,
		})
	}

	log.Info().Str("task", t.GetName()).Msg("Network scanning task completed successfully")
	return findings, nil
}

// ProxyTask produces: PROXYDETECTION
type ProxyTask struct {
	baseTask
}

func NewProxyTask() *ProxyTask {
	return &ProxyTask{
		baseTask: baseTask{
			name:        fmt.Sprintf("%s_proxy_detector", TaskPrefix),
			description: "Detects proxy configuration from environment variables",
		},
	}
}

func (t *ProxyTask) Run(ctx context.Context, ti Inputs) ([]*reportv1.Finding, error) {
	log.Info().Str("task", t.GetName()).Msg("Starting proxy detection task")

	proxy, err := baselineTasks.GetProxy()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get proxy configuration")
		return nil, err
	}

	log.Info().
		Str("http_proxy", proxy.HTTPProxy).
		Str("https_proxy", proxy.HTTPSProxy).
		Msg("Proxy configuration detected")

	// Convert ProxyConfig struct to map for protobuf
	proxyMap := map[string]interface{}{
		"http_proxy":  proxy.HTTPProxy,
		"https_proxy": proxy.HTTPSProxy,
		"all_proxy":   proxy.ALLProxy,
		"no_proxy":    proxy.NoProxy,
		"socks_proxy": proxy.SOCKSProxy,
		"pac_url":     proxy.PACURL,
	}

	proxyValue, err := structpb.NewValue(proxyMap)
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert proxy config to protobuf value")
		return nil, err
	}

	log.Info().Str("task", t.GetName()).Msg("Proxy detection task completed successfully")
	return []*reportv1.Finding{
		{
			FindingType: PROXYDETECTION,
			Task:        t.GetName(),
			Description: "Proxy configuration",
			Value:       proxyValue,
		},
	}, nil
}

// SocketTask produces: UNIXSOCKETDETECTION
type SocketTask struct {
	baseTask
	startPath string
}

func NewSocketTask() *SocketTask {
	return &SocketTask{
		baseTask: baseTask{
			name:        fmt.Sprintf("%s_socket_scanner", TaskPrefix),
			description: "Scans filesystem for Unix domain sockets",
		},
		startPath: "/",
	}
}

func (t *SocketTask) Run(ctx context.Context, ti Inputs) ([]*reportv1.Finding, error) {
	log.Info().Str("task", t.GetName()).Str("start_path", t.startPath).Msg("Starting Unix socket scanning task")

	sockets, err := baselineTasks.GetSockets(t.startPath, ti.Fast)
	if err != nil {
		log.Error().Err(err).Msg("Failed to scan for Unix sockets")
		return nil, err
	}

	log.Info().Int("socket_count", len(sockets)).Msg("Unix socket scan completed")

	socketsValue, err := structpb.NewValue(stringSliceToInterface(sockets))
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert sockets to protobuf value")
		return nil, err
	}

	log.Info().Str("task", t.GetName()).Msg("Unix socket scanning task completed successfully")
	return []*reportv1.Finding{
		{
			FindingType: UNIXSOCKETDETECTION,
			Task:        t.GetName(),
			Description: "Unix sockets detected",
			Value:       socketsValue,
		},
	}, nil
}

// ProcessTask produces: PROCESSDETECTION, PARENTPROCESSDETECTION
type ProcessTask struct {
	baseTask
}

func NewProcessTask() *ProcessTask {
	return &ProcessTask{
		baseTask: baseTask{
			name:        fmt.Sprintf("%s_process_scanner", TaskPrefix),
			description: "Detects running processes and parent process information",
		},
	}
}

func (t *ProcessTask) Run(ctx context.Context, ti Inputs) ([]*reportv1.Finding, error) {
	log.Info().Str("task", t.GetName()).Msg("Starting process scanning task")
	var findings []*reportv1.Finding

	// PROCESSDETECTION - All running processes
	log.Info().Msg("Scanning running processes")
	processes, err := baselineTasks.GetRunningProcesses()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get running processes")
		return nil, err
	}

	log.Info().Int("process_count", len(processes)).Msg("Process scan completed")

	for _, process := range processes {
		// skip error if it happens
		processValue, err := processToInterface(process)
		if err != nil {
			log.Warn().Err(err).Msg("Error converting process to interface")
			continue
		}
		findings = append(findings, &reportv1.Finding{
			FindingType: PROCESSDETECTION,
			Task:        t.GetName(),
			Description: "Running processes",
			Value:       processValue,
		})
	}

	// PARENTPROCESSDETECTION - Parent process
	log.Info().Int("pid", os.Getpid()).Msg("Detecting parent process")
	parentProc, err := baselineTasks.GetRunningParentProcess(os.Getpid())
	// skip error if it happens
	if err != nil {
		log.Info().Err(err).Msgf("Couldn't get parent process of %d", os.Getpid())
		return findings, nil
	}

	// skip error if it happens
	parentProcValue, err := processToInterface(parentProc)
	if err != nil {
		log.Warn().Err(err).Msg("Error converting parent process to interface")
		return findings, nil
	}

	findings = append(findings, &reportv1.Finding{
		FindingType: PARENTPROCESSDETECTION,
		Task:        t.GetName(),
		Description: "Parent process",
		Value:       parentProcValue,
	})

	log.Info().Str("task", t.GetName()).Msg("Process scanning task completed successfully")
	return findings, nil
}

// UserContextTask produces: USERCONTEXTDETECTION
type UserContextTask struct {
	baseTask
}

func NewUserContextTask() *UserContextTask {
	return &UserContextTask{
		baseTask: baseTask{
			name:        fmt.Sprintf("%s_user_context", TaskPrefix),
			description: "Detects user and group context information (UID, GID, EUID, EGID)",
		},
	}
}

func (t *UserContextTask) Run(ctx context.Context, ti Inputs) ([]*reportv1.Finding, error) {
	log.Info().Str("task", t.GetName()).Msg("Starting user context detection task")

	userInfo, err := baselineTasks.GetUserGroupInfo()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user/group information")
		return nil, err
	}

	log.Info().
		Int("uid", userInfo.UID).
		Int("gid", userInfo.GID).
		Int("euid", userInfo.EUID).
		Int("egid", userInfo.EGID).
		Msg("User context information retrieved")

	// Convert UserIdentity struct to map for protobuf (structpb.NewValue only
	// accepts primitives, maps, and slices — not arbitrary structs).
	groupsIface := make([]interface{}, len(userInfo.Groups))
	for i, g := range userInfo.Groups {
		groupsIface[i] = g
	}
	userMap := map[string]interface{}{
		"uid":    userInfo.UID,
		"gid":    userInfo.GID,
		"euid":   userInfo.EUID,
		"egid":   userInfo.EGID,
		"groups": groupsIface,
	}
	userValue, err := structpb.NewValue(userMap)
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert user info to protobuf value")
		return nil, err
	}

	log.Info().Str("task", t.GetName()).Msg("User context detection task completed successfully")
	return []*reportv1.Finding{
		{
			FindingType: USERCONTEXTDETECTION,
			Task:        t.GetName(),
			Description: "User context information",
			Value:       userValue,
		},
	}, nil
}

// HostnameTask produces: HOSTNAMEDETECTION
type HostnameTask struct {
	baseTask
}

func NewHostnameTask() *HostnameTask {
	return &HostnameTask{
		baseTask: baseTask{
			name:        fmt.Sprintf("%s_hostname", TaskPrefix),
			description: "Detects the system hostname",
		},
	}
}

func (t *HostnameTask) Run(ctx context.Context, ti Inputs) ([]*reportv1.Finding, error) {
	log.Info().Str("task", t.GetName()).Msg("Starting hostname detection task")

	hostname, err := baselineTasks.GetHostName()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get hostname")
		return nil, err
	}

	log.Info().Str("hostname", hostname).Msg("Hostname detected")

	hostnameValue, err := structpb.NewValue(hostname)
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert hostname to protobuf value")
		return nil, err
	}

	log.Info().Str("task", t.GetName()).Msg("Hostname detection task completed successfully")
	return []*reportv1.Finding{
		{
			FindingType: HOSTNAMEDETECTION,
			Task:        t.GetName(),
			Description: "Hostname",
			Value:       hostnameValue,
		},
	}, nil
}

// SandboxTask produces: SANDBOXDETECTION
type SandboxTask struct {
	baseTask
}

func NewSandboxTask() *SandboxTask {
	return &SandboxTask{
		baseTask: baseTask{
			name:        fmt.Sprintf("%s_sandbox_detector", TaskPrefix),
			description: "Detects container runtime and sandbox environments (Docker, Podman, LXC, etc.)",
		},
	}
}

func (t *SandboxTask) Run(ctx context.Context, ti Inputs) ([]*reportv1.Finding, error) {
	log.Info().Str("task", t.GetName()).Msg("Starting sandbox detection task")

	var findings []*reportv1.Finding

	// ── Container / wrapper runtime ───────────────────────────────────────────
	// Detects high-level wrappers (Docker, Podman, LXC, bwrap, firejail, etc.)
	// via cgroups, env vars, filesystem markers, and ancestor process names.
	// Reports the outermost named wrapper when one is found.
	log.Info().Msg("Detecting container runtime")
	runtime := baselineTasks.GetContainerRuntime(0, os.Getpid())
	runtimeStr := ""
	switch runtime {
	case baselineTasks.RuntimeDocker:
		runtimeStr = "docker"
	case baselineTasks.RuntimePodman:
		runtimeStr = "podman"
	case baselineTasks.RuntimeLXC:
		runtimeStr = "lxc"
	case baselineTasks.RuntimeOpenVZ:
		runtimeStr = "openvz"
	case baselineTasks.RuntimeGVisor:
		runtimeStr = "gvisor"
	case baselineTasks.RuntimeWSL:
		runtimeStr = "wsl"
	case baselineTasks.RuntimeFirejail:
		runtimeStr = "firejail"
	case baselineTasks.RuntimeSeatbelt:
		runtimeStr = "seatbelt"
	case baselineTasks.RuntimeLandlock:
		runtimeStr = "landlock"
	case baselineTasks.RuntimeNspawn:
		runtimeStr = "nspawn"
	case baselineTasks.RuntimeAppArmor:
		runtimeStr = "apparmor"
	case baselineTasks.RuntimeChroot:
		runtimeStr = "chroot"
	case baselineTasks.RuntimeUnknown:
		runtimeStr = "unknown"
	}

	// Detect bwrap explicitly (uid-map / ancestor walk)
	log.Info().Msg("Checking for bubblewrap")
	isBwrap, _ := baselineTasks.GetBubbleWrap(os.Getpid())
	if isBwrap {
		runtimeStr = "bubblewrap"
	}

	if runtimeStr != "" {
		log.Info().Str("runtime", runtimeStr).Msg("Container/wrapper runtime detected")
		rv, err := structpb.NewValue(runtimeStr)
		if err != nil {
			return nil, fmt.Errorf("failed to convert runtime to protobuf value: %w", err)
		}
		findings = append(findings, &reportv1.Finding{
			FindingType: SANDBOXDETECTION,
			Task:        t.GetName(),
			Description: "Container/wrapper runtime",
			Value:       rv,
		})
	} else {
		log.Info().Msg("No container/wrapper runtime detected")
	}

	// ── Kernel enforcement mechanisms ─────────────────────────────────────────
	// Reports active kernel-level enforcement independently of the wrapper.
	// Sandboxes that do NOT create namespaces (e.g. nono: Landlock + seccomp-
	// notify) are invisible to wrapper detection but appear here.
	// Multiple mechanisms may be active simultaneously.
	log.Info().Msg("Detecting active kernel enforcement mechanisms")
	mechanisms := baselineTasks.ActiveMechanisms()
	log.Info().Strs("mechanisms", mechanisms).Msg("Kernel enforcement mechanisms detected")

	for _, m := range mechanisms {
		mv, err := structpb.NewValue(m)
		if err != nil {
			return nil, fmt.Errorf("failed to convert mechanism to protobuf value: %w", err)
		}
		findings = append(findings, &reportv1.Finding{
			FindingType: SANDBOXDETECTION,
			Task:        t.GetName(),
			Description: "Active kernel enforcement mechanism",
			Value:       mv,
		})
	}

	log.Info().Str("task", t.GetName()).Msg("Sandbox detection task completed successfully")

	return findings, nil
}

// MountTask produces: MOUNTEDVOLUMESDETECTION
type MountTask struct {
	baseTask
}

func NewMountTask() *MountTask {
	return &MountTask{
		baseTask: baseTask{
			name:        fmt.Sprintf("%s_mount_scanner", TaskPrefix),
			description: "Detects host-mounted volumes and filesystem mounts",
		},
	}
}

func (t *MountTask) Run(ctx context.Context, ti Inputs) ([]*reportv1.Finding, error) {
	log.Info().Str("task", t.GetName()).Msg("Starting mount detection task")

	mounts, err := baselineTasks.GetHostMounts()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get host mounts")
		return nil, err
	}

	log.Info().Int("mount_count", len(mounts)).Msg("Host mounts detected")

	// Convert []Mount to []string for easier display
	mountStrings := make([]string, len(mounts))
	for i, m := range mounts {
		mountStrings[i] = fmt.Sprintf("%s -> %s (%s)", m.Source, m.Target, m.FSType)
	}

	mountValue, err := structpb.NewValue(stringSliceToInterface(mountStrings))
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert mounts to protobuf value")
		return nil, err
	}

	log.Info().Str("task", t.GetName()).Msg("Mount detection task completed successfully")
	return []*reportv1.Finding{
		{
			FindingType: MOUNTEDVOLUMESDETECTION,
			Task:        t.GetName(),
			Description: "Host-mounted volumes",
			Value:       mountValue,
		},
	}, nil
}

// GetAllTasks returns all baseline tasks
func GetBaselineTasks() []Task {
	return []Task{
		NewPathTask(),        // WRITEABLEPATHS, SENSITIVEREADABLEPATHS
		NewNetworkTask(),     // EXTERNALHOSTDNSRESOLUTION, EXTERNALHOSTCONNECTIVITY, TCPPORTSOPEN, UDPPORTSOPEN
		NewProxyTask(),       // PROXYDETECTION
		NewSocketTask(),      // UNIXSOCKETDETECTION
		NewProcessTask(),     // PROCESSDETECTION, PARENTPROCESSDETECTION
		NewUserContextTask(), // USERCONTEXTDETECTION
		NewHostnameTask(),    // HOSTNAMEDETECTION
		NewSandboxTask(),     // SANDBOXDETECTION
		NewMountTask(),       // MOUNTEDVOLUMESDETECTION
	}
}
