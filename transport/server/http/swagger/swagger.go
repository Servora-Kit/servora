package swagger

import (
	_ "embed"
	"html/template"
	"net/http"
	"strings"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

//go:embed swagger-ui.html
var swaggerUIHTML string

var uiTmpl = template.Must(template.New("swagger-ui").Parse(swaggerUIHTML))

type config struct {
	Title    string
	BasePath string
}

type Option func(*config)

func WithTitle(title string) Option {
	return func(c *config) {
		c.Title = title
	}
}

func WithBasePath(path string) Option {
	return func(c *config) {
		c.BasePath = path
	}
}

// Register mounts Swagger UI and the OpenAPI spec on the given Kratos HTTP server.
//
//   - GET {basePath}/            → Swagger UI page
//   - GET {basePath}/openapi.yaml → raw OpenAPI spec
func Register(srv *khttp.Server, specData []byte, opts ...Option) {
	cfg := &config{
		Title:    "API Documentation",
		BasePath: "/docs/",
	}
	for _, o := range opts {
		o(cfg)
	}

	basePath := strings.TrimRight(cfg.BasePath, "/")
	specURL := basePath + "/openapi.yaml"

	srv.HandleFunc(specURL, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(specData)
	})

	data := map[string]string{
		"Title":   cfg.Title,
		"SpecURL": specURL,
	}
	srv.HandleFunc(basePath+"/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = uiTmpl.Execute(w, data)
	})
}
