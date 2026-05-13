package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/unclebob/mutate4go/internal/cli"
	"github.com/unclebob/mutate4go/internal/coverage"
	"github.com/unclebob/mutate4go/internal/manifest"
	"github.com/unclebob/mutate4go/internal/mutations"
)

const CoverageProfile = "target/coverage/coverage.out"

type Result struct {
	Site     mutations.Site
	Status   string
	Duration time.Duration
}

func Run(options cli.Options) error {
	if options.Help {
		fmt.Print(cli.UsageSummary)
		return nil
	}
	if options.Error != "" {
		return fmt.Errorf("%s\n\n%s", options.Error, cli.UsageSummary)
	}
	restored, err := manifest.RestoreBackup(options.SourcePath)
	if err != nil {
		return err
	}
	if restored {
		fmt.Println("Restored source from backup (previous run was interrupted).")
	}
	if options.Scan {
		return Scan(options.SourcePath, options.MutationWarning)
	}
	if options.UpdateManifest {
		return UpdateManifest(options.SourcePath)
	}
	return Mutate(options)
}

func Scan(sourcePath string, warning int) error {
	sites, functions, err := mutations.Discover(sourcePath)
	if err != nil {
		return err
	}
	previous, hasManifest, current, err := currentManifest(sourcePath, functions)
	if err != nil {
		return err
	}
	changed := manifest.ChangedFunctionIDs(previous, current)
	changedCount := countChangedSites(sites, changed)
	fmt.Printf("Mutation scan: %s\n", sourcePath)
	fmt.Printf("Total mutation sites: %d\n", len(sites))
	fmt.Printf("Changed mutation sites: %d\n", changedCount)
	fmt.Printf("Manifest exists: %t\n", hasManifest)
	if len(sites) > warning {
		fmt.Printf("Warning: %d mutation sites exceeds threshold %d.\n", len(sites), warning)
	}
	return nil
}

func UpdateManifest(sourcePath string) error {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	clean := manifest.Strip(string(content))
	_, functions, err := mutations.Discover(sourcePath)
	if err != nil {
		return err
	}
	next := manifest.Build(functions, clean, time.Now())
	embedded, err := manifest.Embed(clean, next)
	if err != nil {
		return err
	}
	if err := os.WriteFile(sourcePath, []byte(embedded), 0o644); err != nil {
		return err
	}
	fmt.Println("Updated manifest: " + sourcePath)
	return nil
}

func Mutate(options cli.Options) error {
	originalBytes, err := os.ReadFile(options.SourcePath)
	if err != nil {
		return err
	}
	analysisContent := manifest.Strip(string(originalBytes))
	if analysisContent != string(originalBytes) {
		if err := os.WriteFile(options.SourcePath, []byte(analysisContent), 0o644); err != nil {
			return err
		}
	}
	sites, functions, err := mutations.Discover(options.SourcePath)
	if err != nil {
		return err
	}
	previous, hasManifest := manifest.Extract(string(originalBytes))
	current := manifest.Build(functions, analysisContent, time.Now())
	changed := manifest.ChangedFunctionIDs(previous, current)
	profile, err := ensureCoverage(options)
	if err != nil {
		return err
	}
	covered, uncovered := partitionByCoverage(profile, options.SourcePath, sites)
	effectiveSinceLastRun := options.SinceLastRun || (hasManifest && !options.MutateAll && options.Lines == nil)
	selected := selectSites(covered, options.Lines, effectiveSinceLastRun, changed)
	printHeader(options, sites, covered, uncovered, selected, hasManifest, changed)
	if len(uncovered) > 0 && options.Lines == nil && !effectiveSinceLastRun {
		printUncovered(uncovered)
	}
	baselineDuration, err := baseline(options.TestCommand)
	if err != nil {
		return fmt.Errorf("baseline failed: %w", err)
	}
	timeout := time.Duration(options.TimeoutFactor) * baselineDuration
	if timeout < time.Second {
		timeout = time.Second
	}
	if err := manifest.SaveBackup(options.SourcePath, analysisContent); err != nil {
		return err
	}
	defer manifest.CleanupBackup(options.SourcePath)
	results, err := runMutations(options.SourcePath, analysisContent, selected, timeout, options.TestCommand, options.MaxWorkers)
	if err != nil {
		return err
	}
	if err := os.WriteFile(options.SourcePath, []byte(analysisContent), 0o644); err != nil {
		return err
	}
	summarize(results, uncovered)
	embedded, err := manifest.Embed(analysisContent, current)
	if err != nil {
		return err
	}
	return os.WriteFile(options.SourcePath, []byte(embedded), 0o644)
}

func ensureCoverage(options cli.Options) (map[string][]coverage.Segment, error) {
	if options.ReuseCoverage {
		profile, err := coverage.LoadProfile(CoverageProfile)
		if err != nil {
			return nil, err
		}
		if profile == nil {
			return nil, fmt.Errorf("Error: --reuse-coverage was requested, but %s does not exist.\nRun without --reuse-coverage once to generate coverage", CoverageProfile)
		}
		fmt.Println("Reusing existing coverage; covered/uncovered classification may be stale.")
		return profile, nil
	}
	if err := os.RemoveAll(filepath.Dir(CoverageProfile)); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(CoverageProfile), 0o755); err != nil {
		return nil, err
	}
	cmd := exec.Command("go", "test", "./...", "-coverprofile="+CoverageProfile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("coverage failed: %w", err)
	}
	return coverage.LoadProfile(CoverageProfile)
}

func baseline(command string) (time.Duration, error) {
	start := time.Now()
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return time.Since(start), err
}

func runMutations(sourcePath, original string, sites []mutations.Site, timeout time.Duration, testCommand string, maxWorkers int) ([]Result, error) {
	if maxWorkers <= 1 || len(sites) <= 1 {
		return runMutationsSerial(sourcePath, original, sites, timeout, testCommand)
	}
	return runMutationsParallel(sourcePath, original, sites, timeout, testCommand, maxWorkers)
}

func runMutationsSerial(sourcePath, original string, sites []mutations.Site, timeout time.Duration, testCommand string) ([]Result, error) {
	var results []Result
	total := len(sites)
	for i, site := range sites {
		mutated := mutations.Apply(original, site)
		if err := os.WriteFile(sourcePath, []byte(mutated), 0o644); err != nil {
			return nil, err
		}
		start := time.Now()
		status := runMutant(testCommand, timeout, "")
		result := Result{Site: site, Status: status, Duration: time.Since(start)}
		results = append(results, result)
		if err := os.WriteFile(sourcePath, []byte(original), 0o644); err != nil {
			return nil, err
		}
		fmt.Printf("[%d/%d] %s line %d %s: %s\n", i+1, total, status, site.Line, site.Description, site.FunctionID)
	}
	return results, nil
}

func runMutationsParallel(sourcePath, original string, sites []mutations.Site, timeout time.Duration, testCommand string, maxWorkers int) ([]Result, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, err
	}
	relSource, err := filepath.Rel(root, absSource)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(relSource, ".."+string(os.PathSeparator)) || relSource == ".." || filepath.IsAbs(relSource) {
		return nil, fmt.Errorf("source file must be inside working directory for parallel mutation: %s", sourcePath)
	}

	if maxWorkers > len(sites) {
		maxWorkers = len(sites)
	}
	runRoot := filepath.Join(root, "target", "mutation-workers", fmt.Sprintf("run-%d-%d", os.Getpid(), time.Now().UnixNano()))
	defer os.RemoveAll(runRoot)

	type job struct {
		Number int
		Site   mutations.Site
	}
	type worker struct {
		Root       string
		SourcePath string
	}
	workers := make([]worker, maxWorkers)
	for i := range workers {
		workerRoot := filepath.Join(runRoot, fmt.Sprintf("worker-%d", i+1))
		if err := copyProject(root, workerRoot); err != nil {
			return nil, err
		}
		workers[i] = worker{
			Root:       workerRoot,
			SourcePath: filepath.Join(workerRoot, relSource),
		}
	}

	jobs := make(chan job, len(sites))
	for i, site := range sites {
		jobs <- job{Number: i + 1, Site: site}
	}
	close(jobs)

	results := make(chan Result, len(sites))
	errs := make(chan error, 1)
	var wg sync.WaitGroup
	for i, w := range workers {
		wg.Add(1)
		go func(workerNumber int, w worker) {
			defer wg.Done()
			for job := range jobs {
				mutated := mutations.Apply(original, job.Site)
				if err := os.WriteFile(w.SourcePath, []byte(mutated), 0o644); err != nil {
					sendFirstError(errs, err)
					return
				}
				start := time.Now()
				status := runMutant(testCommand, timeout, w.Root)
				if err := os.WriteFile(w.SourcePath, []byte(original), 0o644); err != nil {
					sendFirstError(errs, err)
					return
				}
				results <- Result{Site: job.Site, Status: status, Duration: time.Since(start)}
				fmt.Printf("[%d/%d] worker-%d %s line %d %s: %s\n", job.Number, len(sites), workerNumber, status, job.Site.Line, job.Site.Description, job.Site.FunctionID)
			}
		}(i+1, w)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	collected := make([]Result, 0, len(sites))
	for result := range results {
		collected = append(collected, result)
	}
	select {
	case err := <-errs:
		return nil, err
	default:
	}
	if len(collected) != len(sites) {
		return nil, fmt.Errorf("mutation workers stopped after %d/%d results", len(collected), len(sites))
	}
	return sortResults(collected), nil
}

func sendFirstError(errs chan<- error, err error) {
	select {
	case errs <- err:
	default:
	}
}

func sortResults(results []Result) []Result {
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Site.Index < results[j].Site.Index
	})
	return results
}

func runMutant(command string, timeout time.Duration, dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "timeout"
	}
	if err != nil {
		return "killed"
	}
	return "survived"
}

func copyProject(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		if shouldSkipCopy(rel, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case info.Mode().IsRegular():
			return copyFile(path, target, info.Mode().Perm())
		default:
			return nil
		}
	})
}

func shouldSkipCopy(rel string, entry os.DirEntry) bool {
	if rel == ".git" || strings.HasPrefix(rel, ".git"+string(os.PathSeparator)) {
		return true
	}
	if rel == filepath.Join("target", "mutation-workers") ||
		strings.HasPrefix(rel, filepath.Join("target", "mutation-workers")+string(os.PathSeparator)) {
		return true
	}
	return false
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func partitionByCoverage(profile map[string][]coverage.Segment, sourcePath string, sites []mutations.Site) ([]mutations.Site, []mutations.Site) {
	var covered []mutations.Site
	var uncovered []mutations.Site
	for _, site := range sites {
		if coverage.Covered(profile, sourcePath, site.Line) {
			covered = append(covered, site)
		} else {
			uncovered = append(uncovered, site)
		}
	}
	return covered, uncovered
}

func selectSites(sites []mutations.Site, lines map[int]bool, sinceLastRun bool, changed map[string]bool) []mutations.Site {
	var selected []mutations.Site
	for _, site := range sites {
		if lines != nil && !lines[site.Line] {
			continue
		}
		if sinceLastRun && !changed[site.FunctionID] {
			continue
		}
		selected = append(selected, site)
	}
	return selected
}

func currentManifest(sourcePath string, functions []mutations.Function) (*manifest.Manifest, bool, manifest.Manifest, error) {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, false, manifest.Manifest{}, err
	}
	clean := manifest.Strip(string(content))
	previous, exists := manifest.Extract(string(content))
	return previous, exists, manifest.Build(functions, clean, time.Now()), nil
}

func countChangedSites(sites []mutations.Site, changed map[string]bool) int {
	n := 0
	for _, site := range sites {
		if changed[site.FunctionID] {
			n++
		}
	}
	return n
}

func printHeader(options cli.Options, all, covered, uncovered, selected []mutations.Site, hasManifest bool, changed map[string]bool) {
	fmt.Printf("Mutation run: %s\n", options.SourcePath)
	fmt.Printf("Total mutation sites: %d\n", len(all))
	fmt.Printf("Covered mutation sites: %d\n", len(covered))
	fmt.Printf("Uncovered mutation sites: %d\n", len(uncovered))
	fmt.Printf("Changed mutation sites: %d\n", countChangedSites(all, changed))
	fmt.Printf("Manifest exists: %t\n", hasManifest)
	fmt.Printf("Selected mutation sites: %d\n", len(selected))
	if len(all) > options.MutationWarning {
		fmt.Printf("Warning: %d mutation sites exceeds threshold %d.\n", len(all), options.MutationWarning)
	}
	if options.MaxWorkers > 0 {
		fmt.Printf("Mutation workers: %d\n", options.MaxWorkers)
	}
}

func printUncovered(sites []mutations.Site) {
	fmt.Println("Uncovered mutations:")
	for _, site := range sites {
		fmt.Printf("  line %d %s %s\n", site.Line, site.Description, site.FunctionID)
	}
}

func summarize(results []Result, uncovered []mutations.Site) {
	counts := map[string]int{}
	for _, result := range results {
		counts[result.Status]++
	}
	keys := []string{"killed", "survived", "timeout"}
	sort.Strings(keys)
	fmt.Println()
	fmt.Println("Mutation Report")
	fmt.Println("===============")
	fmt.Printf("Killed: %d\n", counts["killed"]+counts["timeout"])
	fmt.Printf("Survived: %d\n", counts["survived"])
	fmt.Printf("Uncovered: %d\n", len(uncovered))
	if counts["survived"] > 0 {
		fmt.Println()
		fmt.Println("Survivors:")
		for _, result := range results {
			if result.Status == "survived" {
				fmt.Printf("  line %d %s %s\n", result.Site.Line, result.Site.Description, result.Site.FunctionID)
			}
		}
	}
}

func StatusCode(err error) int {
	if err == nil {
		return 0
	}
	if strings.Contains(err.Error(), cli.UsageSummary) {
		return 1
	}
	return 1
}
