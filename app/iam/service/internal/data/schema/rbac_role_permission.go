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

// RbacRolePermission defines the schema for the rbac_role_permissions table.
// Maps roles to permissions with an effect (ALLOW/DENY) and priority for conflict resolution.
// Initial seed data only uses ALLOW; DENY is reserved for future fine-grained control.
type RbacRolePermission struct {
	ent.Schema
}

func (RbacRolePermission) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		field.UUID("role_id", uuid.UUID{}),
		field.UUID("permission_id", uuid.UUID{}),
		// effect: ALLOW grants the permission, DENY explicitly revokes it
		field.Enum("effect").Values("ALLOW", "DENY").Default("ALLOW"),
		// priority: higher value wins when ALLOW and DENY conflict for the same role+permission
		field.Int("priority").Default(0),
		// tenant_id scopes this mapping to a specific tenant (null = platform-wide)
		field.UUID("tenant_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RbacRolePermission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("role", RbacRole.Type).
			Ref("role_permissions").
			Field("role_id").
			Unique().
			Required(),
		edge.From("permission", RbacPermission.Type).
			Ref("role_permissions").
			Field("permission_id").
			Unique().
			Required(),
	}
}

func (RbacRolePermission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("role_id", "permission_id", "tenant_id").Unique(),
	}
}

func (RbacRolePermission) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "rbac_role_permissions"},
	}
}
