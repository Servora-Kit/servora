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
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/predicate"
	"github.com/Servora-Kit/servora/pkg/ent/scope"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type applicationRepo struct {
	data *Data
	log  *logger.Helper
}

func NewApplicationRepo(data *Data, l logger.Logger) biz.ApplicationRepo {
	return &applicationRepo{
		data: data,
		log:  logger.NewHelper(l, logger.WithModule("application/data/iam-service")),
	}
}

func (r *applicationRepo) Create(ctx context.Context, app *entity.Application) (*entity.Application, error) {
	orgID, err := uuid.Parse(app.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("invalid organization_id: %w", err)
	}
	created, err := r.data.Ent(ctx).Application.Create().
		SetClientID(app.ClientID).
		SetClientSecretHash(app.ClientSecretHash).
		SetName(app.Name).
		SetRedirectUris(app.RedirectURIs).
		SetScopes(app.Scopes).
		SetGrantTypes(app.GrantTypes).
		SetApplicationType(app.ApplicationType).
		SetAccessTokenType(app.AccessTokenType).
		SetOrganizationID(orgID).
		SetIDTokenLifetime(int(app.IDTokenLifetime.Seconds())).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return applicationMapper.Map(created), nil
}

func (r *applicationRepo) GetByID(ctx context.Context, orgID, id string) (*entity.Application, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid application id: %w", err)
	}
	query := r.data.Ent(ctx).Application.Query().
		Where(application.IDEQ(uid), application.DeletedAtIsNil()).
		Where(scope.ByUUID(orgID, application.OrganizationIDEQ)...)
	a, err := query.Only(ctx)
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

func (r *applicationRepo) ListByOrganizationID(ctx context.Context, orgID string, page, pageSize int32) ([]*entity.Application, int64, error) {
	uid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid organization_id: %w", err)
	}
	offset := int((page - 1) * pageSize)
	limit := int(pageSize)

	query := r.data.Ent(ctx).Application.Query().
		Where(application.OrganizationIDEQ(uid), application.DeletedAtIsNil()).
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

func (r *applicationRepo) Update(ctx context.Context, orgID string, app *entity.Application) (*entity.Application, error) {
	uid, err := uuid.Parse(app.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid application id: %w", err)
	}
	predicates := append(
		[]predicate.Application{application.IDEQ(uid), application.DeletedAtIsNil()},
		scope.ByUUID(orgID, application.OrganizationIDEQ)...,
	)
	n, err := r.data.Ent(ctx).Application.Update().
		Where(predicates...).
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
	return r.GetByID(ctx, orgID, app.ID)
}

func (r *applicationRepo) Delete(ctx context.Context, orgID, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid application id: %w", err)
	}
	predicates := append(
		[]predicate.Application{application.IDEQ(uid), application.DeletedAtIsNil()},
		scope.ByUUID(orgID, application.OrganizationIDEQ)...,
	)
	n, err := r.data.Ent(ctx).Application.Update().
		Where(predicates...).
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

func (r *applicationRepo) UpdateClientSecretHash(ctx context.Context, orgID, id string, hash string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid application id: %w", err)
	}
	predicates := append(
		[]predicate.Application{application.IDEQ(uid), application.DeletedAtIsNil()},
		scope.ByUUID(orgID, application.OrganizationIDEQ)...,
	)
	n, err := r.data.Ent(ctx).Application.Update().
		Where(predicates...).
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
