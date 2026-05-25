package protoreach

import (
	"testing"

	confv1 "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func md(m interface{ ProtoReflect() protoreflect.Message }) protoreflect.MessageDescriptor {
	return m.ProtoReflect().Descriptor()
}

func fieldHasDefault(fd protoreflect.FieldDescriptor) bool {
	opts := fd.Options()
	if opts == nil || !proto.HasExtension(opts, confv1.E_Field) {
		return false
	}
	r, _ := proto.GetExtension(opts, confv1.E_Field).(*confv1.FieldRule)
	return r != nil && r.GetDefault() != ""
}

func fieldHasRequired(fd protoreflect.FieldDescriptor) bool {
	opts := fd.Options()
	if opts == nil || !proto.HasExtension(opts, confv1.E_Field) {
		return false
	}
	r, _ := proto.GetExtension(opts, confv1.E_Field).(*confv1.FieldRule)
	return r != nil && r.GetRequired()
}

func TestNeedsCascade_Defaults(t *testing.T) {
	tests := []struct {
		name string
		in   protoreflect.MessageDescriptor
		want bool
	}{
		{"Server.Listen has default(network=tcp)", md(&corev1.Server_Listen{}), true},
		{"Server.HTTP transitive via Listen", md(&corev1.Server_HTTP{}), true},
		{"Server container transitive", md(&corev1.Server{}), true},
		{"Bootstrap container transitive", md(&corev1.Bootstrap{}), true},
		{"App has no default subtree", md(&corev1.App{}), false},
		{"Source oneof has no defaults", md(&corev1.Source{}), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsCascade(tt.in, fieldHasDefault, map[protoreflect.FullName]bool{})
			if got != tt.want {
				t.Fatalf("NeedsCascade(%s, default) = %v, want %v", tt.in.FullName(), got, tt.want)
			}
		})
	}
}

func TestNeedsCascade_Required(t *testing.T) {
	tests := []struct {
		name string
		in   protoreflect.MessageDescriptor
		want bool
	}{
		{"Server.Listen has required(addr)", md(&corev1.Server_Listen{}), true},
		{"Server.HTTP transitive via Listen", md(&corev1.Server_HTTP{}), true},
		{"Server container transitive", md(&corev1.Server{}), true},
		{"Bootstrap container transitive", md(&corev1.Bootstrap{}), true},
		{"App has no required subtree", md(&corev1.App{}), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsCascade(tt.in, fieldHasRequired, map[protoreflect.FullName]bool{})
			if got != tt.want {
				t.Fatalf("NeedsCascade(%s, required) = %v, want %v", tt.in.FullName(), got, tt.want)
			}
		})
	}
}

func TestNeedsCascade_SharedTypeDiamond(t *testing.T) {
	memo := map[protoreflect.FullName]bool{}
	if !NeedsCascade(md(&corev1.Server_HTTP{}), fieldHasDefault, memo) {
		t.Fatal("Server_HTTP should be true")
	}
	if !NeedsCascade(md(&corev1.Server_GRPC{}), fieldHasDefault, memo) {
		t.Fatal("shared memo: Server_GRPC should still be true (diamond must not be misjudged)")
	}
}

func TestNeedsCascade_NilSafe(t *testing.T) {
	if NeedsCascade(nil, fieldHasDefault, map[protoreflect.FullName]bool{}) {
		t.Fatal("nil descriptor should be false")
	}
}
