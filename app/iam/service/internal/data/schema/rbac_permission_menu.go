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

// RbacPermissionMenu defines the schema for the rbac_permission_menus table.
// Links a permission to one or more menu items, enabling GetNavigation to
// return only the menus a user is permitted to see.
type RbacPermissionMenu struct {
	ent.Schema
}

func (RbacPermissionMenu) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		field.UUID("permission_id", uuid.UUID{}),
		field.UUID("menu_id", uuid.UUID{}),
	}
}

func (RbacPermissionMenu) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("permission", RbacPermission.Type).
			Ref("permission_menus").
			Field("permission_id").
			Unique().
			Required(),
		edge.To("menu", RbacMenu.Type).
			Field("menu_id").
			Unique().
			Required(),
	}
}

func (RbacPermissionMenu) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("permission_id", "menu_id").Unique(),
	}
}

func (RbacPermissionMenu) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "rbac_permission_menus"},
	}
}
