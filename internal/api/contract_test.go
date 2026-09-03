package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"

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

	reg, err := registry.New(&config.Config{Sources: []config.Source{{URL: "../../testdata/engineering"}}})
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

			validationInput := &openapi3filter.RequestValidationInput{
				Request:    req,
				PathParams: pathParams,
				Route:      route,
			}
			err = openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
				RequestValidationInput: validationInput,
				Status:                 resp.StatusCode,
				Header:                 resp.Header,
				Body:                   resp.Body,
			})
			if err != nil {
				t.Fatalf("response for %s does not conform to openapi.yaml: %v", tc.path, err)
			}
		})
	}
}
