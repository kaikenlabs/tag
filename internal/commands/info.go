package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/charmbracelet/glamour"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/jsonout"
	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
	varscan "github.com/kaikenlabs/tag/internal/vars"
	"github.com/kaikenlabs/tag/pkg/app"
)

func templateInfoFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "Force refresh of cached remote templates",
		},
		formatFlag(formatText, formatJSON),
	}
}

// InfoCommand returns the info command definition.
func templateInfoCommand(version string) *cli.Command {
	return &cli.Command{
		Name:      "info",
		Usage:     "Show information about a template without scaffolding",
		ArgsUsage: "<template>",
		Description: `Display metadata, variables, hooks, and documentation for a template.

Accepts all template reference formats: local paths, library names,
and remote references (gh:, gl:, bb:, Git URLs, zip URLs).

Library templates are resolved first by name, then as remote/local references.

Examples:
  # Info for a local template
  tag template info ./my-template

  # Info for an installed library template
  tag template info go-api

  # Info for a remote template
  tag template info gh:user/awesome-template

  # Force refresh of cached remote template
  tag template info gh:user/awesome-template --update`,
		Flags: templateInfoFlags(),
		Action: func(c *cli.Context) error {
			return withJSONErrorDoc(c, infoSchemaVersion, version, func() error { return infoAction(c, version) })
		},
		OnUsageError: jsonUsageErrorHandler(infoSchemaVersion, version),
		BashComplete: completeLibraryTemplateNames,
	}
}

func infoAction(c *cli.Context, version string) error {
	args, err := reparseTrailingFlags(c, templateInfoFlags())
	if err != nil {
		return app.UsageErrorf("%s", err)
	}
	if len(args) < 1 {
		return app.UsageErrorf("template reference is required\n\nUsage: tag template info <template>")
	}
	if len(args) > 1 {
		return app.UsageErrorf("expected exactly one template argument, got %d", len(args))
	}

	format, err := resolveFormat(c, formatText, formatJSON)
	if err != nil {
		return err
	}

	ref := args[0]

	templateDir, resolvedCommit, err := resolveTemplateDir(c, ref)
	if err != nil {
		return err
	}

	out := cmdOut(c)

	if format == formatJSON {
		config, loadErr := loadTemplateConfig(templateDir)
		if loadErr != nil {
			return loadErr
		}
		dto := buildTemplateInfoJSON(config,
			docFileHasContent(templateDir, types.TemplateReadme),
			docFileHasContent(templateDir, types.TemplateHowto),
			resolvedCommit, version)
		if writeErr := jsonout.Write(out, dto); writeErr != nil {
			return app.Errorf("write json: %w", writeErr)
		}
		return nil
	}

	return displayTemplateInfo(out, templateDir)
}

// resolveTemplateDir resolves a template reference to a directory path and the
// commit SHA it resolved to.
//
// It tries library lookup first, then falls back to local/remote resolution.
// The SHA is empty for anything that is not a git-backed remote fetch or cache
// hit: a library entry (resolved by name, never through the resolver), a local
// directory, and a zip. Callers surface it as an always-present, possibly-empty
// field rather than omitting it, so a consumer can tell "local template" from
// "field not supported by this binary".
func resolveTemplateDir(c *cli.Context, ref string) (dir, commitSHA string, err error) {
	// Try library first
	lib, libNewErr := newLocalLibrary()
	if libNewErr == nil {
		if templateDir, libErr := lib.TemplatePath(ref); libErr == nil {
			return templateDir, "", nil
		}
	}

	// Fall back to local/remote resolution
	resolver, err := remote.NewResolver()
	if err != nil {
		return "", "", app.Errorf("failed to create resolver: %w", err)
	}

	resolveResult, err := resolver.Resolve(c.Context, ref, remote.ResolveOptions{
		ForceUpdate: c.Bool("update"),
	})
	if err != nil {
		return "", "", app.Errorf("failed to resolve template %q: %w", ref, err)
	}

	return resolveResult.Path, resolveResult.CommitSHA, nil
}

// displayTemplateInfo orchestrates the display of all template information sections.
func displayTemplateInfo(w io.Writer, templateDir string) error {
	config, err := loadTemplateConfig(templateDir)
	if err != nil {
		return err
	}

	displayMetadata(w, config)
	displayVariables(w, config)
	displayHooks(w, config)
	renderDocFile(w, templateDir, types.TemplateReadme, "README")
	renderDocFile(w, templateDir, types.TemplateHowto, "HOWTO")

	return nil
}

// loadTemplateConfig reads and parses templateDir's tag.template.json. It is
// the single loader shared by the text path (displayTemplateInfo) and the
// --format json path (infoAction), so the two formats cannot drift on what
// counts as a missing or invalid template.
func loadTemplateConfig(templateDir string) (*scaffold.TemplateConfig, error) {
	configPath := filepath.Join(templateDir, types.TemplateConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, app.Errorf("not a TAG template: %w in %s", scaffold.ErrConfigNotFound, templateDir)
		}
		return nil, app.Errorf("failed to read %s: %w", types.TemplateConfigFile, err)
	}

	config, err := scaffold.ParseTemplateConfig(data)
	if err != nil {
		return nil, app.Errorf("failed to parse %s: %w", types.TemplateConfigFile, err)
	}

	return config, nil
}

// displayMetadata prints the template header: name, version, description.
func displayMetadata(w io.Writer, config *scaffold.TemplateConfig) {
	if config.Name != "" {
		fmt.Fprintf(w, "Name:         %s\n", config.Name)
	}
	if config.Version != "" {
		fmt.Fprintf(w, "Version:      %s\n", config.Version)
	}
	if config.Description != "" {
		fmt.Fprintf(w, "Description:  %s\n", config.Description)
	}
}

// displayVariables prints the sorted list of template variables with their
// types, defaults and declared prompts.
func displayVariables(w io.Writer, config *scaffold.TemplateConfig) {
	if len(config.Vars) == 0 {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Variables:")

	names := make([]string, 0, len(config.Vars))
	for name := range config.Vars {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		v := config.Vars[name]
		var detail string
		switch {
		case v.Type == scaffold.VarTypeChoice:
			detail = fmt.Sprintf("(choice: %s)", joinOptions(v.Options))
		case v.Type != "" && v.Type != scaffold.VarTypeString:
			detail = fmt.Sprintf("(%s)", v.Type)
		case v.Default != nil:
			detail = fmt.Sprintf("= %v", v.Default)
		default:
			detail = "(string)"
		}
		// v.Prompt, never GetPrompt: its synthesised "Enter value for <name>"
		// would annotate every prompt-less variable with nothing the reader
		// cannot already see. Trailing rather than a padded column because the
		// choice branch is unbounded — one long options list would push every
		// prompt off screen, including those of short variables.
		if v.Prompt != "" {
			detail += "  — " + v.Prompt
		}
		fmt.Fprintf(w, "  %-20s %s\n", name, detail)
	}
}

// displayHooks prints pre/post scaffold hooks if present.
func displayHooks(w io.Writer, config *scaffold.TemplateConfig) {
	if config.Hooks == nil {
		return
	}

	hasHooks := len(config.Hooks.PreScaffold) > 0 || len(config.Hooks.PostScaffold) > 0
	if !hasHooks {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Hooks:")
	if len(config.Hooks.PreScaffold) > 0 {
		fmt.Fprintln(w, "  pre_scaffold:")
		for _, cmd := range config.Hooks.PreScaffold {
			fmt.Fprintf(w, "    - %s\n", cmd)
		}
	}
	if len(config.Hooks.PostScaffold) > 0 {
		fmt.Fprintln(w, "  post_scaffold:")
		for _, cmd := range config.Hooks.PostScaffold {
			fmt.Fprintf(w, "    - %s\n", cmd)
		}
	}
}

// renderDocFile renders a markdown documentation file (README.md, HOWTO.md) with glamour.
// Silently skips if the file does not exist.
func renderDocFile(w io.Writer, templateDir, filename, label string) {
	path := filepath.Join(templateDir, filename)
	content, err := os.ReadFile(path)
	if err != nil || len(content) == 0 {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "--- %s ---\n", label)

	rendered, err := renderMarkdown(string(content))
	if err != nil {
		// Fallback: print raw markdown
		fmt.Fprintln(w, string(content))
		return
	}
	fmt.Fprint(w, rendered)
}

// renderMarkdown renders markdown to the terminal via glamour. It is a
// package-level variable — same pattern as newGitResolver/newLocalLibrary —
// so tests can substitute a spy. That is the only way to assert the
// --format json path never invokes glamour: glamour is near-inert under
// `go test` (no TTY), so an output-based assertion would pass even when the
// code path is wrong.
var renderMarkdown = func(s string) (string, error) {
	return glamour.Render(s, "auto")
}

// docFileHasContent reports whether templateDir/filename exists and is
// non-empty. This mirrors renderDocFile's own "exists and non-empty" check
// exactly (see TestUT_BuildTemplateInfoJSON_DocFlagsMatchTextRendering), kept
// as a separate function rather than a refactor of renderDocFile so the text
// printer, which is golden-fixture pinned, does not change shape.
func docFileHasContent(templateDir, filename string) bool {
	content, err := os.ReadFile(filepath.Join(templateDir, filename))
	return err == nil && len(content) > 0
}

// joinOptions formats choice options for display.
func joinOptions(opts []string) string {
	if len(opts) <= 3 {
		return fmt.Sprintf("%v", opts)
	}
	return fmt.Sprintf("%v +%d more", opts[:3], len(opts)-3)
}

// templateInfoJSON is the --format json shape for `tag template info`.
//
// It is a separate DTO rather than a direct marshal of scaffold.TemplateConfig
// because TemplateConfig.Vars is tagged `json:"-"` (it is populated only after
// ParseTemplateConfig resolves each variable's short/long form) while RawVars
// carries `json:"vars"` — a direct marshal would therefore emit the raw,
// unresolved vars structure instead of the resolved one. Likewise, template
// docs are rendered to ANSI escape codes by glamour for the text path, which
// is never valid JSON output. Do not "simplify" this back to json tags on the
// domain types; see buildTemplateInfoJSON.
type templateInfoJSON struct {
	// SchemaVersion and TagVersion let a pinned consumer detect this binary's
	// contract without a second process spawn. This is NOT the {"ok","data"}
	// envelope jsonout rejects — see that package's doc.
	SchemaVersion int    `json:"schema_version"`
	TagVersion    string `json:"tag_version"`

	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Version     string                     `json:"version"`
	Keywords    []string                   `json:"keywords"`
	Categories  []string                   `json:"categories"`
	Variables   []templateInfoVariableJSON `json:"variables"`
	Hooks       templateInfoHooksJSON      `json:"hooks"`
	HasReadme   bool                       `json:"has_readme"`
	HasHowto    bool                       `json:"has_howto"`

	// ResolvedCommit is the git SHA the reference resolved to, or "" for a
	// library, local or zip template. Never omitempty: absent means "this
	// binary does not emit it", which is not the same claim as "no SHA".
	ResolvedCommit string `json:"resolved_commit"`
}

// infoSchemaVersion is the contract version of the `template info` document.
// Bump it only for a breaking change — a removed or renamed key, a changed
// type, or a changed meaning. Adding a key is not breaking. It is deliberately
// separate from scaffoldSchemaVersion so one document can bump without the
// other; see docs/reference/json-contract.md.
const infoSchemaVersion = 1

// templateInfoVariableJSON is one entry of templateInfoJSON.Variables.
type templateInfoVariableJSON struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Prompt   string   `json:"prompt,omitempty"`
	Default  any      `json:"default,omitempty"`
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
	Secret   bool     `json:"secret"`

	// DependsOn names the declared variables this variable's default expression
	// references, sorted, [] when there are none. `variables` stays sorted
	// alphabetically; this is what lets a consumer recover the order scaffold
	// actually prompts in without info changing its own sort.
	DependsOn []string `json:"depends_on"`

	// The four form-metadata fields describe scaffold-time behaviour so a form
	// generator can decide whether to render an input. They carry no omitempty:
	// a consumer must be able to tell false from absent.
	Prompted            bool `json:"prompted"`
	Derived             bool `json:"derived"`
	Private             bool `json:"private"`
	DefaultIsExpression bool `json:"default_is_expression"`
}

// templateInfoHooksJSON always carries both keys, as "[]" rather than "null"
// when a phase has no hooks — see buildTemplateInfoJSON.
type templateInfoHooksJSON struct {
	PreScaffold  []string `json:"pre_scaffold"`
	PostScaffold []string `json:"post_scaffold"`
}

// buildTemplateInfoJSON is a pure function: given an already-parsed config and
// the doc presence flags (computed separately via docFileHasContent so this
// stays testable without a filesystem), it returns the --format json DTO.
// Using scaffold.TemplateConfig.Vars (never RawVars) is what guarantees the
// JSON path reports resolved variable types/defaults, not the raw JSON shapes
// a template author wrote.
func buildTemplateInfoJSON(config *scaffold.TemplateConfig, hasReadme, hasHowto bool,
	resolvedCommit, tagVersion string,
) templateInfoJSON {
	names := make([]string, 0, len(config.Vars))
	for name := range config.Vars {
		names = append(names, name)
	}
	slices.Sort(names)

	deps := varscan.DeclaredDeps(config.Vars)

	variables := make([]templateInfoVariableJSON, 0, len(names))
	for _, name := range names {
		v := config.Vars[name]
		private, derived := v.IsPrivate(name), v.IsDerived()
		variables = append(variables, templateInfoVariableJSON{
			Name:                name,
			Type:                string(v.Type),
			Prompt:              v.Prompt,
			Default:             v.Default,
			Required:            v.Required,
			Options:             slices.Clone(v.Options),
			Secret:              v.Secret,
			Prompted:            !private && !derived,
			Derived:             derived,
			Private:             private,
			DefaultIsExpression: derived || v.IsEvaluatedDefault(),
			DependsOn:           deps[name],
		})
	}

	hooks := templateInfoHooksJSON{
		PreScaffold:  []string{},
		PostScaffold: []string{},
	}
	// Cloned, not aliased: the DTO is documented as a pure value, and handing
	// out slices backed by the caller's config makes that false — a mutation
	// through the DTO would reach back into the parsed template config.
	if config.Hooks != nil {
		if len(config.Hooks.PreScaffold) > 0 {
			hooks.PreScaffold = slices.Clone(config.Hooks.PreScaffold)
		}
		if len(config.Hooks.PostScaffold) > 0 {
			hooks.PostScaffold = slices.Clone(config.Hooks.PostScaffold)
		}
	}

	return templateInfoJSON{
		SchemaVersion:  infoSchemaVersion,
		TagVersion:     tagVersion,
		Name:           config.Name,
		Description:    config.Description,
		Version:        config.Version,
		Keywords:       cloneOrEmpty(config.Keywords),
		Categories:     cloneOrEmpty(config.Categories),
		Variables:      variables,
		Hooks:          hooks,
		HasReadme:      hasReadme,
		HasHowto:       hasHowto,
		ResolvedCommit: resolvedCommit,
	}
}

// cloneOrEmpty copies src, returning an empty slice rather than nil for empty
// input. slices.Clone(nil) is nil, and a nil slice marshals to null, not [].
func cloneOrEmpty(src []string) []string {
	if len(src) == 0 {
		return []string{}
	}
	return slices.Clone(src)
}
