package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateHelp(t *testing.T) {
	options := ValidateArgs([]string{"--help"})
	if !options.Help {
		t.Fatal("expected help")
	}
}

func TestValidateSourceAndOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.go")
	if err := os.WriteFile(path, []byte("package sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	options := ValidateArgs([]string{path, "--lines", "3,5", "--timeout-factor", "2", "--test-command", "go test ./pkg"})
	if options.Error != "" {
		t.Fatal(options.Error)
	}
	if !options.Lines[3] || !options.Lines[5] {
		t.Fatalf("unexpected lines: %#v", options.Lines)
	}
	if options.TimeoutFactor != 2 || options.TestCommand != "go test ./pkg" {
		t.Fatalf("unexpected options: %#v", options)
	}
}

func TestValidateRejectsMissingSource(t *testing.T) {
	options := ValidateArgs([]string{"--scan"})
	if options.Error == "" {
		t.Fatal("expected error")
	}
}

func TestValidateRejectsConflictingModes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.go")
	if err := os.WriteFile(path, []byte("package sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	options := ValidateArgs([]string{path, "--scan", "--mutate-all"})
	if options.Error == "" {
		t.Fatal("expected error")
	}
}
