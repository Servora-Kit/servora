package config

import (
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

func NormalizeDuration(v *durationpb.Duration, fallback time.Duration) time.Duration {
	if v == nil {
		return fallback
	}
	d := v.AsDuration()
	if d <= 0 {
		return fallback
	}
	return d
}

func NormalizeEndpoint(v string, fallback string) string {
	if endpoint := strings.TrimSpace(v); endpoint != "" {
		return endpoint
	}
	return fallback
}
