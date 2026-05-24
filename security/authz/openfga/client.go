package openfga

import (
	"fmt"

	openfgaconfpb "github.com/Servora-Kit/servora/api/gen/go/servora/security/authz/openfga/v1"
	fgaclient "github.com/openfga/go-sdk/client"
	fgacredentials "github.com/openfga/go-sdk/credentials"
)

// ClientOption configures optional Client behaviour.
type ClientOption func(*clientOptions)

type clientOptions struct {
	computedRelations map[string][]string
}

// WithComputedRelations provides a mapping from object-type to computed relations
// used for cache invalidation. When a tuple with a given object-type is written/deleted,
// all listed relations are also invalidated.
func WithComputedRelations(m map[string][]string) ClientOption {
	return func(o *clientOptions) { o.computedRelations = m }
}

// Client wraps the OpenFGA SDK client with caching and framework integration.
type Client struct {
	sdk               *fgaclient.OpenFgaClient
	computedRelations map[string][]string
}

// NewClient creates a new OpenFGA client from the given configuration.
func NewClient(cfg *openfgaconfpb.Config, opts ...ClientOption) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("openfga: nil config")
	}
	if err := cfg.ApplyConf(); err != nil {
		return nil, fmt.Errorf("openfga: config: %w", err)
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
		computedRelations: o.computedRelations,
	}, nil
}
