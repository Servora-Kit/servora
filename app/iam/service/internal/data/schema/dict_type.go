package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// DictType defines a system-level dictionary category (e.g. "GENDER", "TENANT_TYPE").
// Dictionary types are global (no tenant isolation) since they define system-wide enumerations.
type DictType struct {
	ent.Schema
}

func (DictType) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		// code is globally unique, e.g. "GENDER", "AUDIT_STATUS"
		field.String("code").MaxLen(128).Unique(),
		field.String("name").MaxLen(128),
		field.String("description").MaxLen(512).Optional().Nillable(),
		field.Enum("status").Values("ACTIVE", "DISABLED").Default("ACTIVE"),
		field.Int("sort").Default(0),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (DictType) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("items", DictItem.Type),
	}
}

func (DictType) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "dict_types"},
	}
}
