package integration

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/openapi"
)

// TestIT_GenerateFromOpenAPI drives the full read-side path: extract an
// operation from an OpenAPI spec and render it through the real engine,
// asserting the vars.operation/schemas/info namespaces reached the template.
func TestIT_GenerateFromOpenAPI(t *testing.T) {
	testdataDir := getTestdataDir()
	generatorDir := filepath.Join(testdataDir, "generators", "openapi-handler")
	specPath := filepath.Join(testdataDir, "openapi", "petstore.yaml")

	spec, err := os.ReadFile(specPath)
	require.NoError(t, err)

	vars, err := openapi.ExtractOperation(spec, "getPetById")
	require.NoError(t, err)

	workDir := setupWorkDir(t, "")

	gen, err := engine.NewGenerator(false, generatorDir, "", io.Discard)
	require.NoError(t, err)

	_, err = gen.Generate(engine.Data{Name: "pet", ScaffoldVars: vars})
	require.NoError(t, err)

	out, err := os.ReadFile(filepath.Join(workDir, "pet_handler.go"))
	require.NoError(t, err)
	content := string(out)

	assert.Contains(t, content, "getPetById handles GET /pets/{id}")
	assert.Contains(t, content, "Parameters: 2")
	assert.Contains(t, content, "- id (path): string")
	assert.Contains(t, content, "- expand (query): boolean")
	assert.Contains(t, content, "Response 200 schema ref: Pet")
	assert.Contains(t, content, "Pet.name type: string")
	assert.Contains(t, content, "API: Pet Store v1.0.0")
}

// TestIT_GenerateFromOpenAPIAllOperations drives the multi-op read-side path:
// extract every operation and render a single-file loop over vars.operations,
// asserting all operations appear in the documented path-then-method order.
func TestIT_GenerateFromOpenAPIAllOperations(t *testing.T) {
	testdataDir := getTestdataDir()
	generatorDir := filepath.Join(testdataDir, "generators", "openapi-routes")
	specPath := filepath.Join(testdataDir, "openapi", "petstore_multi.yaml")

	spec, err := os.ReadFile(specPath)
	require.NoError(t, err)

	vars, err := openapi.ExtractOperations(spec, "")
	require.NoError(t, err)

	workDir := setupWorkDir(t, "")

	gen, err := engine.NewGenerator(false, generatorDir, "", io.Discard)
	require.NoError(t, err)

	_, err = gen.Generate(engine.Data{Name: "api", ScaffoldVars: vars})
	require.NoError(t, err)

	out, err := os.ReadFile(filepath.Join(workDir, "api_routes.go"))
	require.NoError(t, err)
	content := string(out)

	assert.Contains(t, content, "operations: 3")
	assert.Contains(t, content, "listPets: GET /pets")
	assert.Contains(t, content, "createPet: POST /pets")
	assert.Contains(t, content, "getPetById: GET /pets/{id}")
	// Sorted by path then method: listPets, createPet, getPetById.
	assert.Less(t, strings.Index(content, "listPets"), strings.Index(content, "createPet"))
	assert.Less(t, strings.Index(content, "createPet"), strings.Index(content, "getPetById"))
}

// TestIT_GenerateFromOpenAPIByTag renders only operations carrying a given tag.
func TestIT_GenerateFromOpenAPIByTag(t *testing.T) {
	testdataDir := getTestdataDir()
	generatorDir := filepath.Join(testdataDir, "generators", "openapi-routes")
	specPath := filepath.Join(testdataDir, "openapi", "petstore_multi.yaml")

	spec, err := os.ReadFile(specPath)
	require.NoError(t, err)

	vars, err := openapi.ExtractOperations(spec, "pets")
	require.NoError(t, err)

	workDir := setupWorkDir(t, "")

	gen, err := engine.NewGenerator(false, generatorDir, "", io.Discard)
	require.NoError(t, err)

	_, err = gen.Generate(engine.Data{Name: "api", ScaffoldVars: vars})
	require.NoError(t, err)

	out, err := os.ReadFile(filepath.Join(workDir, "api_routes.go"))
	require.NoError(t, err)
	content := string(out)

	assert.Contains(t, content, "operations: 2")
	assert.Contains(t, content, "listPets: GET /pets")
	assert.Contains(t, content, "getPetById: GET /pets/{id}")
	assert.NotContains(t, content, "createPet")
}
