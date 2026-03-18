package service

import (
	"context"

	tenantpb "github.com/Servora-Kit/servora/api/gen/go/tenant/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/pkg/pagination"
)

type TenantService struct {
	tenantpb.UnimplementedTenantServiceServer

	uc *biz.TenantUsecase
}

func NewTenantService(uc *biz.TenantUsecase) *TenantService {
	return &TenantService{uc: uc}
}

func (s *TenantService) CreateTenant(ctx context.Context, req *tenantpb.CreateTenantRequest) (*tenantpb.CreateTenantResponse, error) {
	callerID, err := requireAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}

	t, err := s.uc.CreateWithDefaults(ctx, &entity.Tenant{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Slug:        req.Slug,
		Kind:        req.Kind,
		Domain:      req.Domain,
	}, callerID)
	if err != nil {
		return nil, err
	}
	return &tenantpb.CreateTenantResponse{Tenant: tenantInfoMapper.Map(t)}, nil
}

func (s *TenantService) GetTenant(ctx context.Context, req *tenantpb.GetTenantRequest) (*tenantpb.GetTenantResponse, error) {
	t, err := s.uc.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &tenantpb.GetTenantResponse{Tenant: tenantInfoMapper.Map(t)}, nil
}

func (s *TenantService) ListTenants(ctx context.Context, req *tenantpb.ListTenantsRequest) (*tenantpb.ListTenantsResponse, error) {
	callerID, err := requireAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize := pagination.ExtractPage(req.Pagination)
	tenants, total, err := s.uc.List(ctx, callerID, page, pageSize)
	if err != nil {
		return nil, err
	}

	return &tenantpb.ListTenantsResponse{
		Tenants:    tenantInfoMapper.MapSlice(tenants),
		Pagination: pagination.BuildPageResponse(total, page, pageSize),
	}, nil
}

func (s *TenantService) UpdateTenant(ctx context.Context, req *tenantpb.UpdateTenantRequest) (*tenantpb.UpdateTenantResponse, error) {
	t, err := s.uc.Update(ctx, &entity.Tenant{
		ID:          req.Id,
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Domain:      req.Domain,
		Status:      req.Status,
	})
	if err != nil {
		return nil, err
	}
	return &tenantpb.UpdateTenantResponse{Tenant: tenantInfoMapper.Map(t)}, nil
}

func (s *TenantService) DeleteTenant(ctx context.Context, req *tenantpb.DeleteTenantRequest) (*tenantpb.DeleteTenantResponse, error) {
	if err := s.uc.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &tenantpb.DeleteTenantResponse{Success: true}, nil
}

func (s *TenantService) TransferOwnership(ctx context.Context, req *tenantpb.TransferTenantOwnershipRequest) (*tenantpb.TransferTenantOwnershipResponse, error) {
	callerID, err := requireAuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.uc.TransferOwnership(ctx, req.TenantId, callerID, req.NewOwnerUserId); err != nil {
		return nil, err
	}
	return &tenantpb.TransferTenantOwnershipResponse{Success: true}, nil
}
