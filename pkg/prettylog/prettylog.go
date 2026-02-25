package prettylog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
)

const (
	LevelFatal = slog.Level(12)

	timeFormat = "[15:04:05.000]"
	reset      = "\033[0m"

	black        = 30
	red          = 31
	green        = 32
	yellow       = 33
	blue         = 34
	magenta      = 35
	cyan         = 36
	lightGray    = 37
	darkGray     = 90
	lightRed     = 91
	lightGreen   = 92
	lightYellow  = 93
	lightBlue    = 94
	lightMagenta = 95
	lightCyan    = 96
	white        = 97
)

type Handler struct {
	h            slog.Handler
	b            *bytes.Buffer
	m            *sync.Mutex
	out          io.Writer
	app          string
	colorEnabled bool
}

func NewHandler(appName string, opts *slog.HandlerOptions) *Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	b := &bytes.Buffer{}
	return &Handler{
		b: b,
		h: slog.NewTextHandler(b, &slog.HandlerOptions{
			Level:       opts.Level,
			AddSource:   opts.AddSource,
			ReplaceAttr: suppressDefaults(opts.ReplaceAttr),
		}),
		out:          os.Stderr,
		app:          appName,
		m:            &sync.Mutex{},
		colorEnabled: shouldColorize(os.Stderr),
	}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.h.Enabled(ctx, level)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{h: h.h.WithAttrs(attrs), b: h.b, m: h.m, out: h.out, app: h.app, colorEnabled: h.colorEnabled}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{h: h.h.WithGroup(name), b: h.b, m: h.m, out: h.out, app: h.app, colorEnabled: h.colorEnabled}
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String() + ":"

	switch r.Level {
	case slog.LevelDebug:
		level = h.colorize(darkGray, level)
	case slog.LevelInfo:
		level = h.colorize(cyan, level)
	case slog.LevelWarn:
		level = h.colorize(lightYellow, level)
	case slog.LevelError:
		level = h.colorize(lightRed, level)
	}

	var attrs strings.Builder
	r.Attrs(func(a slog.Attr) bool {
		if a.Equal(slog.Attr{}) {
			return true
		}

		if attrs.Len() > 0 {
			attrs.WriteString(" ")
		}
		attrs.WriteString(formatAttr(a))
		return true
	})

	fmt.Fprintln(h.writer(),
		h.colorize(lightGray, r.Time.Format(timeFormat)),
		h.colorize(lightCyan, h.app),
		strings.ToLower(level),
		h.colorize(white, r.Message),
		attrs.String(),
	)

	return nil
}

func (h *Handler) writer() io.Writer {
	if h.out != nil {
		return h.out
	}
	return os.Stderr
}

func (h *Handler) colorize(colorCode int, v string) string {
	if !h.colorEnabled {
		return v
	}
	return fmt.Sprintf("\033[%sm%s%s", strconv.Itoa(colorCode), v, reset)
}

// shouldColorize checks if the given file supports color output.
func shouldColorize(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if f == nil {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

func suppressDefaults(next func([]string, slog.Attr) slog.Attr) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey ||
			a.Key == slog.LevelKey ||
			a.Key == slog.MessageKey {
			return slog.Attr{}
		}
		if next == nil {
			return a
		}
		return next(groups, a)
	}
}

func formatAttr(a slog.Attr) string {
	key := a.Key
	val := formatValue(a.Value)
	return fmt.Sprintf("%s=%s", key, val)
}

func formatValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		return fmt.Sprintf("%q", v.String())
	case slog.KindTime:
		return v.Time().Format(time.RFC3339)
	case slog.KindAny:
		return fmt.Sprintf("%+v", v.Any())
	case slog.KindGroup:
		var result strings.Builder
		result.WriteString("{")
		attrs := v.Group()
		for i, attr := range attrs {
			if i > 0 {
				result.WriteString(" ")
			}
			result.WriteString(formatAttr(attr))
		}
		result.WriteString("}")
		return result.String()
	default:
		return v.String()
	}
}
