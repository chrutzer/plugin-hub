package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"

	"github.com/chrutzer/plugin-hub/client"
	"github.com/chrutzer/plugin-hub/internal/config"
	"github.com/chrutzer/plugin-hub/internal/registry"
)

// TestOpenAPIContract asserts that the running server's responses conform to
// the schemas declared in openapi.yaml, so the spec and the handlers can't
// silently drift apart.
func TestOpenAPIContract(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile("../../openapi.yaml")
	if err != nil {
		t.Fatalf("load openapi.yaml: %v", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("openapi.yaml is invalid: %v", err)
	}
	// Match requests against any host:port, since the test server doesn't
	// run on the "servers" URL declared in openapi.yaml.
	doc.Servers = nil
	router, err := legacyrouter.NewRouter(doc)
	if err != nil {
		t.Fatalf("build router: %v", err)
	}

	oauthSource := t.TempDir()
	for name, content := range map[string]string{
		"plugin.json": `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"oauth-test"}`,
		"mcp.json": `{
			"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
			"mcpServers":{
				"grafana":{"type":"streamable-http","url":"https://example.com/mcp","oauth":{"client_id":"grafana-client","scopes":["read"],"provider":{"audience":"grafana"}}},
				"empty":{"type":"sse","url":"https://example.com/sse","oauth":{}},
				"absent":{"type":"streamable-http","url":"https://example.com/mcp"}
			}
		}`,
	} {
		if err := os.WriteFile(filepath.Join(oauthSource, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	reg, err := registry.New(&config.Config{Sources: []config.Source{{URL: "../../testdata/engineering"}, {URL: oauthSource}}})
	if err != nil {
		t.Fatalf("registry.New: %v", err)
	}
	reg.Reload()

	ts := httptest.NewServer(New(reg).Handler())
	defer ts.Close()

	cases := []struct {
		name   string
		path   string
		status int
	}{
		{"list plugins", "/", http.StatusOK},
		{"download existing plugin", "/engineering.zip", http.StatusOK},
		{"download missing plugin", "/does-not-exist.zip", http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, ts.URL+tc.path, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}

			route, pathParams, err := router.FindRoute(req)
			if err != nil {
				t.Fatalf("no route in openapi.yaml matches %s: %v", tc.path, err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request %s: %v", tc.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.status)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if tc.path == "/" {
				assertOAuthResponse(t, body)
			}

			validationInput := &openapi3filter.RequestValidationInput{
				Request:    req,
				PathParams: pathParams,
				Route:      route,
			}
			err = openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
				RequestValidationInput: validationInput,
				Status:                 resp.StatusCode,
				Header:                 resp.Header,
				Body:                   io.NopCloser(bytes.NewReader(body)),
			})
			if err != nil {
				t.Fatalf("response for %s does not conform to openapi.yaml: %v", tc.path, err)
			}
		})
	}
}

func assertOAuthResponse(t *testing.T, body []byte) {
	t.Helper()
	var plugins []client.Plugin
	if err := json.Unmarshal(body, &plugins); err != nil {
		t.Fatal(err)
	}
	for _, p := range plugins {
		if p.Name != "oauth-test" {
			continue
		}
		if p.Warnings != nil || p.Mcp == nil || len(*p.Mcp) != 3 {
			t.Fatalf("OAuth plugin was not loaded cleanly: %#v", p)
		}
		servers := *p.Mcp
		want := map[string]interface{}{
			"client_id": "grafana-client",
			"scopes":    []interface{}{"read"},
			"provider":  map[string]interface{}{"audience": "grafana"},
		}
		if servers["grafana"].Oauth == nil || !reflect.DeepEqual(*servers["grafana"].Oauth, want) {
			t.Fatalf("OAuth configuration was not preserved: %s", body)
		}
		if servers["empty"].Oauth == nil || len(*servers["empty"].Oauth) != 0 {
			t.Fatal("empty OAuth object was not preserved")
		}
		if servers["absent"].Oauth != nil {
			t.Fatal("OAuth was added to a server without OAuth configuration")
		}
		return
	}
	t.Fatal("OAuth plugin missing from API response")
}
