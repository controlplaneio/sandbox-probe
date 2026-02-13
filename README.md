# Sandbox Probe

A security enumeration tool designed to detect and analyze sandbox environments in AI code assistants, identifying execution capabilities, system access, and potential security boundaries.

## Overview

Sandbox Probe is specifically designed to fingerprint the execution environment of AI code assistants (such as Claude Code, Gemini CLI, and similar tools) and identify:

- **Sandbox/Container Detection**: Docker, Podman, LXC, Firejail, Bubblewrap, gVisor, WSL, OpenVZ
- **File System Permissions**: Writable system paths and readable sensitive files
- **Network Capabilities**: DNS resolution, external connectivity, open TCP/UDP ports
- **Process Information**: Running processes and parent process detection
- **System Context**: User/group information, hostname, mounted volumes
- **Proxy Configuration**: Environment-based proxy detection

## Supported Code Assistants

- **[Gemini CLI](https://geminicli.com/)**
- **[Claude Code](https://code.claude.com/docs/en/overview)**

## Installation

### Prerequisites

**Required:**
- Go 1.25 or later
- Protocol Buffer compiler (`buf`) - install via `make install-buf`
- `jq` - JSON processor for parsing outputs
- `docker` - For containerized testing
- `podman` - For containerized testing
- `claude-code` - Claude Code CLI for Claude testing
- `gemini-cli` - Gemini Code Assist CLI for Gemini testing

### Build from Source

```bash
# Clone the repository
git clone https://github.com/controlplaneio/sandbox-probe.git
cd sandbox-probe

# Build the binary
make build
```

## Usage

### Basic Execution

Run all baseline probes (outside of the context of an AI assistant). It is useful just for testing the go code

```bash
./bin/sandbox-probe scan
```

### Test an Agent

For testing agents, please find the following scripts under the `tests` folder:

```
tests/
├── baseline_claude.sh
├── baseline_gemini.sh
├── sandbox_claude.sh
└── sandbox_gemini.sh
```

### Output

The tool generates multiple outputs:

1. **Console Output**: Structured logs showing probe execution progress
2. **report.json**: Detailed findings in JSON format
3. **Log Files**: Timestamped logs in `logs/` directory (e.g., `logs/sandbox-probe-2026-02-09-15-30-45.log`)

### Example Report

```json
{
  "version": "1.0.0",
  "timestamp": "2026-02-09T15:30:45Z",
  "probeBinary": {
    "goVersion": "go1.21.0",
    "os": "linux",
    "arch": "amd64",
    "static": false
  },
  "findings": [
    {
      "findingType": "sandbox_detection",
      "task": "baseline_sandbox_detector",
      "description": "Sandbox/container runtime",
      "value": "docker"
    }
  ]
}
```

## Development

### Available Tasks

The baseline probe includes the following tasks:

| Task Name                  | Description                                                                                 |
|----------------------------|---------------------------------------------------------------------------------------------|
| baseline_path_task          | Scans filesystem for writable and sensitive readable paths                                  |
| baseline_network_task       | Scans network for DNS resolution, connectivity, and open TCP/UDP ports                      |
| baseline_proxy_task         | Detects proxy configuration from environment variables                                     |
| baseline_socket_task        | Scans filesystem for Unix domain sockets                                                    |
| baseline_process_task       | Detects running processes and parent process information                                   |
| baseline_user_context_task  | Detects user and group context information (UID, GID, EUID, EGID)                           |
| ps_all_task                 | Lists all running processes using ps command                                               |
| baseline_hostname_task      | Detects the system hostname                                                                |
| baseline_sandbox_task       | Detects container runtime and sandbox environments (Docker, Podman, LXC, etc.)            |
| baseline_mount_task         | Detects host-mounted volumes and filesystem mounts                                         |
| ps_parent_task              | Gets parent process information using ps command                                           |
| ps_single_task              | Gets information about the running process using ps command                                 |


### Running Tests

```bash
# Run all e2e tests
make e2etest

# Format code
make fmt

# Install buf (Protocol Buffer tool)
make install-buf

# Generate Protocol Buffer code
cd api && buf generate
```

### Adding New Tasks

1. Create a new task struct in `pkg/tasks/baseline/`
2. Implement the `Task` interface:
   ```go
   type Task interface {
       GetName() string
       Run(ctx context.Context) ([]*reportv1.Finding, error)
   }
   ```
3. Add the task to `GetBaselineTasks()` in `pkg/tasks/baseline.go`
4. Define expected types in `pkg/tasks/tasks.go`

### Creating Command-Based Probes

For tasks that execute system commands, use the generic command-based probe pattern in `pkg/tasks/cmd-based/`:

1. **Define your probe struct** with the data it will collect:
   ```go
   type myCustomProbe struct {
       result []string
   }
   ```

2. **Implement the `CmdTask[T]` interface**:
   ```go
   // getCommand returns the command and arguments to execute
   func (p *myCustomProbe) getCommand() ([]string, error) {
       return []string{"mycommand", "--arg1", "--arg2"}, nil
   }

   // parseCommandOuput parses the command output into your struct
   func (p *myCustomProbe) parseCommandOuput(out []byte) (*myCustomProbe, error) {
       // Parse the output and populate your struct
       lines := strings.Split(string(out), "\n")
       // ... parsing logic ...
       return &myCustomProbe{result: parsed}, nil
   }
   ```

3. **Execute the probe** using the generic runner:
   ```go
   probe := &myCustomProbe{}
   result, err := runCmdTask(probe)
   ```

4. **Write tests** using the mock pattern:
   ```go
   func TestMyProbe(t *testing.T) {
       mockExec := func(_ string, _ ...string) ([]byte, error) {
           return []byte("mock output"), nil
       }
       testProbe(t, "myCustomProbe", &myCustomProbe{}, mockExec, expectedResult)
   }
   ```

See `pkg/tasks/cmd-based/processes.go` for complete examples

### Known Limitations

#### macOS UDP Port Scanning

UDP port scanning is **disabled on macOS** (Darwin) due to reliability issues:

- **Issue**: The current UDP scanning method relies on timeout behavior to determine port status. On macOS, all ports timeout regardless of their actual state, leading to false positives.
- **Workaround**: The `ScanUDP()` function in `pkg/tasks/baseline/network.go` returns an empty slice on macOS systems.
- **Future Enhancement**: OS-specific UDP scanning methods (e.g., using `netstat`, `lsof`, or native syscalls) are planned for more accurate detection across all platforms.

```go
// From pkg/tasks/baseline/network.go
func ScanUDP(host string) []int {
    // TODO: fix usage in darwin
    // it reports all the ports because they all timeout
    if runtime.GOOS == "darwin" {
        return []int{}
    }
    // ... scanning logic ...
}
```

## Testing in AI Code Assistants

For reference check the scripts in the `tests` folder

### Claude Code

```bash
./scripts/run-claude.sh "Execute !bin/sandbox-probe"
```

### Gemini Code Assist (Podman)

```bash
./scripts/run-gemini-podman.sh "bin/sandbox-probe"
```
