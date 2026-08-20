// Package fileaction defines the shared vocabulary for describing what TAG
// did to a file. It is deliberately dependency-free.
//
// This vocabulary is shared by internal/engine (generate), internal/scaffold
// (scaffold) and internal/history (undo) wherever they need to describe
// "what happened to this file". Keeping a single set of constants means the
// three do not drift into incompatible per-package notions of a file write.
//
// internal/templateupdate.MergeOp is deliberately NOT folded into this
// vocabulary. A 3-way merge decision (keep/delete/conflict/user-added)
// describes reconciling a file against an updated template baseline, which
// is a genuinely different concept from "TAG wrote this file this way" —
// flattening the two would lose information for no consumer benefit. Do not
// "unify" them in a future refactor.
package fileaction

// Action describes what TAG did to a single file during a generate,
// scaffold, or undo operation.
type Action string

const (
	// ActionCreate means the file did not exist and was written for the
	// first time.
	ActionCreate Action = "create"
	// ActionInject means content was inserted into an existing file at a
	// marked location.
	ActionInject Action = "inject"
	// ActionAppend means content was appended to the end of an existing
	// file.
	ActionAppend Action = "append"
	// ActionOverwrite means an existing file was replaced in full.
	ActionOverwrite Action = "overwrite"
	// ActionOpenAPIMerge means an OpenAPI fragment was merged into an
	// existing spec file.
	ActionOpenAPIMerge Action = "openapi-merge"
	// ActionSkip means no write occurred (e.g. an existing file was left
	// untouched under an on-existing=skip policy). ActionSkip is
	// report-only: it is NEVER persisted to .tag/history.json — only the
	// five actions above ever appear on disk, since there is nothing to
	// undo for a file that was never touched.
	ActionSkip Action = "skip"
)
