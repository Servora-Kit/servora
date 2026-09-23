package tracing

import (
	"testing"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"google.golang.org/protobuf/proto"
)

func TestResolveTraceRuntimeConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *corev1.Trace
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
			cfg: &corev1.Trace{
				Endpoint:      "otel.example.internal:4317",
				Insecure:      true,
				SamplingRatio: proto.Float64(0.25),
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
			name: "explicit zero overrides environment",
			env:  "dev",
			cfg:  &corev1.Trace{SamplingRatio: proto.Float64(0)},
			want: traceRuntimeConfig{samplingRatio: 0},
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

func TestInitTracerProviderRejectsInvalidSamplingRatio(t *testing.T) {
	for _, ratio := range []float64{-0.1, 1.1} {
		cleanup, err := InitTracerProvider(&corev1.Trace{SamplingRatio: proto.Float64(ratio)}, "example", "prod")
		if err == nil || cleanup != nil {
			t.Fatalf("sampling ratio %v: cleanup=%t error=%v, want rejection", ratio, cleanup != nil, err)
		}
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
