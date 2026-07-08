package openapi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pb33f/libopenapi"
	base "github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	yaml "go.yaml.in/yaml/v4"

	"github.com/kaikenlabs/tag/pkg/app"
)

// maxSchemaDepth bounds recursion into deeply nested inline schemas (a $ref
// graph can't recurse — nested refs are leaves, see schemaToMap).
// ponytail: cap depth ~20; deepen only if a real spec needs it.
const maxSchemaDepth = 20

// ReservedKeys are the vars namespaces ExtractOperation produces. On collision
// with template- or config-declared vars these win (they are reserved for
// OpenAPI input); an explicit --meta flag still overrides them, as --meta is
// applied later at the highest precedence by the engine.
var ReservedKeys = []string{"operation", "schemas", "info", "servers", "security"}

// ExtractOperation parses an OpenAPI 3.x spec and extracts a single operation
// (selected by operationId or "METHOD /path") into a map[string]any suitable
// for the template `vars` namespace. See claudedocs/openapi-input-plan.md.
func ExtractOperation(spec []byte, selector string) (map[string]any, error) {
	if strings.TrimSpace(selector) == "" {
		return nil, app.Errorf("empty operation selector")
	}

	doc, err := libopenapi.NewDocument(spec)
	if err != nil {
		return nil, app.Errorf("cannot parse OpenAPI spec: %w", err)
	}
	model, buildErr := doc.BuildV3Model()
	if buildErr != nil {
		return nil, app.Errorf("cannot build OpenAPI 3.x model: %w", buildErr)
	}

	method, path, pathItem, op, err := resolveOperation(&model.Model, selector)
	if err != nil {
		return nil, err
	}

	ex := &extractor{collected: map[string]bool{}}
	result := map[string]any{
		"operation": ex.operationToMap(method, path, pathItem, op),
		"info":      infoToMap(model.Model.Info),
		"servers":   serversToList(model.Model.Servers),
		"security":  securityToList(op, &model.Model),
	}
	result["schemas"] = ex.collectComponentSchemas(&model.Model)
	return result, nil
}

// extractor carries walk state: `collected` is the transitive set of referenced
// component names (for vars.schemas); it is never cleared so refs dedupe.
type extractor struct {
	collected map[string]bool
}

// --- selector resolution ---

// resolveOperation finds the operation matching selector by operationId, then
// by "METHOD /path". Not-found and ambiguous both hard-error listing candidates.
func resolveOperation(model *v3.Document, selector string) (string, string, *v3.PathItem, *v3.Operation, error) {
	if model.Paths == nil || model.Paths.PathItems == nil {
		return "", "", nil, nil, app.Errorf("spec has no paths")
	}

	wantMethod, wantPath, isMethodPath := parseMethodPathSelector(selector)
	var matches []opMatch

	forEachOperation(model, func(method, path string, item *v3.PathItem, op *v3.Operation) {
		if isMethodPath {
			if method == wantMethod && path == wantPath {
				matches = append(matches, opMatch{method, path, item, op})
			}
			return
		}
		if op.OperationId == selector {
			matches = append(matches, opMatch{method, path, item, op})
		}
	})

	switch len(matches) {
	case 1:
		m := matches[0]
		return m.method, m.path, m.item, m.op, nil
	case 0:
		return "", "", nil, nil, app.Errorf(
			"no operation matches %q; available operations:\n%s", selector, listCandidates(model))
	default:
		return "", "", nil, nil, app.Errorf(
			"selector %q is ambiguous (%d matches); use the \"METHOD /path\" form. available operations:\n%s",
			selector, len(matches), listCandidates(model))
	}
}

type opMatch struct {
	method string
	path   string
	item   *v3.PathItem
	op     *v3.Operation
}

// parseMethodPathSelector recognizes the "GET /users/{id}" fallback form.
func parseMethodPathSelector(selector string) (method, path string, ok bool) {
	fields := strings.Fields(selector)
	if len(fields) != 2 || !strings.HasPrefix(fields[1], "/") {
		return "", "", false
	}
	return strings.ToUpper(fields[0]), fields[1], true
}

// forEachOperation visits every (METHOD, path) operation in the spec, method
// upper-cased. Shared by selector resolution and candidate listing so the two
// stay in sync.
func forEachOperation(model *v3.Document, fn func(method, path string, item *v3.PathItem, op *v3.Operation)) {
	if model.Paths == nil || model.Paths.PathItems == nil {
		return
	}
	for pair := model.Paths.PathItems.Oldest(); pair != nil; pair = pair.Next() {
		path, item := pair.Key, pair.Value
		for mp := item.GetOperations().Oldest(); mp != nil; mp = mp.Next() {
			fn(strings.ToUpper(mp.Key), path, item, mp.Value)
		}
	}
}

// listCandidates renders "METHOD /path (operationId)" lines for error messages.
func listCandidates(model *v3.Document) string {
	var lines []string
	forEachOperation(model, func(method, path string, _ *v3.PathItem, op *v3.Operation) {
		line := fmt.Sprintf("  %s %s", method, path)
		if op.OperationId != "" {
			line += " (" + op.OperationId + ")"
		}
		lines = append(lines, line)
	})
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// --- operation walk ---

func (e *extractor) operationToMap(method, path string, item *v3.PathItem, op *v3.Operation) map[string]any {
	out := map[string]any{
		"operationId": op.OperationId,
		"method":      method,
		"path":        path,
		"summary":     op.Summary,
		"description": op.Description,
		"tags":        toAnySlice(op.Tags),
		"parameters":  e.paramsToList(effectiveParams(item, op)),
	}
	if op.RequestBody != nil {
		out["requestBody"] = e.requestBodyToMap(op.RequestBody)
	}
	if op.Responses != nil {
		out["responses"] = e.responsesToMap(op.Responses)
	}
	return out
}

// effectiveParams merges path-item-level and operation-level parameters.
// Operation-level parameters override path-level ones with the same (name, in).
func effectiveParams(item *v3.PathItem, op *v3.Operation) []*v3.Parameter {
	index := map[string]int{}
	var merged []*v3.Parameter
	add := func(p *v3.Parameter) {
		key := p.In + "\x00" + p.Name
		if i, ok := index[key]; ok {
			merged[i] = p
			return
		}
		index[key] = len(merged)
		merged = append(merged, p)
	}
	for _, p := range item.Parameters {
		add(p)
	}
	for _, p := range op.Parameters {
		add(p)
	}
	return merged
}

func (e *extractor) paramsToList(params []*v3.Parameter) []any {
	out := make([]any, 0, len(params))
	for _, p := range params {
		m := map[string]any{
			"name":        p.Name,
			"in":          p.In,
			"required":    p.Required != nil && *p.Required,
			"description": p.Description,
		}
		if p.Schema != nil {
			m["schema"] = e.schemaToMap(p.Schema, false, 0)
		}
		out = append(out, m)
	}
	return out
}

func (e *extractor) requestBodyToMap(rb *v3.RequestBody) map[string]any {
	m := map[string]any{
		"required":    rb.Required != nil && *rb.Required,
		"description": rb.Description,
		"content":     e.contentToMap(rb.Content),
	}
	return m
}

func (e *extractor) responsesToMap(resp *v3.Responses) map[string]any {
	out := map[string]any{}
	if resp.Codes != nil {
		for pair := resp.Codes.Oldest(); pair != nil; pair = pair.Next() {
			out[pair.Key] = e.responseToMap(pair.Value)
		}
	}
	if resp.Default != nil {
		out["default"] = e.responseToMap(resp.Default)
	}
	return out
}

func (e *extractor) responseToMap(r *v3.Response) map[string]any {
	return map[string]any{
		"description": r.Description,
		"content":     e.contentToMap(r.Content),
	}
}

// contentToMap keys each media type to its schema. No "pick JSON" magic.
func (e *extractor) contentToMap(content *orderedmap.Map[string, *v3.MediaType]) map[string]any {
	out := map[string]any{}
	if content == nil {
		return out
	}
	for pair := content.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value.Schema != nil {
			out[pair.Key] = e.schemaToMap(pair.Value.Schema, false, 0)
		}
	}
	return out
}

// --- schema walk ---

// schemaToMap converts a schema proxy into the recursive raw-OpenAPI map shape.
// A $ref inlines its body once; nested refs beyond it become leaves (name only,
// no body) so a wide or cyclic $ref graph can neither blow up nor loop — the
// deref'd bodies live once in vars.schemas, keyed by name. refsAsLeaves carries
// that "we are already inside an inlined ref" state; depth is a backstop cap for
// deeply nested inline schemas. Every referenced component name is recorded in
// e.collected for vars.schemas.
func (e *extractor) schemaToMap(proxy *base.SchemaProxy, refsAsLeaves bool, depth int) map[string]any {
	if proxy == nil {
		return nil
	}

	out := map[string]any{}
	if proxy.IsReference() {
		ref := refName(proxy.GetReference())
		out["ref"] = ref
		e.collected[ref] = true
		if refsAsLeaves {
			return out // nested ref: name only; body is in vars.schemas[ref]
		}
		refsAsLeaves = true // inline this ref's body once; deeper refs are leaves
	}

	if depth > maxSchemaDepth {
		return out
	}
	sch := proxy.Schema()
	if sch == nil {
		return out
	}
	e.fillSchema(out, sch, refsAsLeaves, depth)
	return out
}

func (e *extractor) fillSchema(out map[string]any, s *base.Schema, refsAsLeaves bool, depth int) {
	typ, nullable := normalizeType(s.Type)
	if typ != "" {
		out["type"] = typ
	}
	if nullable || (s.Nullable != nil && *s.Nullable) {
		out["nullable"] = true
	}
	if s.Format != "" {
		out["format"] = s.Format
	}
	if s.Description != "" {
		out["description"] = s.Description
	}
	if len(s.Required) > 0 {
		out["required"] = toAnySlice(s.Required)
	}
	if len(s.Enum) > 0 {
		out["enum"] = nodesToValues(s.Enum)
	}
	if s.Default != nil {
		out["default"] = nodeToValue(s.Default)
	}
	if s.Items != nil && s.Items.IsA() {
		out["items"] = e.schemaToMap(s.Items.A, refsAsLeaves, depth+1)
	}
	if s.Properties != nil && s.Properties.Len() > 0 {
		props := map[string]any{}
		for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
			props[pair.Key] = e.schemaToMap(pair.Value, refsAsLeaves, depth+1)
		}
		out["properties"] = props
	}
	if comp := e.composition(s, refsAsLeaves, depth); len(comp) > 0 {
		out["composition"] = comp
	}
}

// composition exposes allOf/oneOf/anyOf raw (no flatten/merge).
// ponytail: no allOf merge; add a flattened view only if a template needs it.
func (e *extractor) composition(s *base.Schema, refsAsLeaves bool, depth int) map[string]any {
	out := map[string]any{}
	add := func(key string, proxies []*base.SchemaProxy) {
		if len(proxies) == 0 {
			return
		}
		list := make([]any, 0, len(proxies))
		for _, p := range proxies {
			list = append(list, e.schemaToMap(p, refsAsLeaves, depth+1))
		}
		out[key] = list
	}
	add("allOf", s.AllOf)
	add("oneOf", s.OneOf)
	add("anyOf", s.AnyOf)
	return out
}

// collectComponentSchemas resolves every transitively-referenced component name
// into vars.schemas, deduped. New refs discovered while walking a component are
// picked up because e.collected grows during the walk.
func (e *extractor) collectComponentSchemas(model *v3.Document) map[string]any {
	out := map[string]any{}
	if model.Components == nil || model.Components.Schemas == nil {
		return out
	}
	// Iterate until no new component names surface (transitive closure).
	for {
		pending := make([]string, 0)
		for name := range e.collected {
			if _, done := out[name]; !done {
				pending = append(pending, name)
			}
		}
		if len(pending) == 0 {
			return out
		}
		for _, name := range pending {
			proxy, ok := model.Components.Schemas.Get(name)
			if !ok {
				out[name] = map[string]any{} // dangling ref: record the name, empty body
				continue
			}
			// refsAsLeaves=true: a component body inlines its own fields, but any
			// nested $ref stays a leaf — each component is expanded exactly once.
			out[name] = e.schemaToMap(proxy, true, 0)
		}
	}
}

// --- top-level sections ---

func infoToMap(info *base.Info) map[string]any {
	if info == nil {
		return map[string]any{}
	}
	return map[string]any{
		"title":       info.Title,
		"version":     info.Version,
		"description": info.Description,
	}
}

func serversToList(servers []*v3.Server) []any {
	out := make([]any, 0, len(servers))
	for _, s := range servers {
		out = append(out, map[string]any{"url": s.URL, "description": s.Description})
	}
	return out
}

// securityToList returns operation-level security if present (even an explicit
// empty list meaning "no auth"), else the spec-level requirements.
func securityToList(op *v3.Operation, model *v3.Document) []any {
	reqs := op.Security
	if reqs == nil {
		reqs = model.Security
	}
	out := make([]any, 0, len(reqs))
	for _, r := range reqs {
		schemes := map[string]any{}
		if r.Requirements != nil {
			for pair := r.Requirements.Oldest(); pair != nil; pair = pair.Next() {
				schemes[pair.Key] = toAnySlice(pair.Value)
			}
		}
		out = append(out, schemes)
	}
	return out
}

// --- helpers ---

// normalizeType folds the OpenAPI 3.1 `type: [T, "null"]` array into a single
// type plus a nullable flag. Multi non-null types collapse to the first
// (per plan); "null" anywhere sets nullable.
func normalizeType(types []string) (typ string, nullable bool) {
	for _, t := range types {
		if t == "null" {
			nullable = true
			continue
		}
		if typ == "" {
			typ = t
		}
	}
	return typ, nullable
}

// refName decodes the final JSON-pointer token of a "$ref" into a component name.
// "#/components/schemas/Foo~1Bar" -> "Foo/Bar".
func refName(ref string) string {
	seg := ref
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		seg = ref[i+1:]
	}
	seg = strings.ReplaceAll(seg, "~1", "/")
	seg = strings.ReplaceAll(seg, "~0", "~")
	return seg
}

func toAnySlice(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

func nodesToValues(nodes []*yaml.Node) []any {
	out := make([]any, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, nodeToValue(n))
	}
	return out
}

// nodeToValue decodes a YAML scalar node into its native Go value, falling back
// to the raw string on decode failure.
func nodeToValue(n *yaml.Node) any {
	if n == nil {
		return nil
	}
	var v any
	if err := n.Decode(&v); err != nil {
		return n.Value
	}
	return v
}
