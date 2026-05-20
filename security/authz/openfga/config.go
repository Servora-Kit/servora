package openfga

import (
	"log/slog"

	openfgaconfpb "github.com/Servora-Kit/servora/api/gen/go/servora/security/authz/openfga/v1"
)

// NewClientOptional creates an OpenFGA client when the given configuration
// contains valid OpenFGA settings, returning nil (instead of an error) when
// the component is not configured or initialisation fails.
func NewClientOptional(cfg *openfgaconfpb.Config, l *slog.Logger, opts ...ClientOption) *Client {
	log := l.With("scope", "security/authz/openfga")
	if cfg == nil || cfg.ApiUrl == "" || cfg.StoreId == "" {
		log.Info("OpenFGA not configured, authorization checks disabled")
		return nil
	}
	c, err := NewClient(cfg, opts...)
	if err != nil {
		log.Warn("failed to create OpenFGA client", "err", err)
		return nil
	}
	return c
}
