package bootstrap

import (
	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"github.com/google/wire"
)

// ProviderSet exposes stable bootstrap roots for application Wire graphs.
var ProviderSet = wire.NewSet(
	wire.FieldsOf(new(*Runtime), "Bootstrap", "Config", "Logger"),
	wire.FieldsOf(new(*corev1.Bootstrap), "Server", "Discovery", "Registry", "Data", "App", "Trace", "Metrics"),
)
