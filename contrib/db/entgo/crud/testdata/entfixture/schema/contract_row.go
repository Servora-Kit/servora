package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	entgomixin "github.com/Servora-Kit/servora/contrib/db/entgo/mixin"
)

//go:generate go run entgo.io/ent/cmd/ent generate --feature=intercept,sql/modifier .

// ContractRow is the private generated-Ent fixture for backend adapter contracts.
type ContractRow struct {
	ent.Schema
}

func (ContractRow) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("id"),
		field.String("text_value").Default(""),
		field.String("unique_text"),
		field.Int32("numeric_value"),
		field.String("nullable_text").Optional().Nillable(),
		field.Time("timestamp_value").Default(time.Now).Immutable(),
		field.Time("updated_timestamp").Default(time.Now).UpdateDefault(time.Now),
		field.Int64("duration_value").GoType(time.Duration(0)).Default(0),
		field.Int32("enum_number").Default(0).GoType(crudpb.CrudErrorReason(0)),
	}
}

func (ContractRow) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("unique_text").Unique(),
	}
}

func (ContractRow) Mixin() []ent.Mixin {
	return []ent.Mixin{
		entgomixin.SoftDeleteMixin{},
	}
}
