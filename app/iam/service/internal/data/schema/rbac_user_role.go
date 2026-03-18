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

// RbacUserRole defines the schema for the rbac_user_roles table.
// Used for CUSTOM role assignments only. Built-in roles (platform_admin, tenant_owner,
// org_admin, org_member) are derived dynamically from structural data:
//   - user.role == 'admin'          → platform_admin
//   - user.id == tenant.owner_user_id → tenant_owner
//   - org_member.role == 'admin'    → org_admin
//   - org_member.role == 'member'   → org_member
type RbacUserRole struct {
	ent.Schema
}

func (RbacUserRole) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("role_id", uuid.UUID{}),
		field.Enum("status").Values("ACTIVE", "DISABLED").Default("ACTIVE"),
		// tenant_id scopes the role assignment; must match the role's tenant_id
		field.UUID("tenant_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RbacUserRole) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("role", RbacRole.Type).
			Ref("user_roles").
			Field("role_id").
			Unique().
			Required(),
	}
}

func (RbacUserRole) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "role_id", "tenant_id").Unique(),
	}
}

func (RbacUserRole) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "rbac_user_roles"},
	}
}
