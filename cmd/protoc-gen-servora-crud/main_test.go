package main

import (
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"

	crudv1 "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
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
		`"github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"`,
		"func UserCRUDDescriptor() protoreflect.MessageDescriptor",
		"type UserFieldPath struct",
		"var UserFields = struct",
		"func ParseUserName(value string) (UserName, error)",
		"ErrorCrudErrorReasonInvalidResourceName",
		"DisplayName",
		"func (name UserName) Validate() error",
		"func (name UserName) Format() (string, error)",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("generated Go missing %q:\n%s", expected, content)
		}
	}
	for _, forbidden := range []string{"core/crud", "fmt.Errorf", "MethodPlan", "UpdateUserCommand", "ToPO", "StringMatch", "FilterPrefix", "FilterSuffix", "FilterContains", "LogicalType"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("generated Go contains forbidden %q", forbidden)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "user_crud.pb.go", content, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, content)
	}
}

func TestGeneratedUserNameReturnsStructuredInvalidResourceNameError(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"", "users/u-1", "tenants//users/u-1", "tenants/acme/users/", "tenants/acme/users/u-1/extra"} {
		_, err := examplev1.ParseUserName(raw)
		if !crudv1.IsCrudErrorReasonInvalidResourceName(err) {
			t.Fatalf("ParseUserName(%q) error = %v, want INVALID_RESOURCE_NAME", raw, err)
		}
	}
	for _, invalid := range []examplev1.UserName{
		examplev1.NewUserName("", "u-1"),
		examplev1.NewUserName("acme", ""),
		examplev1.NewUserName("ac/me", "u-1"),
		examplev1.NewUserName("acme", "u/1"),
	} {
		if err := invalid.Validate(); !crudv1.IsCrudErrorReasonInvalidResourceName(err) {
			t.Fatalf("Validate(%#v) error = %v, want INVALID_RESOURCE_NAME", invalid, err)
		}
		if _, err := invalid.Format(); !crudv1.IsCrudErrorReasonInvalidResourceName(err) {
			t.Fatalf("Format(%#v) error = %v, want INVALID_RESOURCE_NAME", invalid, err)
		}
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
	for _, forbidden := range []string{"class UserService", "interface User", "fetch(", "axios", "StringMatch", "filterPrefix", "filterSuffix", "filterContains", "logicalType", "logical_type"} {
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
				methodByName(file, "DeleteUser").OutputType = new(".test.v1.Other")
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
		methodByName(file, "DeleteUser").OutputType = new(output)
		if _, err := runGenerator(t, file, "go"); err != nil {
			t.Fatalf("DeleteUser output %s: %v", output, err)
		}
	}
}

func TestGenerateValidatesTopLevelContractsWithoutParent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*descriptorpb.FileDescriptorProto)
		want   string
	}{
		{name: "valid top-level Create and List"},
		{
			name: "Create still validates resource ID",
			mutate: func(file *descriptorpb.FileDescriptorProto) {
				removeField(messageByName(file, "CreateUserRequest"), "user_id")
			},
			want: "request field user_id is required",
		},
		{
			name: "List still validates pagination fields",
			mutate: func(file *descriptorpb.FileDescriptorProto) {
				fieldByName(messageByName(file, "ListUsersRequest"), "page_size").Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
			},
			want: "page_size must be an OPTIONAL singular int32",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			file := topLevelCRUDFile()
			if test.mutate != nil {
				test.mutate(file)
			}
			_, err := runGenerator(t, file, "go")
			if test.want == "" {
				if err != nil {
					t.Fatalf("generate top-level CRUD: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("generate error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestGenerateValidatesNestedCreateListParent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request string
		mutate  func(*descriptorpb.DescriptorProto)
	}{
		{
			name:    "Create missing parent",
			request: "CreateUserRequest",
			mutate:  func(request *descriptorpb.DescriptorProto) { removeField(request, "parent") },
		},
		{
			name:    "Create parent wrong type",
			request: "CreateUserRequest",
			mutate: func(request *descriptorpb.DescriptorProto) {
				fieldByName(request, "parent").Type = descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()
			},
		},
		{
			name:    "Create parent repeated",
			request: "CreateUserRequest",
			mutate: func(request *descriptorpb.DescriptorProto) {
				fieldByName(request, "parent").Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
			},
		},
		{
			name:    "Create parent map",
			request: "CreateUserRequest",
			mutate:  func(request *descriptorpb.DescriptorProto) { makeStringMapField(request, "parent") },
		},
		{
			name:    "Create parent not required",
			request: "CreateUserRequest",
			mutate: func(request *descriptorpb.DescriptorProto) {
				fieldByName(request, "parent").Options = &descriptorpb.FieldOptions{}
			},
		},
		{
			name:    "List missing parent",
			request: "ListUsersRequest",
			mutate:  func(request *descriptorpb.DescriptorProto) { removeField(request, "parent") },
		},
		{
			name:    "List parent wrong type",
			request: "ListUsersRequest",
			mutate: func(request *descriptorpb.DescriptorProto) {
				fieldByName(request, "parent").Type = descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()
			},
		},
		{
			name:    "List parent repeated",
			request: "ListUsersRequest",
			mutate: func(request *descriptorpb.DescriptorProto) {
				fieldByName(request, "parent").Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
			},
		},
		{
			name:    "List parent map",
			request: "ListUsersRequest",
			mutate:  func(request *descriptorpb.DescriptorProto) { makeStringMapField(request, "parent") },
		},
		{
			name:    "List parent not required",
			request: "ListUsersRequest",
			mutate: func(request *descriptorpb.DescriptorProto) {
				fieldByName(request, "parent").Options = &descriptorpb.FieldOptions{}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			file := validCRUDFile()
			test.mutate(messageByName(file, test.request))
			_, err := runGenerator(t, file, "go")
			const want = "request field parent must be a REQUIRED singular string"
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("generate error = %v, want containing %q", err, want)
			}
		})
	}
}

func TestGenerateDistinguishableNestedPatternsUseStandardParentValidation(t *testing.T) {
	t.Parallel()

	file := validCRUDFile()
	resource := resourceOption(messageByName(file, "User"))
	resource.Pattern = []string{"tenants/{tenant}/users/{user}", "organizations/{organization}/users/{user}"}
	removeField(messageByName(file, "CreateUserRequest"), "parent")
	_, err := runGenerator(t, file, "go")
	const want = "request field parent must be a REQUIRED singular string"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("generate error = %v, want containing %q", err, want)
	}
}

func TestGenerateAmbiguousPatternsKeepHelpersWithoutChoosingDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		patterns     []string
		constructors []string
	}{
		{
			name:         "multiple top-level patterns",
			patterns:     []string{"users/{user}", "members/{user}"},
			constructors: []string{"NewUserNameForUsers", "NewUserNameForMembers"},
		},
		{
			name:         "mixed top-level and nested patterns",
			patterns:     []string{"users/{user}", "tenants/{tenant}/users/{user}"},
			constructors: []string{"NewUserNameForUsers", "NewUserNameForTenantsUsers"},
		},
		{
			name:         "duplicate parent skeleton",
			patterns:     []string{"tenants/{tenant}/users/{user}", "tenants/{tenant}/members/{user}"},
			constructors: []string{"NewUserNameForTenantsUsers", "NewUserNameForTenantsMembers"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			file := validCRUDFile()
			resourceOption(messageByName(file, "User")).Pattern = test.patterns
			removeField(messageByName(file, "ListUsersRequest"), "parent")
			removeField(messageByName(file, "CreateUserRequest"), "parent")
			fieldByName(messageByName(file, "ListUsersRequest"), "page_size").Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
			plugin, err := runGenerator(t, file, "go")
			if err != nil {
				t.Fatalf("generate ambiguous pattern helpers: %v", err)
			}
			content := generatedFiles(plugin)["test/v1/user_crud.pb.go"]
			for _, constructor := range test.constructors {
				if !strings.Contains(content, constructor) {
					t.Fatalf("missing multi-pattern constructor %s", constructor)
				}
			}
			if strings.Contains(content, "func NewUserName(") {
				t.Fatal("generated a default constructor for ambiguous patterns")
			}
		})
	}
}
func topLevelCRUDFile() *descriptorpb.FileDescriptorProto {
	file := validCRUDFile()
	resourceOption(messageByName(file, "User")).Pattern = []string{"users/{user}"}
	removeField(messageByName(file, "ListUsersRequest"), "parent")
	removeField(messageByName(file, "CreateUserRequest"), "parent")
	return file
}

func makeStringMapField(request *descriptorpb.DescriptorProto, name string) {
	entryName := "ParentEntry"
	entry := message(
		entryName,
		field("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
		field("value", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
	)
	entry.Options = &descriptorpb.MessageOptions{MapEntry: new(true)}
	request.NestedType = append(request.NestedType, entry)
	mapField := fieldByName(request, name)
	mapField.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
	mapField.TypeName = new(".test.v1." + request.GetName() + "." + entryName)
	mapField.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
}

func runGenerator(t *testing.T, file *descriptorpb.FileDescriptorProto, target string) (*protogen.Plugin, error) {
	t.Helper()
	return runGeneratorFiles(t, []*descriptorpb.FileDescriptorProto{file}, target)
}

func runGeneratorFiles(t *testing.T, files []*descriptorpb.FileDescriptorProto, target string) (*protogen.Plugin, error) {
	t.Helper()
	request := &pluginpb.CodeGeneratorRequest{
		ProtoFile: descriptorDependencies(),
		Parameter: new("paths=source_relative"),
	}
	for _, file := range files {
		request.ProtoFile = append(request.ProtoFile, file)
		request.FileToGenerate = append(request.FileToGenerate, file.GetName())
	}
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
		Name:       new("test/v1/user.proto"),
		Package:    new("test.v1"),
		Syntax:     new("proto3"),
		Dependency: []string{"google/api/resource.proto", "google/api/field_behavior.proto", "google/protobuf/empty.proto", "google/protobuf/field_mask.proto", "google/protobuf/timestamp.proto"},
		Options:    &descriptorpb.FileOptions{GoPackage: new("example.com/test/v1;testv1")},
	}
	user := &descriptorpb.DescriptorProto{Name: new("User"), Options: &descriptorpb.MessageOptions{}}
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
		Name: new("UserService"),
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
	result := &descriptorpb.FieldDescriptorProto{Name: new(name), Number: new(number), Type: fieldType.Enum(), Label: label.Enum()}
	if typeName != "" {
		result.TypeName = new(typeName)
	}
	return result
}

func addProto3Optional(message *descriptorpb.DescriptorProto, field *descriptorpb.FieldDescriptorProto) {
	index := int32(len(message.OneofDecl))
	message.OneofDecl = append(message.OneofDecl, &descriptorpb.OneofDescriptorProto{Name: new("_" + field.GetName())})
	field.Proto3Optional = new(true)
	field.OneofIndex = new(index)
	message.Field = append(message.Field, field)
}

func message(name string, fields ...*descriptorpb.FieldDescriptorProto) *descriptorpb.DescriptorProto {
	return &descriptorpb.DescriptorProto{Name: new(name), Field: fields}
}

func method(name, input, output string) *descriptorpb.MethodDescriptorProto {
	return &descriptorpb.MethodDescriptorProto{Name: new(name), InputType: new(input), OutputType: new(output)}
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
			candidate.OneofIndex = new(candidate.GetOneofIndex() - 1)
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

func TestGenerateValidatesTargetBeforeResources(t *testing.T) {
	file := validCRUDFile()
	fieldByName(messageByName(file, "User"), "name").Options = nil
	_, err := runGenerator(t, file, "java")
	if err == nil || !strings.Contains(err.Error(), `unknown target "java"`) {
		t.Fatalf("generate error = %v, want unknown target", err)
	}
	if strings.Contains(err.Error(), "IDENTIFIER") {
		t.Fatalf("target validation ran after resource validation: %v", err)
	}
}

func TestGenerateRejectsUnsafePatternVariables(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		want     string
	}{
		{
			name:     "duplicate variable in pattern",
			patterns: []string{"parents/{parent}/children/{parent}"},
			want:     `variable "parent" is repeated`,
		},
		{
			name:     "non ASCII identifier",
			patterns: []string{"tenants/{租户}/users/{user}"},
			want:     `variable "租户" is not a protobuf ASCII identifier`,
		},
		{
			name:     "Go exported identifier collision",
			patterns: []string{"tenants/{foo_bar}/users/{user}", "organizations/{fooBar}/users/{user}"},
			want:     `resource test.v1.User patterns "tenants/{foo_bar}/users/{user}" and "organizations/{fooBar}/users/{user}" use variables "foo_bar" and "fooBar" both generate Go identifier "FooBar"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := validCRUDFile()
			file.Service = nil
			resourceOption(messageByName(file, "User")).Pattern = test.patterns
			_, err := runGenerator(t, file, "go")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("generate error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestGenerateRecursiveResourceMessagesTerminates(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*descriptorpb.FileDescriptorProto)
	}{
		{
			name: "self reference",
			mutate: func(file *descriptorpb.FileDescriptorProto) {
				user := messageByName(file, "User")
				user.Field = append(user.Field, annotatedField("manager", 7, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".test.v1.User", annotations.FieldBehavior_OPTIONAL))
			},
		},
		{
			name: "mutual reference",
			mutate: func(file *descriptorpb.FileDescriptorProto) {
				user := messageByName(file, "User")
				profile := message("Profile", annotatedField("owner", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".test.v1.User", annotations.FieldBehavior_OPTIONAL))
				user.Field = append(user.Field, annotatedField("profile", 7, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".test.v1.Profile", annotations.FieldBehavior_OPTIONAL))
				file.MessageType = append(file.MessageType, profile)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := validCRUDFile()
			test.mutate(file)
			if _, err := runGenerator(t, file, "go"); err != nil {
				t.Fatalf("generate recursive resource: %v", err)
			}
		})
	}
}

func TestGenerateResponseOrderIsDeterministic(t *testing.T) {
	first := validCRUDFile()
	second := proto.Clone(first).(*descriptorpb.FileDescriptorProto)
	rewriteProtoPackage(second, "test.v1", "other.v1")
	second.Name = new("other/v1/user.proto")
	second.Options.GoPackage = new("example.com/other/v1;otherv1")

	var baseline *pluginpb.CodeGeneratorResponse
	for iteration := range 20 {
		plugin, err := runGeneratorFiles(t, []*descriptorpb.FileDescriptorProto{first, second}, "go")
		if err != nil {
			t.Fatalf("generate iteration %d: %v", iteration, err)
		}
		response := plugin.Response()
		if baseline == nil {
			baseline = proto.Clone(response).(*pluginpb.CodeGeneratorResponse)
			continue
		}
		if !proto.Equal(response, baseline) {
			t.Fatalf("iteration %d response differs\nfirst=%v\nnext=%v", iteration, baseline.File, response.File)
		}
	}
}

func rewriteProtoPackage(file *descriptorpb.FileDescriptorProto, oldPackage, newPackage string) {
	file.Package = new(newPackage)
	oldPrefix := "." + oldPackage + "."
	newPrefix := "." + newPackage + "."
	var rewriteMessage func(*descriptorpb.DescriptorProto)
	rewriteMessage = func(message *descriptorpb.DescriptorProto) {
		for _, field := range message.Field {
			if after, ok := strings.CutPrefix(field.GetTypeName(), oldPrefix); ok {
				field.TypeName = new(newPrefix + after)
			}
		}
		for _, nested := range message.NestedType {
			rewriteMessage(nested)
		}
	}
	for _, message := range file.MessageType {
		rewriteMessage(message)
	}
	for _, service := range file.Service {
		for _, method := range service.Method {
			if after, ok := strings.CutPrefix(method.GetInputType(), oldPrefix); ok {
				method.InputType = new(newPrefix + after)
			}
			if after, ok := strings.CutPrefix(method.GetOutputType(), oldPrefix); ok {
				method.OutputType = new(newPrefix + after)
			}
		}
	}
}
