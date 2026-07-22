package main

import (
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"

	annotations "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestGenerateGoCompanionForValidResource(t *testing.T) {
	t.Parallel()

	plugin, err := runGenerator(t, validCRUDFile(), "go")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	files := generatedFiles(plugin)
	content := files["test/v1/user_crud.pb.go"]
	if content == "" {
		t.Fatalf("missing Go companion; files=%v", sortedKeys(files))
	}
	for _, expected := range []string{
		"func UserCRUDDescriptor() protoreflect.MessageDescriptor",
		"type UserFieldPath struct",
		"var UserFields = struct",
		"func ParseUserName(value string) (UserName, error)",
		"DisplayName",
		"func (name UserName) Validate() error",
		"func (name UserName) Format() (string, error)",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("generated Go missing %q:\n%s", expected, content)
		}
	}
	for _, forbidden := range []string{"core/crud", "MethodPlan", "UpdateUserCommand", "ToPO"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("generated Go contains forbidden %q", forbidden)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "user_crud.pb.go", content, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, content)
	}
}

func TestGenerateTypeScriptCompanionIsLightweight(t *testing.T) {
	t.Parallel()

	plugin, err := runGenerator(t, validCRUDFile(), "ts")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	content := generatedFiles(plugin)["test/v1/user.crud.ts"]
	for _, expected := range []string{"ResourceNameError", "export const UserName", "tryParse", "export const UserFields", "export const UserUpdateFields", "segments[1]!"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("generated TypeScript missing %q:\n%s", expected, content)
		}
	}
	for _, forbidden := range []string{"class UserService", "interface User", "fetch(", "axios"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("generated TypeScript contains transport/type output %q", forbidden)
		}
	}
	if strings.HasSuffix(content, "\n\n") {
		t.Fatalf("generated TypeScript has a blank line at EOF:\n%s", content)
	}
	for lineNumber, line := range strings.Split(content, "\n") {
		if strings.TrimRight(line, " \t") != line {
			t.Fatalf("generated TypeScript line %d has trailing whitespace: %q", lineNumber+1, line)
		}
	}

}

func TestGenerateRejectsInvalidCRUDContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*descriptorpb.FileDescriptorProto)
		want   string
	}{
		{
			name: "missing identifier",
			mutate: func(file *descriptorpb.FileDescriptorProto) {
				fieldByName(messageByName(file, "User"), "name").Options = nil
			},
			want: "missing IDENTIFIER",
		},
		{
			name: "missing lifecycle behavior",
			mutate: func(file *descriptorpb.FileDescriptorProto) {
				fieldByName(messageByName(file, "User"), "display_name").Options = nil
			},
			want: "User.display_name",
		},
		{
			name: "writable scalar without presence",
			mutate: func(file *descriptorpb.FileDescriptorProto) {
				removeProto3OptionalPresence(messageByName(file, "User"), "display_name")
			},
			want: "requires explicit presence",
		},
		{
			name: "create missing id",
			mutate: func(file *descriptorpb.FileDescriptorProto) {
				removeField(messageByName(file, "CreateUserRequest"), "user_id")
			},
			want: "request field user_id is required",
		},
		{
			name: "delete wrong response",
			mutate: func(file *descriptorpb.FileDescriptorProto) {
				methodByName(file, "DeleteUser").OutputType = proto.String(".test.v1.Other")
			},
			want: "Delete response must be",
		},
		{
			name: "total not paired",
			mutate: func(file *descriptorpb.FileDescriptorProto) {
				removeField(messageByName(file, "ListUsersResponse"), "total_size")
			},
			want: "include_total and total_size must be declared together",
		},
		{
			name: "allow missing wrong type",
			mutate: func(file *descriptorpb.FileDescriptorProto) {
				fieldByName(messageByName(file, "UpdateUserRequest"), "allow_missing").Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
			},
			want: "allow_missing must be an OPTIONAL singular bool",
		},
		{
			name: "ambiguous pattern skeleton",
			mutate: func(file *descriptorpb.FileDescriptorProto) {
				resource := resourceOption(messageByName(file, "User"))
				resource.Pattern = append(resource.Pattern, "tenants/{other_tenant}/users/{other_user}")
			},
			want: "ambiguous skeleton",
		},
		{
			name: "generated field symbol collision",
			mutate: func(file *descriptorpb.FileDescriptorProto) {
				profile := message("Profile")
				addProto3Optional(profile, annotatedField("bio", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_OPTIONAL))
				user := messageByName(file, "User")
				user.Field = append(user.Field, annotatedField("profile", 101, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".test.v1.Profile", annotations.FieldBehavior_OPTIONAL))
				addProto3Optional(user, annotatedField("profile_bio", 102, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_OPTIONAL))
				file.MessageType = append(file.MessageType, profile)
			},
			want: "generated field symbol ProfileBio collides for profile.bio and profile_bio",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			file := validCRUDFile()
			test.mutate(file)
			_, err := runGenerator(t, file, "go")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("generate error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestGenerateAcceptsEmptyAndResourceDeleteResponses(t *testing.T) {
	t.Parallel()

	for _, output := range []string{".google.protobuf.Empty", ".test.v1.User"} {
		file := validCRUDFile()
		methodByName(file, "DeleteUser").OutputType = proto.String(output)
		if _, err := runGenerator(t, file, "go"); err != nil {
			t.Fatalf("DeleteUser output %s: %v", output, err)
		}
	}
}

func TestGenerateMultiPatternHelpersWithoutChoosingCreateListDefault(t *testing.T) {
	t.Parallel()

	file := validCRUDFile()
	resource := resourceOption(messageByName(file, "User"))
	resource.Pattern = []string{"tenants/{tenant}/users/{user}", "organizations/{organization}/users/{user}"}
	plugin, err := runGenerator(t, file, "go")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	content := generatedFiles(plugin)["test/v1/user_crud.pb.go"]
	for _, constructor := range []string{"NewUserNameForTenantsUsers", "NewUserNameForOrganizationsUsers"} {
		if !strings.Contains(content, constructor) {
			t.Fatalf("missing multi-pattern constructor %s", constructor)
		}
	}
}

func runGenerator(t *testing.T, file *descriptorpb.FileDescriptorProto, target string) (*protogen.Plugin, error) {
	t.Helper()
	request := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      descriptorDependencies(),
		FileToGenerate: []string{file.GetName()},
		Parameter:      proto.String("paths=source_relative"),
	}
	request.ProtoFile = append(request.ProtoFile, file)
	plugin, err := protogen.Options{}.New(request)
	if err != nil {
		t.Fatalf("protogen.Options.New: %v", err)
	}
	return plugin, generate(plugin, target)
}

func generatedFiles(plugin *protogen.Plugin) map[string]string {
	files := make(map[string]string)
	for _, file := range plugin.Response().File {
		files[file.GetName()] = file.GetContent()
	}
	return files
}

func descriptorDependencies() []*descriptorpb.FileDescriptorProto {
	roots := []protoreflect.FileDescriptor{
		annotations.File_google_api_resource_proto,
		annotations.File_google_api_field_behavior_proto,
		emptypb.File_google_protobuf_empty_proto,
		fieldmaskpb.File_google_protobuf_field_mask_proto,
		timestamppb.File_google_protobuf_timestamp_proto,
	}
	seen := make(map[string]bool)
	var result []*descriptorpb.FileDescriptorProto
	var visit func(protoreflect.FileDescriptor)
	visit = func(file protoreflect.FileDescriptor) {
		if seen[file.Path()] {
			return
		}
		seen[file.Path()] = true
		imports := file.Imports()
		for index := range imports.Len() {
			visit(imports.Get(index).FileDescriptor)
		}
		result = append(result, protodesc.ToFileDescriptorProto(file))
	}
	for _, root := range roots {
		visit(root)
	}
	return result
}

func validCRUDFile() *descriptorpb.FileDescriptorProto {
	file := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("test/v1/user.proto"),
		Package:    proto.String("test.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/api/resource.proto", "google/api/field_behavior.proto", "google/protobuf/empty.proto", "google/protobuf/field_mask.proto", "google/protobuf/timestamp.proto"},
		Options:    &descriptorpb.FileOptions{GoPackage: proto.String("example.com/test/v1;testv1")},
	}
	user := &descriptorpb.DescriptorProto{Name: proto.String("User"), Options: &descriptorpb.MessageOptions{}}
	proto.SetExtension(user.Options, annotations.E_Resource, &annotations.ResourceDescriptor{
		Type: "test.dev/User", Pattern: []string{"tenants/{tenant}/users/{user}"}, Singular: "user", Plural: "users",
	})
	user.Field = append(user.Field, annotatedField("name", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_IDENTIFIER))
	addProto3Optional(user, annotatedField("display_name", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_OPTIONAL))
	addProto3Optional(user, annotatedField("email", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_REQUIRED))
	addProto3Optional(user, annotatedField("tenant_plan", 4, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_OPTIONAL, annotations.FieldBehavior_IMMUTABLE))
	addProto3Optional(user, annotatedField("temporary_password", 5, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_OPTIONAL, annotations.FieldBehavior_INPUT_ONLY))
	user.Field = append(user.Field, field("etag", 6, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""))
	user.Field = append(user.Field, annotatedField("create_time", 100, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".google.protobuf.Timestamp", annotations.FieldBehavior_OUTPUT_ONLY))

	getRequest := message("GetUserRequest", annotatedField("name", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_REQUIRED))
	listRequest := message("ListUsersRequest", annotatedField("parent", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_REQUIRED))
	listRequest.Field = append(listRequest.Field,
		annotatedField("page_size", 2, descriptorpb.FieldDescriptorProto_TYPE_INT32, "", annotations.FieldBehavior_OPTIONAL),
		annotatedField("page_token", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_OPTIONAL),
		annotatedField("skip", 4, descriptorpb.FieldDescriptorProto_TYPE_INT64, "", annotations.FieldBehavior_OPTIONAL),
		annotatedField("filter", 5, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_OPTIONAL),
		annotatedField("order_by", 6, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_OPTIONAL),
		annotatedField("include_total", 7, descriptorpb.FieldDescriptorProto_TYPE_BOOL, "", annotations.FieldBehavior_OPTIONAL),
	)
	listResponse := message("ListUsersResponse",
		field("users", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".test.v1.User"),
		field("next_page_token", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
	)
	addProto3Optional(listResponse, field("total_size", 3, descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""))
	createRequest := message("CreateUserRequest",
		annotatedField("parent", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_REQUIRED),
		annotatedField("user_id", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_REQUIRED),
		annotatedField("user", 3, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".test.v1.User", annotations.FieldBehavior_REQUIRED),
	)
	updateRequest := message("UpdateUserRequest",
		annotatedField("user", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".test.v1.User", annotations.FieldBehavior_REQUIRED),
		annotatedField("update_mask", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".google.protobuf.FieldMask", annotations.FieldBehavior_OPTIONAL),
		annotatedField("allow_missing", 3, descriptorpb.FieldDescriptorProto_TYPE_BOOL, "", annotations.FieldBehavior_OPTIONAL),
	)
	deleteRequest := message("DeleteUserRequest",
		annotatedField("name", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_REQUIRED),
		annotatedField("etag", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_OPTIONAL),
		annotatedField("allow_missing", 3, descriptorpb.FieldDescriptorProto_TYPE_BOOL, "", annotations.FieldBehavior_OPTIONAL),
	)
	other := message("Other")
	file.MessageType = []*descriptorpb.DescriptorProto{user, getRequest, listRequest, listResponse, createRequest, updateRequest, deleteRequest, other}
	file.Service = []*descriptorpb.ServiceDescriptorProto{{
		Name: proto.String("UserService"),
		Method: []*descriptorpb.MethodDescriptorProto{
			method("GetUser", ".test.v1.GetUserRequest", ".test.v1.User"),
			method("ListUsers", ".test.v1.ListUsersRequest", ".test.v1.ListUsersResponse"),
			method("CreateUser", ".test.v1.CreateUserRequest", ".test.v1.User"),
			method("UpdateUser", ".test.v1.UpdateUserRequest", ".test.v1.User"),
			method("DeleteUser", ".test.v1.DeleteUserRequest", ".test.v1.User"),
		},
	}}
	return file
}

func annotatedField(name string, number int32, fieldType descriptorpb.FieldDescriptorProto_Type, typeName string, behaviors ...annotations.FieldBehavior) *descriptorpb.FieldDescriptorProto {
	result := field(name, number, fieldType, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, typeName)
	result.Options = &descriptorpb.FieldOptions{}
	proto.SetExtension(result.Options, annotations.E_FieldBehavior, behaviors)
	return result
}

func field(name string, number int32, fieldType descriptorpb.FieldDescriptorProto_Type, label descriptorpb.FieldDescriptorProto_Label, typeName string) *descriptorpb.FieldDescriptorProto {
	result := &descriptorpb.FieldDescriptorProto{Name: proto.String(name), Number: proto.Int32(number), Type: fieldType.Enum(), Label: label.Enum()}
	if typeName != "" {
		result.TypeName = proto.String(typeName)
	}
	return result
}

func addProto3Optional(message *descriptorpb.DescriptorProto, field *descriptorpb.FieldDescriptorProto) {
	index := int32(len(message.OneofDecl))
	message.OneofDecl = append(message.OneofDecl, &descriptorpb.OneofDescriptorProto{Name: proto.String("_" + field.GetName())})
	field.Proto3Optional = proto.Bool(true)
	field.OneofIndex = proto.Int32(index)
	message.Field = append(message.Field, field)
}

func message(name string, fields ...*descriptorpb.FieldDescriptorProto) *descriptorpb.DescriptorProto {
	return &descriptorpb.DescriptorProto{Name: proto.String(name), Field: fields}
}

func method(name, input, output string) *descriptorpb.MethodDescriptorProto {
	return &descriptorpb.MethodDescriptorProto{Name: proto.String(name), InputType: proto.String(input), OutputType: proto.String(output)}
}

func messageByName(file *descriptorpb.FileDescriptorProto, name string) *descriptorpb.DescriptorProto {
	for _, message := range file.MessageType {
		if message.GetName() == name {
			return message
		}
	}
	panic("message not found: " + name)
}

func fieldByName(message *descriptorpb.DescriptorProto, name string) *descriptorpb.FieldDescriptorProto {
	for _, field := range message.Field {
		if field.GetName() == name {
			return field
		}
	}
	panic("field not found: " + name)
}

func removeField(message *descriptorpb.DescriptorProto, name string) {
	for index, field := range message.Field {
		if field.GetName() != name {
			continue
		}
		if field.GetProto3Optional() {
			removeProto3OptionalPresence(message, name)
		}
		message.Field = append(message.Field[:index], message.Field[index+1:]...)
		return
	}
}

func removeProto3OptionalPresence(message *descriptorpb.DescriptorProto, name string) {
	field := fieldByName(message, name)
	index := field.GetOneofIndex()
	field.Proto3Optional = nil
	field.OneofIndex = nil
	message.OneofDecl = append(message.OneofDecl[:index], message.OneofDecl[index+1:]...)
	for _, candidate := range message.Field {
		if candidate.OneofIndex != nil && candidate.GetOneofIndex() > index {
			candidate.OneofIndex = proto.Int32(candidate.GetOneofIndex() - 1)
		}
	}
}

func methodByName(file *descriptorpb.FileDescriptorProto, name string) *descriptorpb.MethodDescriptorProto {
	for _, service := range file.Service {
		for _, method := range service.Method {
			if method.GetName() == name {
				return method
			}
		}
	}
	panic("method not found: " + name)
}

func resourceOption(message *descriptorpb.DescriptorProto) *annotations.ResourceDescriptor {
	return proto.GetExtension(message.Options, annotations.E_Resource).(*annotations.ResourceDescriptor)
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
