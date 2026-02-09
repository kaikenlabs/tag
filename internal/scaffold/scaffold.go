package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaikenlabs/tag/internal/replay"
	"github.com/kaikenlabs/tag/internal/schema"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
)

// MaxConfigFileSize is the maximum allowed size for tag.template.json (10 MB).
const MaxConfigFileSize = 10 * 1024 * 1024

// Scaffold orchestrates the scaffolding process.
type Scaffold struct {
	Validator   *schema.Validator
	Collector   VariableCollector
	Processor   PathProcessor
	Writer      OutputWriter
	Engine      template.TemplateRenderer
	Prompter    Prompter
	HookRunner  HookRunner // Executes pre/post scaffold hooks
	DryRun      bool
	Verbose     bool
	ProjectName string // Override for project_name variable
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
		Validator:   validator,
		Collector:   collector,
		Processor:   processor,
		Writer:      writer,
		Engine:      engine,
		Prompter:    prompter,
		HookRunner:  NewHookRunner(),
		ProjectName: opts.ProjectName,
	}, nil
}

// Run executes the scaffolding process.
//
//nolint:cyclop // orchestration function coordinates multiple phases
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

	// Step 2b: Set derived variable names on the processor for SSTI protection.
	if processor, ok := s.Processor.(*DefaultPathProcessor); ok {
		derivedNames := make(map[string]bool)
		for name, def := range config.Vars {
			if def.IsDerived() || def.IsPrivate(name) {
				derivedNames[name] = true
			}
		}
		processor.SetDerivedVarNames(derivedNames)
	}

	// Step 3: Collect variables
	collectOpts := opts.CollectOpts()
	if opts.ProjectName != "" {
		if collectOpts.Meta == nil {
			collectOpts.Meta = make(map[string]string)
		}
		collectOpts.Meta["project_name"] = opts.ProjectName
	}

	vars, err := s.Collector.Collect(config, collectOpts)
	if err != nil {
		return fmt.Errorf("failed to collect variables: %w", err)
	}

	// Step 4: Resolve output directory
	outputDir, err := resolveOutputDir(opts.OutputDir, vars)
	if err != nil {
		return err
	}

	// Step 5: Prepare output directory (safety checks, force handling)
	if err := prepareOutputDir(outputDir, opts.Force); err != nil { //nolint:govet // shadow in if-init is idiomatic
		return err
	}

	// Make template directory absolute for hooks
	templateDirAbs := opts.TemplateDir
	if !filepath.IsAbs(templateDirAbs) {
		var cwd string
		cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		templateDirAbs = filepath.Join(cwd, templateDirAbs)
	}

	// Step 6: Build hook environment and check confirmation
	hookEnv := BuildHookEnv(vars, templateDirAbs, outputDir)

	hooksAllowed, err := ConfirmHooks(config.Hooks, opts.AcceptHooks, opts.NoInput, s.Prompter)
	if err != nil {
		return fmt.Errorf("hook confirmation failed: %w", err)
	}

	// Step 7: Run pre-scaffold hooks
	if hooksAllowed {
		if err := RunPreScaffoldHooks(s.HookRunner, config.Hooks, templateDirAbs, hookEnv); err != nil {
			return fmt.Errorf("pre-scaffold hook failed: %w", err)
		}
	}

	// Step 8: Create output directory and write files
	if err := os.MkdirAll(outputDir, types.DirMode); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := s.Writer.Write(opts.TemplateDir, outputDir, vars); err != nil {
		_ = os.RemoveAll(outputDir)
		return fmt.Errorf("failed to process template: %w", err)
	}

	if err := CopyGenerators(opts.TemplateDir, outputDir); err != nil {
		_ = os.RemoveAll(outputDir)
		return fmt.Errorf("failed to copy generators: %w", err)
	}

	// Step 9: Generate config and run post-scaffold hooks
	if err := GenerateTagConfig(outputDir); err != nil {
		return fmt.Errorf("failed to generate tagconfig: %w", err)
	}

	if hooksAllowed {
		RunPostScaffoldHooks(s.HookRunner, config.Hooks, outputDir, hookEnv)
	}

	// Step 10: Save replay data and display summary
	saveReplayData(opts, config, vars)
	s.displaySummary(outputDir, vars)

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
	if err := s.Validator.Validate(data); err != nil { //nolint:govet // shadow in if-init is idiomatic
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
func (s *Scaffold) displaySummary(outputDir string, vars map[string]any) {
	fmt.Println()
	fmt.Println("Scaffolding complete!")
	fmt.Printf("  Output: %s\n", outputDir)

	// Show key variables
	if projectName, ok := vars["project_name"].(string); ok {
		fmt.Printf("  Project: %s\n", projectName)
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", outputDir)
	fmt.Println()
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
