package defaultcascade

import (
	"testing"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func md(m interface{ ProtoReflect() protoreflect.Message }) protoreflect.MessageDescriptor {
	return m.ProtoReflect().Descriptor()
}

func TestNeedsApplyDefaults(t *testing.T) {
	tests := []struct {
		name string
		in   protoreflect.MessageDescriptor
		want bool
	}{
		{"Server.Listen 直接含 default(network=tcp)", md(&corev1.Server_Listen{}), true},
		{"Server.HTTP 传递性需要(经 Listen)", md(&corev1.Server_HTTP{}), true},
		{"Server 容器传递性需要", md(&corev1.Server{}), true},
		{"Bootstrap 容器传递性需要", md(&corev1.Bootstrap{}), true},
		{"App 无 default 子树为 false", md(&corev1.App{}), false},
		{"Config 配置中心 oneof 为 false", md(&corev1.Config{}), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsApplyDefaults(tt.in, map[protoreflect.FullName]bool{})
			if got != tt.want {
				t.Fatalf("NeedsApplyDefaults(%s) = %v, want %v", tt.in.FullName(), got, tt.want)
			}
		})
	}
}

func TestNeedsApplyDefaults_SharedTypeDiamond(t *testing.T) {
	// Server_Listen 被 Server.HTTP.listen 与 Server.GRPC.listen 共同引用，
	// 同一个 memo 内两条路径都必须判 true（验证 diamond 不被 cycle-guard 误判 false）。
	memo := map[protoreflect.FullName]bool{}
	if !NeedsApplyDefaults(md(&corev1.Server_HTTP{}), memo) {
		t.Fatal("Server_HTTP 应为 true")
	}
	if !NeedsApplyDefaults(md(&corev1.Server_GRPC{}), memo) {
		t.Fatal("共享 memo 下 Server_GRPC 仍应为 true（diamond 不得被误判）")
	}
}

func TestNeedsApplyDefaults_NilSafe(t *testing.T) {
	if NeedsApplyDefaults(nil, map[protoreflect.FullName]bool{}) {
		t.Fatal("nil 描述符应为 false")
	}
}
