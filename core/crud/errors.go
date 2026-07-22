package crud

import (
	"fmt"

	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

type errorConstructor func(string, ...any) *kratoserrors.Error

func invalidResourceName(path, format string, args ...any) error {
	return newFrameworkError(crudpb.ErrorCrudErrorReasonInvalidResourceName, path, format, args...)
}

func invalidPageToken(path, format string, args ...any) error {
	return newFrameworkError(crudpb.ErrorCrudErrorReasonInvalidPageToken, path, format, args...)
}

func invalidFilter(path, format string, args ...any) error {
	return newFrameworkError(crudpb.ErrorCrudErrorReasonInvalidFilter, path, format, args...)
}

func invalidOrderBy(path, format string, args ...any) error {
	return newFrameworkError(crudpb.ErrorCrudErrorReasonInvalidOrderBy, path, format, args...)
}

func invalidFieldMask(path, format string, args ...any) error {
	return newFrameworkError(crudpb.ErrorCrudErrorReasonInvalidFieldMask, path, format, args...)
}

func invalidFieldValue(path, format string, args ...any) error {
	return newFrameworkError(crudpb.ErrorCrudErrorReasonInvalidFieldValue, path, format, args...)
}

func internalError(path, format string, args ...any) error {
	return newFrameworkError(crudpb.ErrorCrudErrorReasonInternal, path, format, args...)
}

func newFrameworkError(constructor errorConstructor, path, format string, args ...any) error {
	message := fmt.Sprintf(format, args...)
	switch {
	case path == "":
	case message == "":
		message = path
	default:
		message = path + ": " + message
	}
	return constructor("%s", message)
}
