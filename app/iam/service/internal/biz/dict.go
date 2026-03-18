package biz

import (
	"context"

	dictpb "github.com/Servora-Kit/servora/api/gen/go/dict/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type DictRepo interface {
	ListTypes(ctx context.Context, status *string, page, pageSize int) ([]*entity.DictType, int, error)
	GetTypeByIDOrCode(ctx context.Context, idOrCode string) (*entity.DictType, error)
	CreateType(ctx context.Context, dt *entity.DictType) (*entity.DictType, error)
	UpdateType(ctx context.Context, dt *entity.DictType) (*entity.DictType, error)
	DeleteType(ctx context.Context, id string) error
	ListItems(ctx context.Context, typeIDOrCode string, status *string) ([]*entity.DictItem, error)
	CreateItem(ctx context.Context, item *entity.DictItem) (*entity.DictItem, error)
	GetItemByID(ctx context.Context, id string) (*entity.DictItem, error)
	UpdateItem(ctx context.Context, item *entity.DictItem) (*entity.DictItem, error)
	DeleteItem(ctx context.Context, id string) error
}

type DictUsecase struct {
	repo DictRepo
	log  *logger.Helper
}

func NewDictUsecase(repo DictRepo, l logger.Logger) *DictUsecase {
	return &DictUsecase{
		repo: repo,
		log:  logger.NewHelper(l, logger.WithModule("dict/biz/iam-service")),
	}
}

func (uc *DictUsecase) ListTypes(ctx context.Context, status *string, page, pageSize int) ([]*entity.DictType, int, error) {
	return uc.repo.ListTypes(ctx, status, page, pageSize)
}

func (uc *DictUsecase) GetType(ctx context.Context, idOrCode string) (*entity.DictType, error) {
	dt, err := uc.repo.GetTypeByIDOrCode(ctx, idOrCode)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, dictpb.ErrorDictTypeNotFound("dict type %q not found", idOrCode)
		}
		return nil, err
	}
	return dt, nil
}

func (uc *DictUsecase) CreateType(ctx context.Context, dt *entity.DictType) (*entity.DictType, error) {
	created, err := uc.repo.CreateType(ctx, dt)
	if err != nil {
		uc.log.Errorf("create dict type failed: %v", err)
		return nil, dictpb.ErrorDictTypeCreateFailed("failed to create dict type: %v", err)
	}
	return created, nil
}

func (uc *DictUsecase) UpdateType(ctx context.Context, dt *entity.DictType) (*entity.DictType, error) {
	updated, err := uc.repo.UpdateType(ctx, dt)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, dictpb.ErrorDictTypeNotFound("dict type %s not found", dt.ID)
		}
		return nil, dictpb.ErrorDictTypeUpdateFailed("failed to update dict type: %v", err)
	}
	return updated, nil
}

func (uc *DictUsecase) DeleteType(ctx context.Context, id string) error {
	if _, err := uc.repo.GetTypeByIDOrCode(ctx, id); err != nil {
		if ent.IsNotFound(err) {
			return dictpb.ErrorDictTypeNotFound("dict type %s not found", id)
		}
		return err
	}
	if err := uc.repo.DeleteType(ctx, id); err != nil {
		return dictpb.ErrorDictTypeDeleteFailed("failed to delete dict type: %v", err)
	}
	return nil
}

func (uc *DictUsecase) ListItems(ctx context.Context, typeIDOrCode string, status *string) ([]*entity.DictItem, error) {
	return uc.repo.ListItems(ctx, typeIDOrCode, status)
}

func (uc *DictUsecase) CreateItem(ctx context.Context, item *entity.DictItem) (*entity.DictItem, error) {
	created, err := uc.repo.CreateItem(ctx, item)
	if err != nil {
		uc.log.Errorf("create dict item failed: %v", err)
		return nil, dictpb.ErrorDictItemCreateFailed("failed to create dict item: %v", err)
	}
	return created, nil
}

func (uc *DictUsecase) UpdateItem(ctx context.Context, item *entity.DictItem) (*entity.DictItem, error) {
	updated, err := uc.repo.UpdateItem(ctx, item)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, dictpb.ErrorDictItemNotFound("dict item %s not found", item.ID)
		}
		return nil, dictpb.ErrorDictItemUpdateFailed("failed to update dict item: %v", err)
	}
	return updated, nil
}

func (uc *DictUsecase) DeleteItem(ctx context.Context, id string) error {
	if _, err := uc.repo.GetItemByID(ctx, id); err != nil {
		if ent.IsNotFound(err) {
			return dictpb.ErrorDictItemNotFound("dict item %s not found", id)
		}
		return err
	}
	if err := uc.repo.DeleteItem(ctx, id); err != nil {
		return dictpb.ErrorDictItemDeleteFailed("failed to delete dict item: %v", err)
	}
	return nil
}
