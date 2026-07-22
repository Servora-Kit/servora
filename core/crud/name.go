package crud

import (
	"fmt"
	"maps"
	"strings"
	"unicode"
)

// ResourceNameMatcher parses and formats one resource type's declared patterns.
type ResourceNameMatcher struct {
	patterns  []resourceNamePattern
	byPattern map[string]int
}

// ResourceName contains one matched pattern and its variable values.
type ResourceName struct {
	pattern   string
	variables map[string]string
}

type resourceNamePattern struct {
	raw       string
	segments  []resourceNameSegment
	variables map[string]struct{}
	skeleton  string
}

type resourceNameSegment struct {
	literal  string
	variable string
}

// NewResourceNameMatcher validates and compiles resource patterns once.
func NewResourceNameMatcher(patterns ...string) (*ResourceNameMatcher, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("crud: at least one resource name pattern is required")
	}
	matcher := &ResourceNameMatcher{
		patterns:  make([]resourceNamePattern, 0, len(patterns)),
		byPattern: make(map[string]int, len(patterns)),
	}
	skeletons := make(map[string]string, len(patterns))
	for _, value := range patterns {
		if _, duplicate := matcher.byPattern[value]; duplicate {
			return nil, fmt.Errorf("crud: duplicate resource name pattern %q", value)
		}
		pattern, err := compileResourceNamePattern(value)
		if err != nil {
			return nil, err
		}
		if previous, duplicate := skeletons[pattern.skeleton]; duplicate {
			return nil, fmt.Errorf(
				"crud: resource name patterns %q and %q have the same skeleton",
				previous,
				value,
			)
		}
		skeletons[pattern.skeleton] = value
		matcher.byPattern[value] = len(matcher.patterns)
		matcher.patterns = append(matcher.patterns, pattern)
	}
	return matcher, nil
}

// Parse matches a canonical, unescaped relative resource name.
func (matcher *ResourceNameMatcher) Parse(value string) (ResourceName, error) {
	if matcher == nil {
		return ResourceName{}, fmt.Errorf("crud: resource name matcher is nil")
	}
	if value == "" {
		return ResourceName{}, fmt.Errorf("resource name is empty")
	}
	segments := strings.Split(value, "/")
	var match *ResourceName
	for index := range matcher.patterns {
		pattern := &matcher.patterns[index]
		variables, ok := pattern.match(segments)
		if !ok {
			continue
		}
		if match != nil {
			return ResourceName{}, fmt.Errorf(
				"resource name %q matches multiple patterns %q and %q",
				value,
				match.pattern,
				pattern.raw,
			)
		}
		match = &ResourceName{pattern: pattern.raw, variables: variables}
	}
	if match == nil {
		return ResourceName{}, fmt.Errorf("resource name %q does not match a declared pattern", value)
	}
	return *match, nil
}

// Format builds a canonical, unescaped relative resource name.
func (matcher *ResourceNameMatcher) Format(pattern string, variables map[string]string) (string, error) {
	if matcher == nil {
		return "", fmt.Errorf("crud: resource name matcher is nil")
	}
	index, ok := matcher.byPattern[pattern]
	if !ok {
		return "", fmt.Errorf("unknown resource name pattern %q", pattern)
	}
	return matcher.patterns[index].format(variables)
}

// Pattern returns the matched declared pattern.
func (name ResourceName) Pattern() string { return name.pattern }

// Variables returns a copy of the matched variable values.
func (name ResourceName) Variables() map[string]string { return maps.Clone(name.variables) }

// Variable returns one matched variable value.
func (name ResourceName) Variable(variable string) (string, bool) {
	value, ok := name.variables[variable]
	return value, ok
}

func compileResourceNamePattern(value string) (resourceNamePattern, error) {
	if value == "" {
		return resourceNamePattern{}, fmt.Errorf("crud: resource name pattern is empty")
	}
	parts := strings.Split(value, "/")
	pattern := resourceNamePattern{
		raw:       value,
		segments:  make([]resourceNameSegment, 0, len(parts)),
		variables: make(map[string]struct{}),
	}
	skeleton := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return resourceNamePattern{}, fmt.Errorf("crud: resource name pattern %q contains an empty segment", value)
		}
		if strings.HasPrefix(part, "{") || strings.HasSuffix(part, "}") {
			if len(part) < 3 || part[0] != '{' || part[len(part)-1] != '}' || strings.Count(part, "{") != 1 || strings.Count(part, "}") != 1 {
				return resourceNamePattern{}, fmt.Errorf("crud: resource name pattern %q has invalid variable segment %q", value, part)
			}
			variable := part[1 : len(part)-1]
			if !validPatternVariable(variable) {
				return resourceNamePattern{}, fmt.Errorf("crud: resource name pattern %q has invalid variable %q", value, variable)
			}
			if _, duplicate := pattern.variables[variable]; duplicate {
				return resourceNamePattern{}, fmt.Errorf("crud: resource name pattern %q repeats variable %q", value, variable)
			}
			pattern.variables[variable] = struct{}{}
			pattern.segments = append(pattern.segments, resourceNameSegment{variable: variable})
			skeleton = append(skeleton, "{}")
			continue
		}
		if strings.ContainsAny(part, "{}") {
			return resourceNamePattern{}, fmt.Errorf("crud: resource name pattern %q has invalid literal segment %q", value, part)
		}
		pattern.segments = append(pattern.segments, resourceNameSegment{literal: part})
		skeleton = append(skeleton, part)
	}
	pattern.skeleton = strings.Join(skeleton, "/")
	return pattern, nil
}

func (pattern resourceNamePattern) match(values []string) (map[string]string, bool) {
	if len(values) != len(pattern.segments) {
		return nil, false
	}
	variables := make(map[string]string, len(pattern.variables))
	for index, segment := range pattern.segments {
		value := values[index]
		if segment.variable == "" {
			if value != segment.literal {
				return nil, false
			}
			continue
		}
		if value == "" {
			return nil, false
		}
		variables[segment.variable] = value
	}
	return variables, true
}

func (pattern resourceNamePattern) format(variables map[string]string) (string, error) {
	if len(variables) != len(pattern.variables) {
		return "", fmt.Errorf(
			"resource name pattern %q requires %d variables, got %d",
			pattern.raw,
			len(pattern.variables),
			len(variables),
		)
	}
	parts := make([]string, len(pattern.segments))
	for index, segment := range pattern.segments {
		if segment.variable == "" {
			parts[index] = segment.literal
			continue
		}
		value, ok := variables[segment.variable]
		if !ok {
			return "", fmt.Errorf("resource name pattern %q is missing variable %q", pattern.raw, segment.variable)
		}
		if value == "" {
			return "", fmt.Errorf("resource name variable %q is empty", segment.variable)
		}
		if strings.Contains(value, "/") {
			return "", fmt.Errorf("resource name variable %q contains '/'", segment.variable)
		}
		parts[index] = value
	}
	for variable := range variables {
		if _, ok := pattern.variables[variable]; !ok {
			return "", fmt.Errorf("resource name pattern %q has no variable %q", pattern.raw, variable)
		}
	}
	return strings.Join(parts, "/"), nil
}

func validPatternVariable(value string) bool {
	for index, r := range value {
		if index == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return value != ""
}
