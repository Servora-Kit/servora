package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// RbacPermissionApi defines the schema for the rbac_permission_apis table.
// This is a METADATA table only — it does NOT participate in API authorization.
// API authorization continues to be enforced by OpenFGA middleware (authz.service.v1.rule annotations).
// This table is used for: management UI display ("which APIs does this permission grant?"),
// permission auditing, and documentation.
type RbacPermissionApi struct {
	ent.Schema
}

func (RbacPermissionApi) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		field.UUID("permission_id", uuid.UUID{}),
		// HTTP method: GET, POST, PUT, DELETE, PATCH
		field.String("api_method").MaxLen(16),
		// API path pattern, e.g. "/v1/users" or "/v1/organizations/{id}"
		field.String("api_path").MaxLen(256),
	}
}

func (RbacPermissionApi) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("permission", RbacPermission.Type).
			Ref("permission_apis").
			Field("permission_id").
			Unique().
			Required(),
	}
}

func (RbacPermissionApi) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("permission_id", "api_method", "api_path").Unique(),
	}
}

func (RbacPermissionApi) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "rbac_permission_apis"},
	}
}
