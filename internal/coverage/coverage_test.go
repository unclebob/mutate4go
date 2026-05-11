package coverage

import (
	"strings"
	"testing"
)

func TestParseProfileAndCovered(t *testing.T) {
	profile, err := ParseProfile(strings.NewReader(`mode: set
github.com/unclebob/sample/foo.go:3.1,5.2 2 1
github.com/unclebob/sample/foo.go:8.1,9.2 1 0
`))
	if err != nil {
		t.Fatal(err)
	}
	if !Covered(profile, "foo.go", 4) {
		t.Fatal("expected line 4 covered by suffix match")
	}
	if Covered(profile, "foo.go", 8) {
		t.Fatal("expected line 8 uncovered")
	}
}
