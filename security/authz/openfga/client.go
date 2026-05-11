package openfga

import (
	"fmt"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/audit"
	fgaclient "github.com/openfga/go-sdk/client"
	fgacredentials "github.com/openfga/go-sdk/credentials"
)

// ClientOption configures optional Client behaviour.
type ClientOption func(*clientOptions)

type clientOptions struct {
	recorder          *audit.Recorder
	computedRelations map[string][]string
}

// WithAuditRecorder injects an audit recorder for tuple-change and check events.
// Passing nil is safe and disables audit emission.
func WithAuditRecorder(r *audit.Recorder) ClientOption {
	return func(o *clientOptions) { o.recorder = r }
}

// WithComputedRelations provides a mapping from object-type to computed relations
// used for cache invalidation. When a tuple with a given object-type is written/deleted,
// all listed relations are also invalidated.
func WithComputedRelations(m map[string][]string) ClientOption {
	return func(o *clientOptions) { o.computedRelations = m }
}

// Client wraps the OpenFGA SDK client with caching, audit, and framework integration.
type Client struct {
	sdk               *fgaclient.OpenFgaClient
	recorder          *audit.Recorder
	computedRelations map[string][]string
}

// NewClient creates a new OpenFGA client from the given configuration.
func NewClient(cfg *conf.App_OpenFGA, opts ...ClientOption) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("openfga: nil config")
	}
	if cfg.ApiUrl == "" || cfg.StoreId == "" {
		return nil, fmt.Errorf("openfga: api_url and store_id are required")
	}

	cc := &fgaclient.ClientConfiguration{
		ApiUrl:               cfg.ApiUrl,
		StoreId:              cfg.StoreId,
		AuthorizationModelId: cfg.ModelId,
	}
	if cfg.ApiToken != "" {
		cc.Credentials = &fgacredentials.Credentials{
			Method: fgacredentials.CredentialsMethodApiToken,
			Config: &fgacredentials.Config{ApiToken: cfg.ApiToken},
		}
	}

	sdk, err := fgaclient.NewSdkClient(cc)
	if err != nil {
		return nil, fmt.Errorf("openfga: %w", err)
	}

	var o clientOptions
	for _, opt := range opts {
		opt(&o)
	}

	return &Client{
		sdk:               sdk,
		recorder:          o.recorder,
		computedRelations: o.computedRelations,
	}, nil
}
