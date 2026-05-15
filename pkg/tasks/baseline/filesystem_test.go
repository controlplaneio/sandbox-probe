package tasks

import (
	"os"
	"path/filepath"
	"testing"
)

// Typically writable paths (checking these helps establish baseline)
var ExpectedWritablePaths = []string{
	"/tmp",
	"/var/tmp",
	"/dev/shm",
}

// Home directory patterns to check outside project directory
var HomeDirPatterns = []string{
	"/home",
	"/root",
	"/Users", // macOS
}

func TestScanTargetedPaths(t *testing.T) {
	result := ScanTargetedPaths()

	if result == nil {
		t.Fatal("ScanTargetedPaths returned nil")
	}

	if result.ReadablePaths == nil {
		t.Error("ReadablePaths should not be nil")
	}

	if result.WritablePaths == nil {
		t.Error("WritablePaths should not be nil")
	}

	// Verify that at least some paths were checked (result should not be completely empty)
	// In most environments, /tmp should be writable if checked, or some paths should be readable
	t.Logf("Found %d readable sensitive paths", len(result.ReadablePaths))
	t.Logf("Found %d writable system paths", len(result.WritablePaths))

	// Log some findings for debugging
	if len(result.ReadablePaths) > 0 {
		t.Logf("Sample readable paths: %v", result.ReadablePaths[:min(3, len(result.ReadablePaths))])
	}
	if len(result.WritablePaths) > 0 {
		t.Logf("Sample writable paths: %v", result.WritablePaths[:min(3, len(result.WritablePaths))])
	}
}

// func TestScanFilesystemPermissions(t *testing.T) {
// 	// Create a temporary directory structure for testing
// 	tmpDir := t.TempDir()

// 	// Create test directories and files
// 	readableDir := filepath.Join(tmpDir, "readable")
// 	writableFile := filepath.Join(tmpDir, "writable.txt")

// 	if err := os.Mkdir(readableDir, 0755); err != nil {
// 		t.Fatalf("Failed to create test directory: %v", err)
// 	}

// 	if err := os.WriteFile(writableFile, []byte("test"), 0644); err != nil {
// 		t.Fatalf("Failed to create test file: %v", err)
// 	}

// 	tests := []struct {
// 		name     string
// 		rootPath string
// 		maxDepth int
// 		wantErr  bool
// 	}{
// 		{
// 			name:     "scan temp directory with depth 0",
// 			rootPath: tmpDir,
// 			maxDepth: 0,
// 			wantErr:  false,
// 		},
// 		{
// 			name:     "scan temp directory with depth 1",
// 			rootPath: tmpDir,
// 			maxDepth: 1,
// 			wantErr:  false,
// 		},
// 		{
// 			name:     "scan temp directory with unlimited depth",
// 			rootPath: tmpDir,
// 			maxDepth: -1,
// 			wantErr:  false,
// 		},
// 		{
// 			name:     "scan /tmp with depth 1",
// 			rootPath: "/tmp",
// 			maxDepth: 1,
// 			wantErr:  false,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result, err := ScanFilesystemPermissions(tt.rootPath, tt.maxDepth)

// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("ScanFilesystemPermissions() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}

// 			if result == nil {
// 				t.Fatal("Result should not be nil")
// 			}

// 			if result.ReadablePaths == nil {
// 				t.Error("ReadablePaths should not be nil")
// 			}

// 			if result.WritablePaths == nil {
// 				t.Error("WritablePaths should not be nil")
// 			}

// 			t.Logf("Found %d readable paths and %d writable paths",
// 				len(result.ReadablePaths), len(result.WritablePaths))
// 		})
// 	}
// }

func TestIsReadable(t *testing.T) {
	tmpDir := t.TempDir()
	readableFile := filepath.Join(tmpDir, "readable.txt")

	if err := os.WriteFile(readableFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "readable file",
			path: readableFile,
			want: true,
		},
		{
			name: "non-existent file",
			path: "/path/that/does/not/exist/file.txt",
			want: false,
		},
		{
			name: "temp directory",
			path: tmpDir,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isReadable(tt.path)
			if got != tt.want {
				t.Errorf("isReadable(%s) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsWritable(t *testing.T) {
	tmpDir := t.TempDir()
	writableFile := filepath.Join(tmpDir, "writable.txt")

	if err := os.WriteFile(writableFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "writable temp directory",
			path: tmpDir,
			want: true,
		},
		{
			name: "writable file",
			path: writableFile,
			want: true,
		},
		{
			name: "non-existent file",
			path: "/path/that/does/not/exist/file.txt",
			want: false,
		},
		{
			name: "typically read-only path",
			path: "/usr/bin",
			want: false, // Usually not writable by regular users
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isWritable(tt.path)
			if got != tt.want {
				t.Errorf("isWritable(%s) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsPseudoFilesystem(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "/proc path",
			path: "/proc/self/status",
			want: true,
		},
		{
			name: "/sys path",
			path: "/sys/class/net",
			want: true,
		},
		{
			name: "/dev path",
			path: "/dev/null",
			want: true,
		},
		{
			name: "regular path",
			path: "/etc/passwd",
			want: false,
		},
		{
			name: "home directory",
			path: "/home/user/file.txt",
			want: false,
		},
		{
			name: "tmp directory",
			path: "/tmp/test",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPseudoFilesystem(tt.path)
			if got != tt.want {
				t.Errorf("isPseudoFilesystem(%s) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestPathConstants(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		minCount int
	}{
		{
			name:     "SensitiveReadPaths",
			paths:    func() []string { ps := buildSensitivePaths(); out := make([]string, len(ps)); for i, p := range ps { out[i] = p.path }; return out }(),
			minCount: 10,
		},
		{
			name:     "SystemWritePaths",
			paths:    SystemWritePaths,
			minCount: 5,
		},
		{
			name:     "ExpectedWritablePaths",
			paths:    ExpectedWritablePaths,
			minCount: 1,
		},
		{
			name:     "HomeDirPatterns",
			paths:    HomeDirPatterns,
			minCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.paths) < tt.minCount {
				t.Errorf("%s has %d paths, want at least %d", tt.name, len(tt.paths), tt.minCount)
			}

			// Check for empty strings
			for i, path := range tt.paths {
				if path == "" {
					t.Errorf("%s[%d] is empty", tt.name, i)
				}
			}

			t.Logf("%s contains %d paths", tt.name, len(tt.paths))
		})
	}
}

// func TestScanFilesystemPermissionsSkipsPseudoFS(t *testing.T) {
// 	// Test that scanning /proc with depth 1 doesn't descend into pseudo-filesystems
// 	result, err := ScanFilesystemPermissions("/", 1)
// 	if err != nil {
// 		t.Logf("Error scanning /: %v (expected, may not have permissions)", err)
// 	}

// 	if result == nil {
// 		t.Fatal("Result should not be nil")
// 	}

// 	// Verify that we didn't collect too many paths from pseudo-filesystems
// 	// This is a soft check - we just log the results
// 	t.Logf("Scanned / with depth 1: %d readable, %d writable",
// 		len(result.ReadablePaths), len(result.WritablePaths))
// }

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
