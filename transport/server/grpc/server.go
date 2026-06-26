package grpc

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	kgrpc "github.com/go-kratos/kratos/v3/transport/grpc"

	svrtls "github.com/Servora-Kit/servora/security/tls"
	"github.com/Servora-Kit/servora/transport/server/endpoint"
)

func NewServer(opts ...ServerOption) *kgrpc.Server {
	o := &serverOptions{}
	for _, opt := range opts {
		opt(o)
	}

	var serverOpts []kgrpc.ServerOption

	if len(o.middleware) > 0 {
		serverOpts = append(serverOpts, kgrpc.Middleware(o.middleware...))
	}

	if o.conf != nil {
		listen := o.conf.GetListen()
		if network := strings.TrimSpace(listen.GetNetwork()); network != "" {
			serverOpts = append(serverOpts, kgrpc.Network(network))
		}
		bindAddr := strings.TrimSpace(listen.GetAddr())
		if bindAddr != "" {
			serverOpts = append(serverOpts, kgrpc.Address(bindAddr))
		}
		if timeout := listen.GetTimeout(); timeout != nil {
			serverOpts = append(serverOpts, kgrpc.Timeout(timeout.AsDuration()))
		}
		if tlsCfg := svrtls.MustBuildServerTLS(o.conf.GetTls()); tlsCfg != nil {
			serverOpts = append(serverOpts, kgrpc.TLSConfig(tlsCfg))
		}

		advertiseEndpoint := ""
		advertiseHost := ""
		if adv := o.conf.GetAdvertise(); adv != nil {
			advertiseEndpoint = adv.GetEndpoint()
			advertiseHost = adv.GetHost()
		}

		secure := o.conf.GetTls() != nil && o.conf.GetTls().GetEnable()
		scheme := "grpc"
		if secure {
			scheme = "grpcs"
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
			panic(fmt.Sprintf("resolve grpc registry endpoint: %v", err))
		}
		if ep != nil {
			serverOpts = append(serverOpts, kgrpc.Endpoint(ep))
		}
	}

	srv := kgrpc.NewServer(serverOpts...)

	for _, reg := range o.registrars {
		reg(srv)
	}

	return srv
}
