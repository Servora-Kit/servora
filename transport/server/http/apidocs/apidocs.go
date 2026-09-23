// Package apidocs 使用 Scalar 展示已有 OpenAPI 文档，不生成或改写接口契约。
//
// Handler 实现 net/http.Handler；Servora HTTP Server 通过 WithConfig 自动装配。
// 文件来源相对于进程工作目录，构造时读取一次；部署时必须分发文档或指定绝对路径。
// 文档页面、原始文档及初始化脚本都需要部署方提供适当的访问控制。
package apidocs

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	apidocsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/apidocs/v1"
	"google.golang.org/protobuf/proto"
)

//go:embed scalar.html
var pageHTML string

var (
	pageTemplate    = template.Must(template.New("apidocs").Parse(pageHTML))
	basePathPattern = regexp.MustCompile(`^/[A-Za-z0-9._~-]+(?:/[A-Za-z0-9._~-]+)*$`)
	slugPattern     = regexp.MustCompile(`^[a-z0-9-]+$`)
)

// Handler 保存构造时的内容快照，可并发使用。
// 请求路径包含 BasePath；使用标准 ServeMux 时同时挂载 BasePath() 和 BasePath()+"/"。
type Handler struct {
	basePath  string
	redirect  string
	resources map[string]resource
}

type resource struct {
	body          []byte
	contentType   string
	contentLength string
}

type source struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Default bool   `json:"default,omitempty"`
}

// NewHandler 从 Proto 配置构造文档；未启用时返回 (nil, nil)，且不读取文件。
// 成功后修改输入配置或文件不会影响该实例；构造期间调用方不得并发修改输入。
// 默认值来自 clone 后的 Apply，不修改调用方配置，也不按应用环境隐式开启。
func NewHandler(config *apidocsv1.APIDocs) (*Handler, error) {
	if !config.GetEnable() {
		return nil, nil
	}
	config = proto.Clone(config).(*apidocsv1.APIDocs)
	if config.Scalar == nil {
		config.Scalar = &apidocsv1.Scalar{}
	}
	if err := config.Apply(); err != nil {
		return nil, fmt.Errorf("apidocs: config: %w", err)
	}
	basePath := strings.TrimSuffix(config.GetBasePath(), "/")
	if !basePathPattern.MatchString(basePath) || path.Clean(basePath) != basePath {
		return nil, fmt.Errorf("apidocs: invalid base path %q", config.GetBasePath())
	}
	if err := validateURL(config.GetScriptUrl(), true); err != nil {
		return nil, fmt.Errorf("apidocs: script URL: %w", err)
	}
	scalar, err := scalarOptions(config.Scalar)
	if err != nil {
		return nil, err
	}
	documents := config.Documents
	if len(documents) == 0 {
		documents = []*apidocsv1.Document{{Source: &apidocsv1.Document_Path{Path: config.GetPath()}}}
	}
	h := &Handler{
		basePath:  basePath,
		redirect:  path.Base(basePath) + "/",
		resources: make(map[string]resource, len(documents)+2),
	}
	sources := make([]source, 0, len(documents))
	slugs := make(map[string]struct{}, len(documents))
	for i, doc := range documents {
		slug := doc.GetSlug()
		if slug == "" && len(documents) == 1 {
			slug = "api"
		}
		if !slugPattern.MatchString(slug) {
			return nil, fmt.Errorf("apidocs: document %d has invalid slug %q", i, slug)
		}
		if _, exists := slugs[slug]; exists {
			return nil, fmt.Errorf("apidocs: duplicate document slug %q", slug)
		}
		slugs[slug] = struct{}{}
		title := doc.GetTitle()
		if title == "" {
			title = slug
		}
		body, specURL, err := loadDocument(doc)
		if err != nil {
			return nil, fmt.Errorf("apidocs: document %d: %w", i, err)
		}
		if specURL == "" {
			specURL = "openapi.yaml"
			if len(documents) > 1 {
				specURL = slug + "/openapi.yaml"
			}
			contentType := "application/yaml; charset=utf-8"
			if json.Valid(body) {
				contentType = "application/json; charset=utf-8"
			}
			h.addResource(specURL, contentType, body)
		}
		sources = append(sources, source{Slug: slug, Title: title, URL: specURL, Default: i == 0})
	}
	scalar["sources"] = sources
	data, err := json.Marshal(scalar)
	if err != nil {
		return nil, fmt.Errorf("apidocs: encode Scalar configuration: %w", err)
	}
	var init bytes.Buffer
	init.WriteString("Scalar.createApiReference('#app', ")
	init.Write(data)
	init.WriteString(");\n")
	h.addResource("init.js", "text/javascript; charset=utf-8", init.Bytes())
	var page bytes.Buffer
	if err := pageTemplate.Execute(&page, struct{ Title, ScriptURL string }{config.GetTitle(), config.GetScriptUrl()}); err != nil {
		return nil, fmt.Errorf("apidocs: render page: %w", err)
	}
	h.addResource("", "text/html; charset=utf-8", page.Bytes())
	return h, nil
}

func loadDocument(doc *apidocsv1.Document) (body []byte, specURL string, err error) {
	switch value := doc.GetSource().(type) {
	case *apidocsv1.Document_Url:
		if err := validateURL(value.Url, false); err != nil {
			return nil, "", fmt.Errorf("URL: %w", err)
		}
		return nil, value.Url, nil
	case *apidocsv1.Document_Path:
		if value.Path == "" {
			return nil, "", fmt.Errorf("file path must not be empty")
		}
		absolute, err := filepath.Abs(value.Path)
		if err != nil {
			return nil, "", fmt.Errorf("resolve file path: %w", err)
		}
		body, err = os.ReadFile(absolute)
		if err != nil {
			return nil, "", err
		}
		if len(body) == 0 {
			return nil, "", fmt.Errorf("file %q is empty", absolute)
		}
	case *apidocsv1.Document_Data:
		body = value.Data // 已由配置快照隔离，无需再次复制。
	default:
		return nil, "", fmt.Errorf("source is required")
	}
	if len(body) == 0 {
		return nil, "", fmt.Errorf("document data must not be empty")
	}
	return body, "", nil
}

func scalarOptions(config *apidocsv1.Scalar) (map[string]any, error) {
	if config.GetLayout() != "modern" && config.GetLayout() != "classic" {
		return nil, fmt.Errorf("apidocs: Scalar layout must be modern or classic")
	}
	if key := config.GetSearchHotKey(); len(key) != 1 || key[0] < 'a' || key[0] > 'z' {
		return nil, fmt.Errorf("apidocs: Scalar search hot key must be a lowercase ASCII letter")
	}
	options := make(map[string]any)
	if config.Extra != nil {
		// Struct.AsMap 将非有限数转换为字符串；通过 JSON 编码拒绝这类非法值。
		data, err := json.Marshal(config.Extra)
		if err != nil {
			return nil, fmt.Errorf("apidocs: encode Scalar extra: %w", err)
		}
		if err := json.Unmarshal(data, &options); err != nil {
			return nil, fmt.Errorf("apidocs: decode Scalar extra: %w", err)
		}
		for key := range options {
			switch key {
			case "sources", "url", "content", "theme", "layout", "darkMode", "showSidebar", "searchHotKey",
				"hideTestRequestButton", "telemetry", "persistAuth", "withDefaultFonts", "proxyUrl":
				return nil, fmt.Errorf("apidocs: Scalar extra key %q is managed by typed configuration", key)
			}
		}
	}
	options["theme"] = config.GetTheme()
	options["layout"] = config.GetLayout()
	options["searchHotKey"] = config.GetSearchHotKey()
	options["hideTestRequestButton"] = config.HideTestRequestButton
	options["telemetry"] = config.Telemetry
	options["persistAuth"] = config.PersistAuth
	options["withDefaultFonts"] = config.WithDefaultFonts
	if config.DarkMode != nil {
		options["darkMode"] = *config.DarkMode
	}
	if config.ShowSidebar != nil {
		options["showSidebar"] = *config.ShowSidebar
	}
	if config.ProxyUrl != "" {
		if err := validateURL(config.ProxyUrl, true); err != nil {
			return nil, fmt.Errorf("apidocs: proxy URL: %w", err)
		}
		options["proxyUrl"] = config.ProxyUrl
	}
	return options, nil
}

// BasePath 返回不带末尾斜线的规范挂载路径。
func (h *Handler) BasePath() string { return h.basePath }

// ServeHTTP 提供精确匹配的页面和资源，不将未知子路径回退为文档页面。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	res, found := h.resources[r.URL.Path]
	redirect := r.URL.Path == h.basePath
	if !found && !redirect {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if redirect {
		location := h.redirect
		if r.URL.RawQuery != "" || r.URL.ForceQuery {
			location += "?" + r.URL.RawQuery
		}
		// http.Redirect 会把相对路径改为内部绝对路径，丢失网关剥离的前缀。
		w.Header().Set("Location", location)
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}
	w.Header().Set("Content-Type", res.contentType)
	w.Header().Set("Content-Length", res.contentLength)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodGet {
		_, _ = w.Write(res.body)
	}
}

func (h *Handler) addResource(name, contentType string, body []byte) {
	h.resources[h.basePath+"/"+name] = resource{body: body, contentType: contentType, contentLength: strconv.Itoa(len(body))}
}

func validateURL(raw string, relative bool) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if strings.Contains(raw, "\\") || strings.TrimSpace(raw) != raw {
		return fmt.Errorf("invalid URL")
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		if u.Hostname() != "" && u.User == nil {
			return nil
		}
		return fmt.Errorf("HTTP URL must have a host and no user information")
	}
	if relative && u.Scheme == "" && u.Host == "" && u.Path != "" && !strings.HasPrefix(raw, "//") {
		return nil
	}
	if relative {
		return fmt.Errorf("expected relative path or HTTP/HTTPS URL")
	}
	return fmt.Errorf("expected absolute HTTP/HTTPS URL")
}
