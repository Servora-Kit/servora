package service

import (
	"context"

	dictpb "github.com/Servora-Kit/servora/api/gen/go/dict/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/pkg/pagination"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type DictService struct {
	dictpb.UnimplementedDictServiceServer
	uc     *biz.DictUsecase
	userUC *biz.UserUsecase
}

func NewDictService(uc *biz.DictUsecase, userUC *biz.UserUsecase) *DictService {
	return &DictService{uc: uc, userUC: userUC}
}

func (s *DictService) requirePlatformAdmin(ctx context.Context) error {
	return checkPlatformAdmin(ctx, s.userUC)
}

// ── DictType ──────────────────────────────────────────────────────────────────

func (s *DictService) ListDictTypes(ctx context.Context, req *dictpb.ListDictTypesRequest) (*dictpb.ListDictTypesResponse, error) {
	page, pageSize := pagination.ExtractPage(req.Pagination)
	types, total, err := s.uc.ListTypes(ctx, req.Status, int(page), int(pageSize))
	if err != nil {
		return nil, err
	}
	return &dictpb.ListDictTypesResponse{
		DictTypes:  mapDictTypesToProto(types),
		Pagination: pagination.BuildPageResponse(int64(total), page, pageSize),
	}, nil
}

func (s *DictService) GetDictType(ctx context.Context, req *dictpb.GetDictTypeRequest) (*dictpb.GetDictTypeResponse, error) {
	dt, err := s.uc.GetType(ctx, req.IdOrCode)
	if err != nil {
		return nil, err
	}
	return &dictpb.GetDictTypeResponse{DictType: dictTypeToProto(dt)}, nil
}

func (s *DictService) CreateDictType(ctx context.Context, req *dictpb.CreateDictTypeRequest) (*dictpb.CreateDictTypeResponse, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	dt, err := s.uc.CreateType(ctx, &entity.DictType{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Sort:        int(req.Sort),
	})
	if err != nil {
		return nil, err
	}
	return &dictpb.CreateDictTypeResponse{DictType: dictTypeToProto(dt)}, nil
}

func (s *DictService) UpdateDictType(ctx context.Context, req *dictpb.UpdateDictTypeRequest) (*dictpb.UpdateDictTypeResponse, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	existing, err := s.uc.GetType(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	upd := &entity.DictType{
		ID:          req.Id,
		Sort:        existing.Sort,
		Description: req.Description,
	}
	if req.Name != nil {
		upd.Name = *req.Name
	}
	if req.Sort != nil {
		upd.Sort = int(*req.Sort)
	}
	if req.Status != nil {
		upd.Status = *req.Status
	}
	dt, err := s.uc.UpdateType(ctx, upd)
	if err != nil {
		return nil, err
	}
	return &dictpb.UpdateDictTypeResponse{DictType: dictTypeToProto(dt)}, nil
}

func (s *DictService) DeleteDictType(ctx context.Context, req *dictpb.DeleteDictTypeRequest) (*dictpb.DeleteDictTypeResponse, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if err := s.uc.DeleteType(ctx, req.Id); err != nil {
		return nil, err
	}
	return &dictpb.DeleteDictTypeResponse{Success: true}, nil
}

// ── DictItem ──────────────────────────────────────────────────────────────────

func (s *DictService) ListDictItems(ctx context.Context, req *dictpb.ListDictItemsRequest) (*dictpb.ListDictItemsResponse, error) {
	items, err := s.uc.ListItems(ctx, req.DictTypeIdOrCode, req.Status)
	if err != nil {
		return nil, err
	}
	return &dictpb.ListDictItemsResponse{Items: mapDictItemsToProto(items)}, nil
}

func (s *DictService) CreateDictItem(ctx context.Context, req *dictpb.CreateDictItemRequest) (*dictpb.CreateDictItemResponse, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	item, err := s.uc.CreateItem(ctx, &entity.DictItem{
		DictTypeID: req.DictTypeId,
		Label:      req.Label,
		Value:      req.Value,
		ColorTag:   req.ColorTag,
		Sort:       int(req.Sort),
		IsDefault:  req.IsDefault,
	})
	if err != nil {
		return nil, err
	}
	return &dictpb.CreateDictItemResponse{Item: dictItemToProto(item)}, nil
}

func (s *DictService) UpdateDictItem(ctx context.Context, req *dictpb.UpdateDictItemRequest) (*dictpb.UpdateDictItemResponse, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	upd := &entity.DictItem{
		ID:       req.Id,
		ColorTag: req.ColorTag,
	}
	if req.Label != nil {
		upd.Label = *req.Label
	}
	if req.Sort != nil {
		upd.Sort = int(*req.Sort)
	}
	if req.Status != nil {
		upd.Status = *req.Status
	}
	if req.IsDefault != nil {
		upd.IsDefault = *req.IsDefault
	}
	item, err := s.uc.UpdateItem(ctx, upd)
	if err != nil {
		return nil, err
	}
	return &dictpb.UpdateDictItemResponse{Item: dictItemToProto(item)}, nil
}

func (s *DictService) DeleteDictItem(ctx context.Context, req *dictpb.DeleteDictItemRequest) (*dictpb.DeleteDictItemResponse, error) {
	if err := s.requirePlatformAdmin(ctx); err != nil {
		return nil, err
	}
	if err := s.uc.DeleteItem(ctx, req.Id); err != nil {
		return nil, err
	}
	return &dictpb.DeleteDictItemResponse{Success: true}, nil
}

// ── Mappers ───────────────────────────────────────────────────────────────────

func dictTypeToProto(dt *entity.DictType) *dictpb.DictTypeInfo {
	info := &dictpb.DictTypeInfo{
		Id:          dt.ID,
		Code:        dt.Code,
		Name:        dt.Name,
		Status:      dt.Status,
		Sort:        int32(dt.Sort),
		Description: dt.Description,
		CreatedAt:   timestamppb.New(dt.CreatedAt),
		UpdatedAt:   timestamppb.New(dt.UpdatedAt),
	}
	for _, item := range dt.Items {
		info.Items = append(info.Items, dictItemToProto(item))
	}
	return info
}

func mapDictTypesToProto(types []*entity.DictType) []*dictpb.DictTypeInfo {
	out := make([]*dictpb.DictTypeInfo, len(types))
	for i, dt := range types {
		out[i] = dictTypeToProto(dt)
	}
	return out
}

func dictItemToProto(item *entity.DictItem) *dictpb.DictItemInfo {
	return &dictpb.DictItemInfo{
		Id:         item.ID,
		DictTypeId: item.DictTypeID,
		Label:      item.Label,
		Value:      item.Value,
		ColorTag:   item.ColorTag,
		Sort:       int32(item.Sort),
		Status:     item.Status,
		IsDefault:  item.IsDefault,
		CreatedAt:  timestamppb.New(item.CreatedAt),
		UpdatedAt:  timestamppb.New(item.UpdatedAt),
	}
}

func mapDictItemsToProto(items []*entity.DictItem) []*dictpb.DictItemInfo {
	out := make([]*dictpb.DictItemInfo, len(items))
	for i, item := range items {
		out[i] = dictItemToProto(item)
	}
	return out
}
