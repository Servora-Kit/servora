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

// RbacPermissionGroup defines the schema for the rbac_permission_groups table.
// Groups form a tree structure to organize permissions by module/feature area.
type RbacPermissionGroup struct {
	ent.Schema
}

func (RbacPermissionGroup) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		field.String("name").MaxLen(128),
		field.String("module").MaxLen(64).Optional().Nillable(),
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),
		field.Int("sort").Default(0),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RbacPermissionGroup) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("permissions", RbacPermission.Type),
		// self-referential tree
		edge.To("children", RbacPermissionGroup.Type).
			From("parent").
			Field("parent_id").
			Unique(),
	}
}

func (RbacPermissionGroup) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "rbac_permission_groups"},
	}
}
