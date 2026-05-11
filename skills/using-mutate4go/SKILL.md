---
name: using-mutate4go
description: Use when mutation-testing Go code, assessing test quality beyond coverage, or investigating surviving mutations to strengthen tests
---

# Using mutate4go

## Overview

Mutation testing for Go. Mutates source code systematically, runs tests against each mutant, and reports which mutations survived versus were killed. Surviving mutations reveal gaps in the test suite that line coverage alone cannot detect.

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
# Mutation-test a source file.
# If the file has a footer manifest, this defaults to changed functions only.
mutate4go internal/foo/foo.go

# Scan mutation counts without running coverage or tests
mutate4go internal/foo/foo.go --scan

# Rewrite the embedded manifest without running coverage or mutations
mutate4go internal/foo/foo.go --update-manifest

# Retest only specific lines
mutate4go internal/foo/foo.go --lines 45,67,89

# Retest only functions changed since the last successful mutation run
mutate4go internal/foo/foo.go --since-last-run

# Override differential mode and run all covered mutations
mutate4go internal/foo/foo.go --mutate-all

# Reuse existing coverage data without refreshing it
mutate4go internal/foo/foo.go --reuse-coverage
```

The tool automatically:

- Runs coverage with `go test ./... -coverprofile=target/coverage/coverage.out`
- Runs a baseline test command, defaulting to `go test ./...`
- Applies each mutation, runs tests with a timeout, and restores the source file
- Writes an embedded footer manifest with `tested_at` and function hashes after a run
- Defaults to differential mutation when that footer manifest already exists
- Can reuse existing coverage data with `--reuse-coverage`

Use `--scan` when you want a fast module-size signal without paying for coverage or test execution.

Use `--update-manifest` when you want to accept the current file contents as the new differential baseline without paying for coverage or mutation execution.

## Mutation Rules

| Category | Mutations |
|----------|-----------|
| Arithmetic | `+` -> `-`, `-` -> `+`, `*` -> `/` |
| Comparison | `>` <-> `>=`, `<` <-> `<=` |
| Equality | `==` <-> `!=` |
| Boolean | `true` <-> `false` |
| Logical | `&&` <-> `||` |
| Constant | `0` <-> `1` |

## Interpreting Results

| Result | Meaning | Action |
|--------|---------|--------|
| **Killed** | Test caught the mutation | Tests are strong here |
| **Survived** | Tests passed with mutant code | Write a test that would fail with the mutation |
| **Timeout** | Mutation hit the timeout | Treated as killed because behavior changed |
| **Uncovered** | Coverage did not reach the site | Add or fix tests before mutation work |

## Workflow

1. Run `go test ./...`.
2. Run `mutate4go path/to/file.go --scan`.
3. Run `mutate4go path/to/file.go`.
4. Review survivors and uncovered mutations.
5. Write tests to kill survivors and cover uncovered sites.
6. Retest focused lines with `mutate4go path/to/file.go --lines 45,67`.
7. Repeat until the file has no uncovered mutations and no survivors.
