# TODO List

### Other probes


- [ ] Tmpfs detection
- [ ] Symlink resolution across mount boundaries
- [ ] `/proc/self/root` traversal
- [ ] Current capabilities (`/proc/self/status` `CapEff/CapPrm`)
- [ ] Seccomp status (`/proc/self/status` Seccomp field)
- [ ] Available syscalls (probe specific syscalls and record success/failure) 
- [ ] Kernel version and security modules (`AppArmor`, `SELinux` status)
- [ ] `cgroup` membership (`/proc/self/cgroup`)

### macOS UDP port detection

- [ ] in macOS all UDP ports are reported, we had to disable the scanUDP function

### macOS Seatbelt Detection
- [ ] Research macOS Seatbelt sandbox detection methods

### macOS PS test
- [ ] Test ps commands in macOS

### E2E Testing Expansion
- [ ] Add more controlled environment tests
  - [ ] Firejail detection test
  - [ ] LXC/LXD detection test
  - [ ] gVisor detection test
  - [ ] OpenVZ detection test
- [ ] Add CI/CD integration for automated testing
- [ ] Document test setup for each environment

### Tool Integration
- [ ] Integrate with [`SwiftBelt`](https://github.com/cedowens/SwiftBelt/tree/master)

### Documentation
- [ ] Add architecture diagram

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
- [x] Network namespace detection (/proc/self/ns/net comparison)
- [x] PID namespace detection
- [x] Add a proper CLI framework (e.g., cobra, cli)