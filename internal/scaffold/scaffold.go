package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaikenlabs/tag/internal/replay"
	"github.com/kaikenlabs/tag/internal/schema"
	"github.com/kaikenlabs/tag/internal/template"
)

// Scaffold orchestrates the scaffolding process.
type Scaffold struct {
	Validator   *schema.Validator
	Collector   VariableCollector
	Processor   PathProcessor
	Writer      OutputWriter
	Engine      *template.Engine
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
	processor, err := NewPathProcessor()
	if err != nil {
		return nil, fmt.Errorf("failed to create path processor: %w", err)
	}
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

	// Step 3: Collect variables
	collectOpts := CollectOptions{
		ValuesFile:  opts.ValuesFile,
		Meta:        opts.Meta,
		NoPrompt:    opts.NoInput,
		IsTTY:       IsTTY(),
		Replay:      opts.Replay,
		TemplateRef: opts.TemplateRef,
	}

	// Add project name to meta if provided
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

	// Step 4: Determine output directory
	outputDir := opts.OutputDir
	if outputDir == "" {
		// Use project_name as output directory
		if projectName, ok := vars["project_name"].(string); ok && projectName != "" {
			outputDir = projectName
		} else {
			return fmt.Errorf("output directory not specified and project_name variable not set")
		}
	}

	// Make output directory absolute
	if !filepath.IsAbs(outputDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		outputDir = filepath.Join(cwd, outputDir)
	}

	// Step 5: Safety check for dangerous paths when using --force
	if opts.Force {
		if err := validateSafeOutputDir(outputDir); err != nil {
			return fmt.Errorf("refusing to use --force with unsafe path: %w", err)
		}
	}

	// Check output directory doesn't exist (unless --force)
	if _, err := os.Stat(outputDir); err == nil {
		if !opts.Force {
			return fmt.Errorf("%w: %s (use --force to overwrite)", ErrOutputExists, outputDir)
		}
		// Remove existing directory
		if err := os.RemoveAll(outputDir); err != nil {
			return fmt.Errorf("failed to remove existing output: %w", err)
		}
	}

	// Make template directory absolute for hooks
	templateDirAbs := opts.TemplateDir
	if !filepath.IsAbs(templateDirAbs) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		templateDirAbs = filepath.Join(cwd, templateDirAbs)
	}

	// Step 6: Build hook environment
	hookEnv := BuildHookEnv(vars, templateDirAbs, outputDir)

	// Step 7: Check hook safety for remote templates
	hooksAllowed := !opts.IsRemote || opts.AllowHooks
	if !hooksAllowed && config.Hooks != nil && (len(config.Hooks.PreScaffold) > 0 || len(config.Hooks.PostScaffold) > 0) {
		fmt.Println("Warning: This remote template defines hooks that have been skipped for security.")
		fmt.Println("  To allow hooks, re-run with --allow-hooks")
	}

	// Step 8: Run pre-scaffold hooks (before creating output directory)
	if hooksAllowed {
		if err := RunPreScaffoldHooks(s.HookRunner, config.Hooks, templateDirAbs, hookEnv); err != nil {
			return fmt.Errorf("pre-scaffold hook failed: %w", err)
		}
	}

	// Step 9: Create output directory
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Step 10: Process template files
	if err := s.Writer.Write(opts.TemplateDir, outputDir, vars); err != nil {
		// Clean up on error
		_ = os.RemoveAll(outputDir)
		return fmt.Errorf("failed to process template: %w", err)
	}

	// Step 11: Copy _generators to .tag.templates
	if err := CopyGenerators(opts.TemplateDir, outputDir); err != nil {
		// Clean up on error
		_ = os.RemoveAll(outputDir)
		return fmt.Errorf("failed to copy generators: %w", err)
	}

	// Step 12: Generate .tagconfig.json
	if err := GenerateTagConfig(outputDir); err != nil {
		return fmt.Errorf("failed to generate tagconfig: %w", err)
	}

	// Step 13a: Run post-scaffold hooks (failures are warnings, not errors)
	if hooksAllowed {
		RunPostScaffoldHooks(s.HookRunner, config.Hooks, outputDir, hookEnv)
	}

	// Step 13b: Save replay data (unless --no-save)
	if !opts.NoSave && opts.TemplateRef != "" {
		// Build secrets map from variable definitions
		secrets := make(map[string]bool)
		for name, def := range config.Vars {
			if def.Secret {
				secrets[name] = true
			}
		}

		// Save replay data
		if err := replay.Save(opts.TemplateRef, config.Version, vars, secrets); err != nil {
			// Don't fail the scaffold for replay save errors, just warn
			fmt.Printf("Warning: failed to save replay data: %v\n", err)
		}
	}

	// Step 14: Display summary
	s.displaySummary(outputDir, vars)

	return nil
}

// loadAndValidateConfig loads tag.template.json and validates it against the schema.
func (s *Scaffold) loadAndValidateConfig(templateDir string) (*TemplateConfig, error) {
	configPath := filepath.Join(templateDir, "tag.template.json")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Check if this is a Cookiecutter template
		if ccPath, isCookiecutter := IsCookiecutterTemplate(templateDir); isCookiecutter {
			return nil, &ErrCookiecutterDetected{CookiecutterPath: ccPath}
		}
		return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, configPath)
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Validate against schema
	if err := s.Validator.Validate(data); err != nil {
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
		return fmt.Errorf("cannot use root directory as output")
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
