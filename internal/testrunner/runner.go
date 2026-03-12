package testrunner

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strconv"
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

	// Extract boolean vars.
	boolVars := ExtractBooleanVars(tmplCfg, cfg.SkipVars)

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

	// Build test case plans.
	cases, err := buildCasePlans(cfg, testCfg, boolVars)
	if err != nil {
		return nil, err
	}

	return &TestPlan{
		TemplateDir: templateDir,
		BoolVars:    boolVars,
		Cases:       cases,
		Env:         env,
		ProjectName: projectName,
	}, nil
}

func buildCasePlans(cfg Config, testCfg *tmplconfig.TestConfig, boolVars []string) ([]TestCasePlan, error) {
	// Resolve test cases: CLI --run overrides everything, otherwise use template config.
	var templateCases []tmplconfig.TestCase
	if len(cfg.RunCommands) > 0 {
		// CLI override: single anonymous case with the provided commands.
		templateCases = []tmplconfig.TestCase{
			{Name: "default", Commands: cfg.RunCommands},
		}
	} else if testCfg != nil && len(testCfg.Cases) > 0 {
		templateCases = testCfg.Cases

		// Security: require opt-in for template-defined commands.
		if !cfg.AcceptHooks {
			return nil, fmt.Errorf(
				"template defines %d test case(s); pass --accept-hooks to allow or --run to override",
				len(templateCases),
			)
		}
	}

	if len(templateCases) == 0 {
		// No test cases defined — create a single case with no commands
		// so combinations are still generated (scaffold-only validation).
		templateCases = []tmplconfig.TestCase{
			{Name: "default"},
		}
	}

	// Filter by --case if specified.
	if cfg.CaseName != "" {
		var found bool
		for _, tc := range templateCases {
			if tc.Name == cfg.CaseName {
				templateCases = []tmplconfig.TestCase{tc}
				found = true
				break
			}
		}
		if !found {
			names := make([]string, 0, len(templateCases))
			for _, tc := range templateCases {
				names = append(names, tc.Name)
			}
			return nil, fmt.Errorf("test case %q not found; available: %v", cfg.CaseName, names)
		}
	}

	// Build a TestCasePlan for each case.
	var totalCombos int
	var plans []TestCasePlan
	for _, tc := range templateCases {
		// Merge CLI pin vars with case-level filters.
		mergedPins := make(map[string]string, len(cfg.PinVars)+len(tc.Filters))
		maps.Copy(mergedPins, cfg.PinVars)
		for k, v := range tc.Filters {
			mergedPins[k] = strconv.FormatBool(v)
		}

		count := CombinationCount(boolVars, mergedPins)
		totalCombos += count

		combos := GenerateCombinations(boolVars, mergedPins)
		combos, err := FilterCombinations(combos, cfg.Filter)
		if err != nil {
			return nil, err
		}

		plans = append(plans, TestCasePlan{
			Name:     tc.Name,
			Combos:   combos,
			Commands: tc.Commands,
		})
	}

	// Check safety limit against total combinations across all cases.
	if cfg.MaxCases > 0 && totalCombos > cfg.MaxCases {
		return nil, fmt.Errorf(
			"total combination count %d exceeds safety limit %d (use --max-cases 0 to override)",
			totalCombos, cfg.MaxCases,
		)
	}

	return plans, nil
}

// Execute runs the test plan with the given configuration and returns the report.
func Execute(ctx context.Context, plan *TestPlan, cfg Config) (Report, error) {
	parallel := cfg.Parallel
	if parallel <= 0 {
		parallel = defaultParallel
	}

	start := time.Now()

	// Count total combinations across all cases.
	totalCombos := 0
	for _, cp := range plan.Cases {
		totalCombos += len(cp.Combos)
	}

	report := Report{
		TemplateDir: plan.TemplateDir,
	}

	// Execute each test case.
	for _, cp := range plan.Cases {
		if ctx.Err() != nil {
			break
		}

		p := min(parallel, len(cp.Combos))
		if p <= 0 {
			continue
		}

		results := runWorkerPool(ctx, plan, cfg, cp, p)
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

		if cfg.FailFast && (report.Failed > 0 || report.Errored > 0) {
			break
		}
	}

	report.TotalCases = len(report.Cases)
	report.Duration = time.Since(start)

	return report, nil //nolint:nilerr // ctx.Err above is for early loop exit, not a returned error
}

func runWorkerPool(ctx context.Context, plan *TestPlan, cfg Config, cp TestCasePlan, parallel int) []CaseResult {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	comboCh := make(chan Combination, len(cp.Combos))
	for _, c := range cp.Combos {
		comboCh <- c
	}
	close(comboCh)

	resultCh := make(chan CaseResult, len(cp.Combos))

	var wg sync.WaitGroup
	for range parallel {
		wg.Go(func() {
			for combo := range comboCh {
				if ctx.Err() != nil {
					return
				}
				result := runSingleTest(ctx, plan, cfg, cp, combo)
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

	results := make([]CaseResult, 0, len(cp.Combos))
	for r := range resultCh {
		results = append(results, r)
	}
	return results
}

func runSingleTest(ctx context.Context, plan *TestPlan, cfg Config, cp TestCasePlan, combo Combination) CaseResult {
	start := time.Now()

	// Create isolated temp directory for this combination.
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("tag-test-%d-*", combo.Index))
	if err != nil {
		return CaseResult{
			CaseName:    cp.Name,
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
			CaseName:    cp.Name,
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
			CaseName:    cp.Name,
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

	return runValidation(ctx, plan, cfg, cp, combo, projectDir, tmpDir, &shouldClean, start)
}

func runValidation(
	ctx context.Context,
	plan *TestPlan,
	cfg Config,
	cp TestCasePlan,
	combo Combination,
	outputDir string,
	tmpDir string,
	shouldClean *bool,
	start time.Time,
) CaseResult {
	if len(cp.Commands) == 0 {
		return CaseResult{
			CaseName:    cp.Name,
			Combination: combo,
			Status:      CasePassed,
			Duration:    time.Since(start),
		}
	}

	vResult := RunValidationCommands(ctx, outputDir, cp.Commands, plan.Env, cfg.Timeout)
	if vResult == nil {
		return CaseResult{
			CaseName:    cp.Name,
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
		CaseName:    cp.Name,
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
