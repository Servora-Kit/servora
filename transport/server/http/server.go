package http

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	_ "github.com/go-kratos/kratos/contrib/encoding/json/v3"
	_ "github.com/go-kratos/kratos/v3/encoding/protojson"
	khttp "github.com/go-kratos/kratos/v3/transport/http"

	svrtls "github.com/Servora-Kit/servora/security/tls"
	"github.com/Servora-Kit/servora/transport/server/endpoint"
	"github.com/Servora-Kit/servora/transport/server/http/cors"
	"github.com/Servora-Kit/servora/transport/server/http/swagger"
)

func NewServer(opts ...ServerOption) *khttp.Server {
	o := &serverOptions{}
	for _, opt := range opts {
		opt(o)
	}

	var serverOpts []khttp.ServerOption

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
		if tlsCfg := svrtls.MustBuildServerTLS(o.conf.GetTls()); tlsCfg != nil {
			serverOpts = append(serverOpts, khttp.TLSConfig(tlsCfg))
		}

		advertiseEndpoint := ""
		advertiseHost := ""
		if adv := o.conf.GetAdvertise(); adv != nil {
			advertiseEndpoint = adv.GetEndpoint()
			advertiseHost = adv.GetHost()
		}

		secure := o.conf.GetTls() != nil && o.conf.GetTls().GetEnable()
		scheme := "http"
		if secure {
			scheme = "https"
		}
		q := url.Values{}
		q.Set("isSecure", strconv.FormatBool(secure))

		ep, err := endpoint.ResolveRegistry(endpoint.RegistryInput{
			Scheme:   scheme,
			BindAddr: bindAddr,
			Endpoint: advertiseEndpoint,
			Host:     advertiseHost,
			Query:    q,
		})
		if err != nil {
			panic(fmt.Sprintf("resolve http registry endpoint: %v", err))
		}
		if ep != nil {
			serverOpts = append(serverOpts, khttp.Endpoint(ep))
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
