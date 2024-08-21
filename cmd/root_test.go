package cmd

import (
	"testing"

	"github.com/pterm/pterm"
	"github.com/stretchr/testify/assert"
)

func TestConvertBytes(t *testing.T) {
	// Define test cases
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"Zero bytes", 0, pterm.Red("0 B")},
		{"Less than 1 KB", 512, "512 B"},
		{"Exactly 1 KB", 1024, "1 KB"},
		{"1.5 KB", 1536, "1 KB"}, // Will floor to 1 KB
		{"Less than 1 MB", 1024 * 512, "512 KB"},
		{"Exactly 1 MB", 1024 * 1024, "1 MB"},
		{"1.5 MB", 1024 * 1024 * 1.5, "1 MB"}, // Will floor to 1 MB
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := convertBytes(tt.input)
			assert.Equal(t, tt.expected, output)
		})
	}
}
