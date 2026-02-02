# Sandbox Probe

- [Overview](#Overview)
- [Supported Code Assistants](#SupportedCodeAssistants)
- [Installation](#Installation)
  - [Prerequisites](#Prerequisites)
    - [Building](#Building)
    - [E2E Testing](#E2ETesting)
  - [Build from Source](#BuildfromSource)
- [Usage](#Usage)
  - [CLI Commands](#CLICommands)
    - [scan](#scan)
    - [tasks list](#taskslist)
    - [version](#version)
  - [Basic Execution](#BasicExecution)
  - [Test an Agent](#TestanAgent)
  - [Try with a sandbox](#Trywithasandbox)
  - [Output](#Output)
  - [Example Report](#ExampleReport)
- [Development](#Development)
  - [Available Tasks](#AvailableTasks)

A security enumeration tool designed to detect and analyze sandbox environments in AI code assistants, identifying execution capabilities, system access, and potential security boundaries.

## Overview

Sandbox Probe is specifically designed to fingerprint the execution environment of AI code assistants (such as Claude Code, Gemini CLI, and similar tools) and identify:

- **Sandbox/Container Detection**: Docker, Podman, LXC, Firejail, Bubblewrap, gVisor, WSL, OpenVZ, Seatbelt, Landlock
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

#### Building

- Go 1.25 or later
- Protocol Buffer compiler (`buf`) - install via `make install-buf`
  - required if changing the protobuf definitions

#### E2E Testing

- `jq` - JSON processor for parsing outputs
  - provides pretty printing for JSON reports

Depending on which sandboxing you want to test a combination of these may be
required:

- `docker` - For containerized testing
- `podman` - For containerized testing
- `claude-code` - Claude Code CLI for Claude testing
- `gemini-cli` - Gemini Code Assist CLI for Gemini testing
- `nono` - A sandboxing tool for wrapping AI agents and other programs

### Build from Source

```bash
# Clone the repository
git clone https://github.com/controlplaneio/sandbox-probe.git
cd sandbox-probe

# Build the binary
make build
```

If running `sandbox-probe` inside a container make sure that it was built statically,
with standard library paths, or find a method to mount additional paths.

This isn't typically an issue but can be for non-glibc or non-fhs systems like
alpine or nixos (and via nix).

## Usage

### CLI Commands

#### scan

Run security enumeration probes on the current environment.

```bash
./bin/sandbox-probe scan [flags]
```

**Flags:**

- `--tasks` - Additional individual tasks to run (comma-separated)
- `--tasksets` - Group of tasks to select: `baseline`, `ps`, `all` (default: `baseline`)
- `--output_path` - Path to write the JSON report (default: `report.json`)
- `--tags` - Metadata tags to append to the report (comma-separated)
- `--log_level` - Set log level (default: `info`)

**Examples:**

Run all baseline probes:

```bash
./bin/sandbox-probe scan
```

Run specific tasksets:

```bash
./bin/sandbox-probe scan --tasksets baseline,ps
```

Run with custom output path and tags:

```bash
./bin/sandbox-probe scan --output_path results.json --tags "test,docker"
```

Run specific tasks:

```bash
./bin/sandbox-probe scan --tasks baseline_network_task,baseline_process_task
```

#### tasks list

List all available tasks and tasksets with their descriptions.

```bash
./bin/sandbox-probe tasks list
```

This command displays a formatted table of all available tasks, including:

- Task names (color-coded in blue)
- Task descriptions

**Example output:**

```
baseline_path_task          : Scans filesystem for writable and sensitive readable paths
baseline_network_task       : Scans network for DNS resolution, connectivity, and open TCP/UDP ports
baseline_proxy_task         : Detects proxy configuration from environment variables
...
```

#### version

Display version information for the sandbox-probe binary.

```bash
./bin/sandbox-probe version
```

**Example output:**

```
version v1.0.0
git commit abc1234
build date 2026-02-13T10:30:00Z
```

### Basic Execution

Run all baseline probes (outside of the context of an AI assistant). It is useful just for testing the go code. If running on a desktop device you actually use the report can be very large.
For dedicated servers or containerized environments (such as the environments used by some AI tooling)
there will be less access and as such less output.

```bash
./bin/sandbox-probe scan
```

### Test an Agent

### Try with a sandbox

> [!IMPORTANT]
> Since the goal of these test scripts is to run inside the sandbox many of
> these are executed by the agent.
> Please consider the risk that these agents could execute other actions,
> especially on non-interactive/YOLO modes.
>
> This does not apply to pure sandboxes which you run other AI agents within
> such as nono.
>
> You might reduce this risk by using the interactive version of these scripts
> but the Agent may still take autonomous action you don't expect/trust.

If you have the pre-requisite dependencies consider running a script in `./tests` such as `./tests/sandbox_nono.sh`.
AI Agent tooling such as Gemini and Claude will need login details.

```
tests/
├── baseline_nono.sh
├── baseline_claude.sh
├── baseline_gemini_interactive.sh
├── sandbox_nono.sh
├── sandbox_claude.sh
└── sandbox_gemini_interactive.sh
└── ...
```

These scripts will output to the `./reports` subdirectory.

For more details please see [here](./docs/CONTRIBUTING.md#trialing-against-agent-sandboxes).

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
    {
      "//" "..."
    },
  ]
}
```

## Development

### Available Tasks

The baseline probe includes the following tasks:

| Task Name                  | Description                                                                    |
| -------------------------- | ------------------------------------------------------------------------------ |
| baseline_path_task         | Scans filesystem for writable and sensitive readable paths                     |
| baseline_network_task      | Scans network for DNS resolution, connectivity, and open TCP/UDP ports         |
| baseline_proxy_task        | Detects proxy configuration from environment variables                         |
| baseline_socket_task       | Scans filesystem for Unix domain sockets                                       |
| baseline_process_task      | Detects running processes and parent process information                       |
| baseline_user_context_task | Detects user and group context information (UID, GID, EUID, EGID)              |
| ps_all_task                | Lists all running processes using ps command                                   |
| baseline_hostname_task     | Detects the system hostname                                                    |
| baseline_sandbox_task      | Detects container runtime and sandbox environments (Docker, Podman, LXC, etc.) |
| baseline_mount_task        | Detects host-mounted volumes and filesystem mounts                             |
| ps_parent_task             | Gets parent process information using ps command                               |
| ps_single_task             | Gets information about the running process using ps command                    |
