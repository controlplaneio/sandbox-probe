# TODO List

### Other probes

- [ ] Network namespace detection (/proc/self/ns/net comparison)
- [ ] PID namespace detection
- [ ] Tmpfs detection
- [ ] Symlink resolution across mount boundaries
- [ ] `/proc/self/root` traversal
- [ ] Current capabilities (`/proc/self/status` `CapEff/CapPrm`)
- [ ]Seccomp status (`/proc/self/status` Seccomp field)
- [ ] Available syscalls (probe specific syscalls and record success/failure) 
- [ ] Kernel version and security modules (`AppArmor`, `SELinux` status)
- [ ] `cgroup` membership (`/proc/self/cgroup`)

### CLI Library
- [ ] Add a proper CLI framework (e.g., cobra, cli)
- [ ] Add command-line flags for configuration
  - [ ] Output format selection (JSON, YAML, text)
  - [ ] Log level control
  - [ ] Select specific tasks to run
  - [ ] Custom output file path
- [ ] Add version and help commands

### macOS UDP port detection

- [ ] in macOS all UDP ports are reported, we had to disable the scanUDP function

### macOS Seatbelt Detection
- [ ] Research macOS Seatbelt sandbox detection methods
- [ ] Implement Seatbelt detection in `pkg/tasks/baseline/environment.go`
- [ ] Add Seatbelt to the `ContainerRuntime` enum
- [ ] Add test case for Seatbelt detection

### E2E Testing Expansion
- [ ] Add more controlled environment tests
  - [ ] Firejail detection test
  - [ ] LXC/LXD detection test
  - [ ] gVisor detection test
  - [ ] OpenVZ detection test
  - [ ] WSL detection test
- [ ] Add CI/CD integration for automated testing
- [ ] Document test setup for each environment

### Tool Integration
- [ ] Integrate with Falco
- [ ] Add support for other security tools
  - [ ] AppArmor profile generation
  - [ ] SELinux policy analysis
  - [ ] Seccomp profile recommendations

### Documentation
- [ ] Add architecture diagram
- [ ] Create developer guide
- [ ] Create troubleshooting guide

### Completed
- [x] Basic sandbox detection (Docker, Podman, Bubblewrap)
- [x] File system permission enumeration
- [x] Network capability testing
- [x] Process detection
- [x] Structured logging with zerolog
- [x] Protocol Buffer report format
- [x] Docker e2e test
- [x] Bubblewrap e2e test
- [x] Project README
- [x] Debug why Gemini refuses to run the binary
- [x] Update `scripts/run-gemini-podman.sh` to work correctly
- [x] Document Gemini-specific requirements