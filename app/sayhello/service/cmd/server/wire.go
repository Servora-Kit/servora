//go:build wireinject
// +build wireinject

package main

import (
	"context"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/app/sayhello/service/internal/server"
	"github.com/Servora-Kit/servora/app/sayhello/service/internal/service"
	"github.com/Servora-Kit/servora/pkg/audit"
	"github.com/Servora-Kit/servora/pkg/bootstrap"
	"github.com/Servora-Kit/servora/pkg/broker"
	brokerkafka "github.com/Servora-Kit/servora/pkg/broker/kafka"
	"github.com/Servora-Kit/servora/pkg/logger"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// newKafkaBroker wraps NewBrokerOptional with a background context for Wire injection.
func newKafkaBroker(cfg *conf.Data, l logger.Logger) broker.Broker {
	return brokerkafka.NewBrokerOptional(context.Background(), cfg, l)
}

// newAuditRecorder creates an audit Recorder from App config and a Broker.
func newAuditRecorder(cfg *conf.App, b broker.Broker, l logger.Logger) *audit.Recorder {
	return audit.NewRecorderOptional(cfg, b, l)
}

func wireApp(*conf.Server, *conf.Registry, *conf.Data, *conf.App, *conf.Trace, *conf.Metrics, bootstrap.SvcIdentity, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(
		newKafkaBroker,
		newAuditRecorder,
		service.ProviderSet,
		server.ProviderSet,
		newApp,
	))
}
