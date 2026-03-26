package telemetry

import (
	"testing"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
)

func TestResolveTraceRuntimeConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *conf.Trace
		env  string
		want traceRuntimeConfig
	}{
		{
			name: "defaults to dev full sampling",
			env:  "dev",
			want: traceRuntimeConfig{samplingRatio: defaultDevSamplingRatio},
		},
		{
			name: "defaults to prod reduced sampling",
			env:  "prod",
			want: traceRuntimeConfig{samplingRatio: defaultProdSamplingRatio},
		},
		{
			name: "uses explicit values",
			env:  "prod",
			cfg: &conf.Trace{
				Endpoint:      "otel.example.internal:4317",
				Insecure:      true,
				SamplingRatio: 0.25,
				CaPath:        "/etc/certs/otel-ca.pem",
			},
			want: traceRuntimeConfig{
				endpoint:      "otel.example.internal:4317",
				insecure:      true,
				samplingRatio: 0.25,
				caPath:        "/etc/certs/otel-ca.pem",
			},
		},
		{
			name: "falls back when sampling ratio invalid",
			env:  "production",
			cfg: &conf.Trace{
				SamplingRatio: 1.5,
			},
			want: traceRuntimeConfig{samplingRatio: defaultProdSamplingRatio},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveTraceRuntimeConfig(tt.cfg, tt.env)
			if got != tt.want {
				t.Fatalf("resolveTraceRuntimeConfig() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNewTraceExporterOptionsRejectsConflictingTLSModes(t *testing.T) {
	_, err := newTraceExporterOptions(traceRuntimeConfig{
		endpoint: "otel.example.internal:4317",
		insecure: true,
		caPath:   "/etc/certs/otel-ca.pem",
	})
	if err == nil {
		t.Fatal("expected conflicting insecure and ca_path settings to fail")
	}
}
