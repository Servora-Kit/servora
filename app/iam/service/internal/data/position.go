package data

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/position"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type positionRepo struct {
	data *Data
	log  *logger.Helper
}

func NewPositionRepo(data *Data, l logger.Logger) biz.PositionRepo {
	return &positionRepo{
		data: data,
		log:  logger.NewHelper(l, logger.WithModule("position/data/iam-service")),
	}
}

func (r *positionRepo) List(ctx context.Context, tenantID string, organizationID *string, status *string, page, pageSize int) ([]*entity.Position, int, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid tenant ID: %w", err)
	}
	q := r.data.Ent(ctx).Position.Query().
		Where(position.TenantIDEQ(tid))
	if organizationID != nil {
		if oid, err := uuid.Parse(*organizationID); err == nil {
			q = q.Where(position.OrganizationIDEQ(oid))
		}
	}
	if status != nil {
		q = q.Where(position.StatusEQ(position.Status(*status)))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count positions: %w", err)
	}
	offset := (page - 1) * pageSize
	positions, err := q.Offset(offset).Limit(pageSize).
		Order(position.BySort(), position.ByCreatedAt()).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list positions: %w", err)
	}
	return mapPositions(positions), total, nil
}

func (r *positionRepo) GetByID(ctx context.Context, id string) (*entity.Position, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid position ID: %w", err)
	}
	p, err := r.data.Ent(ctx).Position.Query().
		Where(position.IDEQ(uid)).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return mapPosition(p), nil
}

func (r *positionRepo) Create(ctx context.Context, p *entity.Position) (*entity.Position, error) {
	tid, err := uuid.Parse(p.TenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}
	b := r.data.Ent(ctx).Position.Create().
		SetTenantID(tid).
		SetCode(p.Code).
		SetName(p.Name).
		SetSort(p.Sort).
		SetStatus(position.StatusACTIVE)
	if p.Description != nil {
		b.SetDescription(*p.Description)
	}
	if p.OrganizationID != nil {
		if oid, err := uuid.Parse(*p.OrganizationID); err == nil {
			b.SetOrganizationID(oid)
		}
	}
	created, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create position: %w", err)
	}
	return mapPosition(created), nil
}

func (r *positionRepo) Update(ctx context.Context, p *entity.Position) (*entity.Position, error) {
	uid, err := uuid.Parse(p.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid position ID: %w", err)
	}
	b := r.data.Ent(ctx).Position.UpdateOneID(uid)
	if p.Name != "" {
		b.SetName(p.Name)
	}
	if p.Description != nil {
		b.SetDescription(*p.Description)
	}
	if p.OrganizationID != nil {
		if oid, err := uuid.Parse(*p.OrganizationID); err == nil {
			b.SetOrganizationID(oid)
		}
	}
	if p.Status != "" {
		b.SetStatus(position.Status(p.Status))
	}
	b.SetSort(p.Sort)
	updated, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update position: %w", err)
	}
	return mapPosition(updated), nil
}

func (r *positionRepo) Delete(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid position ID: %w", err)
	}
	return r.data.Ent(ctx).Position.DeleteOneID(uid).Exec(ctx)
}

func mapPosition(p *ent.Position) *entity.Position {
	e := &entity.Position{
		ID:        p.ID.String(),
		TenantID:  p.TenantID.String(),
		Code:      p.Code,
		Name:      p.Name,
		Sort:      p.Sort,
		Status:    string(p.Status),
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
	if p.Description != nil {
		e.Description = p.Description
	}
	if p.OrganizationID != nil {
		s := p.OrganizationID.String()
		e.OrganizationID = &s
	}
	return e
}

func mapPositions(positions []*ent.Position) []*entity.Position {
	result := make([]*entity.Position, len(positions))
	for i, p := range positions {
		result[i] = mapPosition(p)
	}
	return result
}
