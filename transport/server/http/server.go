package http

import (
	"fmt"
	"net/url"
	"strconv"

	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/Servora-Kit/servora/platform/swagger"
	"github.com/Servora-Kit/servora/transport/server"
	svrmw "github.com/Servora-Kit/servora/transport/server/middleware"
	sharedconfig "github.com/Servora-Kit/servora/transport/shared/config"
	sharedendpoint "github.com/Servora-Kit/servora/transport/shared/endpoint"
)

func NewServer(opts ...ServerOption) *khttp.Server {
	o := &serverOptions{}
	for _, opt := range opts {
		opt(o)
	}

	var serverOpts []khttp.ServerOption

	if o.logger != nil {
		serverOpts = append(serverOpts, khttp.Logger(o.logger))
	}
	if len(o.middleware) > 0 {
		serverOpts = append(serverOpts, khttp.Middleware(o.middleware...))
	}

	if o.conf != nil {
		lc := sharedconfig.ParseListenConfig(o.conf.GetListen())
		if lc.Network != "" {
			serverOpts = append(serverOpts, khttp.Network(lc.Network))
		}
		if lc.Addr != "" {
			serverOpts = append(serverOpts, khttp.Address(lc.Addr))
		}
		if lc.Timeout != nil {
			serverOpts = append(serverOpts, khttp.Timeout(lc.Timeout.AsDuration()))
		}
		if o.conf.Tls != nil && o.conf.Tls.Enable {
			tlsCfg := server.MustLoadTLS(o.conf.Tls)
			serverOpts = append(serverOpts, khttp.TLSConfig(tlsCfg))
		}

		registryEndpoint := ""
		registryHost := ""
		if reg := o.conf.GetRegistry(); reg != nil {
			registryEndpoint = reg.GetEndpoint()
			registryHost = reg.GetHost()
		}

		secure := o.conf.GetTls() != nil && o.conf.GetTls().GetEnable()
		scheme := "http"
		if secure {
			scheme = "https"
		}
		q := url.Values{}
		q.Set("isSecure", strconv.FormatBool(secure))

		endpoint, err := sharedendpoint.ResolveRegistryEndpoint(
			scheme,
			lc.Addr,
			registryEndpoint,
			registryHost,
			q,
		)
		if err != nil {
			panic(fmt.Sprintf("resolve http registry endpoint: %v", err))
		}
		if endpoint != nil {
			serverOpts = append(serverOpts, khttp.Endpoint(endpoint))
		}
	}

	if svrmw.IsEnabled(o.cors) {
		serverOpts = append(serverOpts, khttp.Filter(svrmw.Middleware(o.cors)))
	}

	srv := khttp.NewServer(serverOpts...)

	if o.metricsHandler != nil {
		srv.Handle("/metrics", o.metricsHandler)
	}

	if o.healthHandler != nil {
		srv.HandleFunc("/healthz", o.healthHandler.LivenessHandler())
		srv.HandleFunc("/readyz", o.healthHandler.ReadinessHandler())
	}

	if len(o.swaggerSpec) > 0 {
		swagger.Register(srv, o.swaggerSpec, o.swaggerOpts...)
	}

	for _, reg := range o.registrars {
		reg(srv)
	}

	return srv
}
