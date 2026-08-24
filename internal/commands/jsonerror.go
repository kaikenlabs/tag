package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/jsonout"
	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/pkg/app"
)

const (
	codeInvalidReference        = "invalid_reference"
	codeTemplateNotFound        = "template_not_found"
	codeAuthRequired            = "auth_required"
	codeVersionNotFound         = "version_not_found"
	codeRequiredVariableMissing = "required_variable_missing"
	codeOutputExists            = "output_exists"
	codeCircularDependency      = "circular_dependency"
	codeUsage                   = "usage"
	codeInternal                = "internal"
)

type errorDoc struct {
	SchemaVersion int         `json:"schema_version"`
	TagVersion    string      `json:"tag_version"`
	Error         errorDetail `json:"error"`
}

type errorDetail struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	ExitCode int    `json:"exit_code"`
}

func errorCode(err error) string {
	if errors.Is(err, remote.ErrAuthRequired) {
		return codeAuthRequired
	}
	if errors.Is(err, remote.ErrVersionNotFound) {
		return codeVersionNotFound
	}
	if errors.Is(err, remote.ErrNotFound) ||
		errors.Is(err, remote.ErrSubPathNotFound) ||
		errors.Is(err, scaffold.ErrTemplateNotFound) ||
		errors.Is(err, scaffold.ErrConfigNotFound) ||
		errors.Is(err, library.ErrTemplateNotFound) {
		return codeTemplateNotFound
	}
	if errors.Is(err, scaffold.ErrRequiredVariableMissing) {
		return codeRequiredVariableMissing
	}
	if errors.Is(err, scaffold.ErrOutputExists) {
		return codeOutputExists
	}
	if errors.Is(err, scaffold.ErrCircularDependency) {
		return codeCircularDependency
	}
	var parseErr *remote.ParseError
	if errors.As(err, &parseErr) {
		return codeInvalidReference
	}
	if exitCodeOf(err) == app.ExitUsage {
		return codeUsage
	}
	return codeInternal
}

func exitCodeOf(err error) int {
	var cmdErr *app.CommandError
	if errors.As(err, &cmdErr) {
		return cmdErr.ExitCode()
	}
	return app.ExitGeneral
}

type stdoutLatch struct {
	w       io.Writer
	written bool
}

func (l *stdoutLatch) Write(p []byte) (int, error) {
	l.written = true
	return l.w.Write(p)
}

// reportedError MUST implement Unwrap: without it, main.go's
// errors.As(err, &cmdErr) stops finding *app.CommandError through the
// wrapper and every exit code silently collapses to 1.
type reportedError struct{ err error }

func (e reportedError) Error() string { return e.err.Error() }
func (e reportedError) Unwrap() error { return e.err }

func ErrorAlreadyReported(err error) bool {
	var r reportedError
	return errors.As(err, &r)
}

func withJSONErrorDoc(c *cli.Context, schemaVersion int, version string, fn func() error) error {
	var latch *stdoutLatch
	if c.App != nil && c.App.Writer != nil {
		latch = &stdoutLatch{w: c.App.Writer}
		c.App.Writer = latch
		defer func() { c.App.Writer = latch.w }()
	}

	err := fn()
	if err == nil {
		return nil
	}

	format, ferr := resolveFormat(c, formatText, formatJSON)
	if ferr != nil || format != formatJSON {
		return err
	}

	if latch != nil && latch.written {
		return err
	}

	if !writeErrorDoc(c, schemaVersion, version, errorCode(err), err.Error(), exitCodeOf(err)) {
		return err
	}
	return reportedError{err}
}

func writeErrorDoc(c *cli.Context, schemaVersion int, version, code, message string, exitCode int) bool {
	doc := errorDoc{
		SchemaVersion: schemaVersion,
		TagVersion:    version,
		Error: errorDetail{
			Code:     code,
			Message:  message,
			ExitCode: exitCode,
		},
	}
	if writeErr := jsonout.Write(cmdOut(c), doc); writeErr != nil {
		return false
	}
	fmt.Fprintln(cmdErr(c), "tag error: "+message)
	return true
}

// jsonUsageErrorHandler closes the parse-time hole: urfave/cli fails BEFORE
// Action runs for something like `tag template info --format json --bogus`,
// so withJSONErrorDoc (which wraps Action) never gets a chance to see the
// error. This is the OnUsageError callback that does the equivalent job for
// a flag-parse failure.
func jsonUsageErrorHandler(schemaVersion int, version string) func(*cli.Context, error, bool) error {
	return func(c *cli.Context, err error, _ bool) error {
		if jsonRequestedInArgs(c) {
			writeErrorDoc(c, schemaVersion, version, codeUsage, err.Error(), exitCodeOf(err))
			return reportedError{err}
		}

		fmt.Fprintf(c.App.Writer, "%s %s\n\n", "Incorrect Usage:", err.Error())
		if lineage := c.Lineage(); len(lineage) > 1 {
			_ = cli.ShowCommandHelp(lineage[1], c.Command.Name)
		}
		return err
	}
}

// jsonRequestedInArgs answers "was --format json asked for?" without consulting
// the failing command's own context, which cannot answer it: Command.parseFlags
// returns a nil *flag.FlagSet on any parse error, so c.Args() panics outright
// and c.String("format") reads back empty no matter where --format appeared in
// argv. The parent context's argv is intact, so the scan runs there.
//
// A miss falls back to text, which is exactly today's behaviour, so this is
// fail-safe by construction rather than fail-wrong. Last occurrence wins,
// matching what the flag package would have done had parsing succeeded.
func jsonRequestedInArgs(c *cli.Context) bool {
	lineage := c.Lineage()
	if len(lineage) < 2 {
		return false
	}

	args := lineage[1].Args().Slice()
	requested := false
	for i, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		if name != "--format" && name != "-format" {
			continue
		}
		if hasValue {
			requested = value == formatJSON
			continue
		}
		requested = i+1 < len(args) && args[i+1] == formatJSON
	}
	return requested
}
