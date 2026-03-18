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

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		field.String("name").MaxLen(64).Unique(),
		field.String("email").MaxLen(128).Unique(),
		field.String("password").MaxLen(255),
		field.String("role").MaxLen(32).Default("user"),
		field.Bool("email_verified").Default(false),
		field.Time("email_verified_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		entmixin.SoftDeleteMixin{},
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		// 用户可以是多个组织的成员
		edge.To("org_memberships", OrganizationMember.Type),
		// 用户可以 own 多个租户（通常只有一个 personal tenant）
		edge.To("owned_tenants", Tenant.Type),
	}
}

func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "users"},
	}
}
