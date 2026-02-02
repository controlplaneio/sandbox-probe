package tasks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommaSeparatedString(t *testing.T) {
	tests := []struct {
		name     string
		val1     string
		val2     string
		expected string
	}{
		{
			name:     "both empty",
			val1:     "",
			val2:     "",
			expected: "",
		},
		{
			name:     "val1 empty, val2 not empty",
			val1:     "",
			val2:     "value2",
			expected: "value2",
		},
		{
			name:     "val1 not empty, val2 empty",
			val1:     "value1",
			val2:     "",
			expected: "value1",
		},
		{
			name:     "both not empty",
			val1:     "value1",
			val2:     "value2",
			expected: "value1,value2",
		},
		{
			name:     "both with special characters",
			val1:     "http://proxy1:8080",
			val2:     "http://proxy2:8080",
			expected: "http://proxy1:8080,http://proxy2:8080",
		},
		{
			name:     "val1 with spaces, val2 empty",
			val1:     "value with spaces",
			val2:     "",
			expected: "value with spaces",
		},
		{
			name:     "both with spaces",
			val1:     "value 1",
			val2:     "value 2",
			expected: "value 1,value 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := commaSeparatedString(tt.val1, tt.val2)
			assert.Equal(t, tt.expected, result,
				"commaSeparatedString(%q, %q) = %q, expected %q",
				tt.val1, tt.val2, result, tt.expected)
		})
	}
}
