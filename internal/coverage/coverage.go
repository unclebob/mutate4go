package coverage

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type Segment struct {
	File       string
	StartLine  int
	EndLine    int
	Statements int
	Count      int
}

func LoadProfile(path string) (map[string][]Segment, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	return ParseProfile(f)
}

func ParseProfile(r io.Reader) (map[string][]Segment, error) {
	out := map[string][]Segment{}
	scanner := bufio.NewScanner(r)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		segment, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		out[segment.File] = append(out[segment.File], segment)
	}
	return out, scanner.Err()
}

func Covered(profile map[string][]Segment, file string, line int) bool {
	for _, segment := range segmentsForFile(profile, file) {
		if line >= segment.StartLine && line <= segment.EndLine && segment.Count > 0 {
			return true
		}
	}
	return false
}

func parseLine(line string) (Segment, error) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return Segment{}, fmt.Errorf("invalid coverage segment %q", line)
	}
	file, rest, found := strings.Cut(fields[0], ":")
	if !found {
		return Segment{}, fmt.Errorf("missing file separator")
	}
	start, end, found := strings.Cut(rest, ",")
	if !found {
		return Segment{}, fmt.Errorf("missing range separator")
	}
	startLine, err := parseLineNumber(start)
	if err != nil {
		return Segment{}, err
	}
	endLine, err := parseLineNumber(end)
	if err != nil {
		return Segment{}, err
	}
	statements, err := strconv.Atoi(fields[1])
	if err != nil {
		return Segment{}, fmt.Errorf("invalid statement count: %w", err)
	}
	count, err := strconv.Atoi(fields[2])
	if err != nil {
		return Segment{}, fmt.Errorf("invalid execution count: %w", err)
	}
	return Segment{File: file, StartLine: startLine, EndLine: endLine, Statements: statements, Count: count}, nil
}

func parseLineNumber(position string) (int, error) {
	line, _, found := strings.Cut(position, ".")
	if !found {
		return 0, fmt.Errorf("invalid position %q", position)
	}
	return strconv.Atoi(line)
}

func segmentsForFile(profile map[string][]Segment, file string) []Segment {
	normalized := normalizePath(file)
	if segments := profile[normalized]; len(segments) > 0 {
		return segments
	}
	for candidate, segments := range profile {
		if suffixMatch(normalizePath(candidate), normalized) {
			return segments
		}
	}
	return nil
}

func suffixMatch(path, suffix string) bool {
	pathParts := pathSegments(path)
	suffixParts := pathSegments(suffix)
	if len(suffixParts) == 0 || len(suffixParts) > len(pathParts) {
		return false
	}
	offset := len(pathParts) - len(suffixParts)
	for i := range suffixParts {
		if pathParts[offset+i] != suffixParts[i] {
			return false
		}
	}
	return true
}

func normalizePath(path string) string {
	return strings.TrimPrefix(strings.ReplaceAll(path, "\\", "/"), "./")
}

func pathSegments(path string) []string {
	parts := strings.Split(normalizePath(path), "/")
	var out []string
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
