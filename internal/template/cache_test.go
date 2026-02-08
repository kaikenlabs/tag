package template

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Engine_Cache_HitOnRepeatedContent(t *testing.T) {
	engine := MustNewEngine()
	content := "Hello, {{ name }}!"

	// First parse — cache miss
	tmpl1, err := engine.ParseString(content)
	require.NoError(t, err)
	assert.Equal(t, 1, engine.CacheLen())

	// Second parse — cache hit
	tmpl2, err := engine.ParseString(content)
	require.NoError(t, err)
	assert.Equal(t, 1, engine.CacheLen(), "cache should not grow on repeated content")

	// Both should produce the same result
	ctx := Context{"name": "World"}
	r1, err := tmpl1.Execute(ctx)
	require.NoError(t, err)
	r2, err := tmpl2.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, r1, r2)

	// Underlying *exec.Template should be the same pointer (cache hit)
	gt1 := tmpl1.(*gonjaTemplate)
	gt2 := tmpl2.(*gonjaTemplate)
	assert.Same(t, gt1.tmpl, gt2.tmpl, "cached template pointer should be reused")
}

func TestUT_Engine_Cache_MissOnDifferentContent(t *testing.T) {
	engine := MustNewEngine()

	_, err := engine.ParseString("Hello, {{ name }}!")
	require.NoError(t, err)
	assert.Equal(t, 1, engine.CacheLen())

	_, err = engine.ParseString("Goodbye, {{ name }}!")
	require.NoError(t, err)
	assert.Equal(t, 2, engine.CacheLen(), "different content should create separate cache entries")
}

func TestUT_Engine_Cache_SameContentDifferentNames(t *testing.T) {
	engine := MustNewEngine()
	content := "{{ name }}"

	tmpl1, err := engine.ParseStringNamed(content, "file-a.txt")
	require.NoError(t, err)
	tmpl2, err := engine.ParseStringNamed(content, "file-b.txt")
	require.NoError(t, err)

	// Cache should be keyed on content, not name
	assert.Equal(t, 1, engine.CacheLen(), "same content should reuse cache entry regardless of name")

	// Names should differ (for error reporting) but underlying template should be same
	gt1 := tmpl1.(*gonjaTemplate)
	gt2 := tmpl2.(*gonjaTemplate)
	assert.Same(t, gt1.tmpl, gt2.tmpl)
	assert.Equal(t, "file-a.txt", gt1.name)
	assert.Equal(t, "file-b.txt", gt2.name)
}

func TestUT_Engine_Cache_EmptyContent(t *testing.T) {
	engine := MustNewEngine()

	tmpl, err := engine.ParseString("")
	require.NoError(t, err)
	assert.Equal(t, 1, engine.CacheLen())

	result, err := tmpl.Execute(Context{})
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestUT_Engine_Cache_ParseErrorNotCached(t *testing.T) {
	engine := MustNewEngine()

	// Invalid template should return error and NOT be cached
	_, err := engine.ParseString("{{ unclosed")
	require.Error(t, err)
	assert.Equal(t, 0, engine.CacheLen(), "parse errors should not be cached")
}

func TestUT_Engine_Cache_ConcurrentAccess(t *testing.T) {
	engine := MustNewEngine()
	content := "Hello, {{ name }}!"
	ctx := Context{"name": "World"}
	expected := "Hello, World!"

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errs := make([]error, numGoroutines)
	results := make([]string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			result, err := engine.ExecuteToString(content, ctx)
			errs[idx] = err
			results[idx] = result
		}(i)
	}

	wg.Wait()

	for i := 0; i < numGoroutines; i++ {
		assert.NoError(t, errs[i], "goroutine %d should not error", i)
		assert.Equal(t, expected, results[i], "goroutine %d should produce correct result", i)
	}

	// Only one cache entry despite 100 goroutines
	assert.Equal(t, 1, engine.CacheLen())
}

func TestUT_Engine_Cache_DifferentContextsSameTemplate(t *testing.T) {
	engine := MustNewEngine()
	content := "Hello, {{ name }}!"

	r1, err := engine.ExecuteToString(content, Context{"name": "Alice"})
	require.NoError(t, err)
	assert.Equal(t, "Hello, Alice!", r1)

	r2, err := engine.ExecuteToString(content, Context{"name": "Bob"})
	require.NoError(t, err)
	assert.Equal(t, "Hello, Bob!", r2)

	// Should be cached (same template, different contexts)
	assert.Equal(t, 1, engine.CacheLen())
}

// --- Benchmarks ---

func BenchmarkExecuteToString_NewEnginePerCall(b *testing.B) {
	content := generateBenchmarkTemplate()
	ctx := generateBenchmarkContext()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		engine := MustNewEngine()
		_, err := engine.ExecuteToString(content, ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExecuteToString_CachedEngine(b *testing.B) {
	engine := MustNewEngine()
	content := generateBenchmarkTemplate()
	ctx := generateBenchmarkContext()

	// Warm the cache
	_, err := engine.ExecuteToString(content, ctx)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := engine.ExecuteToString(content, ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseString_CachedVsUncached(b *testing.B) {
	content := generateBenchmarkTemplate()

	b.Run("uncached", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			engine := MustNewEngine()
			_, err := engine.ParseString(content)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("cached", func(b *testing.B) {
		engine := MustNewEngine()
		// Warm cache
		_, _ = engine.ParseString(content)

		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			_, err := engine.ParseString(content)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkExecuteToString_50UniqueTemplates(b *testing.B) {
	engine := MustNewEngine()
	templates := make([]string, 50)
	for i := 0; i < 50; i++ {
		templates[i] = fmt.Sprintf("File %d: {{ vars.project_name }} by {{ vars.author }}", i)
	}
	ctx := Context{
		"vars": map[string]any{
			"project_name": "benchmark-project",
			"author":       "test-author",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		for _, tmpl := range templates {
			_, err := engine.ExecuteToString(tmpl, ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func generateBenchmarkTemplate() string {
	return `package {{ vars.package_name }}

import "fmt"

// {{ vars.project_name }} - generated code
type {{ vars.class_name }} struct {
{% for field in vars.fields %}	{{ field.Name }} {{ field.Type }}
{% endfor %}}

func New{{ vars.class_name }}() *{{ vars.class_name }} {
	return &{{ vars.class_name }}{}
}

{% for field in vars.fields %}func (s *{{ vars.class_name }}) Get{{ field.Name }}() {{ field.Type }} {
	return s.{{ field.Name }}
}

{% endfor %}`
}

func generateBenchmarkContext() Context {
	fields := []map[string]any{
		{"Name": "ID", "Type": "int"},
		{"Name": "Name", "Type": "string"},
		{"Name": "Email", "Type": "string"},
		{"Name": "Age", "Type": "int"},
		{"Name": "Active", "Type": "bool"},
	}
	return Context{
		"vars": map[string]any{
			"package_name": "models",
			"project_name": "benchmark",
			"class_name":   "User",
			"fields":       fields,
		},
	}
}
