package commands

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/types/flags"
)

const openapiTestSpec = `openapi: 3.0.3
info:
  title: Pet Store
  version: 1.0.0
paths:
  /pets/{id}:
    get:
      operationId: getPetById
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: A pet
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Pet'
components:
  schemas:
    Pet:
      type: object
      properties:
        name:
          type: string
`

// newOpenAPITestApp builds a cli.App exposing the generate flags used by the
// OpenAPI wiring, so tests exercise real urfave/cli flag parsing.
func newOpenAPITestApp(cfg *config.Config) *cli.App {
	return &cli.App{
		Writer: io.Discard,
		Commands: []*cli.Command{
			{
				Name: "generate",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{Name: flags.MetaFlag, Aliases: []string{"m"}},
					&cli.BoolFlag{Name: flags.DryRunFlag},
					&cli.BoolFlag{Name: flags.NoHooksFlag},
					&cli.StringFlag{Name: flags.OnExistingFlag, Value: ""},
					&cli.BoolFlag{Name: flags.VerboseFlag},
					&cli.StringFlag{Name: flags.OpenAPIFlag},
					&cli.StringFlag{Name: flags.OperationFlag},
				},
				Action: func(c *cli.Context) error {
					return generateAction(c, cfg, defaultGeneratorFactories())
				},
			},
		},
	}
}

func TestUT_GenerateAction_OpenAPIViaAppRun(t *testing.T) {
	tmpDir := setupTempDir(t)

	template := "---\nto: {{ name }}.txt\n---\n" +
		"method: {{ vars.operation.method }}\n" +
		"path: {{ vars.operation.path }}\n" +
		"op: {{ vars.operation.operationId }}\n" +
		"param: {{ vars.operation.parameters[0].name }}\n" +
		"petType: {{ vars.schemas.Pet.type }}\n" +
		"title: {{ vars.info.title }}\n"
	createGenerator(t, tmpDir, "handler", template)
	createSharedDir(t, tmpDir)

	specPath := filepath.Join(tmpDir, "api.yaml")
	require.NoError(t, os.WriteFile(specPath, []byte(openapiTestSpec), 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { os.Chdir(origDir) })

	cfg := createTestConfig(t, tmpDir)

	app := newOpenAPITestApp(cfg)
	err = app.Run([]string{
		"tag", "generate",
		"--openapi", specPath,
		"--operation", "getPetById",
		"--no-hooks",
		"handler", "widget",
	})
	require.NoError(t, err)

	out, err := os.ReadFile(filepath.Join(tmpDir, "widget.txt"))
	require.NoError(t, err)
	content := string(out)
	assert.Contains(t, content, "method: GET")
	assert.Contains(t, content, "path: /pets/{id}")
	assert.Contains(t, content, "op: getPetById")
	assert.Contains(t, content, "param: id")
	assert.Contains(t, content, "petType: object")
	assert.Contains(t, content, "title: Pet Store")
}

func TestUT_GenerateAction_OpenAPIMethodPathSelector(t *testing.T) {
	tmpDir := setupTempDir(t)

	createGenerator(t, tmpDir, "handler",
		"---\nto: {{ name }}.txt\n---\nop: {{ vars.operation.operationId }}\n")
	createSharedDir(t, tmpDir)

	specPath := filepath.Join(tmpDir, "api.yaml")
	require.NoError(t, os.WriteFile(specPath, []byte(openapiTestSpec), 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { os.Chdir(origDir) })

	cfg := createTestConfig(t, tmpDir)
	app := newOpenAPITestApp(cfg)
	err = app.Run([]string{
		"tag", "generate",
		"--openapi", specPath,
		"--operation", "GET /pets/{id}",
		"--no-hooks",
		"handler", "widget",
	})
	require.NoError(t, err)

	out, err := os.ReadFile(filepath.Join(tmpDir, "widget.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(out), "op: getPetById")
}

func TestUT_LoadOpenAPIVars_BothOrNeither(t *testing.T) {
	tests := []struct {
		name      string
		openapi   string
		operation string
		wantErr   string
		wantNil   bool
	}{
		{name: "neither set", wantNil: true},
		{name: "only openapi", openapi: "api.yaml", wantErr: "--openapi requires --operation"},
		{name: "only operation", operation: "getX", wantErr: "--operation requires --openapi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newFlagContext(t, tt.openapi, tt.operation)
			vars, err := loadOpenAPIVars(c)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.wantNil {
				assert.Empty(t, vars)
			}
		})
	}
}

func TestUT_LoadOpenAPIVars_MissingFile(t *testing.T) {
	c := newFlagContext(t, "/no/such/spec.yaml", "getX")
	_, err := loadOpenAPIVars(c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read OpenAPI spec")
}

func TestUT_MergeOpenAPIVars_ReservedWins(t *testing.T) {
	base := map[string]any{
		"operation": "user-defined", // collides -> OpenAPI wins
		"keep":      "untouched",    // unrelated -> survives
	}
	openapiVars := map[string]any{
		"operation": map[string]any{"method": "GET"},
		"schemas":   map[string]any{"Pet": map[string]any{}},
	}

	merged := mergeOpenAPIVars(base, openapiVars)

	assert.Equal(t, "untouched", merged["keep"])
	assert.Equal(t, map[string]any{"method": "GET"}, merged["operation"])
	assert.Contains(t, merged, "schemas")
	// base is not mutated.
	assert.Equal(t, "user-defined", base["operation"])
}

func TestUT_MergeOpenAPIVars_NoOpenAPI(t *testing.T) {
	base := map[string]any{"a": 1}
	assert.Equal(t, base, mergeOpenAPIVars(base, nil))
}

// newFlagContext builds a cli.Context carrying only the --openapi/--operation
// flags, for direct unit testing of loadOpenAPIVars.
func newFlagContext(t *testing.T, openapiVal, operationVal string) *cli.Context {
	t.Helper()
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	set.String(flags.OpenAPIFlag, openapiVal, "")
	set.String(flags.OperationFlag, operationVal, "")
	return cli.NewContext(nil, set, nil)
}
