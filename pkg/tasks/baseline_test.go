package tasks

import (
	"context"
	"reflect"
	"testing"

	"github.com/controlplaneio/sandbox-probe/pkg/models"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestStringSliceToInterface(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []interface{}
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []interface{}{},
		},
		{
			name:     "single element",
			input:    []string{"hello"},
			expected: []interface{}{"hello"},
		},
		{
			name:     "multiple elements",
			input:    []string{"foo", "bar", "baz"},
			expected: []interface{}{"foo", "bar", "baz"},
		},
		{
			name:     "elements with spaces",
			input:    []string{"hello world", "test string"},
			expected: []interface{}{"hello world", "test string"},
		},
		{
			name:     "empty strings",
			input:    []string{"", "test", ""},
			expected: []interface{}{"", "test", ""},
		},
		{
			name:     "special characters",
			input:    []string{"/path/to/file", "user@host", "key=value"},
			expected: []interface{}{"/path/to/file", "user@host", "key=value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringSliceToInterface(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("stringSliceToInterface() length = %d, want %d", len(result), len(tt.expected))
				return
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("stringSliceToInterface() = %v, want %v", result, tt.expected)
			}

			// Verify each element has the correct type
			for i, v := range result {
				if _, ok := v.(string); !ok {
					t.Errorf("element %d is not a string: got type %T", i, v)
				}
			}
		})
	}
}

func TestIntSliceToInterface(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []interface{}
	}{
		{
			name:     "empty slice",
			input:    []int{},
			expected: []interface{}{},
		},
		{
			name:     "single element",
			input:    []int{42},
			expected: []interface{}{42},
		},
		{
			name:     "multiple elements",
			input:    []int{1, 2, 3, 4, 5},
			expected: []interface{}{1, 2, 3, 4, 5},
		},
		{
			name:     "zero values",
			input:    []int{0, 0, 0},
			expected: []interface{}{0, 0, 0},
		},
		{
			name:     "negative numbers",
			input:    []int{-1, -10, -100},
			expected: []interface{}{-1, -10, -100},
		},
		{
			name:     "mixed positive and negative",
			input:    []int{-5, 0, 5, 10, -10},
			expected: []interface{}{-5, 0, 5, 10, -10},
		},
		{
			name:     "large numbers",
			input:    []int{65535, 8080, 3000},
			expected: []interface{}{65535, 8080, 3000},
		},
		{
			name:     "port numbers",
			input:    []int{22, 80, 443, 3306, 5432},
			expected: []interface{}{22, 80, 443, 3306, 5432},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := intSliceToInterface(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("intSliceToInterface() length = %d, want %d", len(result), len(tt.expected))
				return
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("intSliceToInterface() = %v, want %v", result, tt.expected)
			}

			// Verify each element has the correct type
			for i, v := range result {
				if _, ok := v.(int); !ok {
					t.Errorf("element %d is not an int: got type %T", i, v)
				}
			}
		})
	}
}

// Benchmark tests to measure performance
func BenchmarkStringSliceToInterface(b *testing.B) {
	input := []string{"path1", "path2", "path3", "path4", "path5"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = stringSliceToInterface(input)
	}
}

func BenchmarkIntSliceToInterface(b *testing.B) {
	input := []int{80, 443, 22, 3306, 5432}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = intSliceToInterface(input)
	}
}

// TestProxyConfigToProtobuf tests that ProxyConfig can be converted to protobuf values
func TestProxyConfigToProtobuf(t *testing.T) {
	tests := []struct {
		name      string
		proxyMap  map[string]interface{}
		wantError bool
	}{
		{
			name: "all fields populated",
			proxyMap: map[string]interface{}{
				"http_proxy":  "http://proxy.example.com:8080",
				"https_proxy": "https://proxy.example.com:8443",
				"all_proxy":   "socks5://proxy.example.com:1080",
				"no_proxy":    "localhost,127.0.0.1,.local",
				"socks_proxy": "socks5://proxy.example.com:1080",
				"pac_url":     "http://proxy.example.com/proxy.pac",
			},
			wantError: false,
		},
		{
			name: "empty fields",
			proxyMap: map[string]interface{}{
				"http_proxy":  "",
				"https_proxy": "",
				"all_proxy":   "",
				"no_proxy":    "",
				"socks_proxy": "",
				"pac_url":     "",
			},
			wantError: false,
		},
		{
			name: "partial fields",
			proxyMap: map[string]interface{}{
				"http_proxy":  "http://proxy.example.com:8080",
				"https_proxy": "https://proxy.example.com:8443",
				"all_proxy":   "",
				"no_proxy":    "localhost",
				"socks_proxy": "",
				"pac_url":     "",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the map can be converted to structpb.Value
			value, err := structpb.NewValue(tt.proxyMap)

			if (err != nil) != tt.wantError {
				t.Errorf("structpb.NewValue() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if err == nil {
				// Verify the value is a struct
				structValue := value.GetStructValue()
				if structValue == nil {
					t.Error("Expected struct value, got nil")
					return
				}

				// Verify all expected fields are present
				for key, expectedVal := range tt.proxyMap {
					field := structValue.Fields[key]
					if field == nil {
						t.Errorf("Field %s is missing from struct", key)
						continue
					}

					actualVal := field.GetStringValue()
					if actualVal != expectedVal {
						t.Errorf("Field %s = %v, want %v", key, actualVal, expectedVal)
					}
				}
			}
		})
	}
}

// Test_processToInterface tests the processToInterface function
func Test_processToInterface(t *testing.T) {
	tests := []struct {
		name      string
		process   *models.Process
		wantError bool
	}{
		{
			name: "full process with namespaces",
			process: &models.Process{
				Command: "/usr/bin/test --flag value",
				PID:     1234,
				PPID:    1,
				Namespaces: []*models.Namespace{
					{Type: "mnt", Inode: 4026531840},
					{Type: "net", Inode: 4026531956},
				},
			},
			wantError: false,
		},
		{
			name: "process with empty command",
			process: &models.Process{
				Command:    "",
				PID:        5678,
				PPID:       1,
				Namespaces: []*models.Namespace{},
			},
			wantError: false,
		},
		{
			name: "process with nil namespaces",
			process: &models.Process{
				Command:    "/bin/sh",
				PID:        9999,
				PPID:       1000,
				Namespaces: nil,
			},
			wantError: false,
		},
		{
			name: "process with single command element",
			process: &models.Process{
				Command: "init",
				PID:     1,
				PPID:    0,
				Namespaces: []*models.Namespace{
					{Type: "pid", Inode: 4026531836},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call processToInterface
			value, err := processToInterface(tt.process)

			// Check error expectation
			if (err != nil) != tt.wantError {
				t.Errorf("processToInterface() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if err == nil {
				// Verify the value is a struct
				structValue := value.GetStructValue()
				if structValue == nil {
					t.Fatal("Expected struct value, got nil")
				}

				// Verify expected fields exist
				expectedFields := []string{"command", "pid", "ppid", "namespaces"}
				for _, field := range expectedFields {
					if _, ok := structValue.Fields[field]; !ok {
						t.Errorf("Missing expected field: %s", field)
					}
				}

				// Verify PID value
				pidField := structValue.Fields["pid"]
				if pidField != nil {
					pidValue := int(pidField.GetNumberValue())
					if pidValue != tt.process.PID {
						t.Errorf("PID = %d, want %d", pidValue, tt.process.PID)
					}
				}

				// Verify PPID value
				ppidField := structValue.Fields["ppid"]
				if ppidField != nil {
					ppidValue := int(ppidField.GetNumberValue())
					if ppidValue != tt.process.PPID {
						t.Errorf("PPID = %d, want %d", ppidValue, tt.process.PPID)
					}
				}
			}
		})
	}
}

// TestProxyTaskRun tests the ProxyTask.Run method
func TestProxyTaskRun(t *testing.T) {
	task := NewProxyTask()

	// Run the task
	findings, err := task.Run(context.Background())

	// The task should not error (even if no proxy is configured)
	if err != nil {
		t.Fatalf("ProxyTask.Run() unexpected error: %v", err)
	}

	// Should return exactly one finding
	if len(findings) != 1 {
		t.Fatalf("ProxyTask.Run() returned %d findings, want 1", len(findings))
	}

	finding := findings[0]

	// Verify finding type
	if finding.FindingType != PROXYDETECTION {
		t.Errorf("Finding type = %s, want %s", finding.FindingType, PROXYDETECTION)
	}

	// Verify task name
	if finding.Task != task.GetName() {
		t.Errorf("Finding task = %s, want %s", finding.Task, task.GetName())
	}

	// Verify the value is a struct with expected fields
	structValue := finding.Value.GetStructValue()
	if structValue == nil {
		t.Fatal("Finding value is not a struct")
	}

	// Verify expected fields exist in the struct
	expectedFields := []string{"http_proxy", "https_proxy", "all_proxy", "no_proxy", "socks_proxy", "pac_url"}
	for _, field := range expectedFields {
		if _, ok := structValue.Fields[field]; !ok {
			t.Errorf("Missing expected field: %s", field)
		}
	}
}
