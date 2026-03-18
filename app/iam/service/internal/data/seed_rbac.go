package data

import (
	"context"
	"encoding/json"
	"fmt"

	ent "github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacmenu"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacpermission"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacpermissionapi"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacpermissiongroup"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacpermissionmenu"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacrole"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacrolepermission"
	"github.com/google/uuid"
)

// menuMeta is the JSON shape stored in rbac_menus.meta.
type menuMeta struct {
	Icon  string `json:"icon,omitempty"`
	Order int    `json:"order,omitempty"`
}

func mustMeta(m menuMeta) *string {
	b, _ := json.Marshal(m)
	s := string(b)
	return &s
}

// SeedRBAC runs all RBAC seed steps in dependency order. Every step is idempotent.
func (s *Seeder) SeedRBAC(ctx context.Context) error {
	if err := s.seedRoles(ctx); err != nil {
		return err
	}
	permGroupIDs, err := s.seedPermissionGroups(ctx)
	if err != nil {
		return err
	}
	permIDs, err := s.seedPermissions(ctx, permGroupIDs)
	if err != nil {
		return err
	}
	menuIDs, err := s.seedMenus(ctx)
	if err != nil {
		return err
	}
	if err := s.seedRolePermissions(ctx, permIDs); err != nil {
		return err
	}
	if err := s.seedPermissionMenus(ctx, permIDs, menuIDs); err != nil {
		return err
	}
	if err := s.seedPermissionApis(ctx, permIDs); err != nil {
		return err
	}
	s.log.Info("RBAC seed complete")
	return nil
}

// ─── Roles ───────────────────────────────────────────────────────────────────

func (s *Seeder) seedRoles(ctx context.Context) error {
	ec := s.ec
	for _, r := range builtinRoles {
		exists, err := ec.RbacRole.Query().Where(rbacrole.CodeEQ(r.code)).Exist(ctx)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		desc := r.description
		if _, err := ec.RbacRole.Create().
			SetCode(r.code).
			SetName(r.name).
			SetNillableDescription(&desc).
			SetType(rbacrole.TypeBUILTIN).
			SetIsProtected(true).
			SetStatus(rbacrole.StatusACTIVE).
			Save(ctx); err != nil {
			return err
		}
		s.log.Infof("seeded role: %s", r.code)
	}
	return nil
}

// ─── Permission Groups ────────────────────────────────────────────────────────

func (s *Seeder) seedPermissionGroups(ctx context.Context) (map[string]uuid.UUID, error) {
	ec := s.ec
	result := make(map[string]uuid.UUID)
	for i, g := range permGroupSpecs {
		mod := g.module
		existing, err := ec.RbacPermissionGroup.Query().
			Where(rbacpermissiongroup.NameEQ(g.name), rbacpermissiongroup.ModuleEQ(mod)).
			Only(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return nil, err
		}
		if err == nil {
			result[g.key] = existing.ID
			continue
		}
		created, err := ec.RbacPermissionGroup.Create().
			SetName(g.name).
			SetNillableModule(&mod).
			SetSort(i).
			Save(ctx)
		if err != nil {
			return nil, err
		}
		result[g.key] = created.ID
		s.log.Infof("seeded permission group: %s", g.name)
	}
	return result, nil
}

// ─── Permissions ──────────────────────────────────────────────────────────────

func (s *Seeder) seedPermissions(ctx context.Context, groupIDs map[string]uuid.UUID) (map[string]uuid.UUID, error) {
	ec := s.ec
	result := make(map[string]uuid.UUID)
	for _, p := range permSpecs {
		existing, err := ec.RbacPermission.Query().Where(rbacpermission.CodeEQ(p.code)).Only(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return nil, err
		}
		if err == nil {
			result[p.code] = existing.ID
			continue
		}
		desc := p.description
		create := ec.RbacPermission.Create().
			SetCode(p.code).
			SetName(p.name).
			SetNillableDescription(&desc).
			SetStatus(rbacpermission.StatusACTIVE)
		if gid, ok := groupIDs[p.groupKey]; ok {
			create = create.SetGroupID(gid)
		}
		created, err := create.Save(ctx)
		if err != nil {
			return nil, err
		}
		result[p.code] = created.ID
		s.log.Infof("seeded permission: %s", p.code)
	}
	return result, nil
}

// ─── Menus ────────────────────────────────────────────────────────────────────

func (s *Seeder) seedMenus(ctx context.Context) (map[string]uuid.UUID, error) {
	ec := s.ec
	result := make(map[string]uuid.UUID)

	// Pass A: Deduplication — keep oldest, remove duplicates.
	for _, m := range menuSpecs {
		var existing []*ent.RbacMenu
		var qErr error
		if m.path == "" {
			existing, qErr = ec.RbacMenu.Query().
				Where(rbacmenu.NameEQ(m.name), rbacmenu.TypeEQ(m.menuType), rbacmenu.PathIsNil()).
				Order(ent.Asc(rbacmenu.FieldCreatedAt)).All(ctx)
		} else {
			existing, qErr = ec.RbacMenu.Query().
				Where(rbacmenu.PathEQ(m.path)).
				Order(ent.Asc(rbacmenu.FieldCreatedAt)).All(ctx)
		}
		if qErr != nil {
			return nil, fmt.Errorf("dedup query for %s: %w", m.key, qErr)
		}
		if len(existing) == 0 {
			continue
		}
		result[m.key] = existing[0].ID
		for _, dup := range existing[1:] {
			if _, err := ec.RbacPermissionMenu.Delete().
				Where(rbacpermissionmenu.MenuIDEQ(dup.ID)).Exec(ctx); err != nil {
				return nil, fmt.Errorf("remove perm-menu links for dup %s: %w", m.key, err)
			}
			if err := ec.RbacMenu.DeleteOneID(dup.ID).Exec(ctx); err != nil {
				return nil, fmt.Errorf("delete dup menu %s: %w", m.key, err)
			}
			s.log.Infof("removed duplicate menu: %s", m.name)
		}
	}

	// Pass B: Create missing menus (up to 5 passes to handle tree depth).
	remaining := make([]menuSpec, 0, len(menuSpecs))
	for _, m := range menuSpecs {
		if _, ok := result[m.key]; !ok {
			remaining = append(remaining, m)
		}
	}
	for pass := 0; pass < 5 && len(remaining) > 0; pass++ {
		var nextRound []menuSpec
		for _, m := range remaining {
			if m.parentKey != "" {
				if _, ok := result[m.parentKey]; !ok {
					nextRound = append(nextRound, m)
					continue
				}
			}
			meta := mustMeta(menuMeta{Icon: m.icon, Order: m.sort})
			create := ec.RbacMenu.Create().
				SetType(m.menuType).
				SetName(m.name).
				SetSort(m.sort).
				SetStatus(rbacmenu.StatusACTIVE).
				SetNillableMeta(meta)
			if m.path != "" {
				create = create.SetPath(m.path)
			}
			if m.component != "" {
				create = create.SetComponent(m.component)
			}
			if m.parentKey != "" {
				create = create.SetParentID(result[m.parentKey])
			}
			created, err := create.Save(ctx)
			if err != nil {
				return nil, fmt.Errorf("create menu %s: %w", m.key, err)
			}
			result[m.key] = created.ID
			s.log.Infof("seeded menu: %s", m.name)
		}
		remaining = nextRound
	}

	// Pass C: Fix parent relationships for pre-existing records.
	for _, m := range menuSpecs {
		if m.parentKey == "" {
			continue
		}
		parentID, okP := result[m.parentKey]
		childID, okC := result[m.key]
		if !okP || !okC {
			continue
		}
		rec, err := ec.RbacMenu.Get(ctx, childID)
		if err != nil {
			continue
		}
		if rec.ParentID != nil && *rec.ParentID == parentID {
			continue
		}
		if err := ec.RbacMenu.UpdateOneID(childID).SetParentID(parentID).Exec(ctx); err != nil {
			return nil, fmt.Errorf("fix menu parent for %s: %w", m.key, err)
		}
		s.log.Infof("fixed menu parent: %s → %s", m.key, m.parentKey)
	}

	// Pass D: Sync component, name, and meta (icon) for pre-existing records.
	for _, m := range menuSpecs {
		id, ok := result[m.key]
		if !ok {
			continue
		}
		rec, err := ec.RbacMenu.Get(ctx, id)
		if err != nil {
			continue
		}
		upd := ec.RbacMenu.UpdateOneID(id)
		changed := false
		if m.component != "" && (rec.Component == nil || *rec.Component != m.component) {
			upd = upd.SetComponent(m.component)
			changed = true
		}
		if rec.Name != m.name {
			upd = upd.SetName(m.name)
			changed = true
		}
		newMeta := mustMeta(menuMeta{Icon: m.icon, Order: m.sort})
		currentMeta := ""
		if rec.Meta != nil {
			currentMeta = *rec.Meta
		}
		if newMeta != nil && currentMeta != *newMeta {
			upd = upd.SetNillableMeta(newMeta)
			changed = true
		}
		if changed {
			if err := upd.Exec(ctx); err != nil {
				return nil, fmt.Errorf("update menu spec %s: %w", m.key, err)
			}
			s.log.Infof("updated menu spec: %s", m.key)
		}
	}

	return result, nil
}

// ─── Role → Permission mappings ───────────────────────────────────────────────

func (s *Seeder) seedRolePermissions(ctx context.Context, permIDs map[string]uuid.UUID) error {
	ec := s.ec
	for roleCode, codes := range rolePermissions {
		role, err := ec.RbacRole.Query().Where(rbacrole.CodeEQ(roleCode)).Only(ctx)
		if err != nil {
			s.log.Warnf("seedRolePermissions: role %q not found: %v", roleCode, err)
			continue
		}
		for _, permCode := range codes {
			permID, ok := permIDs[permCode]
			if !ok {
				s.log.Warnf("seedRolePermissions: permission %q not found", permCode)
				continue
			}
			exists, err := ec.RbacRolePermission.Query().
				Where(rbacrolepermission.RoleIDEQ(role.ID), rbacrolepermission.PermissionIDEQ(permID)).
				Exist(ctx)
			if err != nil {
				return err
			}
			if exists {
				continue
			}
			if _, err := ec.RbacRolePermission.Create().
				SetRoleID(role.ID).SetPermissionID(permID).Save(ctx); err != nil {
				return err
			}
		}
		s.log.Infof("seeded role-permission mappings for: %s", roleCode)
	}
	return nil
}

// ─── Permission → Menu mappings ───────────────────────────────────────────────

func (s *Seeder) seedPermissionMenus(ctx context.Context, permIDs map[string]uuid.UUID, menuIDs map[string]uuid.UUID) error {
	ec := s.ec
	for permCode, menuKeys := range permissionMenus {
		permID, ok := permIDs[permCode]
		if !ok {
			continue
		}
		for _, menuKey := range menuKeys {
			menuID, ok := menuIDs[menuKey]
			if !ok {
				s.log.Warnf("seedPermissionMenus: menu %q not found for permission %q", menuKey, permCode)
				continue
			}
			exists, err := ec.RbacPermissionMenu.Query().
				Where(rbacpermissionmenu.PermissionIDEQ(permID), rbacpermissionmenu.MenuIDEQ(menuID)).
				Exist(ctx)
			if err != nil {
				return err
			}
			if exists {
				continue
			}
			if _, err := ec.RbacPermissionMenu.Create().
				SetPermissionID(permID).SetMenuID(menuID).Save(ctx); err != nil {
				return err
			}
		}
	}
	s.log.Info("seeded permission-menu mappings")
	return nil
}

// ─── Permission → API mappings ────────────────────────────────────────────────

func (s *Seeder) seedPermissionApis(ctx context.Context, permIDs map[string]uuid.UUID) error {
	ec := s.ec
	for _, a := range permissionApis {
		permID, ok := permIDs[a.permCode]
		if !ok {
			continue
		}
		exists, err := ec.RbacPermissionApi.Query().
			Where(
				rbacpermissionapi.PermissionIDEQ(permID),
				rbacpermissionapi.APIMethodEQ(a.method),
				rbacpermissionapi.APIPathEQ(a.path),
			).Exist(ctx)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := ec.RbacPermissionApi.Create().
			SetPermissionID(permID).
			SetAPIMethod(a.method).
			SetAPIPath(a.path).
			Save(ctx); err != nil {
			return err
		}
	}
	s.log.Info("seeded permission-api mappings")
	return nil
}
