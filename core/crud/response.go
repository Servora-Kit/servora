package crud

import (
	annotations "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ToResponse validates canonical identity, clones the resource and removes INPUT_ONLY fields.
func (plan *ResourcePlan[R]) ToResponse(resource R) (R, error) {
	if isNilInterface(resource) {
		var zero R
		return zero, internalError("resource", "repository returned nil")
	}
	message := resource.ProtoReflect()
	name := message.Get(plan.identifier).String()
	if _, err := plan.nameMatcher.Parse(name); err != nil {
		var zero R
		return zero, internalError("name", "repository returned invalid canonical resource name: %v", err)
	}
	response := cloneResource(resource)
	clearInputOnlyFields(response.ProtoReflect(), plan.descriptor)
	return response, nil
}

// ToResponses applies ToResponse to every item and fails on nil or invalid entries.
func (plan *ResourcePlan[R]) ToResponses(resources []R) ([]R, error) {
	responses := make([]R, len(resources))
	for index, resource := range resources {
		response, err := plan.ToResponse(resource)
		if err != nil {
			return nil, internalError("resources", "item %d: %v", index, err)
		}
		responses[index] = response
	}
	return responses, nil
}

func clearInputOnlyFields(message protoreflect.Message, descriptor protoreflect.MessageDescriptor) {
	fields := descriptor.Fields()
	for index := 0; index < fields.Len(); index++ {
		field := fields.Get(index)
		if descriptorHasBehavior(field, annotations.FieldBehavior_INPUT_ONLY) {
			message.Clear(field)
			continue
		}
		if shouldTraverse(field) && message.Has(field) {
			clearInputOnlyFields(message.Mutable(field).Message(), field.Message())
		}
	}
}
