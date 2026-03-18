package data

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacmenu"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacpermission"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacpermissionmenu"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacrole"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacrolepermission"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacuserrole"
	"github.com/Servora-Kit/servora/pkg/logger"
)

// ──────────────────────────────────────────────
// Role Repo
// ──────────────────────────────────────────────

type rbacRoleRepo struct {
	data *Data
	log  *logger.Helper
}

func NewRbacRoleRepo(data *Data, l logger.Logger) biz.RbacRoleRepo {
	return &rbacRoleRepo{
		data: data,
		log:  logger.NewHelper(l, logger.WithModule("rbac-role/data/iam-service")),
	}
}

func (r *rbacRoleRepo) ListRoles(ctx context.Context, tenantID *string, page, pageSize int) ([]*entity.RbacRole, int, error) {
	q := r.data.Ent(ctx).RbacRole.Query().
		WithRolePermissions(func(rpq *ent.RbacRolePermissionQuery) {
			rpq.WithPermission()
		})
	if tenantID != nil {
		if tid, err := uuid.Parse(*tenantID); err == nil {
			// Include both tenant-specific roles and global built-in roles (tenant_id IS NULL)
			q = q.Where(rbacrole.Or(rbacrole.TenantIDEQ(tid), rbacrole.TenantIDIsNil()))
		}
	} else {
		q = q.Where(rbacrole.TenantIDIsNil())
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count roles: %w", err)
	}

	offset := (page - 1) * pageSize
	roles, err := q.Offset(offset).Limit(pageSize).
		Order(rbacrole.ByCreatedAt()).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list roles: %w", err)
	}
	return mapRoles(roles), total, nil
}

func (r *rbacRoleRepo) GetRole(ctx context.Context, id string) (*entity.RbacRole, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid role ID: %w", err)
	}
	role, err := r.data.Ent(ctx).RbacRole.Query().
		Where(rbacrole.IDEQ(uid)).
		WithRolePermissions(func(rpq *ent.RbacRolePermissionQuery) {
			rpq.WithPermission()
		}).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return mapRole(role), nil
}

func (r *rbacRoleRepo) CreateRole(ctx context.Context, role *entity.RbacRole) (*entity.RbacRole, error) {
	b := r.data.Ent(ctx).RbacRole.Create().
		SetCode(role.Code).
		SetName(role.Name)
	if role.Description != "" {
		b.SetDescription(role.Description)
	}
	if role.Type != "" {
		b.SetType(rbacrole.Type(role.Type))
	}
	b.SetIsProtected(role.IsProtected)
	if role.Status != "" {
		b.SetStatus(rbacrole.Status(role.Status))
	}
	if role.TenantID != nil {
		if tid, err := uuid.Parse(*role.TenantID); err == nil {
			b.SetTenantID(tid)
		}
	}

	created, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}

	if len(role.PermissionIDs) > 0 {
		if err := r.replaceRolePermissions(ctx, created.ID, role.PermissionIDs, role.TenantID); err != nil {
			return nil, err
		}
	}

	return r.GetRole(ctx, created.ID.String())
}

func (r *rbacRoleRepo) UpdateRole(ctx context.Context, id string, name *string, description *string, status *string, permissionIDs []string) (*entity.RbacRole, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid role ID: %w", err)
	}

	b := r.data.Ent(ctx).RbacRole.UpdateOneID(uid)
	if name != nil {
		b.SetName(*name)
	}
	if description != nil {
		b.SetDescription(*description)
	}
	if status != nil {
		b.SetStatus(rbacrole.Status(*status))
	}
	if _, err := b.Save(ctx); err != nil {
		return nil, fmt.Errorf("update role: %w", err)
	}

	if permissionIDs != nil {
		role, err := r.GetRole(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := r.replaceRolePermissions(ctx, uid, permissionIDs, role.TenantID); err != nil {
			return nil, err
		}
	}

	return r.GetRole(ctx, id)
}

// GetCustomRoleCodesByUser returns role codes from explicit RbacUserRole assignments
// for the given user within a tenant. Returns only ACTIVE assignments.
func (r *rbacRoleRepo) GetCustomRoleCodesByUser(ctx context.Context, userID, tenantID string) ([]string, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}

	assignments, err := r.data.Ent(ctx).RbacUserRole.Query().
		Where(
			rbacuserrole.UserIDEQ(uid),
			rbacuserrole.TenantIDEQ(tid),
			rbacuserrole.StatusEQ(rbacuserrole.StatusACTIVE),
		).
		WithRole().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("get user custom roles: %w", err)
	}

	codes := make([]string, 0, len(assignments))
	for _, a := range assignments {
		if a.Edges.Role != nil {
			codes = append(codes, a.Edges.Role.Code)
		}
	}
	return codes, nil
}

func (r *rbacRoleRepo) DeleteRole(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid role ID: %w", err)
	}
	return r.data.RunInEntTx(ctx, func(txCtx context.Context) error {
		client := r.data.Ent(txCtx)
		if _, err := client.RbacRolePermission.Delete().
			Where(rbacrolepermission.RoleIDEQ(uid)).Exec(txCtx); err != nil {
			return fmt.Errorf("delete role permissions: %w", err)
		}
		if _, err := client.RbacUserRole.Delete().
			Where(rbacuserrole.RoleIDEQ(uid)).Exec(txCtx); err != nil {
			return fmt.Errorf("delete user roles: %w", err)
		}
		return client.RbacRole.DeleteOneID(uid).Exec(txCtx)
	})
}

func (r *rbacRoleRepo) replaceRolePermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []string, tenantID *string) error {
	return r.data.RunInEntTx(ctx, func(txCtx context.Context) error {
		client := r.data.Ent(txCtx)
		if _, err := client.RbacRolePermission.Delete().
			Where(rbacrolepermission.RoleIDEQ(roleID)).Exec(txCtx); err != nil {
			return fmt.Errorf("clear role permissions: %w", err)
		}
		for _, permIDStr := range permissionIDs {
			permID, err := uuid.Parse(permIDStr)
			if err != nil {
				continue
			}
			b := client.RbacRolePermission.Create().
				SetRoleID(roleID).
				SetPermissionID(permID)
			if tenantID != nil {
				if tid, err := uuid.Parse(*tenantID); err == nil {
					b.SetTenantID(tid)
				}
			}
			if _, err := b.Save(txCtx); err != nil {
				return fmt.Errorf("add role permission: %w", err)
			}
		}
		return nil
	})
}

// ──────────────────────────────────────────────
// Permission Repo
// ──────────────────────────────────────────────

type rbacPermissionRepo struct {
	data *Data
	log  *logger.Helper
}

func NewRbacPermissionRepo(data *Data, l logger.Logger) biz.RbacPermissionRepo {
	return &rbacPermissionRepo{
		data: data,
		log:  logger.NewHelper(l, logger.WithModule("rbac-permission/data/iam-service")),
	}
}

func (r *rbacPermissionRepo) ListPermissions(ctx context.Context, groupID *string, page, pageSize int) ([]*entity.RbacPermission, int, error) {
	q := r.data.Ent(ctx).RbacPermission.Query().
		WithGroup().
		WithPermissionApis()
	if groupID != nil {
		if gid, err := uuid.Parse(*groupID); err == nil {
			q = q.Where(rbacpermission.GroupIDEQ(gid))
		}
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count permissions: %w", err)
	}

	offset := (page - 1) * pageSize
	perms, err := q.Offset(offset).Limit(pageSize).
		Order(rbacpermission.ByCreatedAt()).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list permissions: %w", err)
	}
	return mapPermissions(perms), total, nil
}

func (r *rbacPermissionRepo) GetPermission(ctx context.Context, id string) (*entity.RbacPermission, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid permission ID: %w", err)
	}
	perm, err := r.data.Ent(ctx).RbacPermission.Query().
		Where(rbacpermission.IDEQ(uid)).
		WithGroup().
		WithPermissionApis().
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return mapPermission(perm), nil
}

func (r *rbacPermissionRepo) CreatePermission(ctx context.Context, p *entity.RbacPermission) (*entity.RbacPermission, error) {
	b := r.data.Ent(ctx).RbacPermission.Create().
		SetCode(p.Code).
		SetName(p.Name)
	if p.Description != "" {
		b.SetDescription(p.Description)
	}
	if p.GroupID != nil {
		if gid, err := uuid.Parse(*p.GroupID); err == nil {
			b.SetGroupID(gid)
		}
	}
	if p.Status != "" {
		b.SetStatus(rbacpermission.Status(p.Status))
	}
	created, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create permission: %w", err)
	}
	return r.GetPermission(ctx, created.ID.String())
}

func (r *rbacPermissionRepo) UpdatePermission(ctx context.Context, id string, name *string, description *string, status *string) (*entity.RbacPermission, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid permission ID: %w", err)
	}
	b := r.data.Ent(ctx).RbacPermission.UpdateOneID(uid)
	if name != nil {
		b.SetName(*name)
	}
	if description != nil {
		b.SetDescription(*description)
	}
	if status != nil {
		b.SetStatus(rbacpermission.Status(*status))
	}
	if _, err := b.Save(ctx); err != nil {
		return nil, fmt.Errorf("update permission: %w", err)
	}
	return r.GetPermission(ctx, id)
}

func (r *rbacPermissionRepo) DeletePermission(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid permission ID: %w", err)
	}
	return r.data.Ent(ctx).RbacPermission.DeleteOneID(uid).Exec(ctx)
}

func (r *rbacPermissionRepo) ListPermissionGroups(ctx context.Context) ([]*entity.RbacPermissionGroup, error) {
	groups, err := r.data.Ent(ctx).RbacPermissionGroup.Query().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list permission groups: %w", err)
	}
	return mapPermissionGroups(groups), nil
}

func (r *rbacPermissionRepo) GetPermissionCodesByRoleCodes(ctx context.Context, roleCodes []string) ([]string, error) {
	if len(roleCodes) == 0 {
		return nil, nil
	}

	roles, err := r.data.Ent(ctx).RbacRole.Query().
		Where(rbacrole.CodeIn(roleCodes...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("get roles by codes: %w", err)
	}
	if len(roles) == 0 {
		return nil, nil
	}

	roleIDs := make([]uuid.UUID, len(roles))
	for i, role := range roles {
		roleIDs[i] = role.ID
	}

	rolePerms, err := r.data.Ent(ctx).RbacRolePermission.Query().
		Where(rbacrolepermission.RoleIDIn(roleIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("get role permissions: %w", err)
	}
	if len(rolePerms) == 0 {
		return nil, nil
	}

	permIDs := make([]uuid.UUID, len(rolePerms))
	for i, rp := range rolePerms {
		permIDs[i] = rp.PermissionID
	}

	perms, err := r.data.Ent(ctx).RbacPermission.Query().
		Where(rbacpermission.IDIn(permIDs...)).
		Select(rbacpermission.FieldCode).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get permission codes: %w", err)
	}
	return perms, nil
}

// ──────────────────────────────────────────────
// Menu Repo
// ──────────────────────────────────────────────

type rbacMenuRepo struct {
	data *Data
	log  *logger.Helper
}

func NewRbacMenuRepo(data *Data, l logger.Logger) biz.RbacMenuRepo {
	return &rbacMenuRepo{
		data: data,
		log:  logger.NewHelper(l, logger.WithModule("rbac-menu/data/iam-service")),
	}
}

func (r *rbacMenuRepo) ListMenus(ctx context.Context) ([]*entity.RbacMenu, error) {
	menus, err := r.data.Ent(ctx).RbacMenu.Query().
		Where(rbacmenu.StatusEQ(rbacmenu.StatusACTIVE)).
		Order(rbacmenu.BySort()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list menus: %w", err)
	}
	return mapMenus(menus), nil
}

func (r *rbacMenuRepo) GetMenu(ctx context.Context, id string) (*entity.RbacMenu, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid menu ID: %w", err)
	}
	m, err := r.data.Ent(ctx).RbacMenu.Query().
		Where(rbacmenu.IDEQ(uid)).Only(ctx)
	if err != nil {
		return nil, err
	}
	return mapMenu(m), nil
}

func (r *rbacMenuRepo) CreateMenu(ctx context.Context, menu *entity.RbacMenu) (*entity.RbacMenu, error) {
	b := r.data.Ent(ctx).RbacMenu.Create().
		SetName(menu.Name)
	if menu.Type != "" {
		b.SetType(rbacmenu.Type(menu.Type))
	}
	if menu.Path != "" {
		b.SetPath(menu.Path)
	}
	if menu.Component != "" {
		b.SetComponent(menu.Component)
	}
	if menu.Redirect != "" {
		b.SetRedirect(menu.Redirect)
	}
	if menu.Meta != "" {
		b.SetMeta(menu.Meta)
	}
	if menu.ParentID != nil {
		if pid, err := uuid.Parse(*menu.ParentID); err == nil {
			b.SetParentID(pid)
		}
	}
	b.SetSort(menu.Sort)
	if menu.Status != "" {
		b.SetStatus(rbacmenu.Status(menu.Status))
	}
	created, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create menu: %w", err)
	}
	return mapMenu(created), nil
}

func (r *rbacMenuRepo) UpdateMenu(ctx context.Context, id string, updates *entity.RbacMenu) (*entity.RbacMenu, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid menu ID: %w", err)
	}
	b := r.data.Ent(ctx).RbacMenu.UpdateOneID(uid)
	if updates.Name != "" {
		b.SetName(updates.Name)
	}
	if updates.Type != "" {
		b.SetType(rbacmenu.Type(updates.Type))
	}
	if updates.Path != "" {
		b.SetPath(updates.Path)
	}
	if updates.Component != "" {
		b.SetComponent(updates.Component)
	}
	if updates.Redirect != "" {
		b.SetRedirect(updates.Redirect)
	}
	if updates.Meta != "" {
		b.SetMeta(updates.Meta)
	}
	if updates.ParentID != nil {
		if pid, err := uuid.Parse(*updates.ParentID); err == nil {
			b.SetParentID(pid)
		}
	}
	if updates.Status != "" {
		b.SetStatus(rbacmenu.Status(updates.Status))
	}
	b.SetSort(updates.Sort)
	saved, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update menu: %w", err)
	}
	return mapMenu(saved), nil
}

func (r *rbacMenuRepo) DeleteMenu(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid menu ID: %w", err)
	}
	return r.data.RunInEntTx(ctx, func(txCtx context.Context) error {
		client := r.data.Ent(txCtx)
		if _, err := client.RbacPermissionMenu.Delete().
			Where(rbacpermissionmenu.MenuIDEQ(uid)).Exec(txCtx); err != nil {
			return fmt.Errorf("delete permission menus: %w", err)
		}
		return client.RbacMenu.DeleteOneID(uid).Exec(txCtx)
	})
}

func (r *rbacMenuRepo) GetMenusByPermissionCodes(ctx context.Context, codes []string) ([]*entity.RbacMenu, error) {
	if len(codes) == 0 {
		return nil, nil
	}

	perms, err := r.data.Ent(ctx).RbacPermission.Query().
		Where(rbacpermission.CodeIn(codes...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("get permissions by codes: %w", err)
	}
	if len(perms) == 0 {
		return nil, nil
	}

	permIDs := make([]uuid.UUID, len(perms))
	for i, p := range perms {
		permIDs[i] = p.ID
	}

	permMenus, err := r.data.Ent(ctx).RbacPermissionMenu.Query().
		Where(rbacpermissionmenu.PermissionIDIn(permIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("get permission menus: %w", err)
	}
	if len(permMenus) == 0 {
		return nil, nil
	}

	menuIDs := make([]uuid.UUID, 0, len(permMenus))
	seen := make(map[uuid.UUID]struct{})
	for _, pm := range permMenus {
		if _, ok := seen[pm.MenuID]; !ok {
			seen[pm.MenuID] = struct{}{}
			menuIDs = append(menuIDs, pm.MenuID)
		}
	}

	menus, err := r.data.Ent(ctx).RbacMenu.Query().
		Where(rbacmenu.IDIn(menuIDs...)).
		Where(rbacmenu.StatusEQ(rbacmenu.StatusACTIVE)).
		Order(rbacmenu.BySort()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("get menus by ids: %w", err)
	}
	return mapMenus(menus), nil
}

// ──────────────────────────────────────────────
// Mappers
// ──────────────────────────────────────────────

func mapRole(r *ent.RbacRole) *entity.RbacRole {
	e := &entity.RbacRole{
		ID:          r.ID.String(),
		Code:        r.Code,
		Name:        r.Name,
		Type:        string(r.Type),
		IsProtected: r.IsProtected,
		Status:      string(r.Status),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
	if r.Description != nil {
		e.Description = *r.Description
	}
	if r.TenantID != nil {
		s := r.TenantID.String()
		e.TenantID = &s
	}
	for _, rp := range r.Edges.RolePermissions {
		if rp.Edges.Permission != nil {
			e.PermissionIDs = append(e.PermissionIDs, rp.PermissionID.String())
		}
	}
	return e
}

func mapRoles(roles []*ent.RbacRole) []*entity.RbacRole {
	result := make([]*entity.RbacRole, len(roles))
	for i, r := range roles {
		result[i] = mapRole(r)
	}
	return result
}

func mapPermission(p *ent.RbacPermission) *entity.RbacPermission {
	e := &entity.RbacPermission{
		ID:        p.ID.String(),
		Code:      p.Code,
		Name:      p.Name,
		Status:    string(p.Status),
		CreatedAt: p.CreatedAt,
	}
	if p.Description != nil {
		e.Description = *p.Description
	}
	if p.GroupID != nil {
		s := p.GroupID.String()
		e.GroupID = &s
	}
	if p.Edges.Group != nil {
		e.GroupName = p.Edges.Group.Name
	}
	for _, api := range p.Edges.PermissionApis {
		e.APIs = append(e.APIs, entity.RbacPermissionAPI{
			Method: api.APIMethod,
			Path:   api.APIPath,
		})
	}
	return e
}

func mapPermissions(perms []*ent.RbacPermission) []*entity.RbacPermission {
	result := make([]*entity.RbacPermission, len(perms))
	for i, p := range perms {
		result[i] = mapPermission(p)
	}
	return result
}

func mapPermissionGroups(groups []*ent.RbacPermissionGroup) []*entity.RbacPermissionGroup {
	result := make([]*entity.RbacPermissionGroup, len(groups))
	for i, g := range groups {
		eg := &entity.RbacPermissionGroup{
			ID:   g.ID.String(),
			Name: g.Name,
			Sort: g.Sort,
		}
		if g.Module != nil {
			eg.Module = *g.Module
		}
		if g.ParentID != nil {
			s := g.ParentID.String()
			eg.ParentID = &s
		}
		result[i] = eg
	}
	return result
}

func mapMenu(m *ent.RbacMenu) *entity.RbacMenu {
	e := &entity.RbacMenu{
		ID:     m.ID.String(),
		Type:   string(m.Type),
		Name:   m.Name,
		Sort:   m.Sort,
		Status: string(m.Status),
	}
	if m.Path != nil {
		e.Path = *m.Path
	}
	if m.Component != nil {
		e.Component = *m.Component
	}
	if m.Redirect != nil {
		e.Redirect = *m.Redirect
	}
	if m.Meta != nil {
		e.Meta = *m.Meta
	}
	if m.ParentID != nil {
		s := m.ParentID.String()
		e.ParentID = &s
	}
	return e
}

func mapMenus(menus []*ent.RbacMenu) []*entity.RbacMenu {
	result := make([]*entity.RbacMenu, len(menus))
	for i, m := range menus {
		result[i] = mapMenu(m)
	}
	return result
}
