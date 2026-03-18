package service

import (
	"context"

	iampb "github.com/Servora-Kit/servora/api/gen/go/iam/service/v1"
	rbacpb "github.com/Servora-Kit/servora/api/gen/go/rbac/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/pkg/pagination"
)

type RbacService struct {
	iampb.UnimplementedRbacServiceServer

	rbacUC   *biz.RbacUsecase
	userUC   *biz.UserUsecase
	tenantUC *biz.TenantUsecase
	orgUC    *biz.OrganizationUsecase
}

func NewRbacService(
	rbacUC *biz.RbacUsecase,
	userUC *biz.UserUsecase,
	tenantUC *biz.TenantUsecase,
	orgUC *biz.OrganizationUsecase,
) *RbacService {
	return &RbacService{
		rbacUC:   rbacUC,
		userUC:   userUC,
		tenantUC: tenantUC,
		orgUC:    orgUC,
	}
}

// resolvePermissionCodes is a helper shared by GetMyPermissionCodes and GetNavigation
// to avoid duplicating the permission code derivation logic.
func (s *RbacService) resolvePermissionCodes(ctx context.Context) ([]string, error) {
	callerID, err := requireAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}

	user, err := s.userUC.CurrentUserInfo(ctx, callerID)
	if err != nil {
		return nil, err
	}

	tenantID := ""
	tenantOwnerUserID := ""
	if tid, ok := requireTenantScopeOptional(ctx); ok && tid != "" {
		tenantID = tid
		if tenant, terr := s.tenantUC.Get(ctx, tid); terr == nil {
			tenantOwnerUserID = tenant.OwnerUserID
		}
	}

	orgMemberRole := ""
	if memberships, merr := s.orgUC.GetUserMemberships(ctx, callerID); merr == nil {
		for _, m := range memberships {
			if m.Role == "admin" {
				orgMemberRole = "admin"
				break
			} else if m.Role == "member" {
				orgMemberRole = "member"
			}
		}
	}

	return s.rbacUC.GetMyPermissionCodes(ctx, user.Role, tenantOwnerUserID, callerID, tenantID, orgMemberRole)
}

// requirePlatformAdmin returns an error if the caller is not a platform admin (user.role = "admin").
func (s *RbacService) requirePlatformAdmin(ctx context.Context) error {
	return checkPlatformAdmin(ctx, s.userUC)
}

func (s *RbacService) GetMyPermissionCodes(ctx context.Context, _ *rbacpb.GetMyPermissionCodesRequest) (*rbacpb.GetMyPermissionCodesResponse, error) {
	codes, err := s.resolvePermissionCodes(ctx)
	if err != nil {
		return nil, err
	}
	return &rbacpb.GetMyPermissionCodesResponse{Codes: codes}, nil
}

func (s *RbacService) GetNavigation(ctx context.Context, _ *rbacpb.GetNavigationRequest) (*rbacpb.GetNavigationResponse, error) {
	codes, err := s.resolvePermissionCodes(ctx)
	if err != nil {
		return nil, err
	}

	menus, err := s.rbacUC.GetNavigation(ctx, codes)
	if err != nil {
		return nil, err
	}

	return &rbacpb.GetNavigationResponse{Menus: mapMenusToProto(menus)}, nil
}

// ── Role CRUD ─────────────────────────────────────────────────────────────────

func (s *RbacService) ListRoles(ctx context.Context, req *rbacpb.ListRolesRequest) (*rbacpb.ListRolesResponse, error) {
	_, tenantID, err := requireTenantScope(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize := pagination.ExtractPage(req.GetPagination())
	roles, total, err := s.rbacUC.ListRoles(ctx, &tenantID, int(page), int(pageSize))
	if err != nil {
		return nil, err
	}
	return &rbacpb.ListRolesResponse{
		Roles:      mapRolesToProto(roles),
		Pagination: pagination.BuildPageResponse(int64(total), page, pageSize),
	}, nil
}

func (s *RbacService) GetRole(ctx context.Context, req *rbacpb.GetRoleRequest) (*rbacpb.GetRoleResponse, error) {
	role, err := s.rbacUC.GetRole(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &rbacpb.GetRoleResponse{Role: mapRoleToProto(role)}, nil
}

func (s *RbacService) CreateRole(ctx context.Context, req *rbacpb.CreateRoleRequest) (*rbacpb.CreateRoleResponse, error) {
	_, tenantID, err := requireTenantScope(ctx)
	if err != nil {
		return nil, err
	}
	role, err := s.rbacUC.CreateRole(ctx, &entity.RbacRole{
		Code:          req.Code,
		Name:          req.Name,
		Description:   req.GetDescription(),
		Type:          "CUSTOM",
		TenantID:      &tenantID,
		PermissionIDs: req.PermissionIds,
	})
	if err != nil {
		return nil, err
	}
	return &rbacpb.CreateRoleResponse{Role: mapRoleToProto(role)}, nil
}

func (s *RbacService) UpdateRole(ctx context.Context, req *rbacpb.UpdateRoleRequest) (*rbacpb.UpdateRoleResponse, error) {
	if err := s.requireRoleOwnership(ctx, req.Id); err != nil {
		return nil, err
	}
	role, err := s.rbacUC.UpdateRole(ctx, req.Id, req.Name, req.Description, req.Status, req.PermissionIds)
	if err != nil {
		return nil, err
	}
	return &rbacpb.UpdateRoleResponse{Role: mapRoleToProto(role)}, nil
}

func (s *RbacService) DeleteRole(ctx context.Context, req *rbacpb.DeleteRoleRequest) (*rbacpb.DeleteRoleResponse, error) {
	if err := s.requireRoleOwnership(ctx, req.Id); err != nil {
		return nil, err
	}
	if err := s.rbacUC.DeleteRole(ctx, req.Id); err != nil {
		return nil, err
	}
	return &rbacpb.DeleteRoleResponse{}, nil
}

// requireRoleOwnership verifies that the role belongs to the current tenant scope.
// Built-in roles (tenant_id == nil) can only be modified by platform admins.
func (s *RbacService) requireRoleOwnership(ctx context.Context, roleID string) error {
	role, err := s.rbacUC.GetRole(ctx, roleID)
	if err != nil {
		return err
	}
	if role.TenantID == nil {
		// Built-in role — only platform admin may modify
		return s.requirePlatformAdmin(ctx)
	}
	_, tenantID, err := requireTenantScope(ctx)
	if err != nil {
		return err
	}
	if *role.TenantID != tenantID {
		return errForbidden("role does not belong to the current tenant")
	}
	return nil
}

// ── Permission CRUD ───────────────────────────────────────────────────────────

func (s *RbacService) ListPermissions(ctx context.Context, req *rbacpb.ListPermissionsRequest) (*rbacpb.ListPermissionsResponse, error) {
	page, pageSize := pagination.ExtractPage(req.GetPagination())
	perms, total, err := s.rbacUC.ListPermissions(ctx, req.GroupId, int(page), int(pageSize))
	if err != nil {
		return nil, err
	}
	return &rbacpb.ListPermissionsResponse{
		Permissions: mapPermissionsToProto(perms),
		Pagination:  pagination.BuildPageResponse(int64(total), page, pageSize),
	}, nil
}

func (s *RbacService) ListPermissionGroups(ctx context.Context, _ *rbacpb.ListPermissionGroupsRequest) (*rbacpb.ListPermissionGroupsResponse, error) {
	groups, err := s.rbacUC.ListPermissionGroups(ctx)
	if err != nil {
		return nil, err
	}
	return &rbacpb.ListPermissionGroupsResponse{Groups: mapPermissionGroupsToProto(groups)}, nil
}

func (s *RbacService) GetPermission(ctx context.Context, req *rbacpb.GetPermissionRequest) (*rbacpb.GetPermissionResponse, error) {
	perm, err := s.rbacUC.GetPermission(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &rbacpb.GetPermissionResponse{Permission: mapPermissionToProto(perm)}, nil
}

func (s *RbacService) CreatePermission(ctx context.Context, req *rbacpb.CreatePermissionRequest) (*rbacpb.CreatePermissionResponse, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	perm, err := s.rbacUC.CreatePermission(ctx, &entity.RbacPermission{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.GetDescription(),
		GroupID:     req.GroupId,
	})
	if err != nil {
		return nil, err
	}
	return &rbacpb.CreatePermissionResponse{Permission: mapPermissionToProto(perm)}, nil
}

func (s *RbacService) UpdatePermission(ctx context.Context, req *rbacpb.UpdatePermissionRequest) (*rbacpb.UpdatePermissionResponse, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	perm, err := s.rbacUC.UpdatePermission(ctx, req.Id, req.Name, req.Description, req.Status)
	if err != nil {
		return nil, err
	}
	return &rbacpb.UpdatePermissionResponse{Permission: mapPermissionToProto(perm)}, nil
}

func (s *RbacService) DeletePermission(ctx context.Context, req *rbacpb.DeletePermissionRequest) (*rbacpb.DeletePermissionResponse, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if err := s.rbacUC.DeletePermission(ctx, req.Id); err != nil {
		return nil, err
	}
	return &rbacpb.DeletePermissionResponse{}, nil
}

// ── Menu CRUD ─────────────────────────────────────────────────────────────────

func (s *RbacService) ListMenus(ctx context.Context, _ *rbacpb.ListMenusRequest) (*rbacpb.ListMenusResponse, error) {
	menus, err := s.rbacUC.ListMenus(ctx)
	if err != nil {
		return nil, err
	}
	return &rbacpb.ListMenusResponse{Menus: mapMenusToProto(menus)}, nil
}

func (s *RbacService) GetMenu(ctx context.Context, req *rbacpb.GetMenuRequest) (*rbacpb.GetMenuResponse, error) {
	menu, err := s.rbacUC.GetMenu(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &rbacpb.GetMenuResponse{Menu: mapMenuToProto(menu)}, nil
}

func (s *RbacService) CreateMenu(ctx context.Context, req *rbacpb.CreateMenuRequest) (*rbacpb.CreateMenuResponse, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	menu, err := s.rbacUC.CreateMenu(ctx, &entity.RbacMenu{
		Type:      req.Type,
		Name:      req.Name,
		Path:      req.GetPath(),
		Component: req.GetComponent(),
		Redirect:  req.GetRedirect(),
		Meta:      req.GetMeta(),
		ParentID:  req.ParentId,
		Sort:      int(req.Sort),
	})
	if err != nil {
		return nil, err
	}
	return &rbacpb.CreateMenuResponse{Menu: mapMenuToProto(menu)}, nil
}

func (s *RbacService) UpdateMenu(ctx context.Context, req *rbacpb.UpdateMenuRequest) (*rbacpb.UpdateMenuResponse, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	updates := &entity.RbacMenu{
		Name:      req.GetName(),
		Path:      req.GetPath(),
		Component: req.GetComponent(),
		Redirect:  req.GetRedirect(),
		Meta:      req.GetMeta(),
		ParentID:  req.ParentId,
		Status:    req.GetStatus(),
	}
	if req.Sort != nil {
		updates.Sort = int(*req.Sort)
	}
	menu, err := s.rbacUC.UpdateMenu(ctx, req.Id, updates)
	if err != nil {
		return nil, err
	}
	return &rbacpb.UpdateMenuResponse{Menu: mapMenuToProto(menu)}, nil
}

func (s *RbacService) DeleteMenu(ctx context.Context, req *rbacpb.DeleteMenuRequest) (*rbacpb.DeleteMenuResponse, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if err := s.rbacUC.DeleteMenu(ctx, req.Id); err != nil {
		return nil, err
	}
	return &rbacpb.DeleteMenuResponse{}, nil
}
