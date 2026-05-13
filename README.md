# mutate4go

Mutation testing for Go. Discovers mutation sites, applies each one, runs your tests, and reports killed, survived, and uncovered mutations.

## Setup

Run from the root of a Go module:

```bash
go run github.com/unclebob/mutate4go/cmd/mutate4go@latest path/to/file.go
```

Or install it:

```bash
go install github.com/unclebob/mutate4go/cmd/mutate4go@latest
mutate4go path/to/file.go
```

## Usage

```bash
# Mutate-test a source file.
# If the file already has a footer manifest, this defaults to changed functions only.
mutate4go internal/foo/foo.go

# Scan a file for mutation counts without running coverage or tests
mutate4go internal/foo/foo.go --scan

# Rewrite the embedded manifest without running coverage or mutations
mutate4go internal/foo/foo.go --update-manifest

# Retest only specific lines
mutate4go internal/foo/foo.go --lines 45,67,89

# Force differential mutation explicitly
mutate4go internal/foo/foo.go --since-last-run

# Override differential behavior and mutate all covered sites
mutate4go internal/foo/foo.go --mutate-all

# Reuse existing coverage data without refreshing coverage
mutate4go internal/foo/foo.go --reuse-coverage

# Warn when a module exceeds a mutation-count threshold
mutate4go internal/foo/foo.go --mutation-warning 75

# Use a custom timeout multiplier
mutate4go internal/foo/foo.go --timeout-factor 15

# Use a custom test command
mutate4go internal/foo/foo.go --test-command "go test ./internal/foo"

# Run mutations in parallel with isolated workers
mutate4go internal/foo/foo.go --max-workers 4

# Show help
mutate4go --help
```

The tool automatically:

- Runs coverage with `go test ./... -coverprofile=target/coverage/coverage.out`
- Runs a baseline test command, defaulting to `go test ./...`
- Applies each covered mutation, runs tests with a timeout, and restores the original file
- Runs mutations in parallel with isolated worker directories when `--max-workers` is greater than `1`
- Writes an embedded footer manifest with the last test date and function hashes
- Defaults to differential mutation when that footer manifest already exists
- Prints a warning when mutation count exceeds `--mutation-warning` (default `50`)
- Can reuse existing coverage data with `--reuse-coverage`

`--scan` is the fast structural mode. It skips coverage, skips test execution, and reports:

- total mutation sites
- changed mutation sites relative to the embedded manifest
- the standard mutation-count warning

`--update-manifest` rewrites the embedded footer manifest for the file's current contents without running coverage, baseline tests, or mutation workers.

## Recommended Workflow

Run mutation testing one file at a time.

```bash
go test ./...
mutate4go internal/foo/foo.go --scan
mutate4go internal/foo/foo.go
```

Recommended loop for each file:

1. Run `mutate4go path/to/file.go --max-workers 3`.
2. If any mutations are uncovered, add or fix tests until they are covered.
3. If any mutations survive, change code or tests until they are killed.
4. Rerun the same single-file mutation command.
5. Only start the next file when the current file has no uncovered mutations and no survivors.

For local incremental work, once a file has a footer manifest the default run is differential. You can still be explicit:

```bash
mutate4go internal/foo/foo.go --since-last-run
```

To force a full rerun on a file with a manifest:

```bash
mutate4go internal/foo/foo.go --mutate-all
```

## Mutation Rules

| Category | Mutations |
|----------|-----------|
| Arithmetic | `+` -> `-`, `-` -> `+`, `*` -> `/` |
| Comparison | `>` <-> `>=`, `<` <-> `<=` |
| Equality | `==` <-> `!=` |
| Boolean | `true` <-> `false` |
| Logical | `&&` <-> `||` |
| Constant | `0` <-> `1` |

## Coverage Integration

mutate4go reads Go coverage profiles from:

```bash
target/coverage/coverage.out
```

Coverage freshness is regenerated automatically unless `--reuse-coverage` is supplied. If a mutation site is not covered by the profile, mutate4go reports it as uncovered and does not spend time running the mutation.

## Manifest

The footer manifest is embedded at the end of the source file and records:

- the last successful mutation test date
- each function's id
- its line span
- a hash of its normalized source text

Differential mutation runs update the footer manifest after completion, so the next differential run compares against the latest mutation baseline.

## Claude Code Skill

This repo includes a Claude Code skill at `skills/using-mutate4go/SKILL.md`.

## Development

```bash
go test ./...
go run ./cmd/mutate4go --help
go run ./cmd/mutate4go internal/mutations/mutations.go --scan
```

## License

Copyright (c) Robert C. Martin. All rights reserved.
