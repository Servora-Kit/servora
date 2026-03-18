package service

import (
	"context"

	positionpb "github.com/Servora-Kit/servora/api/gen/go/position/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/pkg/pagination"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PositionService struct {
	positionpb.UnimplementedPositionServiceServer
	uc *biz.PositionUsecase
}

func NewPositionService(uc *biz.PositionUsecase) *PositionService {
	return &PositionService{uc: uc}
}

func (s *PositionService) ListPositions(ctx context.Context, req *positionpb.ListPositionsRequest) (*positionpb.ListPositionsResponse, error) {
	_, tenantID, err := requireTenantScope(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize := pagination.ExtractPage(req.Pagination)
	positions, total, err := s.uc.List(ctx, tenantID, req.OrganizationId, req.Status, int(page), int(pageSize))
	if err != nil {
		return nil, err
	}
	return &positionpb.ListPositionsResponse{
		Positions:  mapPositionsToProto(positions),
		Pagination: pagination.BuildPageResponse(int64(total), page, pageSize),
	}, nil
}

func (s *PositionService) GetPosition(ctx context.Context, req *positionpb.GetPositionRequest) (*positionpb.GetPositionResponse, error) {
	p, err := s.uc.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &positionpb.GetPositionResponse{Position: positionToProto(p)}, nil
}

func (s *PositionService) CreatePosition(ctx context.Context, req *positionpb.CreatePositionRequest) (*positionpb.CreatePositionResponse, error) {
	_, tenantID, err := requireTenantScope(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.uc.Create(ctx, &entity.Position{
		TenantID:       tenantID,
		OrganizationID: req.OrganizationId,
		Code:           req.Code,
		Name:           req.Name,
		Description:    req.Description,
		Sort:           int(req.Sort),
	})
	if err != nil {
		return nil, err
	}
	return &positionpb.CreatePositionResponse{Position: positionToProto(p)}, nil
}

func (s *PositionService) UpdatePosition(ctx context.Context, req *positionpb.UpdatePositionRequest) (*positionpb.UpdatePositionResponse, error) {
	existing, err := s.uc.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	upd := &entity.Position{
		ID:             req.Id,
		OrganizationID: req.OrganizationId,
		Sort:           existing.Sort,
	}
	if req.Name != nil {
		upd.Name = *req.Name
	}
	if req.Description != nil {
		upd.Description = req.Description
	}
	if req.Sort != nil {
		upd.Sort = int(*req.Sort)
	}
	if req.Status != nil {
		upd.Status = *req.Status
	}
	p, err := s.uc.Update(ctx, upd)
	if err != nil {
		return nil, err
	}
	return &positionpb.UpdatePositionResponse{Position: positionToProto(p)}, nil
}

func (s *PositionService) DeletePosition(ctx context.Context, req *positionpb.DeletePositionRequest) (*positionpb.DeletePositionResponse, error) {
	if err := s.uc.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &positionpb.DeletePositionResponse{Success: true}, nil
}

func positionToProto(p *entity.Position) *positionpb.PositionInfo {
	info := &positionpb.PositionInfo{
		Id:        p.ID,
		TenantId:  p.TenantID,
		Code:      p.Code,
		Name:      p.Name,
		Sort:      int32(p.Sort),
		Status:    p.Status,
		CreatedAt: timestamppb.New(p.CreatedAt),
		UpdatedAt: timestamppb.New(p.UpdatedAt),
	}
	if p.OrganizationID != nil {
		info.OrganizationId = p.OrganizationID
	}
	if p.Description != nil {
		info.Description = p.Description
	}
	return info
}

func mapPositionsToProto(positions []*entity.Position) []*positionpb.PositionInfo {
	out := make([]*positionpb.PositionInfo, len(positions))
	for i, p := range positions {
		out[i] = positionToProto(p)
	}
	return out
}
