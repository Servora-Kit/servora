package crud_test

import (
	"testing"

	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	"github.com/Servora-Kit/servora/core/crud"
	"google.golang.org/protobuf/proto"
)

func TestListResultPreservesOptionalZeroTotal(t *testing.T) {
	t.Parallel()

	query := preparedListQuery(t, true)
	zero := int64(0)
	result, err := crud.NewListResult(query, []string{"one"}, "next", &zero)
	if err != nil {
		t.Fatalf("NewListResult: %v", err)
	}
	if got, present := result.TotalSize(); !present || got != 0 {
		t.Fatalf("TotalSize() = (%d, %v), want (0, true)", got, present)
	}
	if got, want := result.NextPageToken(), "next"; got != want {
		t.Fatalf("NextPageToken() = %q, want %q", got, want)
	}
}

func TestListResultOmitsUnrequestedTotal(t *testing.T) {
	t.Parallel()

	query := preparedListQuery(t, false)
	result, err := crud.NewListResult(query, []string{"one"}, "", nil)
	if err != nil {
		t.Fatalf("NewListResult: %v", err)
	}
	if _, present := result.TotalSize(); present {
		t.Fatal("TotalSize is present when include_total=false")
	}
}

func TestListResultClonesProtobufItems(t *testing.T) {
	t.Parallel()

	input := &examplev1.User{DisplayName: proto.String("original")}
	result, err := crud.NewListResult(preparedListQuery(t, false), []*examplev1.User{input}, "", nil)
	if err != nil {
		t.Fatalf("NewListResult: %v", err)
	}
	input.DisplayName = proto.String("input changed")
	first := result.Items()
	if got := first[0].GetDisplayName(); got != "original" {
		t.Fatalf("stored item = %q, want original", got)
	}
	first[0].DisplayName = proto.String("output changed")
	if got := result.Items()[0].GetDisplayName(); got != "original" {
		t.Fatalf("Items() aliases stored item: %q", got)
	}
}

func TestListResultRejectsTotalPresenceMismatch(t *testing.T) {
	t.Parallel()

	zero := int64(0)
	for _, test := range []struct {
		query crud.ListQuery
		total *int64
	}{
		{preparedListQuery(t, true), nil},
		{preparedListQuery(t, false), &zero},
	} {
		if _, err := crud.NewListResult(test.query, []string(nil), "", test.total); err == nil {
			t.Fatal("NewListResult accepted include_total/total_size mismatch")
		}
	}
}

func preparedListQuery(t *testing.T, includeTotal bool) crud.ListQuery {
	t.Helper()
	preparer, err := crud.NewListPreparer()
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)
	query, err := preparer.PrepareList(plan, crud.ListInput{IncludeTotal: includeTotal})
	if err != nil {
		t.Fatalf("PrepareList: %v", err)
	}
	return query
}
