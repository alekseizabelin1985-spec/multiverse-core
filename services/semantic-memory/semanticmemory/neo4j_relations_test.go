package semanticmemory

import (
	"testing"
)

func TestSanitizeRelType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"FOUND", "FOUND"},
		{"LOCATED_IN", "LOCATED_IN"},
		{"ALLIED_WITH", "ALLIED_WITH"},
		{"invalid-type!", "invalidtype"},
		{"", "RELATED"},
		{"!@#$%", "RELATED"},
		{"Rel123", "Rel123"},
		{"UPPER_CASE_123", "UPPER_CASE_123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeRelType(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeRelType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
