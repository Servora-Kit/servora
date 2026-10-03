//go:build integration

package crud_test

import (
	"context"
	stdsql "database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	"github.com/Servora-Kit/servora/contrib/db/entgo/crud"
	"github.com/Servora-Kit/servora/contrib/db/entgo/crud/testdata/entfixture"
	entrow "github.com/Servora-Kit/servora/contrib/db/entgo/crud/testdata/entfixture/contractrow"
	_ "github.com/Servora-Kit/servora/contrib/db/entgo/crud/testdata/entfixture/runtime"
	corecrud "github.com/Servora-Kit/servora/core/crud"
	crudmapper "github.com/Servora-Kit/servora/core/crud/mapper"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	sqlite3 "github.com/mattn/go-sqlite3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

const (
	sqliteDSNEnv   = "SERVORA_ENT_SQLITE_DSN"
	postgresDSNEnv = "SERVORA_ENT_POSTGRES_DSN"
)

var liveBaseTime = time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC)

func TestSQLiteLiveContract(t *testing.T) {
	client := openLiveFixture(t, dialect.SQLite, "sqlite3", sqliteDSNEnv)
	fields := liveListFields(t)

	t.Run("full list and reference flow", func(t *testing.T) {
		rows := resetContractRows(t, client)
		testFullListContract(t, client, fields, rows)
	})
	t.Run("wildcard string match", func(t *testing.T) {
		resetContractRows(t, client)
		testWildcardStringMatches(t, client, fields)
	})
	t.Run("nullable keyset", func(t *testing.T) {
		resetContractRows(t, client)
		testNullableKeyset(t, client, fields)
	})
	t.Run("tied timestamp cursor", func(t *testing.T) {
		resetContractRows(t, client)
		testTiedTimestampCursor(t, client, fields)
	})
	t.Run("duration cursor", func(t *testing.T) {
		resetContractRows(t, client)
		testDurationCursor(t, client)
	})
	t.Run("write mask clear", func(t *testing.T) {
		rows := resetContractRows(t, client)
		testWriteMaskClear(t, client, rows[0])
	})
	t.Run("caller query scope", func(t *testing.T) {
		rows := resetContractRows(t, client)
		testScopedQuery(t, client, fields, rows)
	})
	t.Run("native error chain", func(t *testing.T) {
		resetContractRows(t, client)
		err := executeInvalidColumnList(t, client)
		var driverError sqlite3.Error
		if !errors.As(err, &driverError) {
			t.Fatalf("errors.As(%T) failed for %v", driverError, err)
		}
	})
}

func TestPostgresLiveContract(t *testing.T) {
	client := openLiveFixture(t, dialect.Postgres, "pgx", postgresDSNEnv)
	fields := liveListFields(t)

	t.Run("binary collation and nullable keyset", func(t *testing.T) {
		resetContractRows(t, client)
		testBinaryCollation(t, client, fields)
		testNullableKeyset(t, client, fields)
	})
	t.Run("wildcard string match", func(t *testing.T) {
		resetContractRows(t, client)
		testWildcardStringMatches(t, client, fields)
	})
	t.Run("numeric storage and time cursors", func(t *testing.T) {
		rows := resetContractRows(t, client)
		testNumericStorage(t, client, rows)
		testTimestampCursor(t, client, fields)
		testDurationCursor(t, client)
	})
	t.Run("tied timestamp cursor", func(t *testing.T) {
		resetContractRows(t, client)
		testTiedTimestampCursor(t, client, fields)
	})
	t.Run("write mask clear", func(t *testing.T) {
		rows := resetContractRows(t, client)
		testWriteMaskClear(t, client, rows[0])
	})
	t.Run("caller query scope", func(t *testing.T) {
		rows := resetContractRows(t, client)
		testScopedQuery(t, client, fields, rows)
	})
	t.Run("native error chain", func(t *testing.T) {
		resetContractRows(t, client)
		err := executeInvalidColumnList(t, client)
		var driverError *pgconn.PgError
		if !errors.As(err, &driverError) {
			t.Fatalf("errors.As(*pgconn.PgError) failed for %v", err)
		}
	})
}

func openLiveFixture(t *testing.T, entDialect, driverName, envName string) *entfixture.Client {
	t.Helper()
	dsn := os.Getenv(envName)
	if dsn == "" {
		t.Fatalf("%s is required; use the explicit Just integration recipe", envName)
	}
	database, err := stdsql.Open(driverName, dsn)
	if err != nil {
		t.Fatalf("sql.Open(%s): %v", driverName, err)
	}
	if entDialect == dialect.SQLite {
		database.SetMaxOpenConns(1)
	}
	if err := database.PingContext(context.Background()); err != nil {
		_ = database.Close()
		t.Fatalf("ping %s DSN: %v", entDialect, err)
	}
	driver := entsql.OpenDB(entDialect, database)
	client := entfixture.NewClient(entfixture.Driver(driver))
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close %s fixture: %v", entDialect, err)
		}
	})
	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("create %s fixture schema: %v", entDialect, err)
	}
	return client
}

func resetContractRows(t *testing.T, client *entfixture.Client) []*entfixture.ContractRow {
	t.Helper()
	ctx := context.Background()
	if _, err := client.ContractRow.Delete().Exec(ctx); err != nil {
		t.Fatalf("clear fixture contract rows: %v", err)
	}
	type seed struct {
		id            uint32
		textValue     string
		uniqueText    string
		numericValue  int32
		enumNumber    crudpb.CrudErrorReason
		nullableText  *string
		timestamp     time.Time
		durationValue time.Duration
	}
	alpha := "Alpha"
	beta := "beta"
	seeds := []seed{
		{id: 10, textValue: "zulu", uniqueText: "a@example.com", numericValue: 1, enumNumber: crudpb.CrudErrorReason_CRUD_ERROR_REASON_INVALID_RESOURCE_NAME, nullableText: &beta, timestamp: liveBaseTime.Add(time.Second), durationValue: time.Second},
		{id: 20, textValue: "Alpha", uniqueText: "b@example.com", numericValue: 2, enumNumber: crudpb.CrudErrorReason_CRUD_ERROR_REASON_INVALID_PAGE_TOKEN, timestamp: liveBaseTime.Add(2 * time.Second), durationValue: 2 * time.Second},
		{id: 30, textValue: "beta", uniqueText: "c@example.com", numericValue: 3, enumNumber: crudpb.CrudErrorReason_CRUD_ERROR_REASON_INVALID_FILTER, nullableText: &alpha, timestamp: liveBaseTime.Add(3 * time.Second), durationValue: 3 * time.Second},
		{id: 40, textValue: "Bravo", uniqueText: "d@example.com", numericValue: 4, enumNumber: crudpb.CrudErrorReason_CRUD_ERROR_REASON_INVALID_ORDER_BY, timestamp: liveBaseTime.Add(4 * time.Second), durationValue: 4 * time.Second},
	}
	result := make([]*entfixture.ContractRow, len(seeds))
	for index, item := range seeds {
		created, err := client.ContractRow.Create().
			SetID(item.id).
			SetTextValue(item.textValue).
			SetUniqueText(item.uniqueText).
			SetProfile(map[string]any{"contact": map[string]any{"email": item.textValue}}).
			SetNumericValue(item.numericValue).
			SetEnumNumber(item.enumNumber).
			SetNillableNullableText(item.nullableText).
			SetTimestampValue(item.timestamp).
			SetUpdatedTimestamp(item.timestamp).
			SetDurationValue(item.durationValue).
			Save(ctx)
		if err != nil {
			t.Fatalf("seed contract row %d: %v", item.id, err)
		}
		result[index] = created
	}
	return result
}

func liveListFields(t *testing.T) *crud.ListFields[*entfixture.ContractRow] {
	t.Helper()
	fields, err := crud.NewListFields[*entfixture.ContractRow](
		crud.Columns(entrow.ValidColumn),
		crud.Bind(examplev1.UserFields.DisplayName, entrow.FieldTextValue).Filter().Order(),
		crud.JSONPath(examplev1.UserFields.Email, entrow.FieldProfile, "contact", "email").Filter().Nullable(),
		crud.Custom(examplev1.UserFields.TenantPlan, entrow.FieldNumericValue, planPredicate).Filter(),
		crud.Bind(examplev1.UserFields.Nickname, entrow.FieldNullableText).Filter().Order().Nullable(),
		crud.Bind(examplev1.UserFields.CreateTime, entrow.FieldTimestampValue).Filter().Order(),
		crud.Bind(examplev1.UserFields.UpdateTime, entrow.FieldUpdatedTimestamp).Filter().Order(),
		crud.Bind(examplev1.UserFields.DeleteTime, entrow.FieldDeleteTime).Filter().Order().Nullable(),
		crud.DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderDescending),
		crud.CursorKey[uint32](entrow.FieldID, corecrud.OrderAscending),
	)
	if err != nil {
		t.Fatalf("NewListFields: %v", err)
	}
	return fields
}

func planPredicate(
	operator corecrud.FilterOperator,
	value corecrud.FilterValue,
) (crud.SelectorPredicate, error) {
	match, ok := value.StringMatch()
	if !ok || match.Kind() != corecrud.StringMatchExact {
		return nil, fmt.Errorf("tenant_plan requires an exact string match")
	}
	number, ok := map[string]int32{"free": 1, "team": 2, "business": 3, "enterprise": 4}[match.Literal()]
	if !ok {
		return nil, fmt.Errorf("unknown tenant_plan %q", match.Literal())
	}
	if operator != corecrud.FilterOperatorEqual && operator != corecrud.FilterOperatorNotEqual {
		return nil, fmt.Errorf("tenant_plan does not support operator %q", operator)
	}
	return func(selector *entsql.Selector) *entsql.Predicate {
		if operator == corecrud.FilterOperatorEqual {
			return entsql.EQ(selector.C(entrow.FieldNumericValue), number)
		}
		return entsql.NEQ(selector.C(entrow.FieldNumericValue), number)
	}, nil
}

func prepareLiveQuery(t *testing.T, input corecrud.ListInput) corecrud.ListQuery {
	t.Helper()
	preparer, err := corecrud.NewListPreparer(corecrud.WithApplicationDefaults(
		corecrud.WithDefaultPageSize(2),
		corecrud.WithMaxPageSize(3),
	))
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := corecrud.MustBuildResourcePlan[*examplev1.User](descriptor)
	query, err := preparer.PrepareList(plan, input)
	if err != nil {
		t.Fatalf("PrepareList(%+v): %v", input, err)
	}
	return query
}

func listLive(t *testing.T, client *entfixture.Client, fields *crud.ListFields[*entfixture.ContractRow], input corecrud.ListInput, scope []byte) corecrud.ListResult[*entfixture.ContractRow] {
	t.Helper()
	result, err := crud.List(context.Background(), client.ContractRow.Query(), prepareLiveQuery(t, input), fields, scope)
	if err != nil {
		t.Fatalf("List(%+v): %v", input, err)
	}
	return result
}

func testFullListContract(t *testing.T, client *entfixture.Client, fields *crud.ListFields[*entfixture.ContractRow], users []*entfixture.ContractRow) {
	t.Helper()
	scope := []byte("tenant:acme")
	filtered := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", Filter: `display_name = "Alpha"`, IncludeTotal: true,
	}, scope)
	if got := ids(filtered.Items()); !slices.Equal(got, []uint32{20}) {
		t.Fatalf("filtered IDs = %v, want [20]", got)
	}
	if total, present := filtered.TotalSize(); !present || total != 1 {
		t.Fatalf("filtered TotalSize = (%d, %v), want (1, true)", total, present)
	}

	first := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", PageSize: 2,
	}, scope)
	if got := ids(first.Items()); !slices.Equal(got, []uint32{40, 30}) || first.NextPageToken() == "" {
		t.Fatalf("first page = %v token=%q", got, first.NextPageToken())
	}
	second := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", PageSize: 2, PageToken: first.NextPageToken(),
	}, scope)
	if got := ids(second.Items()); !slices.Equal(got, []uint32{20, 10}) || second.NextPageToken() != "" {
		t.Fatalf("second page = %v token=%q", got, second.NextPageToken())
	}

	skipped := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", PageSize: 2, Skip: 1, OrderBy: "display_name",
	}, scope)
	if got := ids(skipped.Items()); !slices.Equal(got, []uint32{40, 30}) {
		t.Fatalf("skip/order IDs = %v, want [40 30]", got)
	}
	capped := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", PageSize: 99,
	}, scope)
	if got := len(capped.Items()); got != 3 {
		t.Fatalf("capped page len = %d, want 3", got)
	}

	mapper := liveResourceMapper(t)
	dtos, err := mapper.TryToDTOs(first.Items())
	if err != nil {
		t.Fatalf("TryToDTOs: %v", err)
	}
	response := &examplev1.ListUsersResponse{Users: dtos, NextPageToken: first.NextPageToken()}
	if got, want := response.GetUsers()[0].GetName(), "tenants/acme/users/40"; got != want {
		t.Fatalf("reference flow first name = %q, want %q", got, want)
	}
	if got, want := response.GetUsers()[1].GetCreateTime().AsTime(), users[2].TimestampValue; !got.Equal(want) {
		t.Fatalf("reference flow create_time = %v, want %v", got, want)
	}

	_, err = crud.List(context.Background(), client.ContractRow.Query(), prepareLiveQuery(t, corecrud.ListInput{
		Collection: "tenants/acme/users", PageToken: first.NextPageToken(),
	}), fields, []byte("tenant:other"))
	if err == nil {
		t.Fatal("List accepted a page token under a different scope fingerprint")
	}
}

func testBinaryCollation(t *testing.T, client *entfixture.Client, fields *crud.ListFields[*entfixture.ContractRow]) {
	t.Helper()
	result := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", PageSize: 3, OrderBy: "display_name",
	}, nil)
	if got := displayNames(result.Items()); !slices.Equal(got, []string{"Alpha", "Bravo", "beta"}) {
		t.Fatalf("binary collation names = %v, want [Alpha Bravo beta]", got)
	}
}

func testWildcardStringMatches(
	t *testing.T,
	client *entfixture.Client,
	fields *crud.ListFields[*entfixture.ContractRow],
) {
	t.Helper()
	ctx := context.Background()
	extras := []struct {
		id      uint32
		text    string
		profile map[string]any
	}{
		{id: 50, text: "CaseTail", profile: map[string]any{"contact": map[string]any{"email": "CaseTail"}}},
		{id: 60, text: "casetail", profile: map[string]any{"contact": map[string]any{"email": "casetail"}}},
		{id: 70, text: "café", profile: map[string]any{"contact": map[string]any{"email": "café"}}},
		{id: 80, text: "cafe", profile: map[string]any{"contact": map[string]any{"email": "cafe"}}},
		{id: 90, text: "", profile: map[string]any{"contact": map[string]any{"email": ""}}},
		{id: 100, text: "literal*", profile: map[string]any{"contact": map[string]any{"email": "literal*"}}},
		{id: 110, text: "missing", profile: map[string]any{}},
		{id: 120, text: "json-null", profile: map[string]any{"contact": map[string]any{"email": nil}}},
		{id: 130, text: "string-null", profile: map[string]any{"contact": map[string]any{"email": "null"}}},
	}
	for _, extra := range extras {
		if _, err := client.ContractRow.Create().
			SetID(extra.id).
			SetTextValue(extra.text).
			SetUniqueText(fmt.Sprintf("wildcard-%d@example.com", extra.id)).
			SetProfile(extra.profile).
			SetNumericValue(int32(extra.id)).
			Save(ctx); err != nil {
			t.Fatalf("seed wildcard row %d: %v", extra.id, err)
		}
	}

	patterns := []struct {
		name  string
		value string
		want  []uint32
	}{
		{name: "prefix is case-sensitive", value: `"Case*"`, want: []uint32{50}},
		{name: "suffix is case-sensitive", value: `"*Tail"`, want: []uint32{50}},
		{name: "contains is case-sensitive", value: `"*aseT*"`, want: []uint32{50}},
		{name: "accent is significant", value: `"*é*"`, want: []uint32{70}},
		{name: "literal star", value: `"literal\\*"`, want: []uint32{100}},
		{name: "empty exact", value: `""`, want: []uint32{90}},
	}
	for _, test := range patterns {
		t.Run(test.name, func(t *testing.T) {
			column := listLive(t, client, fields, corecrud.ListInput{
				Collection: "tenants/acme/users", Filter: "display_name = " + test.value,
			}, nil)
			jsonPath := listLive(t, client, fields, corecrud.ListInput{
				Collection: "tenants/acme/users", Filter: "email = " + test.value,
			}, nil)
			if got := ids(column.Items()); !slices.Equal(got, test.want) {
				t.Fatalf("column IDs = %v, want %v", got, test.want)
			}
			if got := ids(jsonPath.Items()); !slices.Equal(got, test.want) {
				t.Fatalf("JSONPath IDs = %v, want %v", got, test.want)
			}
		})
	}

	columnAll := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", Filter: `display_name = "*"`, IncludeTotal: true,
	}, nil)
	if total, present := columnAll.TotalSize(); !present || total != 13 {
		t.Fatalf("column match-all total = (%d, %v), want (13, true)", total, present)
	}
	jsonAll := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", Filter: `email = "*"`, IncludeTotal: true,
	}, nil)
	if total, present := jsonAll.TotalSize(); !present || total != 11 {
		t.Fatalf("JSONPath match-all total = (%d, %v), want (11, true)", total, present)
	}

	jsonNotAll := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", Filter: `email != "*"`, IncludeTotal: true,
	}, nil)
	if total, present := jsonNotAll.TotalSize(); !present || total != 0 {
		t.Fatalf("JSONPath negated match-all total = (%d, %v), want (0, true)", total, present)
	}
	jsonNull := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", Filter: `email = null`,
	}, nil)
	jsonNullIDs := ids(jsonNull.Items())
	slices.Sort(jsonNullIDs)
	if !slices.Equal(jsonNullIDs, []uint32{110, 120}) {
		t.Fatalf("JSONPath NULL IDs = %v, want [110 120]", jsonNullIDs)
	}
	jsonNotNull := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", Filter: `email != null`, IncludeTotal: true,
	}, nil)
	if total, present := jsonNotNull.TotalSize(); !present || total != 11 {
		t.Fatalf("JSONPath non-NULL total = (%d, %v), want (11, true)", total, present)
	}
	jsonStringNull := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", Filter: `email = "null"`,
	}, nil)
	if got := ids(jsonStringNull.Items()); !slices.Equal(got, []uint32{130}) {
		t.Fatalf("JSONPath string-null IDs = %v, want [130]", got)
	}
	jsonNotPrefix := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", Filter: `email != "Case*"`, IncludeTotal: true,
	}, nil)
	if total, present := jsonNotPrefix.TotalSize(); !present || total != 10 {
		t.Fatalf("JSONPath negated prefix total = (%d, %v), want (10, true)", total, present)
	}
}

func testNullableKeyset(t *testing.T, client *entfixture.Client, fields *crud.ListFields[*entfixture.ContractRow]) {
	t.Helper()
	for _, orderBy := range []string{"nickname", "nickname desc"} {
		var got []uint32
		var token string
		for {
			result := listLive(t, client, fields, corecrud.ListInput{
				Collection: "tenants/acme/users", PageSize: 1, PageToken: token, OrderBy: orderBy,
			}, nil)
			got = append(got, ids(result.Items())...)
			token = result.NextPageToken()
			if token == "" {
				break
			}
		}
		want := []uint32{30, 10, 20, 40}
		if orderBy == "nickname desc" {
			want = []uint32{10, 30, 20, 40}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("%s keyset IDs = %v, want %v", orderBy, got, want)
		}
	}
}

func testTimestampCursor(t *testing.T, client *entfixture.Client, fields *crud.ListFields[*entfixture.ContractRow]) {
	t.Helper()
	first := listLive(t, client, fields, corecrud.ListInput{Collection: "tenants/acme/users", PageSize: 1}, nil)
	second := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", PageSize: 1, PageToken: first.NextPageToken(),
	}, nil)
	if got := ids(second.Items()); !slices.Equal(got, []uint32{30}) {
		t.Fatalf("timestamp cursor second page = %v, want [30]", got)
	}
}

func testTiedTimestampCursor(t *testing.T, client *entfixture.Client, fields *crud.ListFields[*entfixture.ContractRow]) {
	t.Helper()
	tied := liveBaseTime.Add(10 * time.Second).In(time.FixedZone("UTC+8", 8*60*60))
	if _, err := client.ContractRow.Update().
		Where(entrow.IDIn(30, 40)).
		Modify(func(builder *entsql.UpdateBuilder) {
			builder.Set(entrow.FieldTimestampValue, tied)
		}).
		Save(context.Background()); err != nil {
		t.Fatalf("set tied timestamp: %v", err)
	}
	first := listLive(t, client, fields, corecrud.ListInput{Collection: "tenants/acme/users", PageSize: 1}, nil)
	if got := ids(first.Items()); !slices.Equal(got, []uint32{30}) || first.NextPageToken() == "" {
		t.Fatalf("tied timestamp first page = %v token=%q", got, first.NextPageToken())
	}
	second := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", PageSize: 1, PageToken: first.NextPageToken(),
	}, nil)
	if got := ids(second.Items()); !slices.Equal(got, []uint32{40}) {
		t.Fatalf("tied timestamp second page = %v, want [40]", got)
	}
}

func testDurationCursor(t *testing.T, client *entfixture.Client) {
	t.Helper()
	fields, err := crud.NewListFields[*entfixture.ContractRow](
		crud.Columns(entrow.ValidColumn),
		crud.Bind(examplev1.UserFields.CreateTime, entrow.FieldTimestampValue).Order(),
		crud.DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderAscending),
		crud.CursorKey[time.Duration](entrow.FieldDurationValue, corecrud.OrderAscending).WithConverter(func(value any) (time.Duration, error) {
			switch typed := value.(type) {
			case time.Duration:
				return typed, nil
			case int64:
				return time.Duration(typed), nil
			default:
				return 0, fmt.Errorf("retention_period cursor has type %T", value)
			}
		}),
	)
	if err != nil {
		t.Fatalf("duration ListFields: %v", err)
	}
	var got []uint32
	var token string
	for {
		result := listLive(t, client, fields, corecrud.ListInput{
			Collection: "tenants/acme/users", PageSize: 1, PageToken: token,
		}, nil)
		got = append(got, ids(result.Items())...)
		token = result.NextPageToken()
		if token == "" {
			break
		}
	}
	if !slices.Equal(got, []uint32{10, 20, 30, 40}) {
		t.Fatalf("duration cursor IDs = %v", got)
	}
}

func testNumericStorage(t *testing.T, client *entfixture.Client, rows []*entfixture.ContractRow) {
	t.Helper()
	found, err := client.ContractRow.Query().Where(entrow.NumericValueEQ(3)).Only(context.Background())
	if err != nil {
		t.Fatalf("query numeric value: %v", err)
	}
	if found.ID != rows[2].ID || found.NumericValue != 3 {
		t.Fatalf("numeric value result = %+v", found)
	}
	found, err = client.ContractRow.Query().Where(entrow.EnumNumberEQ(crudpb.CrudErrorReason_CRUD_ERROR_REASON_INVALID_FILTER)).Only(context.Background())
	if err != nil {
		t.Fatalf("query Proto enum number: %v", err)
	}
	if found.ID != rows[2].ID || found.EnumNumber != crudpb.CrudErrorReason_CRUD_ERROR_REASON_INVALID_FILTER {
		t.Fatalf("Proto enum number result = %+v", found)
	}
}

func testWriteMaskClear(t *testing.T, client *entfixture.Client, row *entfixture.ContractRow) {
	t.Helper()
	plan := corecrud.MustBuildResourcePlan[*examplev1.User](
		examplev1.File_servora_example_v1_example_proto.Messages().ByName("User"),
	)
	prepared, err := plan.PrepareUpdate(
		&examplev1.User{Name: fmt.Sprintf("tenants/acme/users/%d", row.ID)},
		&fieldmaskpb.FieldMask{Paths: []string{"display_name", "nickname"}},
		corecrud.UpdateOptions{},
	)
	if err != nil {
		t.Fatalf("PrepareUpdate clear: %v", err)
	}
	helper, err := crud.NewClearHelper[*entfixture.ContractRowMutation](
		crud.ClearToValue[*entfixture.ContractRowMutation](examplev1.UserFields.DisplayName, func(mutation *entfixture.ContractRowMutation) error {
			mutation.SetTextValue("")
			return nil
		}),
		crud.RenameClear[*entfixture.ContractRowMutation](examplev1.UserFields.Nickname, entrow.FieldNullableText),
	)
	if err != nil {
		t.Fatalf("NewClearHelper: %v", err)
	}
	update := client.ContractRow.UpdateOneID(row.ID)
	if err := helper.Apply(prepared.Resource(), prepared.WriteMask(), update.Mutation()); err != nil {
		t.Fatalf("ClearHelper.Apply: %v", err)
	}
	updated, err := update.Save(context.Background())
	if err != nil {
		t.Fatalf("save cleared contract row: %v", err)
	}
	if updated.TextValue != "" || updated.NullableText != nil {
		t.Fatalf("cleared contract row text_value=%q nullable_text=%v", updated.TextValue, updated.NullableText)
	}
}

func testScopedQuery(t *testing.T, client *entfixture.Client, fields *crud.ListFields[*entfixture.ContractRow], rows []*entfixture.ContractRow) {
	t.Helper()
	ctx := context.Background()
	if _, err := client.ContractRow.UpdateOneID(rows[2].ID).SetDeleteTime(liveBaseTime.Add(5 * time.Second)).Save(ctx); err != nil {
		t.Fatalf("set fixture tombstone: %v", err)
	}

	// The caller owns both predicates; the adapter preserves them in count and page clones.
	builder := client.ContractRow.Query().Where(entrow.DeleteTimeIsNil(), entrow.NumericValueGTE(2))
	scope := []byte("active:numeric_value>=2")
	listScoped := func(t *testing.T, input corecrud.ListInput) corecrud.ListResult[*entfixture.ContractRow] {
		t.Helper()
		result, err := crud.List(ctx, builder, prepareLiveQuery(t, input), fields, scope)
		if err != nil {
			t.Fatalf("List caller-scoped query (%+v): %v", input, err)
		}
		return result
	}

	first := listScoped(t, corecrud.ListInput{
		Collection: "tenants/acme/users", PageSize: 1, IncludeTotal: true,
	})
	if got := ids(first.Items()); !slices.Equal(got, []uint32{40}) || first.NextPageToken() == "" {
		t.Fatalf("scoped first page = %v token=%q, want [40] with continuation", got, first.NextPageToken())
	}
	if total, present := first.TotalSize(); !present || total != 2 {
		t.Fatalf("scoped first total = (%d, %v), want (2, true)", total, present)
	}
	second := listScoped(t, corecrud.ListInput{
		Collection: "tenants/acme/users", PageSize: 1, PageToken: first.NextPageToken(), IncludeTotal: true,
	})
	if got := ids(second.Items()); !slices.Equal(got, []uint32{20}) || second.NextPageToken() != "" {
		t.Fatalf("scoped second page = %v token=%q, want [20] without continuation", got, second.NextPageToken())
	}
	if total, present := second.TotalSize(); !present || total != 2 {
		t.Fatalf("scoped continuation total = (%d, %v), want (2, true)", total, present)
	}
	skipped := listScoped(t, corecrud.ListInput{
		Collection: "tenants/acme/users", PageSize: 1, Skip: 1, IncludeTotal: true,
	})
	if got := ids(skipped.Items()); !slices.Equal(got, []uint32{20}) || skipped.NextPageToken() != "" {
		t.Fatalf("scoped skipped page = %v token=%q, want [20] without continuation", got, skipped.NextPageToken())
	}
	if total, present := skipped.TotalSize(); !present || total != 2 {
		t.Fatalf("scoped skipped total = (%d, %v), want (2, true)", total, present)
	}

	for _, test := range []struct {
		name   string
		filter string
		want   []uint32
	}{
		{name: "filter within scope", filter: `tenant_plan = "team"`, want: []uint32{20}},
		{name: "filter cannot expose tombstone", filter: `tenant_plan = "business"`},
		{name: "filter cannot widen numeric scope", filter: `tenant_plan = "free"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := listScoped(t, corecrud.ListInput{
				Collection: "tenants/acme/users", Filter: test.filter, IncludeTotal: true,
			})
			if got := ids(result.Items()); !slices.Equal(got, test.want) || result.NextPageToken() != "" {
				t.Fatalf("scoped filtered page = %v token=%q, want %v without continuation", got, result.NextPageToken(), test.want)
			}
			if total, present := result.TotalSize(); !present || total != int64(len(test.want)) {
				t.Fatalf("scoped filtered total = (%d, %v), want (%d, true)", total, present, len(test.want))
			}
		})
	}
	if count, err := builder.Count(ctx); err != nil || count != 2 {
		t.Fatalf("caller builder count after lists = (%d, %v), want (2, nil)", count, err)
	}

	unscoped := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", IncludeTotal: true,
	}, nil)
	if total, present := unscoped.TotalSize(); !present || total != int64(len(rows)) {
		t.Fatalf("unscoped total = (%d, %v), want (%d, true)", total, present, len(rows))
	}
	tombstones := listLive(t, client, fields, corecrud.ListInput{
		Collection: "tenants/acme/users", Filter: "delete_time != null", IncludeTotal: true,
	}, nil)
	if got := ids(tombstones.Items()); !slices.Equal(got, []uint32{rows[2].ID}) {
		t.Fatalf("unscoped tombstone IDs = %v, want [%d]", got, rows[2].ID)
	}
	if total, present := tombstones.TotalSize(); !present || total != 1 {
		t.Fatalf("unscoped tombstone total = (%d, %v), want (1, true)", total, present)
	}
}

func executeInvalidColumnList(t *testing.T, client *entfixture.Client) error {
	t.Helper()
	bad, err := crud.NewListFields[*entfixture.ContractRow](
		crud.Columns(entrow.ValidColumn),
		crud.Bind(examplev1.UserFields.CreateTime, entrow.FieldTimestampValue).Order(),
		crud.DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderAscending),
		crud.Custom(examplev1.UserFields.Email, "broken_email", func(_ corecrud.FilterOperator, value corecrud.FilterValue) (crud.SelectorPredicate, error) {
			match, ok := value.StringMatch()
			if !ok || match.Kind() != corecrud.StringMatchExact {
				return nil, fmt.Errorf("email requires an exact string match")
			}
			return func(selector *entsql.Selector) *entsql.Predicate {
				return entsql.EQ(selector.C("missing_live_contract_column"), match.Literal())
			}, nil
		}).Filter(),
		crud.CursorKey[uint32](entrow.FieldID, corecrud.OrderAscending),
	)
	if err != nil {
		t.Fatalf("invalid-column ListFields: %v", err)
	}
	_, err = crud.List(context.Background(), client.ContractRow.Query(), prepareLiveQuery(t, corecrud.ListInput{
		Collection: "tenants/acme/users", Filter: `email = "a@example.com"`,
	}), bad, nil)
	if err == nil {
		t.Fatal("invalid column query unexpectedly succeeded")
	}
	return err
}

func liveResourceMapper(t *testing.T) *crudmapper.ResourceMapper[*examplev1.User, entfixture.ContractRow] {
	t.Helper()
	mapper, err := crudmapper.NewResourceMapper[*examplev1.User, entfixture.ContractRow](
		crudmapper.WithConverters(crudmapper.TypeConverter{
			SrcType: int32(0), DstType: (*string)(nil),
			Fn: func(source any) (any, error) {
				value := map[int32]string{1: "free", 2: "team", 3: "business", 4: "enterprise"}[source.(int32)]
				return proto.String(value), nil
			},
		}),
		crudmapper.WithFieldMapping("TextValue", examplev1.UserFields.DisplayName),
		crudmapper.WithFieldMapping("UniqueText", examplev1.UserFields.Email),
		crudmapper.WithFieldMapping("NumericValue", examplev1.UserFields.TenantPlan),
		crudmapper.WithFieldMapping("NullableText", examplev1.UserFields.Nickname),
		crudmapper.WithFieldMapping("TimestampValue", examplev1.UserFields.CreateTime),
		crudmapper.WithFieldMapping("UpdatedTimestamp", examplev1.UserFields.UpdateTime),
		crudmapper.WithResourceName(func(row *entfixture.ContractRow) (string, error) {
			return fmt.Sprintf("tenants/acme/users/%d", row.ID), nil
		}),
	)
	if err != nil {
		t.Fatalf("NewResourceMapper: %v", err)
	}
	return mapper
}

func ids(rows []*entfixture.ContractRow) []uint32 {
	result := make([]uint32, len(rows))
	for index, row := range rows {
		result[index] = row.ID
	}
	return result
}

func displayNames(rows []*entfixture.ContractRow) []string {
	result := make([]string, len(rows))
	for index, row := range rows {
		result[index] = row.TextValue
	}
	return result
}
