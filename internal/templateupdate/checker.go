package templateupdate

import (
	"context"
	"errors"
	"fmt"

	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
)

// CheckOptions configures the check operation.
type CheckOptions struct {
	ProjectDir string // Project directory (default: ".")
	Ref        string // Override the template ref to check against
}

// CheckResult contains the outcome of a template freshness check.
type CheckResult struct {
	UpToDate   bool   `json:"up_to_date"`  // true if project matches latest template commit
	CurrentSHA string `json:"current_sha"` // commit SHA stored in .tagconfig.json
	LatestSHA  string `json:"latest_sha"`  // latest commit SHA from remote
	Source     string `json:"source"`      // template source string
}

// Checker checks whether a project's template is up to date with upstream.
type Checker struct {
	resolver remote.LatestCommitResolver
}

// NewChecker creates a Checker with the given commit resolver.
func NewChecker(resolver remote.LatestCommitResolver) *Checker {
	return &Checker{resolver: resolver}
}

// Check reads the project's .tagconfig.json, resolves the latest upstream
// commit, and compares them.
func (c *Checker) Check(ctx context.Context, opts CheckOptions) (*CheckResult, error) {
	projectDir := opts.ProjectDir
	if projectDir == "" {
		projectDir = "."
	}

	cfg, err := scaffold.LoadTagConfig(projectDir)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	if valErr := validateTagConfig(cfg); valErr != nil {
		return nil, valErr
	}

	// Build a reference for resolution.
	ref, err := remote.Parse(cfg.Template.Source)
	if err != nil {
		return nil, fmt.Errorf("parse template source %q: %w", cfg.Template.Source, err)
	}

	// Override ref version if --ref flag provided.
	if opts.Ref != "" {
		ref.Version = opts.Ref
	} else if cfg.Template.Ref != "" {
		ref.Version = cfg.Template.Ref
	}

	latestSHA, err := c.resolver.ResolveLatestCommit(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("resolve latest commit: %w", err)
	}

	return &CheckResult{
		UpToDate:   cfg.Template.CommitSHA == latestSHA,
		CurrentSHA: cfg.Template.CommitSHA,
		LatestSHA:  latestSHA,
		Source:     cfg.Template.Source,
	}, nil
}

// validateTagConfig checks that the tagconfig has the required fields for
// template lifecycle operations.
func validateTagConfig(cfg *scaffold.TagConfig) error {
	if cfg.Template == nil {
		return errors.New("no template metadata in .tagconfig.json — this project may predate template tracking")
	}

	if cfg.Template.Source == "" {
		return errors.New("no template source in .tagconfig.json — cannot determine upstream template")
	}

	if cfg.Template.CommitSHA == "" {
		return errors.New("no commit SHA in .tagconfig.json — this project was scaffolded before update tracking was added; re-scaffold or manually add a 'commit' field")
	}

	return nil
}
