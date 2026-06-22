//go:build windows
// +build windows

package tasks

import (
	"os"
	"path/filepath"
	"syscall"
)

// On Windows we don't have POSIX access(2). The cheapest reliable signal
// is to open the file with the requested access and report success/failure;
// the OS evaluates the ACL during the open call.
func isReadable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if info.IsDir() {
		// For directories, try to read the directory contents
		_, err := os.ReadDir(path)
		return err == nil
	}

	// For files, try to open
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func isWritable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if info.IsDir() {
		// For directories, try to open with write access using syscall
		// Need FILE_FLAG_BACKUP_SEMANTICS to open directories on Windows
		pathPtr, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			return false
		}
		handle, err := syscall.CreateFile(
			pathPtr,
			syscall.GENERIC_WRITE,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
			nil,
			syscall.OPEN_EXISTING,
			syscall.FILE_FLAG_BACKUP_SEMANTICS,
			0,
		)
		if err != nil {
			return false
		}
		syscall.CloseHandle(handle)
		return true
	}

	// For files, try to open with write access
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// platformDefaultHome returns the fallback home directory when os.UserHomeDir fails.
func platformDefaultHome() string {
	// On Windows, C:\Users\Default is the closest equivalent to /root.
	return filepath.Join(os.Getenv("SystemDrive"), "Users", "Default")
}

// platformSensitivePaths returns Windows-specific absolute paths to check.
// These are credential stores and sensitive locations that exist on Windows
// but have no Unix equivalent (and vice versa for the Unix list).
func platformSensitivePaths(home string) []SensitivePath {
	appData := os.Getenv("APPDATA")         // C:\Users\<name>\AppData\Roaming
	localAppData := os.Getenv("LOCALAPPDATA") // C:\Users\<name>\AppData\Local

	var paths []SensitivePath

	if appData != "" {
		paths = append(paths,
			// ── Windows Credential Manager / generic stores ───────────────
			sp(filepath.Join(appData, "Microsoft", "Credentials")),
			sp(filepath.Join(appData, "Microsoft", "Protect")),

			// ── Cloud SDKs on Windows ─────────────────────────────────────
			sp(filepath.Join(appData, "gcloud")),
			sp(filepath.Join(appData, "doctl", "config.yaml")),
		)
	}

	if localAppData != "" {
		paths = append(paths,
			// ── Local credential caches ───────────────────────────────────
			sp(filepath.Join(localAppData, "Google", "Cloud SDK", "application_default_credentials.json")),

			// ── 1Password / password manager local data ───────────────────
			sp(filepath.Join(localAppData, "1Password")),
		)
	}

	if home != "" {
		paths = append(paths,
			// ── OpenSSH for Windows stores keys in the same relative location ──
			// (already covered by the cross-platform home-relative list in
			// buildSensitivePathsForHome, included here for completeness)

			// ── Windows Git Credential Manager ────────────────────────────
			sp(filepath.Join(home, ".config", "git", "credentials")),
		)
	}

	return paths
}

// platformSystemWritePaths lists Windows system directories that should be
// read-only. The equivalent of /etc, /usr/bin etc. on Windows.
var platformSystemWritePaths = []string{
	// Windows system directories — these should never be writable by an AI agent.
	`C:\Windows\System32`,
	`C:\Windows\SysWOW64`,
	`C:\Windows`,
	`C:\Program Files`,
	`C:\Program Files (x86)`,
}
