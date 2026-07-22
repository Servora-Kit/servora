package main

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	annotations "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type resourceInfo struct {
	file       *protogen.File
	message    *protogen.Message
	descriptor *annotations.ResourceDescriptor
	identifier *protogen.Field
	patterns   []namePattern
	variables  []string
	fields     []resourceField
	writable   []resourceField
}

type resourceField struct {
	path   string
	symbol string
	field  *protogen.Field
}

type namePattern struct {
	raw               string
	segments          []nameSegment
	variables         []string
	constructorSuffix string
	parentSkeleton    string
	topLevel          bool
}

type nameSegment struct {
	literal  string
	variable string
}

func discoverResources(file *protogen.File) ([]*resourceInfo, error) {
	var resources []*resourceInfo
	var visit func(messages []*protogen.Message) error
	visit = func(messages []*protogen.Message) error {
		for _, message := range messages {
			resource, present := resourceDescriptor(message.Desc)
			if present {
				info, err := buildResourceInfo(file, message, resource)
				if err != nil {
					return err
				}
				resources = append(resources, info)
			}
			if err := visit(message.Messages); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(file.Messages); err != nil {
		return nil, err
	}
	sort.Slice(resources, func(left, right int) bool {
		return resources[left].message.Desc.FullName() < resources[right].message.Desc.FullName()
	})
	return resources, nil
}

func buildResourceInfo(
	file *protogen.File,
	message *protogen.Message,
	descriptor *annotations.ResourceDescriptor,
) (*resourceInfo, error) {
	fullName := message.Desc.FullName()
	if strings.TrimSpace(descriptor.GetType()) == "" {
		return nil, fmt.Errorf("crud: resource %s: google.api.resource type is empty", fullName)
	}
	if len(descriptor.GetPattern()) == 0 {
		return nil, fmt.Errorf("crud: resource %s: google.api.resource pattern is empty", fullName)
	}
	var identifier *protogen.Field
	for _, field := range message.Fields {
		if !hasBehavior(field.Desc, annotations.FieldBehavior_IDENTIFIER) {
			continue
		}
		if identifier != nil {
			return nil, fmt.Errorf("crud: resource %s: multiple IDENTIFIER fields (%s and %s)", fullName, identifier.Desc.Name(), field.Desc.Name())
		}
		identifier = field
	}
	if identifier == nil {
		return nil, fmt.Errorf("crud: resource %s: missing IDENTIFIER field name", fullName)
	}
	if identifier.Desc.Name() != "name" || identifier.Desc.Kind() != protoreflect.StringKind || identifier.Desc.IsList() || identifier.Desc.IsMap() {
		return nil, fmt.Errorf("crud: resource %s: IDENTIFIER must be singular string field name", fullName)
	}
	patterns := make([]namePattern, len(descriptor.GetPattern()))
	skeletons := make(map[string]string, len(patterns))
	variableSet := make(map[string]struct{})
	var variables []string
	for index, raw := range descriptor.GetPattern() {
		pattern, err := parseNamePattern(raw)
		if err != nil {
			return nil, fmt.Errorf("crud: resource %s pattern %q: %w", fullName, raw, err)
		}
		skeleton := patternSkeleton(pattern)
		if previous, duplicate := skeletons[skeleton]; duplicate {
			return nil, fmt.Errorf("crud: resource %s patterns %q and %q have ambiguous skeleton %q", fullName, previous, raw, skeleton)
		}
		skeletons[skeleton] = raw
		patterns[index] = pattern
		for _, variable := range pattern.variables {
			if _, seen := variableSet[variable]; seen {
				continue
			}
			variableSet[variable] = struct{}{}
			variables = append(variables, variable)
		}
	}
	fields, err := collectResourceFields(message)
	if err != nil {
		return nil, fmt.Errorf("crud: resource %s: %w", fullName, err)
	}
	writable := make([]resourceField, 0, len(fields))
	for _, field := range fields {
		if isWritableResourceField(field.field.Desc) {
			writable = append(writable, field)
		}
	}
	return &resourceInfo{
		file: file, message: message, descriptor: descriptor, identifier: identifier,
		patterns: patterns, variables: variables, fields: fields, writable: writable,
	}, nil
}

func resourceDescriptor(message protoreflect.MessageDescriptor) (*annotations.ResourceDescriptor, bool) {
	options := message.Options()
	if options == nil || !proto.HasExtension(options, annotations.E_Resource) {
		return nil, false
	}
	resource, ok := proto.GetExtension(options, annotations.E_Resource).(*annotations.ResourceDescriptor)
	return resource, ok && resource != nil
}

func parseNamePattern(raw string) (namePattern, error) {
	if raw == "" || strings.HasPrefix(raw, "/") || strings.HasSuffix(raw, "/") {
		return namePattern{}, fmt.Errorf("must be a non-empty relative path")
	}
	parts := strings.Split(raw, "/")
	pattern := namePattern{raw: raw, segments: make([]nameSegment, len(parts))}
	var fixed []string
	for index, part := range parts {
		if part == "" {
			return namePattern{}, fmt.Errorf("contains an empty segment")
		}
		if strings.HasPrefix(part, "{") || strings.HasSuffix(part, "}") {
			if len(part) < 3 || part[0] != '{' || part[len(part)-1] != '}' || strings.Contains(part[1:len(part)-1], "{") || strings.Contains(part[1:len(part)-1], "}") {
				return namePattern{}, fmt.Errorf("segment %q is not a simple variable", part)
			}
			variable := part[1 : len(part)-1]
			if !validProtoIdentifier(variable) {
				return namePattern{}, fmt.Errorf("variable %q is not a protobuf identifier", variable)
			}
			pattern.segments[index].variable = variable
			pattern.variables = append(pattern.variables, variable)
			continue
		}
		if strings.ContainsAny(part, "{}") {
			return namePattern{}, fmt.Errorf("literal segment %q contains braces", part)
		}
		pattern.segments[index].literal = part
		fixed = append(fixed, part)
	}
	if len(pattern.variables) == 0 {
		return namePattern{}, fmt.Errorf("must contain at least one variable")
	}
	last := pattern.segments[len(pattern.segments)-1]
	if last.variable == "" || len(pattern.segments) < 2 || pattern.segments[len(pattern.segments)-2].literal == "" {
		return namePattern{}, fmt.Errorf("must end with a collection literal and variable")
	}
	pattern.topLevel = len(pattern.segments) == 2
	pattern.parentSkeleton = patternSkeleton(namePattern{segments: pattern.segments[:len(pattern.segments)-2]})
	if len(fixed) == 0 {
		pattern.constructorSuffix = "Pattern"
	} else {
		var suffix strings.Builder
		for _, part := range fixed {
			suffix.WriteString(goExportedName(part))
		}
		pattern.constructorSuffix = suffix.String()
	}
	return pattern, nil
}

func patternSkeleton(pattern namePattern) string {
	parts := make([]string, len(pattern.segments))
	for index, segment := range pattern.segments {
		if segment.variable != "" {
			parts[index] = "{}"
		} else {
			parts[index] = segment.literal
		}
	}
	return strings.Join(parts, "/")
}

func validProtoIdentifier(value string) bool {
	for index, character := range value {
		if index == 0 {
			if character != '_' && !unicode.IsLetter(character) {
				return false
			}
			continue
		}
		if character != '_' && !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			return false
		}
	}
	return value != ""
}

func collectResourceFields(message *protogen.Message) ([]resourceField, error) {
	var fields []resourceField
	bySymbol := make(map[string]string)
	var collision error
	var visit func(current *protogen.Message, prefix, symbolPrefix string, ancestors map[protoreflect.FullName]bool)
	visit = func(current *protogen.Message, prefix, symbolPrefix string, ancestors map[protoreflect.FullName]bool) {
		if collision != nil || ancestors[current.Desc.FullName()] {
			return
		}
		nextAncestors := make(map[protoreflect.FullName]bool, len(ancestors)+1)
		for name := range ancestors {
			nextAncestors[name] = true
		}
		nextAncestors[current.Desc.FullName()] = true
		for _, field := range current.Fields {
			path := string(field.Desc.Name())
			if prefix != "" {
				path = prefix + "." + path
			}
			symbol := symbolPrefix + goExportedName(string(field.Desc.Name()))
			if previous, exists := bySymbol[symbol]; exists {
				collision = fmt.Errorf("generated field symbol %s collides for %s and %s", symbol, previous, path)
				return
			}
			bySymbol[symbol] = path
			fields = append(fields, resourceField{path: path, symbol: symbol, field: field})
			if field.Message == nil || field.Desc.IsList() || field.Desc.IsMap() || strings.HasPrefix(string(field.Message.Desc.FullName()), "google.protobuf.") {
				continue
			}
			visit(field.Message, path, symbol, nextAncestors)
		}
	}
	visit(message, "", "", map[protoreflect.FullName]bool{})
	return fields, collision

}

func hasBehavior(field protoreflect.FieldDescriptor, wanted annotations.FieldBehavior) bool {
	behaviors, _ := proto.GetExtension(field.Options(), annotations.E_FieldBehavior).([]annotations.FieldBehavior)
	for _, behavior := range behaviors {
		if behavior == wanted {
			return true
		}
	}
	return false
}

func isWritableResourceField(field protoreflect.FieldDescriptor) bool {
	return !hasBehavior(field, annotations.FieldBehavior_IDENTIFIER) &&
		!hasBehavior(field, annotations.FieldBehavior_OUTPUT_ONLY) &&
		!hasBehavior(field, annotations.FieldBehavior_IMMUTABLE) &&
		field.Name() != "etag"
}

func goExportedName(value string) string {
	parts := strings.FieldsFunc(value, func(character rune) bool { return character == '_' || character == '-' || character == '.' })
	var result strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		result.WriteString(string(runes))
	}
	return result.String()
}
