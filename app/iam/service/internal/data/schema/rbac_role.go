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

// RbacRole defines the schema for the rbac_roles table.
// Built-in roles (platform_admin, tenant_owner, org_admin, org_member) are seeded and protected.
// Tenants may create custom roles (tenant_id not null).
type RbacRole struct {
	ent.Schema
}

func (RbacRole) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		// code is unique within a tenant scope (null tenant_id = platform-level role)
		field.String("code").MaxLen(128),
		field.String("name").MaxLen(128),
		field.String("description").MaxLen(512).Optional().Nillable(),
		field.Enum("type").Values("BUILTIN", "CUSTOM").Default("CUSTOM"),
		// is_protected prevents built-in roles from being deleted or modified
		field.Bool("is_protected").Default(false),
		field.Enum("status").Values("ACTIVE", "DISABLED").Default("ACTIVE"),
		// null tenant_id means platform-level role visible to all tenants
		field.UUID("tenant_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RbacRole) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("role_permissions", RbacRolePermission.Type),
		edge.To("user_roles", RbacUserRole.Type),
	}
}

func (RbacRole) Indexes() []ent.Index {
	return []ent.Index{
		// code must be unique per tenant (null tenant_id = global scope)
		index.Fields("code", "tenant_id").Unique(),
	}
}

func (RbacRole) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "rbac_roles"},
	}
}
