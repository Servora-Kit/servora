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

// Position defines a job position within a tenant's organizational structure.
type Position struct {
	ent.Schema
}

func (Position) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		field.UUID("tenant_id", uuid.UUID{}),
		// Optional linkage to a specific organization node
		field.UUID("organization_id", uuid.UUID{}).Optional().Nillable(),
		field.String("code").MaxLen(128),
		field.String("name").MaxLen(128),
		field.String("description").MaxLen(512).Optional().Nillable(),
		field.Int("sort").Default(0),
		field.Enum("status").Values("ACTIVE", "DISABLED").Default("ACTIVE"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Position) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Tenant.Type).
			Ref("positions").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("organization", Organization.Type).
			Ref("positions").
			Field("organization_id").
			Unique(),
	}
}

func (Position) Indexes() []ent.Index {
	return []ent.Index{
		// Position code must be unique within a tenant
		index.Fields("tenant_id", "code").Unique(),
	}
}

func (Position) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "positions"},
	}
}
