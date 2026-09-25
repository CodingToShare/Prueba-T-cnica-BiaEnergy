package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"bia-energy.local/backend/internal/analysisrun"
	"bia-energy.local/backend/internal/workspace"
)

// Lightweight OpenAPI consistency checks (no code generation): the document
// parses, every $ref resolves, routes and methods match the router both
// ways, authentication is declared per route, and response schemas list
// exactly the JSON fields of the DTOs that serve them.

func loadSpec(t *testing.T) map[string]any {
	t.Helper()
	dir, err := workspace.FindDir("docs/api")
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(dir, "openapi.yaml"))
	require.NoError(t, err)
	var spec map[string]any
	require.NoError(t, yaml.Unmarshal(data, &spec), "openapi.yaml must parse")
	return spec
}

var publicOperations = []string{"GET /healthz", "GET /readyz", "POST /api/v1/auth/login", "POST /api/v1/auth/logout"}

func specOperations(spec map[string]any) map[string]map[string]any {
	ops := map[string]map[string]any{}
	for path, item := range spec["paths"].(map[string]any) {
		for method, op := range item.(map[string]any) {
			switch method {
			case "get", "post", "put", "patch", "delete":
				ops[strings.ToUpper(method)+" "+path] = op.(map[string]any)
			}
		}
	}
	return ops
}

func routerOperations(t *testing.T) []string {
	t.Helper()
	f := newFixture(t, pinger{})
	var routes []string
	require.NoError(t, chi.Walk(f.handler.(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+strings.TrimSuffix(route, "/"))
		return nil
	}))
	slices.Sort(routes)
	return routes
}

func TestOpenAPI_ParsesAndEveryReferenceResolves(t *testing.T) {
	spec := loadSpec(t)

	assert.True(t, strings.HasPrefix(spec["openapi"].(string), "3."), "OpenAPI 3.x")
	var walk func(node any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			if ref, ok := n["$ref"].(string); ok {
				assert.NotNil(t, resolve(spec, ref), "unresolved reference %s", ref)
			}
			for _, v := range n {
				walk(v)
			}
		case []any:
			for _, v := range n {
				walk(v)
			}
		}
	}
	walk(spec)

	cookie := resolve(spec, "#/components/securitySchemes/sessionCookie").(map[string]any)
	assert.Equal(t, "apiKey", cookie["type"])
	assert.Equal(t, "cookie", cookie["in"])
	assert.Equal(t, "bia_session", cookie["name"])
	for _, schema := range []string{"Error", "SourceTime", "SystemTime"} {
		assert.NotNil(t, resolve(spec, "#/components/schemas/"+schema), schema)
	}
	source := resolve(spec, "#/components/schemas/SourceTime").(map[string]any)
	assert.NotContains(t, source["example"], "Z", "source times are documented without an offset")
}

func resolve(spec map[string]any, ref string) any {
	var node any = spec
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		m, ok := node.(map[string]any)
		if !ok {
			return nil
		}
		node = m[part]
	}
	return node
}

func TestOpenAPI_RoutesMatchTheRouterBothWays(t *testing.T) {
	spec := loadSpec(t)
	documented := make([]string, 0)
	for op := range specOperations(spec) {
		documented = append(documented, op)
	}
	slices.Sort(documented)

	implemented := routerOperations(t)

	assert.Equal(t, implemented, documented, "every implemented route is documented and every documented route exists")
}

func TestOpenAPI_AuthenticationIsDeclaredPerRoute(t *testing.T) {
	spec := loadSpec(t)
	global := spec["security"].([]any)
	require.Len(t, global, 1)
	assert.Contains(t, global[0], "sessionCookie", "cookie authentication is the default")

	for name, op := range specOperations(spec) {
		security, overridden := op["security"]
		responses := op["responses"].(map[string]any)
		if slices.Contains(publicOperations, name) {
			assert.True(t, overridden && len(security.([]any)) == 0, "%s must be public (security: [])", name)
			continue
		}
		assert.False(t, overridden, "%s must inherit the session requirement", name)
		assert.Contains(t, responses, "401", "%s must document 401", name)
	}
}

// jsonFields lists a struct's JSON names, flattening embedded structs.
func jsonFields(t reflect.Type) []string {
	var out []string
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Anonymous {
			out = append(out, jsonFields(f.Type)...)
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name != "" && name != "-" {
			out = append(out, name)
		}
	}
	return out
}

// schemaFields lists a schema's property names, following allOf.
func schemaFields(spec map[string]any, schema map[string]any) []string {
	var out []string
	if all, ok := schema["allOf"].([]any); ok {
		for _, part := range all {
			p := part.(map[string]any)
			if ref, ok := p["$ref"].(string); ok {
				p = resolve(spec, ref).(map[string]any)
			}
			out = append(out, schemaFields(spec, p)...)
		}
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		for name := range props {
			out = append(out, name)
		}
	}
	return out
}

func TestOpenAPI_SchemasListExactlyTheDTOFields(t *testing.T) {
	spec := loadSpec(t)
	pairs := map[string]any{
		"Error":            errorBody{},
		"Session":          sessionDTO{},
		"Pagination":       paginationDTO{},
		"AnalysisRef":      analysisRefDTO{},
		"Period":           periodDTO{},
		"MeterSummary":     meterSummaryDTO{},
		"MeterList":        meterListDTO{},
		"MeterFinding":     meterFindingDTO{},
		"MeterDetail":      meterDetailDTO{},
		"Reading":          readingDTO{},
		"ReadingList":      readingsDTO{},
		"AnomalySummary":   anomalySummaryDTO{},
		"AnomalyList":      anomalyListDTO{},
		"AnomalyDetail":    anomalyDetailDTO{},
		"Explanation":      explanationDTO{},
		"Evidence":         analysisrun.Evidence{},
		"AnalysisRun":      runDTO{},
		"AnalyzeResponse":  analyzeDTO{},
		"DashboardSummary": dashboardDTO{},
	}
	for name, dto := range pairs {
		schema, ok := resolve(spec, "#/components/schemas/"+name).(map[string]any)
		require.True(t, ok, "schema %s", name)
		documented := schemaFields(spec, schema)
		implemented := jsonFields(reflect.TypeOf(dto))
		slices.Sort(documented)
		slices.Sort(implemented)
		assert.Equal(t, implemented, documented, "schema %s", name)
		if required, ok := schema["required"].([]any); ok {
			assert.Len(t, required, len(implemented), "schema %s: every field is always present (null when absent)", name)
		}
	}
}
