package engine

import (
	"io"
	"strings"

	"github.com/kaikenlabs/tag/internal/writer"
)

// OnExistingPolicy controls behavior when a create action targets a file that already exists.
type OnExistingPolicy string

const (
	// OnExistingDefault is the zero-value, treated the same as OnExistingFail.
	OnExistingDefault OnExistingPolicy = ""
	// OnExistingFail causes the generation to fail with an error listing conflicts.
	OnExistingFail OnExistingPolicy = "fail"
	// OnExistingSkip silently skips writing files that already exist.
	OnExistingSkip OnExistingPolicy = "skip"
	// OnExistingOverwrite replaces existing files (with backup via history if available).
	OnExistingOverwrite OnExistingPolicy = "overwrite"
)

// IsValid returns true if the policy is one of the recognised values.
func (p OnExistingPolicy) IsValid() bool {
	switch p {
	case OnExistingDefault, OnExistingFail, OnExistingSkip, OnExistingOverwrite:
		return true
	}
	return false
}

// isFail returns true when the effective policy is "fail" (including the default zero value).
func (p OnExistingPolicy) isFail() bool {
	return p == OnExistingFail || p == OnExistingDefault
}

// GenerateResult summarises the outcome of a generation run.
type GenerateResult struct {
	Created     int            // files newly created
	Skipped     int            // existing files skipped (--on-existing=skip)
	Overwritten int            // existing files overwritten (--on-existing=overwrite)
	Modified    int            // files appended to or injected into
	Details     []FileOpDetail // per-file breakdown, always populated
}

// FileOpDetail records the outcome for a single file.
type FileOpDetail struct {
	Path string // destination path
	Op   string // "created", "skipped", "overwritten", "modified"
}

// ConflictError is returned by Generate when OnExistingFail (the default) is in
// effect and one or more target files already exist.
type ConflictError struct {
	Files []string
}

func (e *ConflictError) Error() string {
	return "the following files already exist (use --on-existing=overwrite or --on-existing=skip):\n  - " +
		strings.Join(e.Files, "\n  - ")
}

type Core struct {
	parser TemplateParser
	fwr    writer.FileWriter
	out    io.Writer
}

type Data struct {
	Name         string
	RawMeta      []string
	ScaffoldVars map[string]any   // Variables from scaffold-time .tagconfig.json
	OnExisting   OnExistingPolicy // behaviour when a create action targets an existing file
}

// GeneratorRef is a reference to a generator by name within a bundle configuration.
type GeneratorRef struct {
	Name string `json:"name" yaml:"name"`
}

type Bundle struct {
	Name          string         `json:"name" yaml:"name"`
	Description   string         `json:"description,omitempty" yaml:"description,omitempty"`
	SelfContained bool           `json:"self_contained,omitempty" yaml:"self_contained,omitempty"`
	Generators    []GeneratorRef `json:"generators" yaml:"generators"`
}
