package api

import (
	_ "embed"
	"net/http"
)

// The spec is hand-written and embedded; TestSpecCoversAllRoutes keeps
// it from drifting out of sync with the actual routes.
//
//go:embed openapi.yaml
var openAPISpec []byte

// docsHTML renders the spec with Redoc. The single CDN script is a
// deliberate trade-off: self-hosting a docs bundle isn't worth the
// maintenance for a docs page; the API itself has no external
// dependencies.
const docsHTML = `<!DOCTYPE html>
<html>
<head>
  <title>nepapi documentation</title>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>body { margin: 0; padding: 0; }</style>
</head>
<body>
  <redoc spec-url="/v1/openapi.yaml"></redoc>
  <script src="https://cdn.redoc.ly/redoc/latest/bundles/redoc.standalone.js"></script>
</body>
</html>
`

func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Write(openAPISpec)
}

func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(docsHTML))
}
