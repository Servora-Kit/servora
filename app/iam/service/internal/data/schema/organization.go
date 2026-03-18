package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	entmixin "github.com/Servora-Kit/servora/pkg/ent/mixin"
	"github.com/google/uuid"
)

type Organization struct {
	ent.Schema
}

func (Organization) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		field.UUID("tenant_id", uuid.UUID{}),
		field.String("name").MaxLen(128),
		field.String("slug").MaxLen(128),
		field.String("display_name").MaxLen(255).Optional().Nillable(),
		// Tree structure
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),
		// Organization type: COMPANY > DEPARTMENT > TEAM
		field.Enum("type").Values("COMPANY", "DEPARTMENT", "TEAM").Default("COMPANY"),
		field.Int("sort").Default(0),
		// Optional leader (a user within the org)
		field.UUID("leader_user_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Organization) Mixin() []ent.Mixin {
	return []ent.Mixin{
		entmixin.SoftDeleteMixin{},
	}
}

func (Organization) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Tenant.Type).
			Ref("organizations").
			Field("tenant_id").
			Unique().
			Required(),
		edge.To("members", OrganizationMember.Type),
		edge.To("positions", Position.Type),
		// Self-referential tree: one parent, many children
		edge.To("children", Organization.Type).
			From("parent").
			Field("parent_id").
			Unique(),
	}
}

func (Organization) Indexes() []ent.Index {
	return []ent.Index{
		// Slug is unique per tenant among non-deleted organizations.
		index.Fields("tenant_id", "slug").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")).
			Unique(),
	}
}

func (Organization) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "organizations"},
	}
}
