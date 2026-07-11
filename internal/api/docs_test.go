package api

import (
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSpecCoversAllRoutes fails when openapi.yaml and the served routes
// drift apart in either direction.
func TestSpecCoversAllRoutes(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(openAPISpec, &spec); err != nil {
		t.Fatalf("parsing openapi.yaml: %v", err)
	}

	srv := NewServer(Config{}, &fakeForex{}, &fakePinger{}, slog.New(slog.DiscardHandler))
	defer srv.Close()

	served := map[string]string{} // path -> method
	for _, rt := range srv.routes() {
		served[rt.path] = strings.ToLower(rt.method)
	}

	for path, ops := range spec.Paths {
		method, ok := served[path]
		if !ok {
			t.Errorf("spec documents %s but the server does not serve it", path)
			continue
		}
		if _, ok := ops[method]; !ok {
			t.Errorf("spec documents %s without the served method %s", path, method)
		}
	}
	for path := range served {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("server serves %s but the spec does not document it", path)
		}
	}
}

func TestOpenAPISpecServed(t *testing.T) {
	rec := get(t, testServer(&fakeForex{}), "/v1/openapi.yaml")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/yaml" {
		t.Errorf("Content-Type = %q, want application/yaml", ct)
	}
	if !strings.Contains(rec.Body.String(), "openapi:") {
		t.Error("body does not look like an OpenAPI spec")
	}
}

func TestDocsServed(t *testing.T) {
	rec := get(t, testServer(&fakeForex{}), "/docs")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/v1/openapi.yaml") {
		t.Error("docs page does not reference the spec URL")
	}
}
