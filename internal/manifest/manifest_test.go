package manifest

import (
	"testing"
	"time"

	"github.com/unclebob/mutate4go/internal/mutations"
)

func TestEmbedExtractAndStrip(t *testing.T) {
	content := "package sample\n\nfunc F() int { return 1 }\n"
	built := Build([]mutations.Function{{ID: "func/F", Name: "F", StartLine: 3, EndLine: 3, Text: "func F() int { return 1 }"}}, content, time.Unix(0, 0))
	embedded, err := Embed(content, built)
	if err != nil {
		t.Fatal(err)
	}
	extracted, ok := Extract(embedded)
	if !ok {
		t.Fatal("expected manifest")
	}
	if extracted.Functions[0].ID != "func/F" {
		t.Fatalf("unexpected manifest: %#v", extracted)
	}
	if Strip(embedded) != content {
		t.Fatalf("strip mismatch: %q", Strip(embedded))
	}
}

func TestChangedFunctionIDs(t *testing.T) {
	previous := &Manifest{Functions: []Function{{ID: "func/F", Hash: "old"}, {ID: "func/G", Hash: "same"}}}
	current := Manifest{Functions: []Function{{ID: "func/F", Hash: "new"}, {ID: "func/G", Hash: "same"}}}
	changed := ChangedFunctionIDs(previous, current)
	if !changed["func/F"] || changed["func/G"] {
		t.Fatalf("unexpected changed set: %#v", changed)
	}
}
