package plugin

import (
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func scopedDescriptorTypeName(pkg protoreflect.FullName, desc protoreflect.Descriptor) string {
	name := string(desc.Name())
	var prefix string
	if desc.Parent() != desc.ParentFile() {
		prefix = descriptorTypeName(desc.Parent()) + "_"
	}
	if desc.ParentFile().Package() != pkg {
		prefix = packagePrefix(desc.ParentFile().Package()) + prefix
	}
	return prefix + name
}

func descriptorTypeName(desc protoreflect.Descriptor) string {
	name := string(desc.Name())
	var prefix string
	if desc.Parent() != desc.ParentFile() {
		prefix = descriptorTypeName(desc.Parent()) + "_"
	}
	return prefix + name
}

func packagePrefix(pkg protoreflect.FullName) string {
	return strings.Join(strings.Split(string(pkg), "."), "") + "_"
}

func rangeFields(message protoreflect.MessageDescriptor, f func(field protoreflect.FieldDescriptor)) {
	for i := 0; i < message.Fields().Len(); i++ {
		f(message.Fields().Get(i))
	}
}

func rangeMethods(methods protoreflect.MethodDescriptors, f func(method protoreflect.MethodDescriptor)) {
	for i := 0; i < methods.Len(); i++ {
		f(methods.Get(i))
	}
}

func rangeEnumValues(enum protoreflect.EnumDescriptor, f func(value protoreflect.EnumValueDescriptor, last bool)) {
	for i := 0; i < enum.Values().Len(); i++ {
		if i == enum.Values().Len()-1 {
			f(enum.Values().Get(i), true)
		} else {
			f(enum.Values().Get(i), false)
		}
	}
}

func t(n int) string {
	return strings.Repeat("  ", n)
}

// tsSingleQuote wraps s in TypeScript single quotes, escaping backslashes
// and single quotes as needed. Prettier enforces single quotes for TS strings.
func tsSingleQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return "'" + s + "'"
}

// localeCompare compares two strings in a way that matches JavaScript's
// String.prototype.localeCompare for the common cases encountered in generated
// code. The Unicode Collation Algorithm (used by localeCompare) sorts
// punctuation/symbols before digits before letters, and is case-insensitive.
// Go's native string comparison treats punctuation (e.g. underscore 95, '{' 123)
// as greater than letters (65-122), which conflicts with the perfectionist ESLint
// plugin's sort order. sortKey transforms each character so that Go byte
// comparison matches the UCA ordering.
func localeCompare(a, b string) bool {
	return sortKey(a) < sortKey(b)
}

// sortKey transforms a string into a comparison key where:
//   - Non-alphanumeric characters (punctuation, symbols) sort before alphanumerics
//   - Digits sort after punctuation but before letters
//   - Letters are compared case-insensitively (lowercase form used)
func sortKey(s string) string {
	var sb strings.Builder
	for _, c := range []byte(s) {
		switch {
		case c >= 'a' && c <= 'z':
			sb.WriteByte(2)
			sb.WriteByte(c)
		case c >= 'A' && c <= 'Z':
			sb.WriteByte(2)
			sb.WriteByte(c + 32)
		case c >= '0' && c <= '9':
			sb.WriteByte(1)
			sb.WriteByte(c)
		default:
			sb.WriteByte(0)
			sb.WriteByte(c)
		}
	}
	return sb.String()
}
