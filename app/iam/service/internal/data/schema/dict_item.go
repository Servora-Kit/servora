package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// DictItem defines a single entry within a DictType (e.g. label="Male", value="male").
type DictItem struct {
	ent.Schema
}

func (DictItem) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		field.UUID("dict_type_id", uuid.UUID{}),
		// Display label shown to users (e.g. "男", "企业")
		field.String("label").MaxLen(128),
		// Actual stored value (e.g. "male", "business")
		field.String("value").MaxLen(128),
		// Optional frontend tag color hint (e.g. "blue", "green", "orange")
		field.String("color_tag").MaxLen(64).Optional().Nillable(),
		field.Int("sort").Default(0),
		field.Enum("status").Values("ACTIVE", "DISABLED").Default("ACTIVE"),
		field.Bool("is_default").Default(false),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (DictItem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("dict_type", DictType.Type).
			Ref("items").
			Field("dict_type_id").
			Unique().
			Required().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (DictItem) Indexes() []ent.Index {
	return []ent.Index{
		// Value must be unique within a dict type
		index.Fields("dict_type_id", "value").Unique(),
	}
}

func (DictItem) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "dict_items"},
	}
}
