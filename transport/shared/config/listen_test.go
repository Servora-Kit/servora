package config

import (
	"testing"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestParseListenConfig(t *testing.T) {
	tenSeconds := durationpb.New(10e9)

	tests := []struct {
		name    string
		listen  *conf.Server_Listen
		want    ListenConfig
	}{
		{
			name:   "nil input returns zero value",
			listen: nil,
			want:   ListenConfig{},
		},
		{
			name:   "empty proto returns zero value",
			listen: &conf.Server_Listen{},
			want:   ListenConfig{},
		},
		{
			name: "whitespace-only fields are trimmed to empty",
			listen: &conf.Server_Listen{
				Network: "  ",
				Addr:    "\t",
			},
			want: ListenConfig{},
		},
		{
			name: "all fields populated",
			listen: &conf.Server_Listen{
				Network: " tcp ",
				Addr:    " :8080 ",
				Timeout: tenSeconds,
			},
			want: ListenConfig{
				Network: "tcp",
				Addr:    ":8080",
				Timeout: tenSeconds,
			},
		},
		{
			name: "addr only",
			listen: &conf.Server_Listen{
				Addr: "0.0.0.0:9000",
			},
			want: ListenConfig{
				Addr: "0.0.0.0:9000",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseListenConfig(tc.listen)
			if got.Network != tc.want.Network {
				t.Errorf("Network: got %q, want %q", got.Network, tc.want.Network)
			}
			if got.Addr != tc.want.Addr {
				t.Errorf("Addr: got %q, want %q", got.Addr, tc.want.Addr)
			}
			if (got.Timeout == nil) != (tc.want.Timeout == nil) {
				t.Errorf("Timeout nil mismatch: got %v, want %v", got.Timeout, tc.want.Timeout)
			}
		})
	}
}
