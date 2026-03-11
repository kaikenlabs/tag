package testrunner

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/tmplconfig"
)

const (
	defaultParallel    = 4
	defaultMaxCases    = 64
	defaultProjectName = "test-scaffold"
	maxOutputLen       = 4096
)

// Run executes the matrix test with the given configuration.
func Run(ctx context.Context, cfg Config) (Report, error) {
	templateDir, err := filepath.Abs(cfg.TemplateDir)
	if err != nil {
		return Report{}, fmt.Errorf("resolve template dir: %w", err)
	}

	// Load and parse template config.
	tmplCfg, testCfg, err := loadTemplateConfig(templateDir)
	if err != nil {
		return Report{}, err
	}

	// Resolve validation commands: CLI override > template config > none.
	commands := cfg.RunCommands
	if len(commands) == 0 && testCfg != nil {
		commands = testCfg.Commands
	}

	// Resolve environment variables: CLI env merged over template config env.
	env := make(map[string]string)
	if testCfg != nil {
		maps.Copy(env, testCfg.Env)
	}
	maps.Copy(env, cfg.Env)

	// Resolve project name.
	projectName := defaultProjectName
	if testCfg != nil && testCfg.ProjectName != "" {
		projectName = testCfg.ProjectName
	}

	// Extract boolean vars and generate combinations.
	boolVars := ExtractBooleanVars(tmplCfg, cfg.SkipVars)
	combos := GenerateCombinations(boolVars, cfg.PinVars)
	combos = FilterCombinations(combos, cfg.Filter)

	// Safety limit check.
	maxCases := cfg.MaxCases
	if maxCases == 0 {
		maxCases = defaultMaxCases
	}
	if maxCases > 0 && len(combos) > maxCases {
		return Report{}, fmt.Errorf(
			"combination count %d exceeds safety limit %d (use --max-cases 0 to override)",
			len(combos), maxCases,
		)
	}

	report := Report{
		TotalCases:  len(combos),
		TemplateDir: templateDir,
	}

	// Dry run: just return the report with all cases listed.
	if cfg.DryRun {
		for _, c := range combos {
			report.Cases = append(report.Cases, CaseResult{
				Combination: c,
				Status:      CasePassed,
				Phase:       "dry-run",
			})
			report.Passed++
		}
		return report, nil
	}

	// Run tests with worker pool.
	parallel := cfg.Parallel
	if parallel <= 0 {
		parallel = defaultParallel
	}
	if parallel > len(combos) {
		parallel = len(combos)
	}

	start := time.Now()

	results := runWorkerPool(ctx, workerConfig{
		combos:      combos,
		templateDir: templateDir,
		projectName: projectName,
		meta:        cfg.Meta,
		valuesFile:  cfg.ValuesFile,
		commands:    commands,
		env:         env,
		timeout:     cfg.Timeout,
		acceptHooks: cfg.AcceptHooks,
		keepFailed:  cfg.KeepFailed,
		verbose:     cfg.Verbose,
		parallel:    parallel,
		failFast:    cfg.FailFast,
	})

	report.Duration = time.Since(start)

	for _, r := range results {
		report.Cases = append(report.Cases, r)
		switch r.Status {
		case CasePassed:
			report.Passed++
		case CaseFailed:
			report.Failed++
		case CaseErrored:
			report.Errored++
		}
	}

	return report, nil
}

type workerConfig struct {
	combos      []Combination
	templateDir string
	projectName string
	meta        map[string]string
	valuesFile  string
	commands    []string
	env         map[string]string
	timeout     time.Duration
	acceptHooks bool
	keepFailed  bool
	verbose     bool
	parallel    int
	failFast    bool
}

func runWorkerPool(ctx context.Context, wcfg workerConfig) []CaseResult {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	comboCh := make(chan Combination, len(wcfg.combos))
	for _, c := range wcfg.combos {
		comboCh <- c
	}
	close(comboCh)

	resultCh := make(chan CaseResult, len(wcfg.combos))

	var wg sync.WaitGroup
	for range wcfg.parallel {
		wg.Go(func() {
			for combo := range comboCh {
				if ctx.Err() != nil {
					return
				}
				result := runSingleTest(ctx, wcfg, combo)
				resultCh <- result

				if wcfg.failFast && result.Status != CasePassed {
					cancel()
					return
				}
			}
		})
	}

	wg.Wait()
	close(resultCh)

	results := make([]CaseResult, 0, len(wcfg.combos))
	for r := range resultCh {
		results = append(results, r)
	}
	return results
}

func runSingleTest(ctx context.Context, wcfg workerConfig, combo Combination) CaseResult {
	start := time.Now()

	// Create isolated temp directory for this combination.
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("tag-test-%d-*", combo.Index))
	if err != nil {
		return CaseResult{
			Combination: combo,
			Status:      CaseErrored,
			Phase:       "setup",
			Error:       "create temp dir: " + err.Error(),
			Duration:    time.Since(start),
		}
	}

	shouldClean := true
	defer func() {
		if shouldClean {
			os.RemoveAll(tmpDir)
		}
	}()

	// Build meta map: base meta + boolean combo vars.
	meta := make(map[string]string, len(wcfg.meta)+len(combo.Vars))
	maps.Copy(meta, wcfg.meta)
	maps.Copy(meta, combo.Vars)

	// Scaffold programmatically.
	opts := scaffold.Options{
		TemplateDir: wcfg.templateDir,
		OutputDir:   tmpDir,
		ProjectName: wcfg.projectName,
		Meta:        meta,
		ValuesFile:  wcfg.valuesFile,
		NoInput:     true,
		Force:       true,
		NoSave:      true,
		AcceptHooks: wcfg.acceptHooks,
	}

	s, err := scaffold.NewScaffold(opts, scaffold.WithOutput(io.Discard))
	if err != nil {
		return CaseResult{
			Combination: combo,
			Status:      CaseErrored,
			Phase:       "scaffold-init",
			Error:       err.Error(),
			Duration:    time.Since(start),
		}
	}

	result, err := s.Run(opts)
	if err != nil {
		return CaseResult{
			Combination: combo,
			Status:      CaseFailed,
			Phase:       "scaffold",
			Error:       err.Error(),
			Duration:    time.Since(start),
		}
	}

	return runValidation(ctx, wcfg, combo, result.OutputDir, &shouldClean, start)
}

func runValidation(
	ctx context.Context,
	wcfg workerConfig,
	combo Combination,
	outputDir string,
	shouldClean *bool,
	start time.Time,
) CaseResult {
	if len(wcfg.commands) == 0 {
		return CaseResult{
			Combination: combo,
			Status:      CasePassed,
			Duration:    time.Since(start),
		}
	}

	vResult := RunValidationCommands(ctx, outputDir, wcfg.commands, wcfg.env, wcfg.timeout)
	if vResult == nil {
		return CaseResult{
			Combination: combo,
			Status:      CasePassed,
			Duration:    time.Since(start),
		}
	}

	output := vResult.Output
	if !wcfg.verbose {
		output = TruncateOutput(output, maxOutputLen)
	}

	if wcfg.keepFailed {
		*shouldClean = false
	}

	errMsg := fmt.Sprintf("command %q failed (exit %d)", vResult.Command, vResult.ExitCode)
	if vResult.Err != nil {
		errMsg = fmt.Sprintf("command %q: %s", vResult.Command, vResult.Err)
	}

	return CaseResult{
		Combination: combo,
		Status:      CaseFailed,
		Phase:       "validate: " + vResult.Command,
		Output:      output,
		Error:       errMsg,
		Duration:    time.Since(start),
	}
}

func loadTemplateConfig(templateDir string) (*tmplconfig.TemplateConfig, *tmplconfig.TestConfig, error) {
	configPath := filepath.Join(templateDir, "tag.template.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read template config: %w", err)
	}

	tmplCfg, err := tmplconfig.ParseTemplateConfig(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parse template config: %w", err)
	}

	testCfg, err := tmplconfig.ParseTestConfig(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parse test config: %w", err)
	}

	return tmplCfg, testCfg, nil
}
