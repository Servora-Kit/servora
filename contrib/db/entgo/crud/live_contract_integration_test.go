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
	entgomixin "github.com/Servora-Kit/servora/contrib/db/entgo/mixin"
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
	t.Run("soft delete", func(t *testing.T) {
		rows := resetContractRows(t, client)
		testSoftDelete(t, client, fields, rows[0])
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
	t.Run("soft delete", func(t *testing.T) {
		rows := resetContractRows(t, client)
		testSoftDelete(t, client, fields, rows[0])
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
		t.Fatalf("%s is required; use the explicit Makefile integration target", envName)
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
	ctx := entgomixin.SkipSoftDelete(context.Background())
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
		crud.Bind(examplev1.UserFields.Email, entrow.FieldUniqueText).Filter(),
		crud.Bind(examplev1.UserFields.TenantPlan, entrow.FieldNumericValue).Filter().WithQueryConverter(planNumber),
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

func planNumber(value corecrud.FilterValue) (any, error) {
	plan, ok := value.StringValue()
	if !ok {
		return nil, fmt.Errorf("tenant_plan is not a string literal")
	}
	number, ok := map[string]int32{"free": 1, "team": 2, "business": 3, "enterprise": 4}[plan]
	if !ok {
		return nil, fmt.Errorf("unknown tenant_plan %q", plan)
	}
	return number, nil
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
		Collection: "tenants/acme/users", Filter: `email = "b@example.com"`, IncludeTotal: true,
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

func testSoftDelete(t *testing.T, client *entfixture.Client, fields *crud.ListFields[*entfixture.ContractRow], row *entfixture.ContractRow) {
	t.Helper()
	ctx := entgomixin.WithDeletedBy(context.Background(), "principals/tester")
	if err := client.ContractRow.DeleteOneID(row.ID).Exec(ctx); err != nil {
		t.Fatalf("soft delete contract row: %v", err)
	}
	if exists, err := client.ContractRow.Query().Where(entrow.IDEQ(row.ID)).Exist(context.Background()); err != nil || exists {
		t.Fatalf("default query after delete = (%v, %v), want (false, nil)", exists, err)
	}
	bypass := entgomixin.SkipSoftDelete(context.Background())
	tombstone, err := client.ContractRow.Query().Where(entrow.IDEQ(row.ID)).Only(bypass)
	if err != nil {
		t.Fatalf("read tombstone with bypass: %v", err)
	}
	if tombstone.DeleteTime == nil || tombstone.DeletedBy == nil || *tombstone.DeletedBy != "principals/tester" {
		t.Fatalf("tombstone fields = delete_time:%v deleted_by:%v", tombstone.DeleteTime, tombstone.DeletedBy)
	}
	result, err := crud.List(bypass, client.ContractRow.Query(), prepareLiveQuery(t, corecrud.ListInput{
		Collection: "tenants/acme/users", IncludeTotal: true,
	}), fields, nil)
	if err != nil {
		t.Fatalf("List tombstones with bypass: %v", err)
	}
	if total, present := result.TotalSize(); !present || total != 4 {
		t.Fatalf("bypass total = (%d, %v), want (4, true)", total, present)
	}
	if _, err := client.ContractRow.UpdateOneID(row.ID).ClearDeleteTime().ClearDeletedBy().Save(bypass); err != nil {
		t.Fatalf("undelete contract row: %v", err)
	}
	if exists, err := client.ContractRow.Query().Where(entrow.IDEQ(row.ID)).Exist(context.Background()); err != nil || !exists {
		t.Fatalf("default query after undelete = (%v, %v), want (true, nil)", exists, err)
	}
}

func executeInvalidColumnList(t *testing.T, client *entfixture.Client) error {
	t.Helper()
	bad, err := crud.NewListFields[*entfixture.ContractRow](
		crud.Columns(entrow.ValidColumn),
		crud.Bind(examplev1.UserFields.CreateTime, entrow.FieldTimestampValue).Order(),
		crud.DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderAscending),
		crud.Custom(examplev1.UserFields.Email, "broken_email", func(_ corecrud.FilterOperator, value corecrud.FilterValue) (crud.SelectorPredicate, error) {
			raw, ok := value.StringValue()
			if !ok {
				return nil, fmt.Errorf("email literal is not a string")
			}
			return func(selector *entsql.Selector) *entsql.Predicate {
				return entsql.EQ(selector.C("missing_live_contract_column"), raw)
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
