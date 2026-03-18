package service

import (
	"context"

	orgpb "github.com/Servora-Kit/servora/api/gen/go/organization/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/pkg/pagination"
)

type OrganizationService struct {
	orgpb.UnimplementedOrganizationServiceServer

	uc *biz.OrganizationUsecase
}

func NewOrganizationService(uc *biz.OrganizationUsecase) *OrganizationService {
	return &OrganizationService{uc: uc}
}

func (s *OrganizationService) CreateOrganization(ctx context.Context, req *orgpb.CreateOrganizationRequest) (*orgpb.CreateOrganizationResponse, error) {
	callerID, tenantID, err := requireTenantScope(ctx)
	if err != nil {
		return nil, err
	}
	orgType := req.Type
	if orgType == "" {
		orgType = "COMPANY"
	}
	org, err := s.uc.Create(ctx, callerID, &entity.Organization{
		Name:        req.Name,
		Slug:        req.Slug,
		DisplayName: req.DisplayName,
		TenantID:    tenantID,
		ParentID:    req.ParentId,
		Type:        orgType,
		Sort:        int(req.Sort),
	})
	if err != nil {
		return nil, err
	}
	return &orgpb.CreateOrganizationResponse{Organization: orgToProto(org)}, nil
}

func (s *OrganizationService) GetOrganization(ctx context.Context, req *orgpb.GetOrganizationRequest) (*orgpb.GetOrganizationResponse, error) {
	org, err := s.uc.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &orgpb.GetOrganizationResponse{Organization: orgInfoMapper.Map(org)}, nil
}

func (s *OrganizationService) ListOrganizations(ctx context.Context, req *orgpb.ListOrganizationsRequest) (*orgpb.ListOrganizationsResponse, error) {
	callerID, err := requireAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	tenantID, _ := requireTenantScopeOptional(ctx)
	page, pageSize := pagination.ExtractPage(req.Pagination)
	orgs, total, err := s.uc.List(ctx, callerID, tenantID, page, pageSize)
	if err != nil {
		return nil, err
	}

	items := orgInfoMapper.MapSlice(orgs)
	return &orgpb.ListOrganizationsResponse{
		Organizations: items,
		Pagination:    pagination.BuildPageResponse(total, page, pageSize),
	}, nil
}

func (s *OrganizationService) UpdateOrganization(ctx context.Context, req *orgpb.UpdateOrganizationRequest) (*orgpb.UpdateOrganizationResponse, error) {
	org, err := s.uc.Update(ctx, &entity.Organization{
		ID:           req.Id,
		Name:         req.Name,
		DisplayName:  req.DisplayName,
		ParentID:     req.ParentId,
		Type:         req.Type,
		Sort:         int(req.Sort),
		LeaderUserID: req.LeaderUserId,
	})
	if err != nil {
		return nil, err
	}
	return &orgpb.UpdateOrganizationResponse{Organization: orgToProto(org)}, nil
}

func (s *OrganizationService) ListOrganizationTree(ctx context.Context, _ *orgpb.ListOrganizationTreeRequest) (*orgpb.ListOrganizationTreeResponse, error) {
	_, tenantID, err := requireTenantScope(ctx)
	if err != nil {
		return nil, err
	}
	roots, err := s.uc.ListTree(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]*orgpb.OrganizationInfo, len(roots))
	for i, r := range roots {
		out[i] = orgToProto(r)
	}
	return &orgpb.ListOrganizationTreeResponse{Roots: out}, nil
}

func (s *OrganizationService) DeleteOrganization(ctx context.Context, req *orgpb.DeleteOrganizationRequest) (*orgpb.DeleteOrganizationResponse, error) {
	if err := s.uc.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &orgpb.DeleteOrganizationResponse{Success: true}, nil
}

func (s *OrganizationService) PurgeOrganization(ctx context.Context, req *orgpb.PurgeOrganizationRequest) (*orgpb.PurgeOrganizationResponse, error) {
	if err := s.uc.Purge(ctx, req.Id); err != nil {
		return nil, err
	}
	return &orgpb.PurgeOrganizationResponse{Success: true}, nil
}

func (s *OrganizationService) RestoreOrganization(ctx context.Context, req *orgpb.RestoreOrganizationRequest) (*orgpb.RestoreOrganizationResponse, error) {
	org, err := s.uc.Restore(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &orgpb.RestoreOrganizationResponse{Organization: orgInfoMapper.Map(org)}, nil
}

func (s *OrganizationService) AddMember(ctx context.Context, req *orgpb.AddMemberRequest) (*orgpb.AddMemberResponse, error) {
	m, err := s.uc.AddMember(ctx, &entity.OrganizationMember{
		OrganizationID: req.OrganizationId,
		UserID:         req.UserId,
		Role:           req.Role,
	})
	if err != nil {
		return nil, err
	}
	return &orgpb.AddMemberResponse{Member: orgMemberInfoMapper.Map(m)}, nil
}

func (s *OrganizationService) RemoveMember(ctx context.Context, req *orgpb.RemoveMemberRequest) (*orgpb.RemoveMemberResponse, error) {
	if err := s.uc.RemoveMember(ctx, req.OrganizationId, req.UserId); err != nil {
		return nil, err
	}
	return &orgpb.RemoveMemberResponse{Success: true}, nil
}

func (s *OrganizationService) ListMembers(ctx context.Context, req *orgpb.ListMembersRequest) (*orgpb.ListMembersResponse, error) {
	page, pageSize := pagination.ExtractPage(req.Pagination)
	members, total, err := s.uc.ListMembers(ctx, req.OrganizationId, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := orgMemberInfoMapper.MapSlice(members)
	return &orgpb.ListMembersResponse{
		Members:    items,
		Pagination: pagination.BuildPageResponse(total, page, pageSize),
	}, nil
}

func (s *OrganizationService) UpdateMemberRole(ctx context.Context, req *orgpb.UpdateMemberRoleRequest) (*orgpb.UpdateMemberRoleResponse, error) {
	m, err := s.uc.UpdateMemberRole(ctx, req.OrganizationId, req.UserId, req.Role)
	if err != nil {
		return nil, err
	}
	return &orgpb.UpdateMemberRoleResponse{Member: orgMemberInfoMapper.Map(m)}, nil
}
