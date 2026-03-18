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

// RbacMenu defines the schema for the rbac_menus table.
// Menus form a tree and are linked to permissions via rbac_permission_menus.
// type: CATALOG = folder/group, MENU = navigable page, BUTTON = action button within a page.
type RbacMenu struct {
	ent.Schema
}

func (RbacMenu) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(newUUIDv7),
		field.Enum("type").Values("CATALOG", "MENU", "BUTTON").Default("MENU"),
		field.String("name").MaxLen(128),
		// path is the frontend route path, e.g. "/dashboard"
		field.String("path").MaxLen(256).Optional().Nillable(),
		// component is the frontend component identifier, e.g. "_app/dashboard"
		field.String("component").MaxLen(256).Optional().Nillable(),
		field.String("redirect").MaxLen(256).Optional().Nillable(),
		// meta is a JSON blob: icon, title, order, hideInMenu, etc.
		field.Text("meta").Optional().Nillable(),
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),
		field.Int("sort").Default(0),
		field.Enum("status").Values("ACTIVE", "DISABLED").Default("ACTIVE"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RbacMenu) Edges() []ent.Edge {
	return []ent.Edge{
		// self-referential tree
		edge.To("children", RbacMenu.Type).
			From("parent").
			Field("parent_id").
			Unique(),
		edge.From("permission_menus", RbacPermissionMenu.Type).
			Ref("menu"),
	}
}

func (RbacMenu) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "rbac_menus"},
	}
}
