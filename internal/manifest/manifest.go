package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/unclebob/mutate4go/internal/mutations"
)

const beginMarker = "// mutate4go-manifest-begin"
const endMarker = "// mutate4go-manifest-end"

type Manifest struct {
	Version    int        `json:"version"`
	TestedAt   string     `json:"tested_at"`
	ModuleHash string     `json:"module_hash"`
	Functions  []Function `json:"functions"`
}

type Function struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Line    int    `json:"line"`
	EndLine int    `json:"end_line"`
	Hash    string `json:"hash"`
}

func Strip(content string) string {
	start := strings.Index(content, beginMarker)
	if start < 0 {
		return content
	}
	prefix := strings.TrimRight(content[:start], "\n")
	return prefix + "\n"
}

func Extract(content string) (*Manifest, bool) {
	start := strings.Index(content, beginMarker)
	end := strings.Index(content, endMarker)
	if start < 0 || end < 0 || end <= start {
		return nil, false
	}
	block := content[start+len(beginMarker) : end]
	var lines []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "//")
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	var manifest Manifest
	if err := json.Unmarshal([]byte(strings.Join(lines, "\n")), &manifest); err != nil {
		return nil, false
	}
	return &manifest, true
}

func Build(functions []mutations.Function, content string, now time.Time) Manifest {
	out := Manifest{
		Version:    1,
		TestedAt:   now.Format(time.RFC3339),
		ModuleHash: hash(content),
	}
	for _, fn := range functions {
		out.Functions = append(out.Functions, Function{
			ID: fn.ID, Name: fn.Name, Line: fn.StartLine, EndLine: fn.EndLine, Hash: hash(normalize(fn.Text)),
		})
	}
	return out
}

func Embed(content string, manifest Manifest) (string, error) {
	clean := Strip(content)
	data, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(clean, "\n"))
	b.WriteString("\n\n")
	b.WriteString(beginMarker)
	b.WriteString("\n// ")
	b.Write(data)
	b.WriteString("\n")
	b.WriteString(endMarker)
	b.WriteString("\n")
	return b.String(), nil
}

func ChangedFunctionIDs(previous *Manifest, current Manifest) map[string]bool {
	changed := map[string]bool{}
	if previous == nil {
		for _, fn := range current.Functions {
			changed[fn.ID] = true
		}
		return changed
	}
	prior := map[string]string{}
	for _, fn := range previous.Functions {
		prior[fn.ID] = fn.Hash
	}
	for _, fn := range current.Functions {
		if prior[fn.ID] != fn.Hash {
			changed[fn.ID] = true
		}
	}
	return changed
}

func SaveBackup(path, content string) error {
	return os.WriteFile(backupPath(path), []byte(content), 0o644)
}

func RestoreBackup(path string) (bool, error) {
	data, err := os.ReadFile(backupPath(path))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, os.WriteFile(path, data, 0o644)
}

func CleanupBackup(path string) error {
	err := os.Remove(backupPath(path))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func backupPath(path string) string {
	return path + ".mutate4go.bak"
}

func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func normalize(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
