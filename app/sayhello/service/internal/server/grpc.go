package server

import (
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"

	"github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	sayhellov1 "github.com/Servora-Kit/servora/api/gen/go/servora/sayhello/service/v1"
	"github.com/Servora-Kit/servora/app/sayhello/service/internal/service"
	"github.com/Servora-Kit/servora/pkg/audit"
	"github.com/Servora-Kit/servora/pkg/governance/telemetry"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/transport/server/grpc"
	"github.com/Servora-Kit/servora/pkg/transport/server/middleware"
)

func NewGRPCServer(c *conf.Server, trace *conf.Trace, mtc *telemetry.Metrics, recorder *audit.Recorder, l logger.Logger, sayhello *service.SayHelloService) *kgrpc.Server {
	grpcLogger := logger.With(l, "grpc/server/sayhello")

	mw := middleware.NewChainBuilder(grpcLogger).
		WithTrace(trace).
		WithMetrics(mtc).
		WithoutRateLimit().
		Build()

	// Emit an audit event for every Hello call to demonstrate E2E audit pipeline.
	auditMw := audit.Audit(
		audit.WithRecorder(recorder),
		audit.WithRules(map[string]audit.Rule{
			sayhellov1.SayHelloService_Hello_FullMethodName: {
				EventType:     audit.EventTypeResourceMutation,
				TargetType:    "greeting",
				RecordOnError: true,
			},
		}),
	)
	mw = append(mw, auditMw)

	opts := []grpc.ServerOption{
		grpc.WithLogger(grpcLogger),
		grpc.WithMiddleware(mw...),
		grpc.WithServices(
			func(s *kgrpc.Server) { sayhellov1.RegisterSayHelloServiceServer(s, sayhello) },
		),
	}
	if c != nil && c.Grpc != nil {
		opts = append(opts, grpc.WithConfig(c.Grpc))
	}

	return grpc.NewServer(opts...)
}
