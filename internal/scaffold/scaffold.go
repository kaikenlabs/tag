package scaffold

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaikenlabs/tag/internal/history"
	"github.com/kaikenlabs/tag/internal/hooks"
	"github.com/kaikenlabs/tag/internal/replay"
	"github.com/kaikenlabs/tag/internal/schema"
	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
)

// MaxConfigFileSize is the maximum allowed size for tag.template.json (10 MB).
const MaxConfigFileSize = 10 * 1024 * 1024

// dangerousPaths is the set of system paths that should never be used as scaffold output.
var dangerousPaths = []string{
	"/", "/usr", "/etc", "/var", "/bin", "/sbin",
	"/lib", "/opt", "/tmp", "/home", "/root",
}

// Scaffold orchestrates the scaffolding process.
type Scaffold struct {
	validator  *schema.Validator
	collector  VariableCollector
	processor  PathProcessor
	writer     OutputWriter
	engine     template.TemplateRenderer
	prompter   Prompter
	hookRunner hooks.HookRunner  // Executes pre/post scaffold hooks
	output     io.Writer         // Destination for user-facing messages (default: os.Stdout)
	recorder   *history.Recorder // Optional history recorder; nil = no recording
	isTTY      bool              // Whether stdout is an interactive terminal
}

// SetRecorder attaches a history recorder. When set, file operations during
// scaffolding are recorded and a manifest entry is written on success.
func (s *Scaffold) SetRecorder(r *history.Recorder) {
	s.recorder = r
	if w, ok := s.writer.(*DefaultOutputWriter); ok {
		w.SetRecorder(r)
	}
}

// NewScaffold creates a new scaffold instance with default dependencies.

// ScaffoldOption is a functional option for NewScaffold.
type ScaffoldOption func(*Scaffold)

// WithIsTTY overrides the TTY detection for testing. In production, IsTTY()
// is called automatically by NewScaffold.
func WithIsTTY(v bool) ScaffoldOption {
	return func(s *Scaffold) { s.isTTY = v }
}

// WithOutput sets the writer for user-facing messages.
func WithOutput(w io.Writer) ScaffoldOption {
	return func(s *Scaffold) { s.output = w }
}

// WithEngine injects a pre-configured template engine. This allows callers to
// create an engine with specific options (e.g., dialect registry) before
// scaffold construction. If not provided, a default engine is created.
func WithEngine(e *template.Engine) ScaffoldOption {
	return func(s *Scaffold) { s.engine = e }
}

func NewScaffold(opts Options, fopts ...ScaffoldOption) (*Scaffold, error) {
	s := &Scaffold{
		output:     os.Stdout,
		hookRunner: hooks.NewHookRunner(),
		isTTY:      IsTTY(), // auto-detected; WithIsTTY can override for tests
	}

	// Apply functional options (may override engine, validator, isTTY, etc.)
	for _, opt := range fopts {
		opt(s)
	}

	// Create template engine if not injected
	if s.engine == nil {
		e, err := template.NewEngine()
		if err != nil {
			return nil, fmt.Errorf("failed to create template engine: %w", err)
		}
		s.engine = e
	}

	// Create schema validator if not injected
	if s.validator == nil {
		v, err := schema.NewValidator()
		if err != nil {
			return nil, fmt.Errorf("failed to create schema validator: %w", err)
		}
		s.validator = v
	}

	// Create prompter based on TTY status and --no-input flag
	prompter := GetPrompter(opts.NoInput)

	// Build remaining components from the (possibly injected) engine
	collector := NewVariableCollector(prompter, s.output)
	collector.WithEngine(s.engine)
	processor := NewPathProcessor(s.engine)
	processor.SetAllowRecursiveRender(opts.AllowRecursiveRender)
	writer := NewOutputWriter(s.engine, processor)
	writer.SetAllowRecursiveRender(opts.AllowRecursiveRender)
	writer.SetDryRun(opts.DryRun)

	s.collector = collector
	s.processor = processor
	s.writer = writer
	s.prompter = prompter

	return s, nil
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
func (s *Scaffold) Run(opts Options) (ScaffoldResult, error) {
	ctx := &runContext{opts: opts}

	if err := s.loadConfig(ctx); err != nil {
		return ScaffoldResult{}, err
	}
	if err := s.confirmHooks(ctx); err != nil {
		return ScaffoldResult{}, err
	}
	if err := s.collectVars(ctx); err != nil {
		return ScaffoldResult{}, err
	}
	if err := s.planOutput(ctx); err != nil {
		return ScaffoldResult{}, err
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

	// Set derived variable names on the processor and writer for SSTI protection.
	derivedNames := make(map[string]bool)
	for name, def := range config.Vars {
		if def.IsDerived() || def.IsPrivate(name) {
			derivedNames[name] = true
		}
	}
	if configurable, ok := s.processor.(SSTIConfigurable); ok {
		configurable.SetDerivedVarNames(derivedNames)
	}
	if configurable, ok := s.writer.(SSTIConfigurable); ok {
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
		// The positional [project-name] only *defaults* project_name; an explicit
		// -m project_name wins (documented precedence). The positional still drives
		// the output directory — see planOutput.
		if _, ok := ctx.opts.Meta["project_name"]; !ok {
			ctx.opts.Meta["project_name"] = ctx.opts.ProjectName
		}
	}

	vars, err := s.collector.Collect(ctx.config, ctx.opts, s.isTTY)
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
	// Output directory precedence: --output flag, then the positional
	// [project-name], then the project_name variable. The positional must keep
	// driving the output dir even when -m project_name overrides the variable.
	outDirArg := ctx.opts.OutputDir
	if outDirArg == "" {
		outDirArg = ctx.opts.ProjectName
	}
	outputDir, err := resolveOutputDir(outDirArg, ctx.vars, ctx.cwd)
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
// executeScaffold prepares the output directory, runs hooks, writes files, and finalizes.
//
// Every step that mutates the filesystem is gated on !opts.DryRun. A dry run is
// documented (docs/commands/scaffold.md:46,185) as a preview that creates no
// output directory and writes nothing; before #383 only the per-file writes in
// OutputWriter honoured that, so a preview created the output directory, wrote
// .tagconfig.json and the history manifest, executed hooks, and — under
// --force — deleted the user's existing directory.
func (s *Scaffold) executeScaffold(ctx *runContext) (ScaffoldResult, error) {
	dryRun := ctx.opts.DryRun

	// Prepare output directory (safety checks, force handling). In dry-run the
	// validation still runs — a preview whose real run would refuse to start
	// must say so — but the removal does not.
	if err := prepareOutputDir(ctx.outputDir, ctx.opts.Force, dryRun); err != nil {
		return ScaffoldResult{}, err
	}

	// Build hook environment — use projectRoot so TAG_OUTPUT_DIR matches the
	// actual working directory where hooks execute (inside the wrapper when present).
	ctx.hookEnv = hooks.BuildHookEnv(ctx.vars, ctx.templateDirAbs, ctx.projectRoot, os.Stderr)

	// Render and run hooks only when allowed and this is a real run. Rendering
	// is skipped as well as execution: a hook template that fails to render
	// must not abort a preview that would never have executed the hook.
	var renderedHooks *types.HooksConfig
	if ctx.hooksAllowed && !dryRun {
		var err error
		renderedHooks, err = renderHooksConfig(s.engine, ctx.config.Hooks, ctx.vars)
		if err != nil {
			return ScaffoldResult{}, err
		}
		if err := hooks.RunPreScaffoldHooks(s.hookRunner, renderedHooks, ctx.templateDirAbs, ctx.hookEnv, s.output); err != nil {
			return ScaffoldResult{}, fmt.Errorf("pre-scaffold hook failed: %w", err)
		}
	}

	// Create the output directory and register the failure rollback. Both are
	// real-run only: the rollback removes ctx.outputDir wholesale, so
	// registering it during a preview would destroy a pre-existing directory
	// TAG never created as soon as any later step failed.
	success := false
	if !dryRun {
		if err := os.MkdirAll(ctx.outputDir, types.DirMode); err != nil {
			return ScaffoldResult{}, fmt.Errorf("failed to create output directory: %w", err)
		}
		defer func() {
			if !success {
				_ = os.RemoveAll(ctx.outputDir)
			}
		}()
	}

	if err := s.writer.Write(ctx.effectiveTemplateDir, ctx.outputDir, ctx.vars); err != nil {
		return ScaffoldResult{}, fmt.Errorf("failed to process template: %w", err)
	}

	if !dryRun {
		if finalizeErr := s.finalizeRun(ctx, renderedHooks); finalizeErr != nil {
			return ScaffoldResult{}, finalizeErr
		}
	}

	success = true
	return ScaffoldResult{
		OutputDir:   ctx.outputDir,
		ProjectRoot: ctx.projectRoot,
		TemplateDir: ctx.templateDirAbs,
		Vars:        ctx.vars,
		Opts:        ctx.opts,
	}, nil
}

// finalizeRun performs the real-run-only tail of a scaffold: the generated
// config, generator copying, post hooks, replay data and the history manifest.
// It is split out so executeScaffold's dry-run gating stays legible.
func (s *Scaffold) finalizeRun(ctx *runContext, renderedHooks *types.HooksConfig) error {
	// Generate config — filter out secret variables before writing to .tagconfig.json
	configVars := replay.FilterSecrets(ctx.vars, secretKeys(ctx.config.Vars))
	templateType := types.TemplateTypeLocal
	if ctx.opts.IsRemote {
		templateType = types.TemplateTypeRemote
	}
	tagConfigOpts := TagConfigOptions{
		TemplateType:    templateType,
		TemplateSource:  ctx.opts.TemplateRef,
		TemplateName:    ctx.opts.TemplateName,
		TemplateVersion: ctx.opts.TemplateVersion,
		Variables:       configVars,
	}
	if err := GenerateTagConfig(ctx.projectRoot, tagConfigOpts); err != nil {
		return fmt.Errorf("failed to generate tagconfig: %w", err)
	}

	// Copy generators, bundles, and shared templates into the output .tag/ dir
	// when the template is not being added to the library (where generators
	// are resolved from instead).
	if !ctx.opts.SkipGeneratorCopy {
		if err := copyGeneratorsToOutput(ctx.templateDirAbs, ctx.projectRoot); err != nil {
			return fmt.Errorf("failed to copy generators: %w", err)
		}
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

	// Save replay data
	saveReplayData(s.output, ctx.opts, ctx.config, ctx.vars)

	// Write history manifest entry to the output project's .tag/ directory.
	if s.recorder != nil {
		gen := s.recorder.Build(ctx.opts.TemplateName, "scaffold")
		tagDir := filepath.Join(ctx.projectRoot, types.TemplatesDir)
		if appendErr := history.Append(tagDir, gen); appendErr != nil {
			fmt.Fprintf(s.output, "Warning: could not write history manifest: %v\n", appendErr)
		}
	}

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

	// Track whether the original path was absolute (explicit user choice via --output).
	wasAbsolute := filepath.IsAbs(outputDir)

	if !wasAbsolute {
		outputDir = filepath.Join(cwd, outputDir)
	}

	absOutput, err := filepath.Abs(filepath.Clean(outputDir))
	if err != nil {
		return "", fmt.Errorf("invalid output directory: %w", err)
	}

	// Always validate that resolved paths stay within the working directory.
	// For explicitly provided absolute paths, skip containment (user's explicit choice).
	if !wasAbsolute {
		absCwd, err := filepath.Abs(cwd)
		if err != nil {
			return "", fmt.Errorf("invalid working directory: %w", err)
		}

		rel, err := filepath.Rel(absCwd, absOutput)
		if err != nil {
			return "", fmt.Errorf("cannot resolve output directory relative to working directory: %w", err)
		}

		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("output directory %q escapes working directory", outputDir)
		}
	}

	return absOutput, nil
}

// prepareOutputDir validates and prepares the output directory,
// handling --force safety checks and existing directory removal.
// prepareOutputDir validates and prepares the output directory,
// handling --force safety checks and existing directory removal.
//
// Under dryRun the validation is unchanged — a preview whose real run would
// refuse to start has to report that — but the removal is skipped: deleting
// the user's existing directory to preview a scaffold is data loss (#383).
func prepareOutputDir(outputDir string, force, dryRun bool) error {
	if force {
		if err := validateSafeOutputDir(outputDir); err != nil {
			return fmt.Errorf("refusing to use --force with unsafe path: %w", err)
		}
	}

	if _, err := os.Stat(outputDir); err == nil {
		if !force {
			return fmt.Errorf("%w: %s (use --force to overwrite)", ErrOutputExists, outputDir)
		}
		if dryRun {
			return nil
		}
		if err := os.RemoveAll(outputDir); err != nil {
			return fmt.Errorf("failed to remove existing output: %w", err)
		}
	}

	return nil
}

// saveReplayData saves input values for reproducible scaffolding.
func saveReplayData(w io.Writer, opts Options, config *TemplateConfig, vars map[string]any) {
	if opts.NoSave || opts.TemplateRef == "" {
		return
	}

	if err := replay.Save(opts.TemplateRef, config.Version, vars, secretKeys(config.Vars)); err != nil {
		fmt.Fprintf(w, "Warning: failed to save replay data: %v\n", err)
	}
}

// secretKeys returns a map of variable names that are marked as secret.
func secretKeys(varDefs map[string]VariableDef) map[string]bool {
	secrets := make(map[string]bool)
	for name, def := range varDefs {
		if def.Secret {
			secrets[name] = true
		}
	}
	return secrets
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
	dangerous := make([]string, 0, len(dangerousPaths)+1)
	dangerous = append(dangerous, dangerousPaths...)
	dangerous = append(dangerous, homeDir)

	for _, dp := range dangerous {
		if dp == "" {
			continue
		}
		absDangerous, absErr := filepath.Abs(dp)
		if absErr != nil {
			continue
		}
		if absPath == absDangerous {
			return fmt.Errorf("cannot use %s as output directory", dp)
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
