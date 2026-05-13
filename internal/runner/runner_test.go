package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unclebob/mutate4go/internal/mutations"
)

func TestRunMutationsParallelUsesIsolatedWorkerCopies(t *testing.T) {
	root := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previousDir)

	original := "package sample\n\nvar Flag = true\nvar Other = true\n"
	sourcePath := "sample.go"
	if err := os.WriteFile(sourcePath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	sites := []mutations.Site{
		booleanSite(0, original, "Flag = true"),
		booleanSite(1, original, "Other = true"),
	}
	results, err := runMutations(sourcePath, original, sites, time.Second, "! grep -q false sample.go", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, result := range results {
		if result.Status != "killed" {
			t.Fatalf("expected killed result, got %#v", result)
		}
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != original {
		t.Fatalf("parallel run changed source file:\n%s", content)
	}
	entries, err := os.ReadDir(filepath.Join(root, "target", "mutation-workers"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected worker run directory cleanup, found %d entries", len(entries))
	}
}

func booleanSite(index int, content, marker string) mutations.Site {
	start := strings.Index(content, marker) + strings.Index(marker, "true")
	return mutations.Site{
		Index:       index,
		Line:        index + 3,
		StartOffset: start,
		EndOffset:   start + len("true"),
		Original:    "true",
		Mutant:      "false",
		Description: "true -> false",
		FunctionID:  "func/sample",
	}
}
