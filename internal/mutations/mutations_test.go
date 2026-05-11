package mutations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverMutationSites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.go")
	source := `package sample

func Score(x int) bool {
	return x+1 > 0 && true
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	sites, functions, err := Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(functions) != 1 || functions[0].ID != "func/Score" {
		t.Fatalf("unexpected functions: %#v", functions)
	}
	descriptions := map[string]bool{}
	for _, site := range sites {
		descriptions[site.Description] = true
		if site.FunctionID != "func/Score" {
			t.Fatalf("unexpected function id: %#v", site)
		}
	}
	for _, want := range []string{"+ -> -", "1 -> 0", "> -> >=", "0 -> 1", "&& -> ||", "true -> false"} {
		if !descriptions[want] {
			t.Fatalf("missing mutation %q in %#v", want, descriptions)
		}
	}
}

func TestApplyMutation(t *testing.T) {
	content := "package sample\nfunc F() int { return 1 }\n"
	path := filepath.Join(t.TempDir(), "sample.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sites, _, err := Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	mutated := Apply(content, sites[0])
	if !strings.Contains(mutated, "return 0") {
		t.Fatalf("unexpected mutation: %s", mutated)
	}
}
