package jwks

import (
	"encoding/json"
	"net/http"

	conf "github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	"github.com/Servora-Kit/servora/pkg/logger"
)

const (
	DefaultJWKSPath      = "/.well-known/jwks.json"
	DefaultDiscoveryPath = "/.well-known/openid-configuration"
)

// RouteRegistrar is satisfied by *khttp.Server and *http.ServeMux.
type RouteRegistrar interface {
	Handle(path string, h http.Handler)
}

// Endpoints aggregates JWKS and OIDC Discovery handlers with their
// standard well-known paths, providing a single Register call.
type Endpoints struct {
	jwksPath      string
	discoveryPath string
	issuerURL     string
	log           *logger.Helper
	km            *KeyManager
}

type EndpointOption func(*Endpoints)

func WithJWKSPath(p string) EndpointOption {
	return func(e *Endpoints) { e.jwksPath = p }
}

func WithDiscoveryPath(p string) EndpointOption {
	return func(e *Endpoints) { e.discoveryPath = p }
}

func WithIssuerURLOverride(url string) EndpointOption {
	return func(e *Endpoints) { e.issuerURL = url }
}

// NewEndpoints creates an Endpoints that reads external_url from appCfg.
// If external_url is empty a warning is logged and the issuer URL in the
// OIDC Discovery response will be blank.
func NewEndpoints(km *KeyManager, appCfg *conf.App, l logger.Logger, opts ...EndpointOption) *Endpoints {
	log := logger.For(l, "jwks/pkg")
	e := &Endpoints{
		jwksPath:      DefaultJWKSPath,
		discoveryPath: DefaultDiscoveryPath,
		issuerURL:     appCfg.GetExternalUrl(),
		log:           log,
		km:            km,
	}
	for _, o := range opts {
		o(e)
	}
	if e.issuerURL == "" {
		log.Warn("external_url is empty; OIDC Discovery issuer will be blank")
	}
	return e
}

// Register mounts the JWKS and OIDC Discovery handlers onto r.
func (e *Endpoints) Register(r RouteRegistrar) {
	r.Handle(e.jwksPath, e.jwksHandler())
	r.Handle(e.discoveryPath, e.discoveryHandler())
}

func (e *Endpoints) jwksHandler() http.HandlerFunc {
	data, _ := json.Marshal(e.km.JWKSResponse())
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		if _, err := w.Write(data); err != nil {
			e.log.Errorf("write jwks response: %v", err)
		}
	}
}

type oidcDiscovery struct {
	Issuer                           string   `json:"issuer"`
	JWKSURI                          string   `json:"jwks_uri"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
}

func (e *Endpoints) discoveryHandler() http.HandlerFunc {
	disc := oidcDiscovery{
		Issuer:                           e.issuerURL,
		JWKSURI:                          e.issuerURL + DefaultJWKSPath,
		IDTokenSigningAlgValuesSupported: []string{"RS256"},
	}
	data, _ := json.Marshal(disc)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		if _, err := w.Write(data); err != nil {
			e.log.Errorf("write oidc discovery response: %v", err)
		}
	}
}
