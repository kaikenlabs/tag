package openapi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return data
}

// asMap / asList narrow the map[string]any tree without repetitive casts.
func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	require.True(t, ok, "expected map[string]any, got %T", v)
	return m
}

func asList(t *testing.T, v any) []any {
	t.Helper()
	l, ok := v.([]any)
	require.True(t, ok, "expected []any, got %T", v)
	return l
}

func TestUT_ExtractOperation_ByOperationID(t *testing.T) {
	spec := loadFixture(t, "petstore_30.yaml")

	res, err := ExtractOperation(spec, "getPetById")
	require.NoError(t, err)

	op := asMap(t, res["operation"])
	assert.Equal(t, "getPetById", op["operationId"])
	assert.Equal(t, "GET", op["method"])
	assert.Equal(t, "/pets/{id}", op["path"])
	assert.Equal(t, "Get a pet by id", op["summary"])

	params := asList(t, op["parameters"])
	require.Len(t, params, 1)
	p := asMap(t, params[0])
	assert.Equal(t, "id", p["name"])
	assert.Equal(t, "path", p["in"])
	assert.Equal(t, true, p["required"])
	sch := asMap(t, p["schema"])
	assert.Equal(t, "string", sch["type"])
	assert.Equal(t, "uuid", sch["format"])
}

func TestUT_ExtractOperation_ByMethodPath(t *testing.T) {
	spec := loadFixture(t, "petstore_30.yaml")

	res, err := ExtractOperation(spec, "GET /pets/{id}")
	require.NoError(t, err)
	op := asMap(t, res["operation"])
	assert.Equal(t, "getPetById", op["operationId"])
	assert.Equal(t, "GET", op["method"])
}

func TestUT_ExtractOperation_MethodPathCaseInsensitive(t *testing.T) {
	spec := loadFixture(t, "petstore_30.yaml")

	res, err := ExtractOperation(spec, "get /pets/{id}")
	require.NoError(t, err)
	op := asMap(t, res["operation"])
	assert.Equal(t, "getPetById", op["operationId"])
}

func TestUT_ExtractOperation_NotFound(t *testing.T) {
	spec := loadFixture(t, "petstore_30.yaml")

	_, err := ExtractOperation(spec, "nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no operation matches")
	// Error lists candidates so the user can pick one.
	assert.Contains(t, err.Error(), "getPetById")
	assert.Contains(t, err.Error(), "GET /pets")
}

func TestUT_ExtractOperation_EmptySelector(t *testing.T) {
	spec := loadFixture(t, "petstore_30.yaml")

	_, err := ExtractOperation(spec, "   ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty operation selector")
}

func TestUT_ExtractOperation_Ambiguous(t *testing.T) {
	spec := []byte(`openapi: 3.0.0
info: {title: t, version: v}
paths:
  /a:
    get:
      operationId: dup
      responses: {'200': {description: ok}}
  /b:
    get:
      operationId: dup
      responses: {'200': {description: ok}}
`)
	_, err := ExtractOperation(spec, "dup")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
	assert.Contains(t, err.Error(), "GET /a")
	assert.Contains(t, err.Error(), "GET /b")
}

func TestUT_ExtractOperation_RefResolutionAndSchemas(t *testing.T) {
	spec := loadFixture(t, "petstore_30.yaml")

	res, err := ExtractOperation(spec, "getPetById")
	require.NoError(t, err)

	// 200 response schema is a $ref to Pet; ref name preserved and deref'd.
	op := asMap(t, res["operation"])
	responses := asMap(t, op["responses"])
	r200 := asMap(t, responses["200"])
	content := asMap(t, r200["content"])
	petSchema := asMap(t, content["application/json"])
	assert.Equal(t, "Pet", petSchema["ref"])
	assert.Equal(t, "object", petSchema["type"])

	// vars.schemas collects referenced components transitively (Pet -> Owner ->
	// Address) plus Error (default response), deduped.
	schemas := asMap(t, res["schemas"])
	assert.Contains(t, schemas, "Pet")
	assert.Contains(t, schemas, "Owner")
	assert.Contains(t, schemas, "Address")
	assert.Contains(t, schemas, "Error")

	// Pet.status enum + Pet.photoUrls array items.
	pet := asMap(t, schemas["Pet"])
	props := asMap(t, pet["properties"])
	status := asMap(t, props["status"])
	assert.Equal(t, []any{"available", "pending", "sold"}, status["enum"])
	photoUrls := asMap(t, props["photoUrls"])
	assert.Equal(t, "array", photoUrls["type"])
	items := asMap(t, photoUrls["items"])
	assert.Equal(t, "string", items["type"])
	assert.Equal(t, []any{"id", "name"}, pet["required"])
}

func TestUT_ExtractOperation_RequestBody(t *testing.T) {
	spec := loadFixture(t, "petstore_30.yaml")

	res, err := ExtractOperation(spec, "createPet")
	require.NoError(t, err)
	op := asMap(t, res["operation"])
	rb := asMap(t, op["requestBody"])
	assert.Equal(t, true, rb["required"])
	assert.Equal(t, "Pet to add", rb["description"])
	content := asMap(t, rb["content"])
	schema := asMap(t, content["application/json"])
	assert.Equal(t, "Pet", schema["ref"])
}

func TestUT_ExtractOperation_TopLevelSections(t *testing.T) {
	spec := loadFixture(t, "petstore_30.yaml")

	res, err := ExtractOperation(spec, "listPets")
	require.NoError(t, err)

	info := asMap(t, res["info"])
	assert.Equal(t, "Pet Store", info["title"])
	assert.Equal(t, "1.0.0", info["version"])

	servers := asList(t, res["servers"])
	require.Len(t, servers, 1)
	assert.Equal(t, "https://api.example.com/v1", asMap(t, servers[0])["url"])

	// listPets has no operation-level security -> spec-level applies.
	security := asList(t, res["security"])
	require.Len(t, security, 1)
	assert.Contains(t, asMap(t, security[0]), "apiKey")
}

func TestUT_ExtractOperation_NullableNormalization_31(t *testing.T) {
	spec := loadFixture(t, "petstore_31.yaml")

	res, err := ExtractOperation(spec, "getPet")
	require.NoError(t, err)

	schemas := asMap(t, res["schemas"])
	pet := asMap(t, schemas["Pet"])
	props := asMap(t, pet["properties"])

	// type: [string, "null"] -> type: string, nullable: true
	nickname := asMap(t, props["nickname"])
	assert.Equal(t, "string", nickname["type"])
	assert.Equal(t, true, nickname["nullable"])

	age := asMap(t, props["age"])
	assert.Equal(t, "integer", age["type"])
	assert.Equal(t, true, age["nullable"])
	assert.Equal(t, "int32", age["format"])

	// Non-nullable field stays plain.
	id := asMap(t, props["id"])
	assert.Equal(t, "string", id["type"])
	assert.NotContains(t, id, "nullable")
}

func TestUT_ExtractOperation_CircularRef(t *testing.T) {
	spec := []byte(`openapi: 3.0.0
info: {title: t, version: v}
paths:
  /nodes:
    get:
      operationId: getNodes
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Node'
components:
  schemas:
    Node:
      type: object
      properties:
        value:
          type: string
        next:
          $ref: '#/components/schemas/Node'
`)
	res, err := ExtractOperation(spec, "getNodes")
	require.NoError(t, err) // terminates, no stack overflow

	schemas := asMap(t, res["schemas"])
	node := asMap(t, schemas["Node"])
	props := asMap(t, node["properties"])
	next := asMap(t, props["next"])
	// The self-ref emits the ref name and stops descending.
	assert.Equal(t, "Node", next["ref"])
}

func TestUT_ExtractOperation_NestedRefIsLeaf(t *testing.T) {
	// A $ref inlines its body once; a $ref nested inside that body is a leaf
	// (name only, no inline body) so a wide $ref graph cannot blow up. The
	// deref'd body lives once in vars.schemas.
	spec := loadFixture(t, "petstore_30.yaml")

	res, err := ExtractOperation(spec, "getPetById")
	require.NoError(t, err)

	// vars.schemas.Pet.owner is a nested ref -> leaf (ref name, no body).
	pet := asMap(t, asMap(t, res["schemas"])["Pet"])
	owner := asMap(t, asMap(t, pet["properties"])["owner"])
	assert.Equal(t, "Owner", owner["ref"])
	assert.NotContains(t, owner, "properties", "nested ref must not inline its body")
	assert.NotContains(t, owner, "type")

	// The body is available once under vars.schemas.Owner.
	ownerSchema := asMap(t, asMap(t, res["schemas"])["Owner"])
	assert.Equal(t, "object", ownerSchema["type"])
	assert.Contains(t, asMap(t, ownerSchema["properties"]), "address")
}

func TestUT_ExtractOperation_RefParameter(t *testing.T) {
	// A parameter that is itself a component $ref must be deref'd with name/in
	// populated, else effectiveParams' (name,in) key would collide.
	spec := []byte(`openapi: 3.0.0
info: {title: t, version: v}
paths:
  /x:
    get:
      operationId: getX
      parameters:
        - $ref: '#/components/parameters/PageParam'
        - $ref: '#/components/parameters/SizeParam'
      responses:
        '200': {description: ok}
components:
  parameters:
    PageParam:
      name: page
      in: query
      required: false
      schema: {type: integer}
    SizeParam:
      name: size
      in: query
      required: true
      schema: {type: integer}
`)
	res, err := ExtractOperation(spec, "getX")
	require.NoError(t, err)
	params := asList(t, asMap(t, res["operation"])["parameters"])
	require.Len(t, params, 2, "ref parameters must not collide")
	names := map[string]bool{}
	for _, p := range params {
		names[asMap(t, p)["name"].(string)] = true
	}
	assert.True(t, names["page"])
	assert.True(t, names["size"])
}

func TestUT_ExtractOperation_MutualCycle(t *testing.T) {
	// Indirect cycle A -> B -> A terminates and both are collected once.
	spec := []byte(`openapi: 3.0.0
info: {title: t, version: v}
paths:
  /x:
    get:
      operationId: getX
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/A'
components:
  schemas:
    A:
      type: object
      properties:
        b: {$ref: '#/components/schemas/B'}
    B:
      type: object
      properties:
        a: {$ref: '#/components/schemas/A'}
`)
	res, err := ExtractOperation(spec, "getX")
	require.NoError(t, err)
	schemas := asMap(t, res["schemas"])
	assert.Contains(t, schemas, "A")
	assert.Contains(t, schemas, "B")
	// Each component's nested ref is a leaf, so no infinite structure.
	a := asMap(t, schemas["A"])
	assert.Equal(t, "B", asMap(t, asMap(t, a["properties"])["b"])["ref"])
}

func TestUT_ExtractOperation_Composition(t *testing.T) {
	spec := []byte(`openapi: 3.0.0
info: {title: t, version: v}
paths:
  /x:
    get:
      operationId: getX
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                allOf:
                  - $ref: '#/components/schemas/A'
                  - type: object
                    properties:
                      extra: {type: string}
components:
  schemas:
    A:
      type: object
      properties:
        a: {type: string}
`)
	res, err := ExtractOperation(spec, "getX")
	require.NoError(t, err)
	op := asMap(t, res["operation"])
	content := asMap(t, asMap(t, asMap(t, op["responses"])["200"])["content"])
	schema := asMap(t, content["application/json"])
	comp := asMap(t, schema["composition"])
	allOf := asList(t, comp["allOf"])
	require.Len(t, allOf, 2)
	assert.Equal(t, "A", asMap(t, allOf[0])["ref"])
}

func TestUT_ExtractOperation_PathLevelParams(t *testing.T) {
	// Path-item params merge with operation params; operation overrides same (name,in).
	spec := []byte(`openapi: 3.0.0
info: {title: t, version: v}
paths:
  /items/{id}:
    parameters:
      - name: id
        in: path
        required: true
        description: from path level
        schema: {type: string}
      - name: trace
        in: header
        required: false
        schema: {type: string}
    get:
      operationId: getItem
      parameters:
        - name: id
          in: path
          required: true
          description: overridden by operation
          schema: {type: integer}
      responses:
        '200': {description: ok}
`)
	res, err := ExtractOperation(spec, "getItem")
	require.NoError(t, err)
	op := asMap(t, res["operation"])
	params := asList(t, op["parameters"])
	require.Len(t, params, 2) // id (merged/overridden) + trace (inherited)

	byName := map[string]map[string]any{}
	for _, p := range params {
		pm := asMap(t, p)
		byName[pm["name"].(string)] = pm
	}
	require.Contains(t, byName, "id")
	require.Contains(t, byName, "trace")
	// Operation-level id wins.
	assert.Equal(t, "overridden by operation", byName["id"]["description"])
	assert.Equal(t, "integer", asMap(t, byName["id"]["schema"])["type"])
}

func TestUT_ExtractOperation_OperationSecurityOverridesSpec(t *testing.T) {
	spec := []byte(`openapi: 3.0.0
info: {title: t, version: v}
security:
  - global: []
paths:
  /x:
    get:
      operationId: getX
      security: []
      responses:
        '200': {description: ok}
`)
	res, err := ExtractOperation(spec, "getX")
	require.NoError(t, err)
	// Explicit empty operation security means "no auth" -> empty list, not spec-level.
	security := asList(t, res["security"])
	assert.Empty(t, security)
}

func TestUT_ExtractOperation_MalformedSpec(t *testing.T) {
	_, err := ExtractOperation([]byte("this is not: [valid: openapi"), "x")
	require.Error(t, err)
}

func TestUT_ExtractOperation_NonOpenAPISpec(t *testing.T) {
	_, err := ExtractOperation([]byte("foo: bar\n"), "x")
	require.Error(t, err)
}

func TestUT_RefName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"#/components/schemas/User", "User"},
		{"#/components/schemas/Foo~1Bar", "Foo/Bar"},
		{"#/components/schemas/Tilde~0Name", "Tilde~Name"},
		{"User", "User"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, refName(tt.in), tt.in)
	}
}

func TestUT_NormalizeType(t *testing.T) {
	tests := []struct {
		name     string
		in       []string
		wantType string
		wantNull bool
	}{
		{"single", []string{"string"}, "string", false},
		{"nullable", []string{"string", "null"}, "string", true},
		{"null first", []string{"null", "integer"}, "integer", true},
		{"multi collapses to first", []string{"string", "integer", "null"}, "string", true},
		{"empty", nil, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ, null := normalizeType(tt.in)
			assert.Equal(t, tt.wantType, typ)
			assert.Equal(t, tt.wantNull, null)
		})
	}
}
