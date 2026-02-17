package scaffold

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/glamour"

	"github.com/kaikenlabs/tag/internal/hooks"
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
	hookRunner hooks.HookRunner // Executes pre/post scaffold hooks
	output     io.Writer        // Destination for user-facing messages (default: os.Stdout)
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
		hookRunner: hooks.NewHookRunner(),
		output:     os.Stdout,
	}, nil
}

// runContext holds shared state across scaffolding phases.
type runContext struct {
	opts                 Options
	cwd                  string
	config               *TemplateConfig
	templateDirAbs       string
	hooksAllowed         bool
	vars                 map[string]any
	outputDir            string
	effectiveTemplateDir string
	projectRoot          string // actual project directory (may differ from outputDir when wrapper + explicit --output-dir)
	hookEnv              []string
}

// Run executes the scaffolding process.
func (s *Scaffold) Run(opts Options) error {
	ctx := &runContext{opts: opts}

	if err := s.loadConfig(ctx); err != nil {
		return err
	}
	if err := s.confirmHooks(ctx); err != nil {
		return err
	}
	if err := s.collectVars(ctx); err != nil {
		return err
	}
	if err := s.planOutput(ctx); err != nil {
		return err
	}
	return s.executeScaffold(ctx)
}

// loadConfig validates the template directory, loads the config, and resolves paths.
func (s *Scaffold) loadConfig(ctx *runContext) error {
	// Validate template directory exists
	if _, err := os.Stat(ctx.opts.TemplateDir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", ErrTemplateNotFound, ctx.opts.TemplateDir)
	}

	// Resolve working directory once (reused for template dir and output dir resolution)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	ctx.cwd = cwd

	// Load and validate tag.template.json
	config, err := s.loadAndValidateConfig(ctx.opts.TemplateDir)
	if err != nil {
		return err
	}
	ctx.config = config

	// Capture template version from loaded config (used in .tagconfig.json)
	if config.Version != "" && ctx.opts.TemplateVersion == "" {
		ctx.opts.TemplateVersion = config.Version
	}

	// Set derived variable names on the processor for SSTI protection.
	if configurable, ok := s.processor.(SSTIConfigurable); ok {
		derivedNames := make(map[string]bool)
		for name, def := range config.Vars {
			if def.IsDerived() || def.IsPrivate(name) {
				derivedNames[name] = true
			}
		}
		configurable.SetDerivedVarNames(derivedNames)
	}

	// Make template directory absolute for hooks
	ctx.templateDirAbs = ctx.opts.TemplateDir
	if !filepath.IsAbs(ctx.templateDirAbs) {
		ctx.templateDirAbs = filepath.Join(cwd, ctx.templateDirAbs)
	}

	return nil
}

// confirmHooks checks whether the user accepts hook execution.
func (s *Scaffold) confirmHooks(ctx *runContext) error {
	hooksAllowed, err := hooks.ConfirmHooks(ctx.config.Hooks, ctx.opts.AcceptHooks, ctx.opts.NoInput, s.prompter, ctx.templateDirAbs, s.output)
	if err != nil {
		return fmt.Errorf("hook confirmation failed: %w", err)
	}
	ctx.hooksAllowed = hooksAllowed
	return nil
}

// collectVars gathers template variables from all sources and resolves derived values.
func (s *Scaffold) collectVars(ctx *runContext) error {
	if ctx.opts.ProjectName != "" {
		if ctx.opts.Meta == nil {
			ctx.opts.Meta = make(map[string]string)
		}
		ctx.opts.Meta["project_name"] = ctx.opts.ProjectName
	}

	vars, err := s.collector.Collect(ctx.config, ctx.opts)
	if err != nil {
		return fmt.Errorf("failed to collect variables: %w", err)
	}

	// Resolve derived variables — template expression defaults (e.g., "{{ vars.name | lower }}")
	// must be evaluated before template rendering so their computed values are used.
	if err := ResolveDerivedVars(s.engine, ctx.config, vars); err != nil {
		return fmt.Errorf("failed to resolve derived variables: %w", err)
	}

	ctx.vars = vars
	return nil
}

// planOutput resolves the output directory and detects project wrappers.
func (s *Scaffold) planOutput(ctx *runContext) error {
	outputDir, err := resolveOutputDir(ctx.opts.OutputDir, ctx.vars, ctx.cwd)
	if err != nil {
		return err
	}
	ctx.outputDir = outputDir
	ctx.projectRoot = outputDir // default: project root is the output directory

	// Detect project wrapper to avoid double nesting.
	// Cookiecutter-style templates wrap project files in a directory named after
	// the project (e.g., "{{ vars.project_name }}"). When no explicit --output-dir
	// is set, the output dir is derived from project_name, which would create
	// project_name/project_name nesting. Unwrap by using the wrapper as the
	// effective template root for Write().
	ctx.effectiveTemplateDir = ctx.opts.TemplateDir
	wrapperDir := findProjectWrapper(ctx.opts.TemplateDir)
	if wrapperDir != "" {
		if ctx.opts.OutputDir == "" {
			// No explicit output dir — unwrap to avoid double nesting
			ctx.effectiveTemplateDir = filepath.Join(ctx.opts.TemplateDir, wrapperDir)
		} else {
			// Explicit output dir — wrapper creates a subdirectory inside outputDir.
			// The project root (for .tagconfig.json, hooks workdir) is that subdirectory.
			tmplCtx := template.NewContextBuilder().WithVars(ctx.vars).Build()
			rendered, err := s.engine.ExecuteToString(wrapperDir, tmplCtx)
			if err == nil && rendered != "" {
				ctx.projectRoot = filepath.Join(ctx.outputDir, rendered)
			}
		}
	}

	return nil
}

// executeScaffold prepares the output directory, runs hooks, writes files, and finalizes.
func (s *Scaffold) executeScaffold(ctx *runContext) error {
	// Prepare output directory (safety checks, force handling)
	if err := prepareOutputDir(ctx.outputDir, ctx.opts.Force); err != nil {
		return err
	}

	// Build hook environment — use projectRoot so TAG_OUTPUT_DIR matches the
	// actual working directory where hooks execute (inside the wrapper when present).
	ctx.hookEnv = hooks.BuildHookEnv(ctx.vars, ctx.templateDirAbs, ctx.projectRoot)

	// Render and run hooks only when allowed — avoid failing on invalid hook
	// templates when the user has opted to skip hooks.
	var renderedHooks *types.HooksConfig
	if ctx.hooksAllowed {
		var err error
		renderedHooks, err = renderHooksConfig(s.engine, ctx.config.Hooks, ctx.vars)
		if err != nil {
			return err
		}
		if err := hooks.RunPreScaffoldHooks(s.hookRunner, renderedHooks, ctx.templateDirAbs, ctx.hookEnv, s.output); err != nil {
			return fmt.Errorf("pre-scaffold hook failed: %w", err)
		}
	}

	// Create output directory and write files
	if err := os.MkdirAll(ctx.outputDir, types.DirMode); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Clean up output directory on failure
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(ctx.outputDir)
		}
	}()

	if err := s.writer.Write(ctx.effectiveTemplateDir, ctx.outputDir, ctx.vars); err != nil {
		return fmt.Errorf("failed to process template: %w", err)
	}

	// Generate config — filter out secret variables before writing to .tagconfig.json
	configVars := filterSecrets(ctx.vars, ctx.config.Vars)
	tagConfigOpts := TagConfigOptions{
		TemplateSource:  ctx.opts.TemplateRef,
		TemplateName:    ctx.opts.TemplateName,
		TemplateVersion: ctx.opts.TemplateVersion,
		Variables:       configVars,
	}
	if err := GenerateTagConfig(ctx.projectRoot, tagConfigOpts); err != nil {
		return fmt.Errorf("failed to generate tagconfig: %w", err)
	}

	// Run post-scaffold hooks
	if ctx.hooksAllowed {
		// When a project wrapper was unwrapped, hook scripts live in the
		// original template root (not the output dir). Resolve file-based
		// hook commands to absolute paths so they are found correctly while
		// still executing with workDir=projectRoot.
		postHooks := renderedHooks
		if ctx.effectiveTemplateDir != ctx.opts.TemplateDir && renderedHooks != nil {
			postHooks = hooks.ResolveHookPaths(renderedHooks, ctx.templateDirAbs)
		}
		hooks.RunPostScaffoldHooks(s.hookRunner, postHooks, ctx.projectRoot, ctx.hookEnv, s.output)
	}

	// Save replay data and display summary
	saveReplayData(ctx.opts, ctx.config, ctx.vars)
	s.displaySummary(ctx.outputDir, ctx.templateDirAbs, ctx.vars, ctx.opts)

	success = true
	return nil
}

// resolveOutputDir determines and returns the absolute output directory path.
// The cwd parameter is the caller's cached working directory, avoiding duplicate os.Getwd() calls.
func resolveOutputDir(outputDir string, vars map[string]any, cwd string) (string, error) {
	if outputDir == "" {
		if projectName, ok := vars["project_name"].(string); ok && projectName != "" {
			outputDir = projectName
		} else {
			return "", errors.New("output directory not specified and project_name variable not set")
		}
	}

	if !filepath.IsAbs(outputDir) {
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
	w := s.output
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Scaffolding complete!")
	fmt.Fprintf(w, "  Output: %s\n", outputDir)

	// Show key variables
	if projectName, ok := vars["project_name"].(string); ok {
		fmt.Fprintf(w, "  Project: %s\n", projectName)
	}

	// Show template origin
	if opts.TemplateName != "" {
		version := ""
		if opts.TemplateVersion != "" {
			version = " (" + opts.TemplateVersion + ")"
		}
		fmt.Fprintf(w, "  Template: %s%s\n", opts.TemplateRef, version)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintf(w, "  cd %s\n", outputDir)

	// Check if the template has generators
	hasGenerators := hasSubdir(templateDir, types.TemplatesDir) || hasSubdir(templateDir, types.GeneratorsDir)

	if hasGenerators && opts.TemplateName != "" {
		fmt.Fprintln(w, "  tag generate list    # see available generators")
	} else if hasGenerators && opts.TemplateName == "" {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  Add to library for generators: tag lib add %s\n", opts.TemplateRef)
	}
	fmt.Fprintln(w)

	// Display template README if present
	readmePath := filepath.Join(templateDir, types.TemplateReadme)
	if content, err := os.ReadFile(readmePath); err == nil && len(content) > 0 {
		rendered, err := glamour.Render(string(content), "auto")
		if err != nil {
			// Fallback: print raw markdown
			fmt.Fprintln(w, string(content))
		} else {
			fmt.Fprint(w, rendered)
		}
	}
}

// hasSubdir checks if a directory contains a subdirectory with the given name.
func hasSubdir(dir, subdir string) bool {
	info, err := os.Stat(filepath.Join(dir, subdir))
	return err == nil && info.IsDir()
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

// renderHookCommands renders template expressions in hook command strings.
// Commands without template syntax ({{ }}) are returned unchanged.
func renderHookCommands(engine template.TemplateRenderer, commands []string, vars map[string]any) ([]string, error) {
	if len(commands) == 0 {
		return commands, nil
	}
	ctx := template.NewContextBuilder().WithVars(vars).Build()
	rendered := make([]string, len(commands))
	for i, cmd := range commands {
		if !strings.Contains(cmd, "{{") {
			rendered[i] = cmd
			continue
		}
		result, err := engine.ExecuteToString(cmd, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to render hook command %q: %w", cmd, err)
		}
		rendered[i] = result
	}
	return rendered, nil
}

// renderHooksConfig renders template expressions in all hook commands.
// Returns the input unchanged when hc is nil or has no template expressions.
func renderHooksConfig(engine template.TemplateRenderer, hc *types.HooksConfig, vars map[string]any) (*types.HooksConfig, error) {
	if hc == nil {
		return &types.HooksConfig{}, nil
	}
	pre, err := renderHookCommands(engine, hc.PreScaffold, vars)
	if err != nil {
		return nil, err
	}
	post, err := renderHookCommands(engine, hc.PostScaffold, vars)
	if err != nil {
		return nil, err
	}
	return &types.HooksConfig{
		PreScaffold:  pre,
		PostScaffold: post,
	}, nil
}
