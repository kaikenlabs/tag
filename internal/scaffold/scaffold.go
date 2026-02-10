package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/glamour"

	"github.com/kaikenlabs/tag/internal/replay"
	"github.com/kaikenlabs/tag/internal/schema"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
)

// MaxConfigFileSize is the maximum allowed size for tag.template.json (10 MB).
const MaxConfigFileSize = 10 * 1024 * 1024

// Scaffold orchestrates the scaffolding process.
type Scaffold struct {
	validator  *schema.Validator
	collector  VariableCollector
	processor  PathProcessor
	writer     OutputWriter
	engine     template.TemplateRenderer
	prompter   Prompter
	hookRunner HookRunner // Executes pre/post scaffold hooks
}

// NewScaffold creates a new scaffold instance with default dependencies.
func NewScaffold(opts Options) (*Scaffold, error) {
	// Create template engine
	engine, err := template.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("failed to create template engine: %w", err)
	}

	// Create schema validator
	validator, err := schema.NewValidator()
	if err != nil {
		return nil, fmt.Errorf("failed to create schema validator: %w", err)
	}

	// Create prompter based on TTY status and --no-input flag
	prompter := GetPrompter(opts.NoInput)

	// Create other components
	collector := NewVariableCollector(prompter)
	processor := NewPathProcessor(engine)
	processor.SetAllowRecursiveRender(opts.AllowRecursiveRender)
	writer := NewOutputWriter(engine, processor)

	return &Scaffold{
		validator:  validator,
		collector:  collector,
		processor:  processor,
		writer:     writer,
		engine:     engine,
		prompter:   prompter,
		hookRunner: NewHookRunner(),
	}, nil
}

// Run executes the scaffolding process.
//
//nolint:cyclop,gocognit,funlen // orchestration function coordinates multiple phases
func (s *Scaffold) Run(opts Options) error {
	// Step 1: Validate template directory exists
	if _, err := os.Stat(opts.TemplateDir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", ErrTemplateNotFound, opts.TemplateDir)
	}

	// Step 2: Load and validate tag.template.json
	config, err := s.loadAndValidateConfig(opts.TemplateDir)
	if err != nil {
		return err
	}

	// Capture template version from loaded config (used in .tagconfig.json)
	if config.Version != "" && opts.TemplateVersion == "" {
		opts.TemplateVersion = config.Version
	}

	// Step 2b: Set derived variable names on the processor for SSTI protection.
	if configurable, ok := s.processor.(SSTIConfigurable); ok {
		derivedNames := make(map[string]bool)
		for name, def := range config.Vars {
			if def.IsDerived() || def.IsPrivate(name) {
				derivedNames[name] = true
			}
		}
		configurable.SetDerivedVarNames(derivedNames)
	}

	// Step 3: Make template directory absolute for hooks
	templateDirAbs := opts.TemplateDir
	if !filepath.IsAbs(templateDirAbs) {
		var cwd string
		cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		templateDirAbs = filepath.Join(cwd, templateDirAbs)
	}

	// Step 4: Check hook confirmation (before variable collection so user sees hooks first)
	hooksAllowed, err := ConfirmHooks(config.Hooks, opts.AcceptHooks, opts.NoInput, s.prompter, templateDirAbs)
	if err != nil {
		return fmt.Errorf("hook confirmation failed: %w", err)
	}

	// Step 5: Collect variables
	if opts.ProjectName != "" {
		if opts.Meta == nil {
			opts.Meta = make(map[string]string)
		}
		opts.Meta["project_name"] = opts.ProjectName
	}

	vars, err := s.collector.Collect(config, opts)
	if err != nil {
		return fmt.Errorf("failed to collect variables: %w", err)
	}

	// Step 5b: Resolve derived variables.
	// Derived variables have template expression defaults (e.g., "{{ vars.name | lower }}").
	// These must be evaluated before template rendering so their computed values are used.
	if err := ResolveDerivedVars(s.engine, config, vars); err != nil { //nolint:govet // shadow in if-init is idiomatic
		return fmt.Errorf("failed to resolve derived variables: %w", err)
	}

	// Step 6: Resolve output directory
	outputDir, err := resolveOutputDir(opts.OutputDir, vars)
	if err != nil {
		return err
	}

	// Step 6b: Detect project wrapper to avoid double nesting.
	// Cookiecutter-style templates wrap project files in a directory named after
	// the project (e.g., "{{ vars.project_name }}"). When no explicit --output-dir
	// is set, the output dir is derived from project_name, which would create
	// project_name/project_name nesting. Unwrap by using the wrapper as the
	// effective template root for Write().
	effectiveTemplateDir := opts.TemplateDir
	if opts.OutputDir == "" {
		if wrapperDir := findProjectWrapper(opts.TemplateDir); wrapperDir != "" {
			effectiveTemplateDir = filepath.Join(opts.TemplateDir, wrapperDir)
		}
	}

	// Step 7: Prepare output directory (safety checks, force handling)
	if err := prepareOutputDir(outputDir, opts.Force); err != nil {
		return err
	}

	// Step 8: Build hook environment
	hookEnv := BuildHookEnv(vars, templateDirAbs, outputDir)

	// Step 9: Run pre-scaffold hooks
	if hooksAllowed {
		if err := RunPreScaffoldHooks(s.hookRunner, config.Hooks, templateDirAbs, hookEnv); err != nil {
			return fmt.Errorf("pre-scaffold hook failed: %w", err)
		}
	}

	// Step 10: Create output directory and write files
	if err := os.MkdirAll(outputDir, types.DirMode); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Clean up output directory on failure
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(outputDir)
		}
	}()

	if err := s.writer.Write(effectiveTemplateDir, outputDir, vars); err != nil {
		return fmt.Errorf("failed to process template: %w", err)
	}

	// Step 11: Generate config and run post-scaffold hooks
	// Filter out secret variables before writing to .tagconfig.json
	// (same pattern as saveReplayData — secrets should not be persisted)
	configVars := filterSecrets(vars, config.Vars)

	tagConfigOpts := TagConfigOptions{
		TemplateSource:  opts.TemplateRef,
		TemplateName:    opts.TemplateName,
		TemplateVersion: opts.TemplateVersion,
		Variables:       configVars,
	}
	if err := GenerateTagConfig(outputDir, tagConfigOpts); err != nil {
		return fmt.Errorf("failed to generate tagconfig: %w", err)
	}

	if hooksAllowed {
		// When a project wrapper was unwrapped, hook scripts live in the
		// original template root (not the output dir). Resolve file-based
		// hook commands to absolute paths so they are found correctly while
		// still executing with workDir=outputDir.
		postHooks := config.Hooks
		if effectiveTemplateDir != opts.TemplateDir && config.Hooks != nil {
			postHooks = resolveHookPaths(config.Hooks, templateDirAbs)
		}
		RunPostScaffoldHooks(s.hookRunner, postHooks, outputDir, hookEnv)
	}

	// Step 12: Save replay data and display summary
	saveReplayData(opts, config, vars)
	s.displaySummary(outputDir, templateDirAbs, vars, opts)

	success = true
	return nil
}

// resolveOutputDir determines and returns the absolute output directory path.
func resolveOutputDir(outputDir string, vars map[string]any) (string, error) {
	if outputDir == "" {
		if projectName, ok := vars["project_name"].(string); ok && projectName != "" {
			outputDir = projectName
		} else {
			return "", errors.New("output directory not specified and project_name variable not set")
		}
	}

	if !filepath.IsAbs(outputDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
		outputDir = filepath.Join(cwd, outputDir)
	}

	return outputDir, nil
}

// prepareOutputDir validates and prepares the output directory,
// handling --force safety checks and existing directory removal.
func prepareOutputDir(outputDir string, force bool) error {
	if force {
		if err := validateSafeOutputDir(outputDir); err != nil {
			return fmt.Errorf("refusing to use --force with unsafe path: %w", err)
		}
	}

	if _, err := os.Stat(outputDir); err == nil {
		if !force {
			return fmt.Errorf("%w: %s (use --force to overwrite)", ErrOutputExists, outputDir)
		}
		if err := os.RemoveAll(outputDir); err != nil {
			return fmt.Errorf("failed to remove existing output: %w", err)
		}
	}

	return nil
}

// saveReplayData saves input values for reproducible scaffolding.
func saveReplayData(opts Options, config *TemplateConfig, vars map[string]any) {
	if opts.NoSave || opts.TemplateRef == "" {
		return
	}

	secrets := make(map[string]bool)
	for name, def := range config.Vars {
		if def.Secret {
			secrets[name] = true
		}
	}

	if err := replay.Save(opts.TemplateRef, config.Version, vars, secrets); err != nil {
		fmt.Printf("Warning: failed to save replay data: %v\n", err)
	}
}

// filterSecrets returns a copy of vars with secret variables removed.
func filterSecrets(vars map[string]any, varDefs map[string]VariableDef) map[string]any {
	result := make(map[string]any, len(vars))
	for k, v := range vars {
		if def, ok := varDefs[k]; ok && def.Secret {
			continue
		}
		result[k] = v
	}
	return result
}

// loadAndValidateConfig loads tag.template.json and validates it against the schema.
func (s *Scaffold) loadAndValidateConfig(templateDir string) (*TemplateConfig, error) {
	configPath := filepath.Join(templateDir, types.TemplateConfigFile)

	// Check if config file exists and its size
	info, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Check if this is a Cookiecutter template
			if ccPath, isCookiecutter := IsCookiecutterTemplate(templateDir); isCookiecutter {
				return nil, &CookiecutterDetectedError{CookiecutterPath: ccPath}
			}
			return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, configPath)
		}
		return nil, fmt.Errorf("failed to stat config file: %w", err)
	}

	if info.Size() > MaxConfigFileSize {
		return nil, fmt.Errorf("config file too large: %d bytes (max %d bytes)", info.Size(), MaxConfigFileSize)
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Validate against schema
	if err := s.validator.Validate(data); err != nil { //nolint:govet // shadow in if-init is idiomatic
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Parse config
	config, err := ParseTemplateConfig(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return config, nil
}

// displaySummary prints a summary of the scaffolding operation.
func (s *Scaffold) displaySummary(outputDir, templateDir string, vars map[string]any, opts Options) {
	fmt.Println()
	fmt.Println("Scaffolding complete!")
	fmt.Printf("  Output: %s\n", outputDir)

	// Show key variables
	if projectName, ok := vars["project_name"].(string); ok {
		fmt.Printf("  Project: %s\n", projectName)
	}

	// Show template origin
	if opts.TemplateName != "" {
		version := ""
		if opts.TemplateVersion != "" {
			version = " (" + opts.TemplateVersion + ")"
		}
		fmt.Printf("  Template: %s%s\n", opts.TemplateRef, version)
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", outputDir)

	// Check if the template has generators
	hasGenerators := hasSubdir(templateDir, types.TemplatesDir) || hasSubdir(templateDir, types.GeneratorsDir)

	if hasGenerators && opts.TemplateName != "" {
		fmt.Println("  tag generate list    # see available generators")
	} else if hasGenerators && opts.TemplateName == "" {
		fmt.Println()
		fmt.Printf("  Add to library for generators: tag lib add %s\n", opts.TemplateRef)
	}
	fmt.Println()

	// Display template README if present
	readmePath := filepath.Join(templateDir, types.TemplateReadme)
	if content, err := os.ReadFile(readmePath); err == nil && len(content) > 0 {
		rendered, err := glamour.Render(string(content), "auto")
		if err != nil {
			// Fallback: print raw markdown
			fmt.Println(string(content))
		} else {
			fmt.Print(rendered)
		}
	}
}

// hasSubdir checks if a directory contains a subdirectory with the given name.
func hasSubdir(dir, subdir string) bool {
	info, err := os.Stat(filepath.Join(dir, subdir))
	return err == nil && info.IsDir()
}

// Result contains the result of a scaffolding operation.
type Result struct {
	OutputDir      string
	Variables      map[string]any
	FilesCount     int
	DirsCount      int
	TemplatesCount int
}

// validateSafeOutputDir checks that the output directory is safe for deletion with --force.
// This prevents accidentally deleting critical system directories.
func validateSafeOutputDir(outputDir string) error {
	// Get absolute, cleaned path
	absPath, err := filepath.Abs(filepath.Clean(outputDir))
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Reject root directories
	if absPath == "/" || absPath == filepath.VolumeName(absPath)+string(filepath.Separator) {
		return errors.New("cannot use root directory as output")
	}

	// Reject common dangerous paths
	homeDir, _ := os.UserHomeDir()
	dangerousPaths := []string{
		"/",
		"/usr",
		"/etc",
		"/var",
		"/bin",
		"/sbin",
		"/lib",
		"/opt",
		"/tmp",
		"/home",
		"/root",
		homeDir, // User's home directory
	}

	for _, dangerous := range dangerousPaths {
		if dangerous == "" {
			continue
		}
		absDangerous, err := filepath.Abs(dangerous)
		if err != nil {
			continue
		}
		if absPath == absDangerous {
			return fmt.Errorf("cannot use %s as output directory", dangerous)
		}
	}

	// Reject paths that are too short (e.g., /a, /ab) as they're likely mistakes
	parts := strings.Split(strings.TrimPrefix(absPath, filepath.VolumeName(absPath)), string(filepath.Separator))
	// Filter empty parts
	nonEmpty := make([]string, 0)
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	if len(nonEmpty) < 2 {
		return fmt.Errorf("output path %s is too shallow (must be at least 2 levels deep)", outputDir)
	}

	return nil
}

// findProjectWrapper scans the template root for a project wrapper directory.
// Cookiecutter-style templates wrap all project files in a single directory whose
// name is a template expression (e.g., "{{ vars.project_name }}"). When detected,
// the wrapper directory name is returned so callers can use it as the effective
// template root, avoiding double nesting (e.g., my-service/my-service/).
// Returns empty string if no single wrapper directory is found.
func findProjectWrapper(templateRoot string) string {
	entries, err := os.ReadDir(templateRoot)
	if err != nil {
		return ""
	}

	var wrapper string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.Contains(name, "{{") && strings.Contains(name, "}}") {
			if wrapper != "" {
				// Multiple template-expression directories — don't unwrap
				return ""
			}
			wrapper = name
		}
	}

	return wrapper
}
