package authz

import (
	"fmt"
	"strings"

	authzpb "github.com/Servora-Kit/servora/api/gen/go/servora/authz/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// resolveResource determines the resource type and ID for the given rule and request.
func resolveResource(rule *authzpb.AuthzRule, req any, defaultResourceID string) (resourceType, resourceID string, err error) {
	resourceType = rule.GetResourceType()
	if resourceType == "" {
		return "", "", fmt.Errorf("resource_type not specified in authz rule")
	}

	fieldPath := rule.GetResourceIdField()
	if fieldPath == "" {
		return resourceType, defaultResourceID, nil
	}

	resourceID, err = extractProtoField(req, fieldPath)
	return
}

// extractProtoField resolves a dot-path against a proto message and returns
// the scalar value at the path's terminus. Constraints:
//   - Each non-leaf segment must be a singular message field (no list/map).
//   - The terminus segment must be a scalar (not a message).
//   - An empty terminus value is treated as an error to preserve the existing
//     "field is required for authorization" contract.
//
// Single-segment paths preserve the prior behavior (top-level scalar lookup).
func extractProtoField(req any, fieldPath string) (string, error) {
	if fieldPath == "" {
		return "", fmt.Errorf("resource_id_field not specified")
	}
	msg, ok := req.(proto.Message)
	if !ok {
		return "", fmt.Errorf("request is not a proto message")
	}

	segments := strings.Split(fieldPath, ".")
	current := msg.ProtoReflect()

	for i, seg := range segments {
		fd := current.Descriptor().Fields().ByName(protoreflect.Name(seg))
		if fd == nil {
			return "", fmt.Errorf("field %q not found in %s",
				seg, current.Descriptor().FullName())
		}
		if fd.IsList() || fd.IsMap() {
			return "", fmt.Errorf("field %q is repeated/map; not supported in resource_id_field path", seg)
		}

		isLast := i == len(segments)-1
		val := current.Get(fd)

		if !isLast {
			if fd.Kind() != protoreflect.MessageKind {
				return "", fmt.Errorf("path segment %q is scalar but path continues", seg)
			}
			current = val.Message()
			continue
		}

		if fd.Kind() == protoreflect.MessageKind {
			return "", fmt.Errorf("path %q terminates on a message field; expected scalar", fieldPath)
		}
		s := val.String()
		if s == "" {
			return "", fmt.Errorf("field %q is empty", fieldPath)
		}
		return s, nil
	}

	return "", fmt.Errorf("unreachable: empty path segments")
}
