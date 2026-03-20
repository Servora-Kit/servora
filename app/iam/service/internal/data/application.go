package data

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/application"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type applicationRepo struct {
	data *Data
	log  *logger.Helper
}

func NewApplicationRepo(data *Data, l logger.Logger) biz.ApplicationRepo {
	return &applicationRepo{
		data: data,
		log:  logger.For(l, "application/data/iam"),
	}
}

func (r *applicationRepo) Create(ctx context.Context, app *entity.Application) (*entity.Application, error) {
	created, err := r.data.Ent(ctx).Application.Create().
		SetClientID(app.ClientID).
		SetClientSecretHash(app.ClientSecretHash).
		SetName(app.Name).
		SetRedirectUris(app.RedirectURIs).
		SetScopes(app.Scopes).
		SetGrantTypes(app.GrantTypes).
		SetApplicationType(app.ApplicationType).
		SetAccessTokenType(app.AccessTokenType).
		SetType(app.Type).
		SetIDTokenLifetime(int(app.IDTokenLifetime.Seconds())).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return applicationMapper.Map(created), nil
}

func (r *applicationRepo) GetByID(ctx context.Context, id string) (*entity.Application, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid application id: %w", err)
	}
	a, err := r.data.Ent(ctx).Application.Query().
		Where(application.IDEQ(uid), application.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return applicationMapper.Map(a), nil
}

func (r *applicationRepo) GetByClientID(ctx context.Context, clientID string) (*entity.Application, error) {
	a, err := r.data.Ent(ctx).Application.Query().
		Where(application.ClientIDEQ(clientID), application.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return applicationMapper.Map(a), nil
}

func (r *applicationRepo) List(ctx context.Context, page, pageSize int32) ([]*entity.Application, int64, error) {
	offset := int((page - 1) * pageSize)
	limit := int(pageSize)

	query := r.data.Ent(ctx).Application.Query().
		Where(application.DeletedAtIsNil()).
		Order(application.ByCreatedAt(sql.OrderDesc()))

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	apps, err := query.Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return applicationMapper.MapSlice(apps), int64(total), nil
}

func (r *applicationRepo) Update(ctx context.Context, app *entity.Application) (*entity.Application, error) {
	uid, err := uuid.Parse(app.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid application id: %w", err)
	}
	n, err := r.data.Ent(ctx).Application.Update().
		Where(application.IDEQ(uid), application.DeletedAtIsNil()).
		SetName(app.Name).
		SetRedirectUris(app.RedirectURIs).
		SetScopes(app.Scopes).
		SetGrantTypes(app.GrantTypes).
		SetIDTokenLifetime(int(app.IDTokenLifetime.Seconds())).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, fmt.Errorf("application not found")
	}
	return r.GetByID(ctx, app.ID)
}

func (r *applicationRepo) Delete(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid application id: %w", err)
	}
	n, err := r.data.Ent(ctx).Application.Update().
		Where(application.IDEQ(uid), application.DeletedAtIsNil()).
		SetDeletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("application not found")
	}
	return nil
}

func (r *applicationRepo) UpdateClientSecretHash(ctx context.Context, id string, hash string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid application id: %w", err)
	}
	n, err := r.data.Ent(ctx).Application.Update().
		Where(application.IDEQ(uid), application.DeletedAtIsNil()).
		SetClientSecretHash(hash).
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("application not found")
	}
	return nil
}
