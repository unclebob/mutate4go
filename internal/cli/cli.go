package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const DefaultTestCommand = "go test ./..."

const UsageSummary = `Usage: mutate4go <source-file.go> [options]

Options:
  --scan                Report mutation counts without running tests or coverage
  --update-manifest     Rewrite the embedded manifest without running mutations
  --reuse-coverage      Reuse existing Go coverage data without refreshing coverage
  --lines L1,L2,...      Run only mutations on these source lines
  --since-last-run       Run only mutations in changed functions since last successful run
  --mutate-all           Run all covered mutations even if a manifest exists
  --mutation-warning N   Warn when more than N mutations are found (default 50)
  --timeout-factor N     Mutation timeout multiplier vs baseline (default 10)
  --test-command CMD     Test command to run (default "go test ./...")
  --max-workers N        Accepted for workflow compatibility; runs are serialized
  --help                 Print this help and exit
`

type Options struct {
	SourcePath      string
	Scan            bool
	UpdateManifest  bool
	ReuseCoverage   bool
	Lines           map[int]bool
	SinceLastRun    bool
	MutateAll       bool
	MutationWarning int
	TimeoutFactor   int
	TestCommand     string
	MaxWorkers      int
	Help            bool
	Error           string
}

func DefaultOptions() Options {
	return Options{
		MutationWarning: 50,
		TimeoutFactor:   10,
		TestCommand:     DefaultTestCommand,
	}
}

func ValidateArgs(args []string) Options {
	for _, arg := range args {
		if arg == "--help" {
			return Options{Help: true}
		}
	}
	options := DefaultOptions()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--scan":
			if options.UpdateManifest || options.Lines != nil || options.SinceLastRun || options.MutateAll ||
				options.TimeoutFactor != 10 || options.TestCommand != DefaultTestCommand || options.MaxWorkers != 0 {
				return usageError("Cannot combine --scan with --update-manifest or mutation execution options.")
			}
			options.Scan = true
		case "--update-manifest":
			if options.Scan || options.Lines != nil || options.SinceLastRun || options.MutateAll ||
				options.TimeoutFactor != 10 || options.TestCommand != DefaultTestCommand || options.MaxWorkers != 0 {
				return usageError("Cannot combine --update-manifest with --scan or mutation execution options.")
			}
			options.UpdateManifest = true
		case "--reuse-coverage", "--reuse-lcov":
			options.ReuseCoverage = true
		case "--since-last-run":
			if options.Scan || options.UpdateManifest || options.Lines != nil || options.MutateAll {
				return usageError("Cannot combine --since-last-run with --scan, --update-manifest, --lines, or --mutate-all.")
			}
			options.SinceLastRun = true
		case "--mutate-all":
			if options.Scan || options.UpdateManifest || options.Lines != nil || options.SinceLastRun {
				return usageError("Cannot combine --mutate-all with --scan, --update-manifest, --lines, or --since-last-run.")
			}
			options.MutateAll = true
		case "--lines", "--mutation-warning", "--timeout-factor", "--test-command", "--max-workers":
			if i+1 >= len(args) {
				return usageError("Missing value for " + arg + ".")
			}
			i++
			var err error
			options, err = consumeValueOption(options, arg, args[i])
			if err != nil {
				return usageError(err.Error())
			}
		default:
			if strings.HasPrefix(arg, "--") {
				return usageError("Unknown option: " + arg)
			}
			if options.SourcePath != "" {
				return usageError("Unexpected extra argument: " + arg)
			}
			options.SourcePath = arg
		}
		if options.Error != "" {
			return options
		}
	}
	if options.SourcePath == "" {
		return usageError("Missing source file argument.")
	}
	if _, err := os.Stat(options.SourcePath); err != nil {
		return usageError("Source file not found: " + options.SourcePath)
	}
	return options
}

func consumeValueOption(options Options, name, value string) (Options, error) {
	switch name {
	case "--lines":
		if options.Scan || options.UpdateManifest || options.SinceLastRun || options.MutateAll {
			return options, fmt.Errorf("cannot combine --lines with --scan, --update-manifest, --since-last-run, or --mutate-all")
		}
		lines, err := parseLines(value)
		if err != nil {
			return options, err
		}
		options.Lines = lines
	case "--mutation-warning":
		n, err := parsePositiveInt(value, name)
		if err != nil {
			return options, err
		}
		options.MutationWarning = n
	case "--timeout-factor":
		if options.Scan || options.UpdateManifest {
			return options, fmt.Errorf("cannot combine --scan or --update-manifest with --timeout-factor")
		}
		n, err := parsePositiveInt(value, name)
		if err != nil {
			return options, err
		}
		options.TimeoutFactor = n
	case "--test-command":
		if options.Scan || options.UpdateManifest {
			return options, fmt.Errorf("cannot combine --scan or --update-manifest with --test-command")
		}
		if strings.TrimSpace(value) == "" {
			return options, fmt.Errorf("missing value for --test-command")
		}
		options.TestCommand = value
	case "--max-workers":
		n, err := parsePositiveInt(value, name)
		if err != nil {
			return options, err
		}
		options.MaxWorkers = n
	}
	return options, nil
}

func parseLines(value string) (map[int]bool, error) {
	lines := map[int]bool{}
	for _, part := range strings.Split(value, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid value for --lines. Expected comma-separated positive integers")
		}
		lines[n] = true
	}
	return lines, nil
}

func parsePositiveInt(value, name string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid value for %s. Expected a positive integer", name)
	}
	return n, nil
}

func usageError(message string) Options {
	options := DefaultOptions()
	options.Error = message
	return options
}
