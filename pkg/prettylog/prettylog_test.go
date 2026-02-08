package prettylog

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_NewHandler_NilOpts(t *testing.T) {
	h := NewHandler("test", nil)
	require.NotNil(t, h)
	assert.Equal(t, "test", h.app)
}

func TestUT_NewHandler_WithOpts(t *testing.T) {
	opts := &slog.HandlerOptions{Level: slog.LevelWarn}
	h := NewHandler("myapp", opts)
	require.NotNil(t, h)

	// Debug should be disabled when level is Warn
	assert.False(t, h.Enabled(context.Background(), slog.LevelDebug))
	assert.True(t, h.Enabled(context.Background(), slog.LevelWarn))
	assert.True(t, h.Enabled(context.Background(), slog.LevelError))
}

func TestUT_Handler_Enabled(t *testing.T) {
	h := NewHandler("test", &slog.HandlerOptions{Level: slog.LevelInfo})

	assert.False(t, h.Enabled(context.Background(), slog.LevelDebug))
	assert.True(t, h.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, h.Enabled(context.Background(), slog.LevelWarn))
	assert.True(t, h.Enabled(context.Background(), slog.LevelError))
}

func TestUT_Handler_Handle_NoError(t *testing.T) {
	h := NewHandler("test", nil)

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	err := h.Handle(context.Background(), record)
	assert.NoError(t, err)
}

func TestUT_Handler_Handle_AllLevels(t *testing.T) {
	h := NewHandler("test", &slog.HandlerOptions{Level: slog.LevelDebug})

	levels := []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError}
	for _, level := range levels {
		record := slog.NewRecord(time.Now(), level, "msg", 0)
		err := h.Handle(context.Background(), record)
		assert.NoError(t, err)
	}
}

func TestUT_Handler_Handle_WithAttrs(t *testing.T) {
	h := NewHandler("test", nil)

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	record.AddAttrs(
		slog.String("key", "value"),
		slog.Int("count", 42),
	)

	err := h.Handle(context.Background(), record)
	assert.NoError(t, err)
}

func TestUT_Handler_WithAttrs(t *testing.T) {
	h := NewHandler("test", nil)
	child := h.WithAttrs([]slog.Attr{slog.String("key", "value")})
	require.NotNil(t, child)
	assert.IsType(t, &Handler{}, child)
}

func TestUT_Handler_WithGroup(t *testing.T) {
	h := NewHandler("test", nil)
	child := h.WithGroup("mygroup")
	require.NotNil(t, child)
	assert.IsType(t, &Handler{}, child)
}

func TestUT_Colorize(t *testing.T) {
	result := colorize(red, "hello")
	assert.Contains(t, result, "hello")
	assert.Contains(t, result, "\033[31m")
	assert.Contains(t, result, reset)
}

func TestUT_FormatAttr_String(t *testing.T) {
	attr := slog.String("name", "test")
	result := formatAttr(attr)
	assert.Equal(t, `name="test"`, result)
}

func TestUT_FormatAttr_Int(t *testing.T) {
	attr := slog.Int("count", 42)
	result := formatAttr(attr)
	assert.Equal(t, "count=42", result)
}

func TestUT_FormatValue_String(t *testing.T) {
	v := slog.StringValue("hello")
	result := formatValue(v)
	assert.Equal(t, `"hello"`, result)
}

func TestUT_FormatValue_Time(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	v := slog.TimeValue(ts)
	result := formatValue(v)
	assert.Equal(t, "2025-01-15T10:30:00Z", result)
}

func TestUT_FormatValue_Any(t *testing.T) {
	v := slog.AnyValue([]string{"a", "b"})
	result := formatValue(v)
	assert.Contains(t, result, "a")
	assert.Contains(t, result, "b")
}

func TestUT_FormatValue_Group(t *testing.T) {
	v := slog.GroupValue(
		slog.String("a", "1"),
		slog.String("b", "2"),
	)
	result := formatValue(v)
	assert.Contains(t, result, "{")
	assert.Contains(t, result, "}")
	assert.Contains(t, result, `a="1"`)
	assert.Contains(t, result, `b="2"`)
}

func TestUT_SuppressDefaults(t *testing.T) {
	fn := suppressDefaults(nil)

	// Time, Level, Message should be suppressed
	assert.Equal(t, slog.Attr{}, fn(nil, slog.Attr{Key: slog.TimeKey}))
	assert.Equal(t, slog.Attr{}, fn(nil, slog.Attr{Key: slog.LevelKey}))
	assert.Equal(t, slog.Attr{}, fn(nil, slog.Attr{Key: slog.MessageKey}))

	// Other attributes should pass through
	attr := slog.String("custom", "value")
	assert.Equal(t, attr, fn(nil, attr))
}

func TestUT_SuppressDefaults_WithNext(t *testing.T) {
	called := false
	next := func(groups []string, a slog.Attr) slog.Attr {
		called = true
		return a
	}

	fn := suppressDefaults(next)

	// Suppressed keys should not call next
	fn(nil, slog.Attr{Key: slog.TimeKey})
	assert.False(t, called)

	// Non-suppressed keys should call next
	fn(nil, slog.String("custom", "value"))
	assert.True(t, called)
}

func TestUT_LevelFatal(t *testing.T) {
	// LevelFatal should be higher than Error
	assert.True(t, LevelFatal > slog.LevelError)
}
