package openfga

import (
	conf "github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	"github.com/Servora-Kit/servora/pkg/logger"
)

// NewClientOptional creates an OpenFGA client when the app configuration
// contains valid OpenFGA settings, returning nil (instead of an error) when
// the component is not configured or initialisation fails. This allows
// services to start without OpenFGA for local development or environments
// where authorisation is not required.
func NewClientOptional(cfg *conf.App, l logger.Logger) *Client {
	if cfg.Openfga == nil || cfg.Openfga.ApiUrl == "" || cfg.Openfga.StoreId == "" {
		logger.For(l, "openfga/pkg").
			Info("OpenFGA not configured, authorization checks disabled")
		return nil
	}
	c, err := NewClient(cfg.Openfga)
	if err != nil {
		logger.For(l, "openfga/pkg").
			Warnf("failed to create OpenFGA client: %v", err)
		return nil
	}
	return c
}
