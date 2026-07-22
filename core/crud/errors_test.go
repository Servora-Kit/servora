package crud

import (
	"testing"

	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

func TestFrameworkErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		build  func(string, string, ...any) error
		reason string
		code   int32
	}{
		{"invalid resource name", invalidResourceName, crudpb.CrudErrorReason_CRUD_ERROR_REASON_INVALID_RESOURCE_NAME.String(), 400},
		{"invalid page token", invalidPageToken, crudpb.CrudErrorReason_CRUD_ERROR_REASON_INVALID_PAGE_TOKEN.String(), 400},
		{"invalid filter", invalidFilter, crudpb.CrudErrorReason_CRUD_ERROR_REASON_INVALID_FILTER.String(), 400},
		{"invalid order by", invalidOrderBy, crudpb.CrudErrorReason_CRUD_ERROR_REASON_INVALID_ORDER_BY.String(), 400},
		{"invalid field mask", invalidFieldMask, crudpb.CrudErrorReason_CRUD_ERROR_REASON_INVALID_FIELD_MASK.String(), 400},
		{"invalid field value", invalidFieldValue, crudpb.CrudErrorReason_CRUD_ERROR_REASON_INVALID_FIELD_VALUE.String(), 400},
		{"internal", internalError, crudpb.CrudErrorReason_CRUD_ERROR_REASON_INTERNAL.String(), 500},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := kratoserrors.FromError(test.build("user.email", "must contain %s", "@"))
			if got := err.GetCode(); got != test.code {
				t.Fatalf("Code = %d, want %d", got, test.code)
			}
			if got := err.GetReason(); got != test.reason {
				t.Fatalf("Reason = %q, want %q", got, test.reason)
			}
			if got, want := err.GetMessage(), "user.email: must contain @"; got != want {
				t.Fatalf("Message = %q, want %q", got, want)
			}
			if got := err.GetMetadata(); len(got) != 0 {
				t.Fatalf("Metadata = %v, want empty", got)
			}
		})
	}
}

func TestFrameworkErrorWithoutPath(t *testing.T) {
	t.Parallel()

	err := kratoserrors.FromError(internalError("", "broken invariant"))
	if got, want := err.GetMessage(), "broken invariant"; got != want {
		t.Fatalf("Message = %q, want %q", got, want)
	}
}
