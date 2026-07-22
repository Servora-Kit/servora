package main

import (
	"fmt"
	"strings"
	"unicode"

	annotations "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func validateStandardMethods(files []*protogen.File, resources map[string]*resourceInfo) error {
	for _, resource := range resources {
		allowCreateList := supportsStandardCreateList(resource.patterns)
		lifecycle := false
		for _, file := range files {
			if file.Desc.Package() != resource.file.Desc.Package() {
				continue
			}
			for _, service := range file.Services {
				for _, method := range service.Methods {
					action := standardAction(method.GoName, resource)
					if action == "" || ((action == "create" || action == "list") && !allowCreateList) {
						continue
					}
					standard, err := validateStandardMethod(action, method, resource)
					if err != nil {
						return err
					}
					if standard && (action == "create" || action == "update") {
						lifecycle = true
					}
				}
			}
		}
		if lifecycle {
			if err := validateLifecycleFields(resource); err != nil {
				return err
			}
		}
	}
	return nil
}

func standardAction(methodName string, resource *resourceInfo) string {
	resourceName := resource.message.GoIdent.GoName
	plural := goExportedName(resource.descriptor.GetPlural())
	if plural == "" {
		plural = resourceName + "s"
	}
	switch methodName {
	case "Get" + resourceName:
		return "get"
	case "List" + plural:
		return "list"
	case "Create" + resourceName:
		return "create"
	case "Update" + resourceName:
		return "update"
	case "Delete" + resourceName:
		return "delete"
	default:
		return ""
	}
}

func validateStandardMethod(action string, method *protogen.Method, resource *resourceInfo) (bool, error) {
	methodName := method.Desc.FullName()
	resourceName := resource.message.Desc.FullName()
	outputName := method.Output.Desc.FullName()
	if (action == "create" || action == "update" || action == "delete") && outputName == "google.longrunning.Operation" {
		return false, nil
	}
	switch action {
	case "get":
		if outputName != resourceName {
			return false, fmt.Errorf("crud: method %s: Get response must be %s", methodName, resourceName)
		}
		if err := validateRequiredString(method, "name"); err != nil {
			return false, err
		}
	case "list":
		if err := validateRequiredString(method, "parent"); err != nil {
			return false, err
		}
		if err := validateOptionalScalar(method, "page_size", protoreflect.Int32Kind); err != nil {
			return false, err
		}
		if err := validateOptionalScalar(method, "page_token", protoreflect.StringKind); err != nil {
			return false, err
		}
		if err := validateOptionalScalar(method, "skip", protoreflect.Int64Kind); err != nil {
			return false, err
		}
		if err := validateOptionalScalar(method, "filter", protoreflect.StringKind); err != nil {
			return false, err
		}
		if err := validateOptionalScalar(method, "order_by", protoreflect.StringKind); err != nil {
			return false, err
		}
		if err := validateOptionalScalar(method, "show_deleted", protoreflect.BoolKind); err != nil {
			return false, err
		}
		if err := validateListResponse(method, resource); err != nil {
			return false, err
		}
	case "create":
		if outputName != resourceName {
			return false, fmt.Errorf("crud: method %s: Create response must be %s or google.longrunning.Operation", methodName, resourceName)
		}
		if strings.TrimSpace(resource.descriptor.GetSingular()) == "" {
			return false, fmt.Errorf("crud: method %s: resource singular is required for standard Create", methodName)
		}
		resourceField := protoFieldName(resource.descriptor.GetSingular())
		if err := validateRequiredMessage(method, resourceField, resourceName); err != nil {
			return false, err
		}
		idFieldName := resourceField + "_id"
		idField := method.Input.Desc.Fields().ByName(protoreflect.Name(idFieldName))
		if idField == nil {
			return false, fmt.Errorf("crud: method %s request field %s is required", methodName, idFieldName)
		}
		if idField.Kind() != protoreflect.StringKind || idField.IsList() || idField.IsMap() || idField.ContainingOneof() != nil {
			return false, fmt.Errorf("crud: method %s request field %s must be a singular string outside oneof", methodName, idFieldName)
		}
		if !hasExactlyOneBehavior(idField, annotations.FieldBehavior_REQUIRED, annotations.FieldBehavior_OPTIONAL) {
			return false, fmt.Errorf("crud: method %s request field %s must declare REQUIRED or OPTIONAL", methodName, idFieldName)
		}
		for _, pattern := range resource.patterns {
			lastVariable := pattern.segments[len(pattern.segments)-1].variable
			if lastVariable != resourceField {
				return false, fmt.Errorf("crud: method %s: pattern %q final variable %q must match singular field %q", methodName, pattern.raw, lastVariable, resourceField)
			}
		}
	case "update":
		if outputName != resourceName {
			return false, fmt.Errorf("crud: method %s: Update response must be %s or google.longrunning.Operation", methodName, resourceName)
		}
		resourceField := protoFieldName(resource.descriptor.GetSingular())
		if resourceField == "" {
			resourceField = protoFieldName(resource.message.GoIdent.GoName)
		}
		if err := validateRequiredMessage(method, resourceField, resourceName); err != nil {
			return false, err
		}
		mask := method.Input.Desc.Fields().ByName("update_mask")
		if mask == nil || mask.Message() == nil || mask.Message().FullName() != "google.protobuf.FieldMask" || mask.IsList() || mask.IsMap() {
			return false, fmt.Errorf("crud: method %s request field update_mask must be google.protobuf.FieldMask", methodName)
		}
		if err := validateOptionalScalar(method, "allow_missing", protoreflect.BoolKind); err != nil {
			return false, err
		}
	case "delete":
		if outputName != "google.protobuf.Empty" && outputName != resourceName {
			return false, fmt.Errorf("crud: method %s: Delete response must be google.protobuf.Empty, %s, or google.longrunning.Operation", methodName, resourceName)
		}
		if err := validateRequiredString(method, "name"); err != nil {
			return false, err
		}
		for _, field := range []struct {
			name string
			kind protoreflect.Kind
		}{{"etag", protoreflect.StringKind}, {"allow_missing", protoreflect.BoolKind}, {"force", protoreflect.BoolKind}} {
			if err := validateOptionalScalar(method, protoreflect.Name(field.name), field.kind); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

func validateRequiredString(method *protogen.Method, name protoreflect.Name) error {
	field := method.Input.Desc.Fields().ByName(name)
	if field == nil || field.Kind() != protoreflect.StringKind || field.IsList() || field.IsMap() || !hasBehavior(field, annotations.FieldBehavior_REQUIRED) {
		return fmt.Errorf("crud: method %s request field %s must be a REQUIRED singular string", method.Desc.FullName(), name)
	}
	return nil
}

func validateRequiredMessage(method *protogen.Method, name string, fullName protoreflect.FullName) error {
	field := method.Input.Desc.Fields().ByName(protoreflect.Name(name))
	if field == nil || field.Message() == nil || field.Message().FullName() != fullName || field.IsList() || field.IsMap() || !hasBehavior(field, annotations.FieldBehavior_REQUIRED) {
		return fmt.Errorf("crud: method %s request field %s must be REQUIRED %s", method.Desc.FullName(), name, fullName)
	}
	return nil
}

func validateOptionalScalar(method *protogen.Method, name protoreflect.Name, kind protoreflect.Kind) error {
	field := method.Input.Desc.Fields().ByName(name)
	if field == nil {
		return nil
	}
	if field.Kind() != kind || field.IsList() || field.IsMap() || !hasBehavior(field, annotations.FieldBehavior_OPTIONAL) {
		return fmt.Errorf("crud: method %s request field %s must be an OPTIONAL singular %s", method.Desc.FullName(), name, kind)
	}
	return nil
}

func validateListResponse(method *protogen.Method, resource *resourceInfo) error {
	resourceName := resource.message.Desc.FullName()
	var resources protoreflect.FieldDescriptor
	fields := method.Output.Desc.Fields()
	for index := range fields.Len() {
		field := fields.Get(index)
		if field.IsList() && field.Message() != nil && field.Message().FullName() == resourceName {
			if resources != nil {
				return fmt.Errorf("crud: method %s response has multiple repeated %s fields", method.Desc.FullName(), resourceName)
			}
			resources = field
		}
	}
	if resources == nil {
		return fmt.Errorf("crud: method %s response must contain repeated %s", method.Desc.FullName(), resourceName)
	}
	nextToken := fields.ByName("next_page_token")
	if nextToken == nil || nextToken.Kind() != protoreflect.StringKind || nextToken.IsList() || nextToken.IsMap() {
		return fmt.Errorf("crud: method %s response field next_page_token must be a singular string", method.Desc.FullName())
	}
	includeTotal := method.Input.Desc.Fields().ByName("include_total")
	totalSize := fields.ByName("total_size")
	if (includeTotal == nil) != (totalSize == nil) {
		return fmt.Errorf("crud: method %s fields include_total and total_size must be declared together", method.Desc.FullName())
	}
	if includeTotal != nil {
		if includeTotal.Kind() != protoreflect.BoolKind || includeTotal.IsList() || includeTotal.IsMap() || !hasBehavior(includeTotal, annotations.FieldBehavior_OPTIONAL) {
			return fmt.Errorf("crud: method %s request field include_total must be an OPTIONAL singular bool", method.Desc.FullName())
		}
		if totalSize.Kind() != protoreflect.Int64Kind || totalSize.IsList() || totalSize.IsMap() || !totalSize.HasOptionalKeyword() {
			return fmt.Errorf("crud: method %s response field total_size must be optional int64", method.Desc.FullName())
		}
	}
	return nil
}

func validateLifecycleFields(resource *resourceInfo) error {
	var visit func(message *protogen.Message, prefix string, inheritedOutputOnly bool) error
	visit = func(message *protogen.Message, prefix string, inheritedOutputOnly bool) error {
		for _, field := range message.Fields {
			path := string(field.Desc.Name())
			if prefix != "" {
				path = prefix + "." + path
			}
			outputOnly := inheritedOutputOnly || hasBehavior(field.Desc, annotations.FieldBehavior_OUTPUT_ONLY)
			if err := validateLifecycleField(resource, field.Desc, path, inheritedOutputOnly); err != nil {
				return err
			}
			if field.Message != nil && !field.Desc.IsList() && !field.Desc.IsMap() && !outputOnly && !strings.HasPrefix(string(field.Message.Desc.FullName()), "google.protobuf.") {
				if err := visit(field.Message, path, false); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(resource.message, "", false)
}

func validateLifecycleField(resource *resourceInfo, field protoreflect.FieldDescriptor, path string, inheritedOutputOnly bool) error {
	fullPath := fmt.Sprintf("%s.%s", resource.message.Desc.FullName(), path)
	if inheritedOutputOnly {
		return nil
	}
	identifier := hasBehavior(field, annotations.FieldBehavior_IDENTIFIER)
	required := hasBehavior(field, annotations.FieldBehavior_REQUIRED)
	optional := hasBehavior(field, annotations.FieldBehavior_OPTIONAL)
	outputOnly := hasBehavior(field, annotations.FieldBehavior_OUTPUT_ONLY)
	inputOnly := hasBehavior(field, annotations.FieldBehavior_INPUT_ONLY)
	immutable := hasBehavior(field, annotations.FieldBehavior_IMMUTABLE)
	if identifier {
		if field.Name() != "name" || required || optional || outputOnly || inputOnly || immutable {
			return fmt.Errorf("crud: field %s: IDENTIFIER name cannot combine with lifecycle behaviors", fullPath)
		}
		return nil
	}
	if field.Name() == "etag" || realOneofField(field) {
		return nil
	}
	primary := 0
	for _, present := range []bool{required, optional, outputOnly} {
		if present {
			primary++
		}
	}
	if primary != 1 {
		return fmt.Errorf("crud: field %s: must declare exactly one of REQUIRED, OPTIONAL, OUTPUT_ONLY", fullPath)
	}
	if outputOnly && (inputOnly || immutable) {
		return fmt.Errorf("crud: field %s: OUTPUT_ONLY cannot combine with INPUT_ONLY or IMMUTABLE", fullPath)
	}
	if !outputOnly && !field.IsList() && !field.IsMap() && field.Kind() != protoreflect.MessageKind && field.Kind() != protoreflect.GroupKind && !field.HasPresence() {
		return fmt.Errorf("crud: field %s: client-writable singular scalar or enum requires explicit presence", fullPath)
	}
	return nil
}

func hasExactlyOneBehavior(field protoreflect.FieldDescriptor, behaviors ...annotations.FieldBehavior) bool {
	count := 0
	for _, behavior := range behaviors {
		if hasBehavior(field, behavior) {
			count++
		}
	}
	return count == 1
}

func realOneofField(field protoreflect.FieldDescriptor) bool {
	oneof := field.ContainingOneof()
	return oneof != nil && !oneof.IsSynthetic()
}

func supportsStandardCreateList(patterns []namePattern) bool {
	if len(patterns) == 1 {
		return true
	}
	parents := make(map[string]struct{}, len(patterns))
	for _, pattern := range patterns {
		if pattern.topLevel {
			return false
		}
		if _, duplicate := parents[pattern.parentSkeleton]; duplicate {
			return false
		}
		parents[pattern.parentSkeleton] = struct{}{}
	}
	return true
}

func protoFieldName(value string) string {
	var result strings.Builder
	for index, character := range value {
		if character == '-' || character == '.' {
			character = '_'
		}
		if unicode.IsUpper(character) {
			if index > 0 {
				result.WriteByte('_')
			}
			character = unicode.ToLower(character)
		}
		result.WriteRune(character)
	}
	return result.String()
}
