package http

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/Servora-Kit/servora/transport/server/http/cors"
	"github.com/Servora-Kit/servora/transport/server/http/swagger"
	sharedendpoint "github.com/Servora-Kit/servora/transport/shared/endpoint"
	sharedtls "github.com/Servora-Kit/servora/transport/shared/tls"
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
		listen := o.conf.GetListen()
		if network := strings.TrimSpace(listen.GetNetwork()); network != "" {
			serverOpts = append(serverOpts, khttp.Network(network))
		}
		bindAddr := strings.TrimSpace(listen.GetAddr())
		if bindAddr != "" {
			serverOpts = append(serverOpts, khttp.Address(bindAddr))
		}
		if timeout := listen.GetTimeout(); timeout != nil {
			serverOpts = append(serverOpts, khttp.Timeout(timeout.AsDuration()))
		}
		if tlsCfg := sharedtls.MustBuildServerTLS(o.conf.GetTls()); tlsCfg != nil {
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

		endpoint, err := sharedendpoint.ResolveRegistryEndpoint(sharedendpoint.RegistryEndpointInput{
			Scheme:   scheme,
			BindAddr: bindAddr,
			Endpoint: registryEndpoint,
			Host:     registryHost,
			Query:    q,
		})
		if err != nil {
			panic(fmt.Sprintf("resolve http registry endpoint: %v", err))
		}
		if endpoint != nil {
			serverOpts = append(serverOpts, khttp.Endpoint(endpoint))
		}
	}

	if cors.IsEnabled(o.cors) {
		serverOpts = append(serverOpts, khttp.Filter(cors.Middleware(o.cors)))
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
