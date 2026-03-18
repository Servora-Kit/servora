package service

import (
	apppb "github.com/Servora-Kit/servora/api/gen/go/application/service/v1"
	orgpb "github.com/Servora-Kit/servora/api/gen/go/organization/service/v1"
	rbacpb "github.com/Servora-Kit/servora/api/gen/go/rbac/service/v1"
	tenantpb "github.com/Servora-Kit/servora/api/gen/go/tenant/service/v1"
	userpb "github.com/Servora-Kit/servora/api/gen/go/user/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/pkg/mapper"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var userInfoMapper = mapper.NewForwardMapper(func(u *entity.User) *userpb.UserInfo {
	return &userpb.UserInfo{
		Id:              u.ID,
		Name:            u.Name,
		Email:           u.Email,
		Role:            u.Role,
		EmailVerified:   u.EmailVerified,
		OrganizationIds: u.OrganizationIDs,
	}
})

var orgInfoMapper = mapper.NewForwardMapper(func(o *entity.Organization) *orgpb.OrganizationInfo {
	return orgToProto(o)
})

func orgToProto(o *entity.Organization) *orgpb.OrganizationInfo {
	info := &orgpb.OrganizationInfo{
		Id:          o.ID,
		Name:        o.Name,
		Slug:        o.Slug,
		DisplayName: o.DisplayName,
		Type:        o.Type,
		Sort:        int32(o.Sort),
		CreatedAt:   timestamppb.New(o.CreatedAt),
		UpdatedAt:   timestamppb.New(o.UpdatedAt),
	}
	if o.ParentID != nil {
		info.ParentId = o.ParentID
	}
	if o.LeaderUserID != nil {
		info.LeaderUserId = o.LeaderUserID
	}
	for _, child := range o.Children {
		info.Children = append(info.Children, orgToProto(child))
	}
	return info
}

var orgMemberInfoMapper = mapper.NewForwardMapper(func(m *entity.OrganizationMember) *orgpb.OrganizationMemberInfo {
	return &orgpb.OrganizationMemberInfo{
		Id:             m.ID,
		OrganizationId: m.OrganizationID,
		UserId:         m.UserID,
		UserName:       m.UserName,
		UserEmail:      m.UserEmail,
		Role:           m.Role,
		CreatedAt:      timestamppb.New(m.CreatedAt),
	}
})

var tenantInfoMapper = mapper.NewForwardMapper(func(t *entity.Tenant) *tenantpb.TenantInfo {
	return &tenantpb.TenantInfo{
		Id:          t.ID,
		Slug:        t.Slug,
		Name:        t.Name,
		DisplayName: t.DisplayName,
		Kind:        t.Kind,
		Domain:      t.Domain,
		Status:      t.Status,
		OwnerUserId: t.OwnerUserID,
		CreatedAt:   timestamppb.New(t.CreatedAt),
		UpdatedAt:   timestamppb.New(t.UpdatedAt),
	}
})

var applicationInfoMapper = mapper.NewForwardMapper(func(a *entity.Application) *apppb.ApplicationInfo {
	return &apppb.ApplicationInfo{
		Id:              a.ID,
		ClientId:        a.ClientID,
		Name:            a.Name,
		RedirectUris:    a.RedirectURIs,
		Scopes:          a.Scopes,
		GrantTypes:      a.GrantTypes,
		ApplicationType: a.ApplicationType,
		AccessTokenType: a.AccessTokenType,
		TenantId:        a.TenantID,
		IdTokenLifetime: int32(a.IDTokenLifetime.Seconds()),
		CreatedAt:       timestamppb.New(a.CreatedAt),
		UpdatedAt:       timestamppb.New(a.UpdatedAt),
	}
})

func mapRoleToProto(r *entity.RbacRole) *rbacpb.RoleInfo {
	info := &rbacpb.RoleInfo{
		Id:            r.ID,
		Code:          r.Code,
		Name:          r.Name,
		Type:          r.Type,
		IsProtected:   r.IsProtected,
		Status:        r.Status,
		PermissionIds: r.PermissionIDs,
		CreatedAt:     timestamppb.New(r.CreatedAt),
		UpdatedAt:     timestamppb.New(r.UpdatedAt),
	}
	if r.Description != "" {
		info.Description = &r.Description
	}
	if r.TenantID != nil {
		info.TenantId = r.TenantID
	}
	return info
}

func mapRolesToProto(roles []*entity.RbacRole) []*rbacpb.RoleInfo {
	out := make([]*rbacpb.RoleInfo, len(roles))
	for i, r := range roles {
		out[i] = mapRoleToProto(r)
	}
	return out
}

func mapPermissionToProto(p *entity.RbacPermission) *rbacpb.PermissionInfo {
	info := &rbacpb.PermissionInfo{
		Id:        p.ID,
		Code:      p.Code,
		Name:      p.Name,
		Status:    p.Status,
		CreatedAt: timestamppb.New(p.CreatedAt),
	}
	if p.Description != "" {
		info.Description = &p.Description
	}
	if p.GroupID != nil {
		info.GroupId = p.GroupID
	}
	if p.GroupName != "" {
		info.GroupName = &p.GroupName
	}
	for _, api := range p.APIs {
		info.Apis = append(info.Apis, &rbacpb.PermissionApiInfo{
			ApiMethod: api.Method,
			ApiPath:   api.Path,
		})
	}
	return info
}

func mapPermissionsToProto(perms []*entity.RbacPermission) []*rbacpb.PermissionInfo {
	out := make([]*rbacpb.PermissionInfo, len(perms))
	for i, p := range perms {
		out[i] = mapPermissionToProto(p)
	}
	return out
}

func mapPermissionGroupToProto(g *entity.RbacPermissionGroup) *rbacpb.PermissionGroupInfo {
	info := &rbacpb.PermissionGroupInfo{
		Id:   g.ID,
		Name: g.Name,
		Sort: int32(g.Sort),
	}
	if g.Module != "" {
		info.Module = &g.Module
	}
	if g.ParentID != nil {
		info.ParentId = g.ParentID
	}
	for _, child := range g.Children {
		info.Children = append(info.Children, mapPermissionGroupToProto(child))
	}
	return info
}

func mapPermissionGroupsToProto(groups []*entity.RbacPermissionGroup) []*rbacpb.PermissionGroupInfo {
	out := make([]*rbacpb.PermissionGroupInfo, len(groups))
	for i, g := range groups {
		out[i] = mapPermissionGroupToProto(g)
	}
	return out
}

func mapMenuToProto(m *entity.RbacMenu) *rbacpb.MenuInfo {
	info := &rbacpb.MenuInfo{
		Id:     m.ID,
		Type:   m.Type,
		Name:   m.Name,
		Sort:   int32(m.Sort),
		Status: m.Status,
	}
	if m.Path != "" {
		info.Path = &m.Path
	}
	if m.Component != "" {
		info.Component = &m.Component
	}
	if m.Redirect != "" {
		info.Redirect = &m.Redirect
	}
	if m.Meta != "" {
		info.Meta = &m.Meta
	}
	if m.ParentID != nil {
		info.ParentId = m.ParentID
	}
	for _, child := range m.Children {
		info.Children = append(info.Children, mapMenuToProto(child))
	}
	return info
}

func mapMenusToProto(menus []*entity.RbacMenu) []*rbacpb.MenuInfo {
	out := make([]*rbacpb.MenuInfo, len(menus))
	for i, m := range menus {
		out[i] = mapMenuToProto(m)
	}
	return out
}
