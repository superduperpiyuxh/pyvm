package utils

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"3.12.7", "3.12.7", 0},
		{"3.12.7", "3.12.8", -1},
		{"3.12.8", "3.12.7", 1},
		{"3.11", "3.12", -1},
		{"3.13", "3.12", 1},
		{"3.12.0", "3.12", 0},
	}

	for _, tt := range tests {
		t.Run(tt.v1+"_"+tt.v2, func(t *testing.T) {
			got := compareVersions(tt.v1, tt.v2)
			if got != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d; want %d", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}

func TestExtractPythonVersion(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"cpython-3.12.7+20241016-x86_64-unknown-linux-gnu-install_only.tar.gz", "3.12.7"},
		{"cpython-3.11.0+20240101-aarch64-apple-darwin-install_only.tar.gz", "3.11.0"},
		{"invalid-filename", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := extractPythonVersion(tt.filename)
			if got != tt.expected {
				t.Errorf("extractPythonVersion(%q) = %q; want %q", tt.filename, got, tt.expected)
			}
		})
	}
}
