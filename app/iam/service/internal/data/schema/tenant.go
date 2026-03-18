package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	entmixin "github.com/Servora-Kit/servora/pkg/ent/mixin"
	"github.com/google/uuid"
)

type Tenant struct {
	ent.Schema
}

func (Tenant) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		field.UUID("owner_user_id", uuid.UUID{}),
		field.String("slug").MaxLen(64).Unique(),
		field.String("name").MaxLen(128),
		field.String("display_name").MaxLen(255).Optional().Nillable(),
		field.Enum("kind").Values("business", "personal").Default("business"),
		field.String("domain").MaxLen(128).Optional().Nillable().Unique(),
		field.Enum("status").Values("active", "disabled").Default("active"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Tenant) Mixin() []ent.Mixin {
	return []ent.Mixin{
		entmixin.SoftDeleteMixin{},
	}
}

func (Tenant) Edges() []ent.Edge {
	return []ent.Edge{
		// owner_user_id 外键指向 User
		edge.From("owner", User.Type).
			Ref("owned_tenants").
			Field("owner_user_id").
			Unique().
			Required(),
		edge.To("organizations", Organization.Type),
		edge.To("applications", Application.Type),
		edge.To("positions", Position.Type),
	}
}

func (Tenant) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "tenants"},
	}
}
