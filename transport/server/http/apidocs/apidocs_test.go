package apidocs_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apidocsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/apidocs/v1"
	"github.com/Servora-Kit/servora/transport/server/http/apidocs"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

var yamlDocument = []byte("openapi: 3.1.0\ninfo: {title: Orders, version: '1'}\npaths: {}\n")

func inlineConfig() *apidocsv1.APIDocs {
	return &apidocsv1.APIDocs{Enable: true, Documents: []*apidocsv1.Document{{Source: &apidocsv1.Document_Data{Data: bytes.Clone(yamlDocument)}}}}
}

func newHandler(t *testing.T, config *apidocsv1.APIDocs) *apidocs.Handler {
	t.Helper()
	h, err := apidocs.NewHandler(config)
	if err != nil {
		t.Fatal(err)
	}
	if h == nil {
		t.Fatal("enabled handler is nil")
	}
	return h
}

func request(h http.Handler, method, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, target, nil))
	return w
}

func TestDocumentResources(t *testing.T) {
	jsonDocument := []byte(`{"openapi":"3.1.0","info":{"title":"Users","version":"2"},"paths":{}}`)
	file := filepath.Join(t.TempDir(), "orders.yaml")
	if err := os.WriteFile(file, yamlDocument, 0o600); err != nil {
		t.Fatal(err)
	}
	h := newHandler(t, &apidocsv1.APIDocs{Enable: true, Documents: []*apidocsv1.Document{
		{Slug: "orders", Source: &apidocsv1.Document_Path{Path: file}},
		{Slug: "users", Source: &apidocsv1.Document_Data{Data: jsonDocument}},
		{Slug: "remote", Source: &apidocsv1.Document_Url{Url: "https://example.invalid/openapi.json"}},
	}})
	for _, tc := range []struct {
		path        string
		body        []byte
		contentType string
	}{
		{"/docs/orders/openapi.yaml", yamlDocument, "application/yaml; charset=utf-8"},
		{"/docs/users/openapi.yaml", jsonDocument, "application/json; charset=utf-8"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := request(h, http.MethodGet, tc.path)
			if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), tc.body) || w.Header().Get("Content-Type") != tc.contentType {
				t.Fatalf("GET %s = %d %s %q", tc.path, w.Code, w.Header().Get("Content-Type"), w.Body.Bytes())
			}
		})
	}
	for _, target := range []string{"/docs/openapi.yaml", "/docs/remote/openapi.yaml", "/docs/missing"} {
		if w := request(h, http.MethodGet, target); w.Code != http.StatusNotFound {
			t.Fatalf("unknown or remote mirror %s: %d", target, w.Code)
		}
	}
}

func TestResourceMethods(t *testing.T) {
	h := newHandler(t, inlineConfig())
	for _, target := range []string{"/docs/", "/docs/init.js", "/docs/openapi.yaml"} {
		t.Run(target, func(t *testing.T) {
			get := request(h, http.MethodGet, target)
			head := request(h, http.MethodHead, target)
			if get.Code != http.StatusOK || head.Code != get.Code || head.Body.Len() != 0 || head.Header().Get("Content-Type") != get.Header().Get("Content-Type") || head.Header().Get("Content-Length") != get.Header().Get("Content-Length") {
				t.Fatalf("HEAD does not describe GET without a body: GET=%d %v HEAD=%d %v %q", get.Code, get.Header(), head.Code, head.Header(), head.Body.String())
			}
			post := request(h, http.MethodPost, target)
			if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != "GET, HEAD" {
				t.Fatalf("POST = %d, Allow = %q", post.Code, post.Header().Get("Allow"))
			}
		})
	}
}

func TestDisabledConfigurationSkipsValidationAndFiles(t *testing.T) {
	for _, c := range []*apidocsv1.APIDocs{nil, {}, {Path: proto.String("missing/file.yaml"), BasePath: proto.String("not/a/route"), ScriptUrl: proto.String("javascript:bad"), Scalar: &apidocsv1.Scalar{Layout: proto.String("invalid")}}} {
		h, err := apidocs.NewHandler(c)
		if h != nil || err != nil {
			t.Fatalf("disabled configuration = %v, %v", h, err)
		}
	}
}

func TestConstructionRejectsInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*apidocsv1.APIDocs)
	}{
		{"nil document", func(c *apidocsv1.APIDocs) { c.Documents[0] = nil }},
		{"no source", func(c *apidocsv1.APIDocs) { c.Documents[0].Source = nil }},
		{"empty data", func(c *apidocsv1.APIDocs) { c.Documents[0].Source = &apidocsv1.Document_Data{} }},
		{"empty file path", func(c *apidocsv1.APIDocs) { c.Documents[0].Source = &apidocsv1.Document_Path{} }},
		{"relative document URL", func(c *apidocsv1.APIDocs) { c.Documents[0].Source = &apidocsv1.Document_Url{Url: "openapi.yaml"} }},
		{"missing multi slug", func(c *apidocsv1.APIDocs) {
			c.Documents = append(c.Documents, &apidocsv1.Document{Slug: "other", Source: &apidocsv1.Document_Data{Data: yamlDocument}})
		}},
		{"duplicate slug", func(c *apidocsv1.APIDocs) {
			c.Documents[0].Slug = "api"
			c.Documents = append(c.Documents, proto.Clone(c.Documents[0]).(*apidocsv1.Document))
		}},
		{"slug traversal", func(c *apidocsv1.APIDocs) { c.Documents[0].Slug = "../api" }},
		{"root path", func(c *apidocsv1.APIDocs) { c.BasePath = proto.String("/") }},
		{"path traversal", func(c *apidocsv1.APIDocs) { c.BasePath = proto.String("/docs/../api") }},
		{"encoded separator", func(c *apidocsv1.APIDocs) { c.BasePath = proto.String("/docs%2fapi") }},
		{"route pattern", func(c *apidocsv1.APIDocs) { c.BasePath = proto.String("/docs/{api}") }},
		{"unsafe script", func(c *apidocsv1.APIDocs) { c.ScriptUrl = proto.String("javascript:alert(1)") }},
		{"protocol relative script", func(c *apidocsv1.APIDocs) { c.ScriptUrl = proto.String("//example.invalid/script.js") }},
		{"browser backslash URL", func(c *apidocsv1.APIDocs) { c.ScriptUrl = proto.String(`/\example.invalid/script.js`) }},
		{"invalid layout", func(c *apidocsv1.APIDocs) { c.Scalar = &apidocsv1.Scalar{Layout: proto.String("invalid")} }},
		{"invalid search key", func(c *apidocsv1.APIDocs) { c.Scalar = &apidocsv1.Scalar{SearchHotKey: proto.String("Ctrl+K")} }},
		{"invalid proxy", func(c *apidocsv1.APIDocs) { c.Scalar = &apidocsv1.Scalar{ProxyUrl: "javascript:bad"} }},
		{"non-finite extra", func(c *apidocsv1.APIDocs) {
			c.Scalar = &apidocsv1.Scalar{Extra: &structpb.Struct{Fields: map[string]*structpb.Value{"invalid": structpb.NewNumberValue(math.NaN())}}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := inlineConfig()
			tc.change(c)
			if _, err := apidocs.NewHandler(c); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}

func TestScalarExtraCannotOverrideManagedFields(t *testing.T) {
	for _, key := range []string{"sources", "url", "content", "theme", "darkMode", "telemetry", "proxyUrl"} {
		t.Run(key, func(t *testing.T) {
			c := inlineConfig()
			c.Scalar = &apidocsv1.Scalar{Extra: &structpb.Struct{Fields: map[string]*structpb.Value{key: structpb.NewNullValue()}}}
			if _, err := apidocs.NewHandler(c); err == nil {
				t.Fatal("managed configuration accepted through extra")
			}
		})
	}
}

func TestHandlerSnapshotsCallerData(t *testing.T) {
	c := inlineConfig()
	nested := &structpb.Struct{Fields: map[string]*structpb.Value{"preferredSecurityScheme": structpb.NewStringValue("bearer")}}
	c.Scalar = &apidocsv1.Scalar{Extra: &structpb.Struct{Fields: map[string]*structpb.Value{"authentication": structpb.NewStructValue(nested)}}}
	original := proto.Clone(c)
	h := newHandler(t, c)
	if !proto.Equal(c, original) {
		t.Fatal("constructor mutated caller configuration")
	}
	before := request(h, http.MethodGet, "/docs/init.js")
	c.Documents[0].GetData()[0] = '!'
	c.Scalar.Extra.Fields["authentication"].GetStructValue().Fields["preferredSecurityScheme"] = structpb.NewStringValue("other")
	c.Scalar.Theme = proto.String("purple")
	after := request(h, http.MethodGet, "/docs/init.js")
	if before.Code != http.StatusOK || after.Code != http.StatusOK || !bytes.Equal(before.Body.Bytes(), after.Body.Bytes()) {
		t.Fatal("caller mutation changed served configuration")
	}
	if w := request(h, http.MethodGet, "/docs/openapi.yaml"); w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), yamlDocument) {
		t.Fatal("caller mutation changed served document")
	}
}

func TestFileSourceUsesWorkingDirectoryAndSnapshotsContent(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	file := filepath.Join(root, "api/internal/assets/openapi.yaml")
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, yamlDocument, 0o600); err != nil {
		t.Fatal(err)
	}
	h := newHandler(t, &apidocsv1.APIDocs{Enable: true})
	changed := []byte("openapi: 3.1.0\ninfo: {title: Changed, version: '2'}\n")
	if err := os.WriteFile(file, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	t.Chdir(other)
	_, err := apidocs.NewHandler(&apidocsv1.APIDocs{Enable: true})
	if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), filepath.Join(other, "api/internal/assets/openapi.yaml")) {
		t.Fatalf("wrong-directory error = %v", err)
	}
	absolute := newHandler(t, &apidocsv1.APIDocs{Enable: true, Path: proto.String(file)})
	if w := request(absolute, http.MethodGet, "/docs/openapi.yaml"); !bytes.Equal(w.Body.Bytes(), changed) {
		t.Fatal("absolute path did not load current file")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if w := request(h, http.MethodGet, "/docs/openapi.yaml"); w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), yamlDocument) {
		t.Fatal("file mutation changed existing handler")
	}
}

func TestExplicitDocumentsReplaceDefaultPath(t *testing.T) {
	h := newHandler(t, &apidocsv1.APIDocs{Enable: true, Path: proto.String(filepath.Join(t.TempDir(), "missing")), Documents: []*apidocsv1.Document{{Source: &apidocsv1.Document_Url{Url: "https://example.invalid/openapi.yaml"}}}})
	if w := request(h, http.MethodGet, "/docs/openapi.yaml"); w.Code != http.StatusNotFound {
		t.Fatal("remote document unexpectedly mirrored")
	}
}

func TestGatewayRedirectPreservesPrefixAndQuery(t *testing.T) {
	c := inlineConfig()
	c.BasePath = proto.String("/reference/")
	h := newHandler(t, c)
	mux := http.NewServeMux()
	mux.Handle(h.BasePath(), h)
	mux.Handle(h.BasePath()+"/", h)
	backend := httptest.NewServer(mux)
	defer backend.Close()
	target, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	gateway := httptest.NewServer(http.StripPrefix("/gateway", httputil.NewSingleHostReverseProxy(target)))
	defer gateway.Close()
	resp, err := gateway.Client().Get(gateway.URL + "/gateway/reference?view=api&tag=a%2Fb")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close page response: %v", err)
		}
	}()
	if resp.StatusCode != http.StatusOK || resp.Request.URL.RequestURI() != "/gateway/reference/?view=api&tag=a%2Fb" {
		t.Fatalf("redirect landed at %s with %d", resp.Request.URL, resp.StatusCode)
	}
	specURL := resp.Request.URL.ResolveReference(&url.URL{Path: "openapi.yaml"})
	spec, err := gateway.Client().Get(specURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := spec.Body.Close(); err != nil {
			t.Errorf("close document response: %v", err)
		}
	}()
	body, err := io.ReadAll(spec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if spec.StatusCode != http.StatusOK || !bytes.Equal(body, yamlDocument) {
		t.Fatalf("gateway document = %d %q", spec.StatusCode, body)
	}
}
