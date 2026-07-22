package crud

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	corecrud "github.com/Servora-Kit/servora/core/crud"
)

func TestListExecutesFilterCountPageAndNextToken(t *testing.T) {
	t.Parallel()

	fields := mustFakeListFields(t,
		Bind(examplev1.UserFields.Email, "email").Filter(),
		Bind(examplev1.UserFields.CreateTime, "created_at").Order(),
	)
	query := prepareListInput(t, corecrud.ListInput{
		Collection:   "tenants/acme/users",
		PageSize:     2,
		Filter:       `email = "person@example.com"`,
		IncludeTotal: true,
	})
	rows := []*fakeEntity{
		fakeRow("2026-01-03T00:00:00Z", uint32(3)),
		fakeRow("2026-01-02T00:00:00Z", uint32(2)),
		fakeRow("2026-01-01T00:00:00Z", uint32(1)),
	}
	spy := new(fakeQuerySpy)
	builder := &fakeListQuery{rows: rows, count: 3, spy: spy}

	result, err := List[*fakeEntity](context.Background(), builder, query, fields, []byte("tenant:acme"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	items := result.Items()
	if got, want := len(items), 2; got != want {
		t.Fatalf("items = %d, want %d", got, want)
	}
	if got, ok := result.TotalSize(); !ok || got != 3 {
		t.Fatalf("TotalSize = (%d, %v), want (3, true)", got, ok)
	}
	if result.NextPageToken() == "" {
		t.Fatal("NextPageToken is empty")
	}
	payload, err := corecrud.NewUnsignedPageTokenCodec().Decode(result.NextPageToken())
	if err != nil {
		t.Fatalf("Decode next token: %v", err)
	}
	if got, want := len(payload.GetCursor()), 2; got != want {
		t.Fatalf("cursor count = %d, want %d", got, want)
	}
	if got := payload.GetCursor()[1].GetUint64Value(); got != 2 {
		t.Fatalf("cursor id = %d, want 2", got)
	}
	if !strings.Contains(spy.countSQL, "email") {
		t.Fatalf("count SQL missing filter: %s", spy.countSQL)
	}
	if !strings.Contains(spy.allSQL, "ORDER BY") || !strings.Contains(spy.allSQL, "created_at") || !strings.Contains(spy.allSQL, "id") {
		t.Fatalf("page SQL missing final order: %s", spy.allSQL)
	}
	if spy.allLimit != 3 {
		t.Fatalf("all limit = %d, want page_size + 1", spy.allLimit)
	}
}

func TestListRejectsTokenWithMismatchedScopeFingerprint(t *testing.T) {
	t.Parallel()

	fields := mustFakeListFields(t,
		Bind(examplev1.UserFields.Email, "email").Filter(),
		Bind(examplev1.UserFields.CreateTime, "created_at").Order(),
	)
	firstQuery := prepareListInput(t, corecrud.ListInput{
		Collection: "tenants/acme/users",
		PageSize:   1,
		Filter:     `email = "person@example.com"`,
	})
	rows := []*fakeEntity{
		fakeRow("2026-01-02T00:00:00Z", uint32(2)),
		fakeRow("2026-01-01T00:00:00Z", uint32(1)),
	}
	first, err := List[*fakeEntity](context.Background(), &fakeListQuery{rows: rows, count: 2, spy: new(fakeQuerySpy)}, firstQuery, fields, []byte("scope:a"))
	if err != nil {
		t.Fatalf("first List: %v", err)
	}
	if first.NextPageToken() == "" {
		t.Fatal("first page produced no token")
	}
	secondQuery := prepareListInput(t, corecrud.ListInput{
		Collection: "tenants/acme/users",
		PageSize:   1,
		Filter:     `email = "person@example.com"`,
		PageToken:  first.NextPageToken(),
	})
	_, err = List[*fakeEntity](context.Background(), &fakeListQuery{rows: rows, count: 2, spy: new(fakeQuerySpy)}, secondQuery, fields, []byte("scope:b"))
	if !crudpb.IsCrudErrorReasonInvalidPageToken(err) {
		t.Fatalf("List error = %v, want invalid page token", err)
	}
}

func TestListNullableOrderUsesNullRankAndNullCursorKeyset(t *testing.T) {
	t.Parallel()

	fields, err := NewListFields[*fakeEntity](
		Columns(testValidColumn("nickname", "id")),
		Bind(examplev1.UserFields.Nickname, "nickname").Order().Nullable(),
		DefaultOrder(examplev1.UserFields.Nickname, corecrud.OrderDescending),
		CursorKey[uint32]("id", corecrud.OrderAscending),
	)
	if err != nil {
		t.Fatalf("NewListFields: %v", err)
	}
	firstQuery := prepareListInput(t, corecrud.ListInput{Collection: "tenants/acme/users", PageSize: 1})
	rows := []*fakeEntity{
		fakeOrderedRow("beta", uint32(2)),
		fakeOrderedRow(nil, uint32(1)),
	}
	firstSpy := new(fakeQuerySpy)
	first, err := List[*fakeEntity](context.Background(), &fakeListQuery{rows: rows, count: 2, spy: firstSpy}, firstQuery, fields, []byte("scope"))
	if err != nil {
		t.Fatalf("first List: %v", err)
	}
	if !strings.Contains(firstSpy.allSQL, "CASE WHEN") || !strings.Contains(firstSpy.allSQL, "nickname") {
		t.Fatalf("NULLS LAST rank is absent: %s", firstSpy.allSQL)
	}

	secondQuery := prepareListInput(t, corecrud.ListInput{
		Collection: "tenants/acme/users", PageSize: 1, PageToken: first.NextPageToken(),
	})
	secondSpy := new(fakeQuerySpy)
	_, err = List[*fakeEntity](context.Background(), &fakeListQuery{rows: rows, count: 2, spy: secondSpy}, secondQuery, fields, []byte("scope"))
	if err != nil {
		t.Fatalf("second List: %v", err)
	}
	if !strings.Contains(secondSpy.allSQL, "IS NULL") || !strings.Contains(secondSpy.allSQL, "> ?") {
		t.Fatalf("non-null cursor keyset misses NULLS LAST continuation: %s", secondSpy.allSQL)
	}

	nullRows := []*fakeEntity{
		fakeOrderedRow(nil, uint32(2)),
		fakeOrderedRow(nil, uint32(1)),
	}
	nullFirst, err := List[*fakeEntity](context.Background(), &fakeListQuery{rows: nullRows, count: 2, spy: new(fakeQuerySpy)}, firstQuery, fields, []byte("scope"))
	if err != nil {
		t.Fatalf("NULL first List: %v", err)
	}
	nullSecondQuery := prepareListInput(t, corecrud.ListInput{
		Collection: "tenants/acme/users", PageSize: 1, PageToken: nullFirst.NextPageToken(),
	})
	nullSecondSpy := new(fakeQuerySpy)
	_, err = List[*fakeEntity](context.Background(), &fakeListQuery{rows: nullRows, count: 2, spy: nullSecondSpy}, nullSecondQuery, fields, []byte("scope"))
	if err != nil {
		t.Fatalf("NULL second List: %v", err)
	}
	if !strings.Contains(nullSecondSpy.allSQL, "nickname` IS NULL") || !strings.Contains(nullSecondSpy.allSQL, "id` > ?") {
		t.Fatalf("NULL cursor keyset does not remain in NULL partition: %s", nullSecondSpy.allSQL)
	}
}

func TestCustomOrderRequiresExtractorAndUsesAlias(t *testing.T) {
	t.Parallel()

	expression := func(selector *sql.Selector) sql.Querier {
		return sql.ExprFunc(func(builder *sql.Builder) {
			builder.WriteString("UPPER(").Ident(selector.C("display_name")).WriteString(")")
		})
	}
	_, err := NewListFields[*fakeEntity](
		Columns(testValidColumn("id")),
		CustomOrder[*fakeEntity](examplev1.UserFields.DisplayName, "display_sort", expression, nil),
		DefaultOrder(examplev1.UserFields.DisplayName, corecrud.OrderAscending),
		CursorKey[uint32]("id", corecrud.OrderAscending),
	)
	if err == nil || !strings.Contains(err.Error(), "typed cursor extractor") {
		t.Fatalf("NewListFields missing extractor error = %v", err)
	}

	fields, err := NewListFields[*fakeEntity](
		Columns(testValidColumn("id")),
		CustomOrder(examplev1.UserFields.DisplayName, "display_sort", expression, func(entity *fakeEntity, alias string) (any, error) {
			return entity.Value(alias)
		}),
		DefaultOrder(examplev1.UserFields.DisplayName, corecrud.OrderAscending),
		CursorKey[uint32]("id", corecrud.OrderAscending),
	)
	if err != nil {
		t.Fatalf("NewListFields custom order: %v", err)
	}
	query := prepareListInput(t, corecrud.ListInput{Collection: "tenants/acme/users", PageSize: 1})
	rows := []*fakeEntity{
		{values: map[string]any{cursorAlias(0): "BETA", cursorAlias(1): uint32(2)}},
		{values: map[string]any{cursorAlias(0): "ALPHA", cursorAlias(1): uint32(1)}},
	}
	spy := new(fakeQuerySpy)
	result, err := List[*fakeEntity](context.Background(), &fakeListQuery{rows: rows, count: 2, spy: spy}, query, fields, []byte("scope"))
	if err != nil {
		t.Fatalf("List custom order: %v", err)
	}
	if result.NextPageToken() == "" || !strings.Contains(spy.allSQL, "UPPER(") || !strings.Contains(spy.allSQL, cursorAlias(0)) {
		t.Fatalf("custom order did not select/extract alias: token=%q SQL=%s", result.NextPageToken(), spy.allSQL)
	}
}

func TestMySQLDialectBuildsBinaryNullableQuery(t *testing.T) {
	t.Parallel()

	fields, err := NewListFields[*fakeEntity](
		Columns(testValidColumn("nickname", "id")),
		Bind(examplev1.UserFields.Nickname, "nickname").Filter().Order().Nullable(),
		DefaultOrder(examplev1.UserFields.Nickname, corecrud.OrderAscending),
		CursorKey[uint32]("id", corecrud.OrderAscending),
	)
	if err != nil {
		t.Fatalf("NewListFields: %v", err)
	}
	query := prepareListInput(t, corecrud.ListInput{
		Collection: "tenants/acme/users", PageSize: 2, Filter: `nickname >= "m"`,
	})
	spy := new(fakeQuerySpy)
	_, err = List[*fakeEntity](context.Background(), &fakeListQuery{
		rows: []*fakeEntity{fakeOrderedRow("zulu", uint32(1))},
		spy:  spy, dialect: dialect.MySQL,
	}, query, fields, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !strings.Contains(spy.allSQL, "BINARY") || !strings.Contains(spy.allSQL, "CASE WHEN") {
		t.Fatalf("MySQL SQL misses binary/null-rank contract: %s", spy.allSQL)
	}
	if strings.Contains(spy.allSQL, "COLLATE") {
		t.Fatalf("MySQL SQL contains foreign collation syntax: %s", spy.allSQL)
	}
}

func TestListPreservesEntDriverErrors(t *testing.T) {
	t.Parallel()

	fields := mustFakeListFields(t, Bind(examplev1.UserFields.CreateTime, "created_at").Order())
	queryWithCount := prepareListInput(t, corecrud.ListInput{
		Collection: "tenants/acme/users", IncludeTotal: true,
	})
	countCause := &fakeDriverError{operation: "count"}
	_, err := List[*fakeEntity](context.Background(), &fakeListQuery{
		countErr: countCause, spy: new(fakeQuerySpy),
	}, queryWithCount, fields, nil)
	if !errors.Is(err, countCause) {
		t.Fatalf("Count error chain lost: %v", err)
	}
	var typed *fakeDriverError
	if !errors.As(err, &typed) || typed.operation != "count" {
		t.Fatalf("Count errors.As = %#v, want count driver error", typed)
	}

	query := prepareListInput(t, corecrud.ListInput{Collection: "tenants/acme/users"})
	allCause := &fakeDriverError{operation: "all"}
	_, err = List[*fakeEntity](context.Background(), &fakeListQuery{
		allErr: allCause, spy: new(fakeQuerySpy),
	}, query, fields, nil)
	if !errors.Is(err, allCause) {
		t.Fatalf("All error chain lost: %v", err)
	}
}

func mustFakeListFields(t *testing.T, bindings ...Binding) *ListFields[*fakeEntity] {
	t.Helper()
	options := []ListFieldsOption{Columns(testValidColumn("email", "created_at", "id"))}
	for _, binding := range bindings {
		options = append(options, binding)
	}
	options = append(options,
		DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderDescending),
		CursorKey[uint32]("id", corecrud.OrderAscending),
	)
	fields, err := NewListFields[*fakeEntity](options...)
	if err != nil {
		t.Fatalf("NewListFields: %v", err)
	}
	return fields
}

func prepareListInput(t *testing.T, input corecrud.ListInput) corecrud.ListQuery {
	t.Helper()
	preparer, err := corecrud.NewListPreparer()
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	plan := corecrud.MustBuildResourcePlan[*examplev1.User](examplev1.UserCRUDDescriptor())
	query, err := preparer.PrepareList(plan, input)
	if err != nil {
		t.Fatalf("PrepareList: %v", err)
	}
	return query
}

type fakeEntity struct{ values map[string]any }

func (entity *fakeEntity) Value(name string) (ent.Value, error) {
	value, ok := entity.values[name]
	if !ok {
		return nil, fmt.Errorf("missing value %s", name)
	}
	return value, nil
}

func fakeOrderedRow(value any, id uint32) *fakeEntity {
	return &fakeEntity{values: map[string]any{
		cursorAlias(0): value,
		cursorAlias(1): id,
	}}
}

func fakeRow(created string, id uint32) *fakeEntity {
	parsed, err := time.Parse(time.RFC3339, created)
	if err != nil {
		panic(err)
	}
	return &fakeEntity{values: map[string]any{
		cursorAlias(0): parsed,
		cursorAlias(1): id,
	}}
}

type fakeQuerySpy struct {
	countSQL string
	allSQL   string
	allLimit int
}

type fakeDriverError struct{ operation string }

func (err *fakeDriverError) Error() string { return "driver " + err.operation + " failed" }

type fakeListQuery struct {
	rows      []*fakeEntity
	dialect   string
	count     int
	countErr  error
	allErr    error
	modifiers []func(*sql.Selector)
	limit     int
	offset    int
	spy       *fakeQuerySpy
}

func (query *fakeListQuery) Clone() *fakeListQuery {
	clone := *query
	clone.modifiers = append([]func(*sql.Selector){}, query.modifiers...)
	return &clone
}

func (query *fakeListQuery) Modify(modifiers ...func(*sql.Selector)) *fakeListQuery {
	query.modifiers = append(query.modifiers, modifiers...)
	return query
}

func (query *fakeListQuery) Limit(limit int) *fakeListQuery {
	query.limit = limit
	return query
}

func (query *fakeListQuery) Offset(offset int) *fakeListQuery {
	query.offset = offset
	return query
}

func (query *fakeListQuery) Count(context.Context) (int, error) {
	selector := query.selector()
	statement, _ := selector.Query()
	query.spy.countSQL = statement
	return query.count, query.countErr
}

func (query *fakeListQuery) All(context.Context) ([]*fakeEntity, error) {
	selector := query.selector()
	statement, _ := selector.Query()
	query.spy.allSQL = statement
	query.spy.allLimit = query.limit
	if query.allErr != nil {
		return nil, query.allErr
	}
	start := query.offset
	if start > len(query.rows) {
		start = len(query.rows)
	}
	end := len(query.rows)
	if query.limit > 0 && start+query.limit < end {
		end = start + query.limit
	}
	return query.rows[start:end], nil
}

func (query *fakeListQuery) selector() *sql.Selector {
	queryDialect := query.dialect
	if queryDialect == "" {
		queryDialect = dialect.SQLite
	}
	selector := sql.Dialect(queryDialect).Select("*").From(sql.Table("users"))
	for _, modifier := range query.modifiers {
		modifier(selector)
	}
	return selector
}
