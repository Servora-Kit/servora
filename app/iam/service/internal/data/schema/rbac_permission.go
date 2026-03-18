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

// RbacPermission defines the schema for the rbac_permissions table.
// Permission codes follow the format "resource:action" (e.g. "user:create", "org:manage").
type RbacPermission struct {
	ent.Schema
}

func (RbacPermission) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		// code is globally unique, e.g. "user:create", "org:manage"
		field.String("code").MaxLen(128).Unique(),
		field.String("name").MaxLen(128),
		field.String("description").MaxLen(512).Optional().Nillable(),
		field.UUID("group_id", uuid.UUID{}).Optional().Nillable(),
		field.Enum("status").Values("ACTIVE", "DISABLED").Default("ACTIVE"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RbacPermission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("group", RbacPermissionGroup.Type).
			Ref("permissions").
			Field("group_id").
			Unique(),
		edge.To("role_permissions", RbacRolePermission.Type),
		edge.To("permission_menus", RbacPermissionMenu.Type),
		edge.To("permission_apis", RbacPermissionApi.Type),
	}
}

func (RbacPermission) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "rbac_permissions"},
	}
}
