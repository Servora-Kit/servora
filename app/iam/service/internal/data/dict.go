package data

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/dictitem"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/dicttype"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type dictRepo struct {
	data *Data
	log  *logger.Helper
}

func NewDictRepo(data *Data, l logger.Logger) biz.DictRepo {
	return &dictRepo{
		data: data,
		log:  logger.NewHelper(l, logger.WithModule("dict/data/iam-service")),
	}
}

// ── DictType ──────────────────────────────────────────────────────────────────

func (r *dictRepo) ListTypes(ctx context.Context, status *string, page, pageSize int) ([]*entity.DictType, int, error) {
	q := r.data.Ent(ctx).DictType.Query()
	if status != nil {
		q = q.Where(dicttype.StatusEQ(dicttype.Status(*status)))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count dict types: %w", err)
	}
	offset := (page - 1) * pageSize
	types, err := q.Offset(offset).Limit(pageSize).
		Order(dicttype.BySort(), dicttype.ByCreatedAt()).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list dict types: %w", err)
	}
	return mapDictTypes(types), total, nil
}

func (r *dictRepo) GetTypeByIDOrCode(ctx context.Context, idOrCode string) (*entity.DictType, error) {
	// Try as UUID first
	if uid, err := uuid.Parse(idOrCode); err == nil {
		dt, err := r.data.Ent(ctx).DictType.Query().
			Where(dicttype.IDEQ(uid)).
			WithItems(func(q *ent.DictItemQuery) {
				q.Where(dictitem.StatusEQ(dictitem.StatusACTIVE)).
					Order(dictitem.BySort())
			}).
			Only(ctx)
		if err != nil {
			return nil, err
		}
		return mapDictTypeWithItems(dt), nil
	}
	// Fall back to code lookup
	dt, err := r.data.Ent(ctx).DictType.Query().
		Where(dicttype.CodeEQ(idOrCode)).
		WithItems(func(q *ent.DictItemQuery) {
			q.Where(dictitem.StatusEQ(dictitem.StatusACTIVE)).
				Order(dictitem.BySort())
		}).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return mapDictTypeWithItems(dt), nil
}

func (r *dictRepo) CreateType(ctx context.Context, dt *entity.DictType) (*entity.DictType, error) {
	b := r.data.Ent(ctx).DictType.Create().
		SetCode(dt.Code).
		SetName(dt.Name).
		SetSort(dt.Sort)
	if dt.Description != nil {
		b.SetDescription(*dt.Description)
	}
	created, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create dict type: %w", err)
	}
	return mapDictType(created), nil
}

func (r *dictRepo) UpdateType(ctx context.Context, dt *entity.DictType) (*entity.DictType, error) {
	uid, err := uuid.Parse(dt.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid dict type ID: %w", err)
	}
	b := r.data.Ent(ctx).DictType.UpdateOneID(uid)
	if dt.Name != "" {
		b.SetName(dt.Name)
	}
	if dt.Description != nil {
		b.SetDescription(*dt.Description)
	}
	if dt.Status != "" {
		b.SetStatus(dicttype.Status(dt.Status))
	}
	b.SetSort(dt.Sort)
	updated, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update dict type: %w", err)
	}
	return mapDictType(updated), nil
}

func (r *dictRepo) DeleteType(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid dict type ID: %w", err)
	}
	// Items are deleted via cascade (schema annotation)
	return r.data.Ent(ctx).DictType.DeleteOneID(uid).Exec(ctx)
}

// ── DictItem ──────────────────────────────────────────────────────────────────

func (r *dictRepo) ListItems(ctx context.Context, typeIDOrCode string, status *string) ([]*entity.DictItem, error) {
	dt, err := r.GetTypeByIDOrCode(ctx, typeIDOrCode)
	if err != nil {
		return nil, err
	}
	tid, err := uuid.Parse(dt.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid dict type ID: %w", err)
	}
	q := r.data.Ent(ctx).DictItem.Query().
		Where(dictitem.DictTypeIDEQ(tid))
	if status != nil {
		q = q.Where(dictitem.StatusEQ(dictitem.Status(*status)))
	}
	items, err := q.Order(dictitem.BySort(), dictitem.ByCreatedAt()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list dict items: %w", err)
	}
	return mapDictItems(items), nil
}

func (r *dictRepo) GetItemByID(ctx context.Context, id string) (*entity.DictItem, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid dict item ID: %w", err)
	}
	item, err := r.data.Ent(ctx).DictItem.Query().
		Where(dictitem.IDEQ(uid)).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return mapDictItem(item), nil
}

func (r *dictRepo) CreateItem(ctx context.Context, item *entity.DictItem) (*entity.DictItem, error) {
	tid, err := uuid.Parse(item.DictTypeID)
	if err != nil {
		return nil, fmt.Errorf("invalid dict type ID: %w", err)
	}
	b := r.data.Ent(ctx).DictItem.Create().
		SetDictTypeID(tid).
		SetLabel(item.Label).
		SetValue(item.Value).
		SetSort(item.Sort).
		SetIsDefault(item.IsDefault)
	if item.ColorTag != nil {
		b.SetColorTag(*item.ColorTag)
	}
	created, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create dict item: %w", err)
	}
	return mapDictItem(created), nil
}

func (r *dictRepo) UpdateItem(ctx context.Context, item *entity.DictItem) (*entity.DictItem, error) {
	uid, err := uuid.Parse(item.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid dict item ID: %w", err)
	}
	b := r.data.Ent(ctx).DictItem.UpdateOneID(uid)
	if item.Label != "" {
		b.SetLabel(item.Label)
	}
	if item.ColorTag != nil {
		b.SetColorTag(*item.ColorTag)
	}
	if item.Status != "" {
		b.SetStatus(dictitem.Status(item.Status))
	}
	b.SetSort(item.Sort)
	b.SetIsDefault(item.IsDefault)
	updated, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update dict item: %w", err)
	}
	return mapDictItem(updated), nil
}

func (r *dictRepo) DeleteItem(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid dict item ID: %w", err)
	}
	return r.data.Ent(ctx).DictItem.DeleteOneID(uid).Exec(ctx)
}

// ── Mappers ───────────────────────────────────────────────────────────────────

func mapDictType(dt *ent.DictType) *entity.DictType {
	e := &entity.DictType{
		ID:        dt.ID.String(),
		Code:      dt.Code,
		Name:      dt.Name,
		Status:    string(dt.Status),
		Sort:      dt.Sort,
		CreatedAt: dt.CreatedAt,
		UpdatedAt: dt.UpdatedAt,
	}
	if dt.Description != nil {
		e.Description = dt.Description
	}
	return e
}

func mapDictTypeWithItems(dt *ent.DictType) *entity.DictType {
	e := mapDictType(dt)
	for _, item := range dt.Edges.Items {
		e.Items = append(e.Items, mapDictItem(item))
	}
	return e
}

func mapDictTypes(types []*ent.DictType) []*entity.DictType {
	result := make([]*entity.DictType, len(types))
	for i, dt := range types {
		result[i] = mapDictType(dt)
	}
	return result
}

func mapDictItem(item *ent.DictItem) *entity.DictItem {
	e := &entity.DictItem{
		ID:         item.ID.String(),
		DictTypeID: item.DictTypeID.String(),
		Label:      item.Label,
		Value:      item.Value,
		Sort:       item.Sort,
		Status:     string(item.Status),
		IsDefault:  item.IsDefault,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}
	if item.ColorTag != nil {
		e.ColorTag = item.ColorTag
	}
	return e
}

func mapDictItems(items []*ent.DictItem) []*entity.DictItem {
	result := make([]*entity.DictItem, len(items))
	for i, item := range items {
		result[i] = mapDictItem(item)
	}
	return result
}
