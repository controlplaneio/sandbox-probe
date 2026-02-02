# Contributing

- [Adding New Tasks](#AddingNewTasks)
  - [Creating Command-Based Probes](#CreatingCommand-BasedProbes)
- [Testing](#Testing)
  - [Trialing against Agent Sandboxes](#TrialingagainstAgentSandboxes)
  - [Known Limitations](#KnownLimitations)
    - [macOS UDP Port Scanning](#macOSUDPPortScanning)
- 3. [Testing in AI Code Assistants](#TestinginAICodeAssistants)
  - [Claude Code](#ClaudeCode)
  - [Gemini Code Assist (Podman)](#GeminiCodeAssistPodman)

## Adding New Tasks

1. Create a new task struct in `pkg/tasks/baseline/`
2. Implement the `Task` interface:
   ```go
   type Task interface {
       GetName() string
       Run(ctx context.Context, ti Inputs) ([]*reportv1.Finding, error)
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

## Testing

### Trialing against Agent Sandboxes

The easiest way to run the probe against agent sandboxes will be to use the
scripts in `./tests`

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

### Known Working Tooling Versions

Since several of these tools recieve frequent updates and their CLI interfaces
(or even system prompts) aren't necessarily stable these are the versions we've
tested against:

| Program     | Version  |
| :---------- | :------- |
| Claude Code | `2.1.39` |
| Nono        | `0.4.1`  |
| Gemini      | `0.28.2` |

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
