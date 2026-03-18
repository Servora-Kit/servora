package biz

import (
	"context"

	positionpb "github.com/Servora-Kit/servora/api/gen/go/position/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type PositionRepo interface {
	List(ctx context.Context, tenantID string, organizationID *string, status *string, page, pageSize int) ([]*entity.Position, int, error)
	GetByID(ctx context.Context, id string) (*entity.Position, error)
	Create(ctx context.Context, p *entity.Position) (*entity.Position, error)
	Update(ctx context.Context, p *entity.Position) (*entity.Position, error)
	Delete(ctx context.Context, id string) error
}

type PositionUsecase struct {
	repo PositionRepo
	log  *logger.Helper
}

func NewPositionUsecase(repo PositionRepo, l logger.Logger) *PositionUsecase {
	return &PositionUsecase{
		repo: repo,
		log:  logger.NewHelper(l, logger.WithModule("position/biz/iam-service")),
	}
}

func (uc *PositionUsecase) List(ctx context.Context, tenantID string, organizationID *string, status *string, page, pageSize int) ([]*entity.Position, int, error) {
	return uc.repo.List(ctx, tenantID, organizationID, status, page, pageSize)
}

func (uc *PositionUsecase) Get(ctx context.Context, id string) (*entity.Position, error) {
	p, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, positionpb.ErrorPositionNotFound("position %s not found", id)
		}
		uc.log.Errorf("get position failed: %v", err)
		return nil, err
	}
	return p, nil
}

func (uc *PositionUsecase) Create(ctx context.Context, p *entity.Position) (*entity.Position, error) {
	created, err := uc.repo.Create(ctx, p)
	if err != nil {
		uc.log.Errorf("create position failed: %v", err)
		return nil, positionpb.ErrorPositionCreateFailed("failed to create position: %v", err)
	}
	return created, nil
}

func (uc *PositionUsecase) Update(ctx context.Context, p *entity.Position) (*entity.Position, error) {
	updated, err := uc.repo.Update(ctx, p)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, positionpb.ErrorPositionNotFound("position %s not found", p.ID)
		}
		uc.log.Errorf("update position failed: %v", err)
		return nil, positionpb.ErrorPositionUpdateFailed("failed to update position: %v", err)
	}
	return updated, nil
}

func (uc *PositionUsecase) Delete(ctx context.Context, id string) error {
	if _, err := uc.repo.GetByID(ctx, id); err != nil {
		if ent.IsNotFound(err) {
			return positionpb.ErrorPositionNotFound("position %s not found", id)
		}
		return err
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		uc.log.Errorf("delete position failed: %v", err)
		return positionpb.ErrorPositionDeleteFailed("failed to delete position: %v", err)
	}
	return nil
}
