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
	defaultProjectName = "test-scaffold"
)

// Plan resolves the test configuration and generates the execution plan.
// It validates limits before allocating combinations to prevent OOM.
func Plan(cfg Config) (*TestPlan, error) {
	templateDir, err := filepath.Abs(cfg.TemplateDir)
	if err != nil {
		return nil, fmt.Errorf("resolve template dir: %w", err)
	}

	// Load and parse template config.
	tmplCfg, testCfg, err := loadTemplateConfig(templateDir)
	if err != nil {
		return nil, err
	}

	// Resolve validation commands: CLI override > template config > none.
	commands := cfg.RunCommands
	if len(commands) == 0 && testCfg != nil {
		commands = testCfg.Commands
	}

	// Security: require opt-in for template-defined commands.
	if len(cfg.RunCommands) == 0 && len(commands) > 0 && !cfg.AcceptHooks {
		return nil, fmt.Errorf(
			"template defines test commands %v; pass --accept-hooks to allow or --run to override",
			commands,
		)
	}

	// Resolve environment variables from template config.
	env := make(map[string]string)
	if testCfg != nil {
		maps.Copy(env, testCfg.Env)
	}

	// Resolve project name.
	projectName := defaultProjectName
	if testCfg != nil && testCfg.ProjectName != "" {
		projectName = testCfg.ProjectName
	}

	// Extract boolean vars.
	boolVars := ExtractBooleanVars(tmplCfg, cfg.SkipVars)

	// Check safety limit BEFORE allocating combinations to prevent OOM.
	count := CombinationCount(boolVars, cfg.PinVars)
	if cfg.MaxCases > 0 && count > cfg.MaxCases {
		return nil, fmt.Errorf(
			"combination count %d exceeds safety limit %d (use --max-cases 0 to override)",
			count, cfg.MaxCases,
		)
	}

	// Generate and filter combinations.
	combos := GenerateCombinations(boolVars, cfg.PinVars)
	combos, err = FilterCombinations(combos, cfg.Filter)
	if err != nil {
		return nil, err
	}

	return &TestPlan{
		TemplateDir: templateDir,
		BoolVars:    boolVars,
		Combos:      combos,
		Commands:    commands,
		Env:         env,
		ProjectName: projectName,
	}, nil
}

// Execute runs the test plan with the given configuration and returns the report.
func Execute(ctx context.Context, plan *TestPlan, cfg Config) (Report, error) {
	parallel := cfg.Parallel
	if parallel <= 0 {
		parallel = defaultParallel
	}
	if parallel > len(plan.Combos) {
		parallel = len(plan.Combos)
	}

	start := time.Now()

	results := runWorkerPool(ctx, plan, cfg, parallel)

	report := Report{
		TotalCases:  len(plan.Combos),
		TemplateDir: plan.TemplateDir,
		Duration:    time.Since(start),
	}

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

func runWorkerPool(ctx context.Context, plan *TestPlan, cfg Config, parallel int) []CaseResult {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	comboCh := make(chan Combination, len(plan.Combos))
	for _, c := range plan.Combos {
		comboCh <- c
	}
	close(comboCh)

	resultCh := make(chan CaseResult, len(plan.Combos))

	var wg sync.WaitGroup
	for range parallel {
		wg.Go(func() {
			for combo := range comboCh {
				if ctx.Err() != nil {
					return
				}
				result := runSingleTest(ctx, plan, cfg, combo)
				resultCh <- result

				if cfg.FailFast && result.Status != CasePassed {
					cancel()
					return
				}
			}
		})
	}

	wg.Wait()
	close(resultCh)

	results := make([]CaseResult, 0, len(plan.Combos))
	for r := range resultCh {
		results = append(results, r)
	}
	return results
}

func runSingleTest(ctx context.Context, plan *TestPlan, cfg Config, combo Combination) CaseResult {
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
	meta := make(map[string]string, len(cfg.Meta)+len(combo.Vars))
	maps.Copy(meta, cfg.Meta)
	maps.Copy(meta, combo.Vars)

	// Scaffold programmatically.
	opts := scaffold.Options{
		TemplateDir: plan.TemplateDir,
		OutputDir:   tmpDir,
		ProjectName: plan.ProjectName,
		Meta:        meta,
		ValuesFile:  cfg.ValuesFile,
		NoInput:     true,
		Force:       true,
		NoSave:      true,
		AcceptHooks: cfg.AcceptHooks,
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

	// Use ProjectRoot (not OutputDir) so validation commands run inside the
	// actual project directory, not the parent temp dir. With wrapper-style
	// templates the scaffold creates a subdirectory (e.g. tmpDir/project-name/).
	projectDir := result.ProjectRoot
	if projectDir == "" {
		projectDir = result.OutputDir
	}

	return runValidation(ctx, plan, cfg, combo, projectDir, tmpDir, &shouldClean, start)
}

func runValidation(
	ctx context.Context,
	plan *TestPlan,
	cfg Config,
	combo Combination,
	outputDir string,
	tmpDir string,
	shouldClean *bool,
	start time.Time,
) CaseResult {
	if len(plan.Commands) == 0 {
		return CaseResult{
			Combination: combo,
			Status:      CasePassed,
			Duration:    time.Since(start),
		}
	}

	vResult := RunValidationCommands(ctx, outputDir, plan.Commands, plan.Env, cfg.Timeout)
	if vResult == nil {
		return CaseResult{
			Combination: combo,
			Status:      CasePassed,
			Duration:    time.Since(start),
		}
	}

	output := vResult.Output
	if !cfg.Verbose {
		output = TruncateOutput(output, maxOutputLen)
	}

	var keptDir string
	if cfg.KeepFailed {
		*shouldClean = false
		keptDir = tmpDir
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
		KeptDir:     keptDir,
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
