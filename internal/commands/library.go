package commands

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/google/shlex"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/jsonout"
	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/search"
	"github.com/kaikenlabs/tag/internal/xdg"
	"github.com/kaikenlabs/tag/pkg/app"
)

// LibCommand returns the lib command definition with subcommands.
func LibCommand() *cli.Command {
	return &cli.Command{
		Name:    "lib",
		Aliases: []string{"library"},
		Usage:   "Manage the template library",
		Description: `Install, list, and manage project templates in a persistent library.

Templates added to the library are stored locally and can be used with 'tag scaffold'.
Cookiecutter templates are auto-detected and converted to TAG format on add.

EXAMPLES:
  # Add a template from GitHub
  tag lib add gh:user/go-api-template

  # Add with a custom name
  tag lib add gh:user/cookiecutter-django --as django

  # List installed templates
  tag lib ls

  # Update a template from its original source
  tag lib update go-api

  # Remove a template
  tag lib rm go-api`,
		Subcommands: []*cli.Command{
			libSearchCommand(),
			libAddCommand(),
			libListCommand(),
			libRemoveCommand(),
			libUpdateCommand(),
			libEditCommand(),
		},
	}
}

func newLibrary() (*library.Library, error) {
	dataDir, err := xdg.DataHome()
	if err != nil {
		return nil, fmt.Errorf("failed to determine data directory: %w", err)
	}
	return library.New(dataDir)
}

// newLocalLibrary creates a library without a resolver (for ls, inspect, edit, rm).
// It is a package-level variable to allow test substitution.
var newLocalLibrary = defaultNewLocalLibrary

func defaultNewLocalLibrary() (*library.Library, error) {
	dataDir, err := xdg.DataHome()
	if err != nil {
		return nil, fmt.Errorf("failed to determine data directory: %w", err)
	}
	return library.NewLocal(dataDir), nil
}

func libAddCommand() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Add a template to the library",
		ArgsUsage: "<ref>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "as",
				Usage: "Override the template name in the library",
			},
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Overwrite existing template with same name",
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return app.UsageErrorf("template reference is required\n\nUsage: tag lib add <ref>")
			}

			lib, err := newLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			result, err := lib.Add(c.Context, library.AddOptions{
				Ref:   c.Args().Get(0),
				Name:  c.String("as"),
				Force: c.Bool("force"),
			})
			if err != nil {
				return asAppError(err)
			}

			printAddResult(c.App.Writer, result)
			return nil
		},
	}
}

func printAddResult(w io.Writer, result *library.AddResult) {
	action := "Added"
	if result.IsUpdate {
		action = "Updated"
	}

	fmt.Fprintf(w, "%s template %q\n", action, result.Name)
	fmt.Fprintf(w, "  Source: %s\n", result.Source)
	fmt.Fprintf(w, "  Path:   %s\n", result.TemplateDir)

	if result.ConvertedFrom != "" {
		fmt.Fprintf(w, "  Converted from: %s\n", result.ConvertedFrom)
	}

	if len(result.Warnings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Warnings:")
		for _, warn := range result.Warnings {
			fmt.Fprintf(w, "  - %s\n", warn)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Run with: tag scaffold %s\n", result.Name)
}

func libListFlags() []cli.Flag {
	return []cli.Flag{formatFlag(formatText, formatJSON)}
}

func libListCommand() *cli.Command {
	return &cli.Command{
		Name:    "ls",
		Aliases: []string{"list"},
		Usage:   "List installed templates",
		Flags:   libListFlags(),
		Action: func(c *cli.Context) error {
			format, err := resolveFormat(c, formatText, formatJSON)
			if err != nil {
				return err
			}

			lib, err := newLocalLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			entries, err := lib.List()
			if err != nil {
				return asAppError(err)
			}

			out := cmdOut(c)
			if format == formatJSON {
				return jsonout.Write(out, map[string]any{"templates": entries})
			}

			if len(entries) == 0 {
				fmt.Fprintln(out, "No templates installed.")
				fmt.Fprintln(out)
				fmt.Fprintln(out, "Add one with: tag lib add <ref>")
				return nil
			}

			printLibEntries(out, entries)
			return nil
		},
	}
}

// printLibEntries renders the installed-templates table.
func printLibEntries(w io.Writer, entries []*library.Entry) {
	fmt.Fprintf(w, "%-20s %-30s %-10s %s\n", "NAME", "SOURCE", "VERSION", "DESCRIPTION")
	fmt.Fprintf(w, "%-20s %-30s %-10s %s\n", "----", "------", "-------", "-----------")

	for _, entry := range entries {
		version := entry.Version
		if version == "" {
			version = "-"
		}
		desc := truncate(entry.Description, 40)
		fmt.Fprintf(w, "%-20s %-30s %-10s %s\n",
			truncate(entry.Name, 20),
			truncate(entry.Source, 30),
			truncate(version, 10),
			desc,
		)
	}
}

func libRemoveCommand() *cli.Command {
	return &cli.Command{
		Name:         "rm",
		Aliases:      []string{"remove"},
		Usage:        "Remove a template from the library",
		ArgsUsage:    "<name>",
		BashComplete: completeLibraryTemplateNames,
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return app.UsageErrorf("template name is required\n\nUsage: tag lib rm <name>")
			}

			lib, err := newLocalLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			name := c.Args().Get(0)
			if err := lib.Remove(name); err != nil {
				return asAppError(err)
			}

			fmt.Fprintf(c.App.Writer, "Removed template %q\n", name)
			return nil
		},
	}
}

func libUpdateCommand() *cli.Command {
	return &cli.Command{
		Name:         "update",
		Usage:        "Update a template (or all templates) from the original source",
		ArgsUsage:    "[name]",
		BashComplete: completeLibraryTemplateNames,
		Action: func(c *cli.Context) error {
			lib, err := newLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			if c.NArg() >= 1 {
				return updateSingleTemplate(c, lib)
			}

			return updateAllTemplates(c, lib)
		},
	}
}

func updateSingleTemplate(c *cli.Context, lib *library.Library) error {
	name := c.Args().Get(0)
	result, err := lib.Update(c.Context, name)
	if err != nil {
		return asAppError(err)
	}
	printAddResult(c.App.Writer, result)
	return nil
}

func updateAllTemplates(c *cli.Context, lib *library.Library) error {
	results, err := lib.UpdateAll(c.Context)

	// Print successfully updated templates even on partial failure
	if len(results) > 0 {
		fmt.Fprintf(c.App.Writer, "Updated %d template(s)\n", len(results))
		for _, r := range results {
			fmt.Fprintf(c.App.Writer, "  - %s\n", r.Name)
		}
	}

	if err != nil {
		return asAppError(err)
	}

	return nil
}

// asAppError wraps library errors as CommandErrors without adding redundant context.
// LibraryError already contains operation and name context, so we pass its message through directly.
func asAppError(err error) error {
	var libErr *library.LibraryError
	if errors.As(err, &libErr) {
		return app.Errorf("%s", libErr.Error())
	}
	return app.Errorf("%w", err)
}

func libEditCommand() *cli.Command {
	return &cli.Command{
		Name:         "edit",
		Usage:        "Open a template in your editor",
		ArgsUsage:    "<name>",
		BashComplete: completeLibraryTemplateNames,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "editor",
				Usage: "Editor command to use (e.g. 'code', 'vim')",
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return app.UsageErrorf("template name is required\n\nUsage: tag lib edit <name>")
			}

			lib, err := newLocalLibrary()
			if err != nil {
				return app.Errorf("failed to initialize library: %w", err)
			}

			name := c.Args().Get(0)
			templatePath, err := lib.TemplatePath(name)
			if err != nil {
				return asAppError(err)
			}

			editor, err := resolveEditor(c.String("editor"))
			if err != nil {
				return err
			}

			args, err := splitEditorArgs(editor)
			if err != nil {
				return app.Errorf("invalid editor command %q: %w", editor, err)
			}
			if len(args) == 0 {
				return app.Errorf("editor command is empty")
			}
			args = append(args, templatePath)

			cmd := exec.CommandContext(c.Context, args[0], args[1:]...) //nolint:gosec // editor command is user-configured
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				return app.Errorf("editor exited with error: %w", err)
			}
			return nil
		},
	}
}

// editorSource describes where the editor was resolved from, for testability.
type editorSource struct {
	loadConfig func() (*config.GlobalConfig, error)
	saveConfig func(*config.GlobalConfig) error
	getenv     func(string) string
	isTTY      func() bool
	prompt     func() (string, error)
}

// defaultEditorSource returns the production editor source.
func defaultEditorSource() *editorSource {
	return &editorSource{
		loadConfig: config.LoadGlobalConfig,
		saveConfig: config.SaveGlobalConfig,
		getenv:     os.Getenv,
		isTTY:      func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }, //nolint:gosec // G115: Stdin.Fd()→int is safe on all Go-supported platforms
		prompt:     promptEditorInput,
	}
}

var errNoEditor = app.Errorf("no editor configured\n\nSet one with:\n  tag lib edit --editor <cmd> <name>\n  export EDITOR=<cmd>\n  export VISUAL=<cmd>")

// resolveEditor determines which editor to use.
// Resolution order: flag → global config → $VISUAL → $EDITOR → interactive prompt → error.
func resolveEditor(flagValue string) (string, error) {
	return defaultEditorSource().resolve(flagValue)
}

func (s *editorSource) resolve(flagValue string) (string, error) {
	// 1. --editor flag
	if flagValue != "" {
		return flagValue, nil
	}

	// 2. Global TAG config
	cfg, err := s.loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load editor config: %v\n", err)
	} else if cfg.Editor != "" {
		return cfg.Editor, nil
	}

	// 3. $VISUAL
	if v := s.getenv("VISUAL"); v != "" {
		return v, nil
	}

	// 4. $EDITOR
	if v := s.getenv("EDITOR"); v != "" {
		return v, nil
	}

	// 5. Interactive prompt (TTY only)
	if !s.isTTY() {
		return "", errNoEditor
	}

	editor, err := s.prompt()
	if err != nil {
		return "", err
	}
	if editor == "" {
		return "", errNoEditor
	}

	// Save for future use
	s.saveEditorPreference(editor)

	return editor, nil
}

func (s *editorSource) saveEditorPreference(editor string) {
	saveCfg := &config.GlobalConfig{Editor: editor}
	if existing, loadErr := s.loadConfig(); loadErr == nil {
		existing.Editor = editor
		saveCfg = existing
	}
	if saveErr := s.saveConfig(saveCfg); saveErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save editor preference: %v\n", saveErr)
	}
}

// promptEditorInput asks the user to type their preferred editor command.
func promptEditorInput() (string, error) {
	var result string

	if err := huh.NewInput().
		Title("Enter your preferred editor command (e.g. code, vim, nano)").
		Value(&result).
		Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", app.Errorf("cancelled")
		}
		return "", app.Errorf("prompt failed: %w", err)
	}
	return strings.TrimSpace(result), nil
}

// splitEditorArgs splits an editor command string into command and arguments
// using POSIX shell quoting rules. For example, "code --wait" becomes ["code", "--wait"].
func splitEditorArgs(editor string) ([]string, error) {
	return shlex.Split(editor)
}

func libSearchFlags() []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{
			Name:  "limit",
			Usage: "Maximum number of results (1-100)",
			Value: 10,
		},
		&cli.StringFlag{
			Name:  "sort",
			Usage: "Sort by: stars, forks, or updated",
			Value: "stars",
		},
		&cli.StringFlag{
			Name:  "order",
			Usage: "Order: asc or desc",
			Value: "desc",
		},
		formatFlag(formatText, formatJSON),
	}
}

func libSearchCommand() *cli.Command {
	return &cli.Command{
		Name:      "search",
		Usage:     "Search for templates on GitHub",
		ArgsUsage: "[query]",
		Description: `Search GitHub for TAG-compatible templates tagged with the "tag-template" topic.

EXAMPLES:
  # Search all TAG templates (sorted by stars)
  tag lib search

  # Search for Go API templates
  tag lib search go api

  # Search sorted by recently updated
  tag lib search --sort updated

  # Add a found template
  tag lib add gh:<owner>/<repo>`,
		Flags: libSearchFlags(),
		Action: func(c *cli.Context) error {
			args, err := reparseTrailingFlags(c, libSearchFlags())
			if err != nil {
				return app.UsageErrorf("%s", err)
			}

			format, err := resolveFormat(c, formatText, formatJSON)
			if err != nil {
				return err
			}

			query := strings.Join(args, " ")
			return runLibSearch(c, format, query)
		},
	}
}

// searchBaseURL overrides the GitHub API base URL. Empty means the real API;
// tests point it at an httptest server.
var searchBaseURL string

func runLibSearch(c *cli.Context, format, query string) error {
	token := os.Getenv("GITHUB_TOKEN")
	opts := search.Options{
		Limit: c.Int("limit"),
		Sort:  c.String("sort"),
		Order: c.String("order"),
	}

	client := &http.Client{Timeout: 15 * time.Second}
	results, err := search.SearchGitHub(c.Context, client, query, searchBaseURL, token, opts)
	if err != nil {
		return app.Errorf("search failed: %w", err)
	}

	out := cmdOut(c)
	if format == formatJSON {
		return jsonout.Write(out, map[string]any{"results": results})
	}

	if len(results) == 0 {
		fmt.Fprintln(out, "No templates found.")
		fmt.Fprintln(out)
		if strings.TrimSpace(query) != "" {
			fmt.Fprintf(out, "No results for %q. Try a different query or leave it empty to list all templates.\n", query)
		} else {
			fmt.Fprintln(out, `No templates with topic "tag-template" found on GitHub yet.`)
		}
		return nil
	}

	printSearchResults(out, results)
	return nil
}

func printSearchResults(w io.Writer, results []search.SearchResult) {
	fmt.Fprintf(w, "Found %d template(s):\n\n", len(results))
	fmt.Fprintf(w, "%-40s %-6s %-12s %s\n", "REPOSITORY", "STARS", "UPDATED", "DESCRIPTION")
	fmt.Fprintf(w, "%-40s %-6s %-12s %s\n", "----------", "-----", "-------", "-----------")

	for _, r := range results {
		updated := r.UpdatedAt.Format("2006-01-02")
		fmt.Fprintf(w, "%-40s %-6d %-12s %s\n",
			truncate(r.FullName, 40),
			r.Stars,
			updated,
			truncate(r.Description, 40),
		)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "To install a template:")
	fmt.Fprintln(w, "  tag lib add gh:<owner>/<repo>")
}
