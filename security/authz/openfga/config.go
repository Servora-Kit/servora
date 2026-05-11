package openfga

import (
	openfgaconfpb "github.com/Servora-Kit/servora/api/gen/go/servora/security/authz/openfga/v1"
	logger "github.com/Servora-Kit/servora/obs/logging"
)

// NewClientOptional creates an OpenFGA client when the given configuration
// contains valid OpenFGA settings, returning nil (instead of an error) when
// the component is not configured or initialisation fails. This allows
// services to start without OpenFGA for local development or environments
// where authorisation is not required.
func NewClientOptional(cfg *openfgaconfpb.Config, l logger.Logger, opts ...ClientOption) *Client {
	if cfg == nil || cfg.ApiUrl == "" || cfg.StoreId == "" {
		logger.For(l, "security/authz/openfga").
			Info("OpenFGA not configured, authorization checks disabled")
		return nil
	}
	c, err := NewClient(cfg, opts...)
	if err != nil {
		logger.For(l, "security/authz/openfga").
			Warnf("failed to create OpenFGA client: %v", err)
		return nil
	}
	return c
}
