package audit

import (
	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/infra/broker"
	"github.com/Servora-Kit/servora/obs/logging"
)

// NewRecorderOptional creates a Recorder from App config. When audit is disabled
// or emitter_type is unrecognised, a NoopEmitter-backed Recorder is returned so
// callers never need a nil check.
//
// Follows the optional-initialisation pattern of pkg/openfga.NewClientOptional.
func NewRecorderOptional(cfg *conf.App, b broker.Broker, l logger.Logger) *Recorder {
	serviceName := ""
	if cfg != nil {
		serviceName = cfg.GetName()
	}

	auditCfg := cfg.GetAudit()
	if auditCfg == nil || !auditCfg.GetEnabled() {
		return NewRecorder(NewNoopEmitter(), serviceName)
	}

	auditServiceName := auditCfg.GetServiceName()
	if auditServiceName != "" {
		serviceName = auditServiceName
	}

	var emitter Emitter
	switch auditCfg.GetEmitterType() {
	case "broker":
		if b == nil {
			// Broker not configured — fall back to log emitter.
			emitter = NewLogEmitter(l)
		} else {
			emitter = NewBrokerEmitter(b, auditCfg.GetTopic(), l)
		}
	case "log":
		emitter = NewLogEmitter(l)
	default:
		emitter = NewNoopEmitter()
	}

	return NewRecorder(emitter, serviceName)
}
