package biz

import (
	"context"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type RbacRoleRepo interface {
	ListRoles(ctx context.Context, tenantID *string, page, pageSize int) ([]*entity.RbacRole, int, error)
	GetRole(ctx context.Context, id string) (*entity.RbacRole, error)
	CreateRole(ctx context.Context, role *entity.RbacRole) (*entity.RbacRole, error)
	UpdateRole(ctx context.Context, id string, name *string, description *string, status *string, permissionIDs []string) (*entity.RbacRole, error)
	DeleteRole(ctx context.Context, id string) error
	// GetCustomRoleCodesByUser returns role codes from explicit user-role assignments
	// for the given user within a specific tenant.
	GetCustomRoleCodesByUser(ctx context.Context, userID, tenantID string) ([]string, error)
}

type RbacPermissionRepo interface {
	ListPermissions(ctx context.Context, groupID *string, page, pageSize int) ([]*entity.RbacPermission, int, error)
	GetPermission(ctx context.Context, id string) (*entity.RbacPermission, error)
	CreatePermission(ctx context.Context, perm *entity.RbacPermission) (*entity.RbacPermission, error)
	UpdatePermission(ctx context.Context, id string, name *string, description *string, status *string) (*entity.RbacPermission, error)
	DeletePermission(ctx context.Context, id string) error
	ListPermissionGroups(ctx context.Context) ([]*entity.RbacPermissionGroup, error)
	GetPermissionCodesByRoleCodes(ctx context.Context, roleCodes []string) ([]string, error)
}

type RbacMenuRepo interface {
	ListMenus(ctx context.Context) ([]*entity.RbacMenu, error)
	GetMenu(ctx context.Context, id string) (*entity.RbacMenu, error)
	CreateMenu(ctx context.Context, menu *entity.RbacMenu) (*entity.RbacMenu, error)
	UpdateMenu(ctx context.Context, id string, updates *entity.RbacMenu) (*entity.RbacMenu, error)
	DeleteMenu(ctx context.Context, id string) error
	GetMenusByPermissionCodes(ctx context.Context, codes []string) ([]*entity.RbacMenu, error)
}

type RbacUsecase struct {
	roleRepo RbacRoleRepo
	permRepo RbacPermissionRepo
	menuRepo RbacMenuRepo
	log      *logger.Helper
}

func NewRbacUsecase(roleRepo RbacRoleRepo, permRepo RbacPermissionRepo, menuRepo RbacMenuRepo, l logger.Logger) *RbacUsecase {
	return &RbacUsecase{
		roleRepo: roleRepo,
		permRepo: permRepo,
		menuRepo: menuRepo,
		log:      logger.NewHelper(l, logger.WithModule("rbac/biz/iam-service")),
	}
}

func (uc *RbacUsecase) ListRoles(ctx context.Context, tenantID *string, page, pageSize int) ([]*entity.RbacRole, int, error) {
	return uc.roleRepo.ListRoles(ctx, tenantID, page, pageSize)
}

func (uc *RbacUsecase) GetRole(ctx context.Context, id string) (*entity.RbacRole, error) {
	return uc.roleRepo.GetRole(ctx, id)
}

func (uc *RbacUsecase) CreateRole(ctx context.Context, role *entity.RbacRole) (*entity.RbacRole, error) {
	return uc.roleRepo.CreateRole(ctx, role)
}

func (uc *RbacUsecase) UpdateRole(ctx context.Context, id string, name *string, description *string, status *string, permissionIDs []string) (*entity.RbacRole, error) {
	return uc.roleRepo.UpdateRole(ctx, id, name, description, status, permissionIDs)
}

func (uc *RbacUsecase) DeleteRole(ctx context.Context, id string) error {
	return uc.roleRepo.DeleteRole(ctx, id)
}

func (uc *RbacUsecase) ListPermissions(ctx context.Context, groupID *string, page, pageSize int) ([]*entity.RbacPermission, int, error) {
	return uc.permRepo.ListPermissions(ctx, groupID, page, pageSize)
}

func (uc *RbacUsecase) GetPermission(ctx context.Context, id string) (*entity.RbacPermission, error) {
	return uc.permRepo.GetPermission(ctx, id)
}

func (uc *RbacUsecase) CreatePermission(ctx context.Context, perm *entity.RbacPermission) (*entity.RbacPermission, error) {
	return uc.permRepo.CreatePermission(ctx, perm)
}

func (uc *RbacUsecase) UpdatePermission(ctx context.Context, id string, name *string, description *string, status *string) (*entity.RbacPermission, error) {
	return uc.permRepo.UpdatePermission(ctx, id, name, description, status)
}

func (uc *RbacUsecase) DeletePermission(ctx context.Context, id string) error {
	return uc.permRepo.DeletePermission(ctx, id)
}

func (uc *RbacUsecase) ListPermissionGroups(ctx context.Context) ([]*entity.RbacPermissionGroup, error) {
	return uc.permRepo.ListPermissionGroups(ctx)
}

func (uc *RbacUsecase) ListMenus(ctx context.Context) ([]*entity.RbacMenu, error) {
	return uc.menuRepo.ListMenus(ctx)
}

func (uc *RbacUsecase) GetMenu(ctx context.Context, id string) (*entity.RbacMenu, error) {
	return uc.menuRepo.GetMenu(ctx, id)
}

func (uc *RbacUsecase) CreateMenu(ctx context.Context, menu *entity.RbacMenu) (*entity.RbacMenu, error) {
	return uc.menuRepo.CreateMenu(ctx, menu)
}

func (uc *RbacUsecase) UpdateMenu(ctx context.Context, id string, updates *entity.RbacMenu) (*entity.RbacMenu, error) {
	return uc.menuRepo.UpdateMenu(ctx, id, updates)
}

func (uc *RbacUsecase) DeleteMenu(ctx context.Context, id string) error {
	return uc.menuRepo.DeleteMenu(ctx, id)
}

// GetMyPermissionCodes derives effective role codes from structural data and custom user roles,
// then returns all deduplicated permission codes for those roles.
// tenantID is required to look up custom role assignments; empty string skips custom role lookup.
func (uc *RbacUsecase) GetMyPermissionCodes(ctx context.Context, userRole, tenantOwnerUserID, currentUserID, tenantID, orgMemberRole string) ([]string, error) {
	var roleCodes []string

	switch {
	case userRole == "admin":
		roleCodes = append(roleCodes, "platform_admin")
	case currentUserID != "" && currentUserID == tenantOwnerUserID:
		roleCodes = append(roleCodes, "tenant_owner")
	}

	switch orgMemberRole {
	case "admin":
		roleCodes = append(roleCodes, "org_admin")
	case "member":
		roleCodes = append(roleCodes, "org_member")
	}

	// Merge custom role assignments from RbacUserRole table
	if currentUserID != "" && tenantID != "" {
		customCodes, err := uc.roleRepo.GetCustomRoleCodesByUser(ctx, currentUserID, tenantID)
		if err != nil {
			uc.log.Warnf("get custom role codes failed: %v", err)
		} else {
			roleCodes = append(roleCodes, customCodes...)
		}
	}

	permCodes, err := uc.permRepo.GetPermissionCodesByRoleCodes(ctx, roleCodes)
	if err != nil {
		uc.log.Errorf("get permission codes failed: %v", err)
		return nil, err
	}

	seen := make(map[string]struct{}, len(permCodes))
	result := make([]string, 0, len(permCodes))
	for _, c := range permCodes {
		if _, ok := seen[c]; !ok {
			seen[c] = struct{}{}
			result = append(result, c)
		}
	}
	return result, nil
}

// GetNavigation returns a menu tree filtered by the given permission codes.
func (uc *RbacUsecase) GetNavigation(ctx context.Context, permCodes []string) ([]*entity.RbacMenu, error) {
	menus, err := uc.menuRepo.GetMenusByPermissionCodes(ctx, permCodes)
	if err != nil {
		uc.log.Errorf("get menus by permission codes failed: %v", err)
		return nil, err
	}
	return buildMenuTree(menus), nil
}

// buildMenuTree converts a flat menu list into a tree by grouping children under their parent.
func buildMenuTree(menus []*entity.RbacMenu) []*entity.RbacMenu {
	byID := make(map[string]*entity.RbacMenu, len(menus))
	for _, m := range menus {
		byID[m.ID] = m
	}
	var roots []*entity.RbacMenu
	for _, m := range menus {
		if m.ParentID == nil || *m.ParentID == "" {
			roots = append(roots, m)
		} else if parent, ok := byID[*m.ParentID]; ok {
			parent.Children = append(parent.Children, m)
		} else {
			roots = append(roots, m)
		}
	}
	return roots
}
