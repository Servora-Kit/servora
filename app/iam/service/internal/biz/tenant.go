package biz

import (
	"context"

	authnpb "github.com/Servora-Kit/servora/api/gen/go/authn/service/v1"
	tenantpb "github.com/Servora-Kit/servora/api/gen/go/tenant/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/helpers"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type TenantRepo interface {
	Create(ctx context.Context, t *entity.Tenant) (*entity.Tenant, error)
	GetByID(ctx context.Context, id string) (*entity.Tenant, error)
	GetBySlug(ctx context.Context, slug string) (*entity.Tenant, error)
	GetByDomain(ctx context.Context, domain string) (*entity.Tenant, error)
	List(ctx context.Context, userID string, page, pageSize int32) ([]*entity.Tenant, int64, error)
	Update(ctx context.Context, t *entity.Tenant) (*entity.Tenant, error)
	Delete(ctx context.Context, id string) error
	Purge(ctx context.Context, id string) error
	TransferOwnership(ctx context.Context, tenantID, newOwnerUserID string) error
	GetPersonalTenantByUserID(ctx context.Context, userID string) (*entity.Tenant, error)
}

type TenantUsecase struct {
	repo  TenantRepo
	orgUC *OrganizationUsecase
	authz AuthZRepo
	log   *logger.Helper
}

func NewTenantUsecase(repo TenantRepo, orgUC *OrganizationUsecase, authz AuthZRepo, l logger.Logger) *TenantUsecase {
	return &TenantUsecase{
		repo:  repo,
		orgUC: orgUC,
		authz: authz,
		log:   logger.NewHelper(l, logger.WithModule("tenant/biz/iam-service")),
	}
}

func (uc *TenantUsecase) Create(ctx context.Context, t *entity.Tenant, creatorUserID string) (*entity.Tenant, error) {
	if t.Slug == "" {
		t.Slug = helpers.Slugify(t.Name)
	}
	if t.DisplayName == "" {
		t.DisplayName = t.Name
	}

	if _, err := uc.repo.GetBySlug(ctx, t.Slug); err == nil {
		return nil, tenantpb.ErrorTenantAlreadyExists("slug '%s' already taken", t.Slug)
	} else if !ent.IsNotFound(err) {
		uc.log.Errorf("check slug failed: %v", err)
		return nil, tenantpb.ErrorTenantCreateFailed("internal error")
	}

	if t.Kind == "" {
		t.Kind = "business"
	}
	if t.Status == "" {
		t.Status = "active"
	}
	t.OwnerUserID = creatorUserID

	created, err := uc.repo.Create(ctx, t)
	if err != nil {
		uc.log.Errorf("create tenant failed: %v", err)
		return nil, tenantpb.ErrorTenantCreateFailed("failed to create tenant")
	}

	if uc.authz != nil {
		if err := uc.authz.WriteTuples(ctx,
			Tuple{User: "user:" + creatorUserID, Relation: "owner", Object: "tenant:" + created.ID},
			Tuple{User: "platform:default", Relation: "platform", Object: "tenant:" + created.ID},
		); err != nil {
			uc.log.Errorf("write FGA tuples failed, rolling back: %v", err)
			if delErr := uc.repo.Purge(ctx, created.ID); delErr != nil {
				uc.log.Errorf("rollback purge tenant failed: %v", delErr)
			}
			return nil, tenantpb.ErrorTenantCreateFailed("failed to write authorization tuples")
		}
	}

	return created, nil
}

// CreateWithDefaults creates a tenant and its default organization. The optional
// orgDisplayName argument overrides the computed default org display name; callers
// that do not pass it get "{{tenantDisplayName}} 默认组织".
func (uc *TenantUsecase) CreateWithDefaults(ctx context.Context, t *entity.Tenant, creatorUserID string, orgDisplayName ...string) (*entity.Tenant, error) {
	created, err := uc.Create(ctx, t, creatorUserID)
	if err != nil {
		return nil, err
	}

	defaultOrgSlug := created.Slug + "-default"
	defaultOrgName := created.Slug + "-default"
	defaultOrgDisplayName := created.DisplayName
	if defaultOrgDisplayName == "" {
		defaultOrgDisplayName = created.Name
	}
	defaultOrgDisplayName += " 默认组织"
	if len(orgDisplayName) > 0 && orgDisplayName[0] != "" {
		defaultOrgDisplayName = orgDisplayName[0]
	}

	if _, err := uc.orgUC.CreateDefault(ctx, creatorUserID, defaultOrgName, defaultOrgSlug, defaultOrgDisplayName, created.ID); err != nil {
		uc.log.Errorf("create default org failed, rolling back tenant: %v", err)
		uc.rollbackTenantCreate(ctx, created.ID, creatorUserID)
		return nil, tenantpb.ErrorTenantCreateFailed("failed to create default organization")
	}

	return created, nil
}

func (uc *TenantUsecase) rollbackTenantCreate(ctx context.Context, tenantID, userID string) {
	if uc.authz != nil {
		_ = uc.authz.DeleteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: "owner", Object: "tenant:" + tenantID},
			Tuple{User: "platform:default", Relation: "platform", Object: "tenant:" + tenantID},
		)
	}
	if delErr := uc.repo.Purge(ctx, tenantID); delErr != nil {
		uc.log.Errorf("rollback purge tenant failed: %v", delErr)
	}
}

func (uc *TenantUsecase) EnsurePersonalTenant(ctx context.Context, userID, userName string) (*entity.Tenant, error) {
	existing, err := uc.repo.GetPersonalTenantByUserID(ctx, userID)
	if err == nil {
		return existing, nil
	}
	if !ent.IsNotFound(err) {
		uc.log.Errorf("get personal tenant failed: %v", err)
		return nil, tenantpb.ErrorTenantCreateFailed("internal error")
	}

	// Build a URL-safe slug from the user name. If a slug collision occurs
	// (rare but possible when two names Slugify to the same string), fall back to
	// appending the first 8 characters of the userID.
	baseSlug := "personal-" + helpers.Slugify(userName)
	slug := baseSlug
	if _, err := uc.repo.GetBySlug(ctx, slug); err == nil {
		suffix := userID
		if len(suffix) > 8 {
			suffix = suffix[:8]
		}
		slug = baseSlug + "-" + suffix
	}

	displayName := userName + "的空间"
	t := &entity.Tenant{
		Slug:        slug,
		Name:        slug,
		DisplayName: displayName,
		Kind:        "personal",
		Status:      "active",
	}
	return uc.CreateWithDefaults(ctx, t, userID, userName+"的组织")
}

func (uc *TenantUsecase) Get(ctx context.Context, id string) (*entity.Tenant, error) {
	t, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, tenantpb.ErrorTenantNotFound("tenant %s not found", id)
		}
		uc.log.Errorf("get tenant failed: %v", err)
		return nil, tenantpb.ErrorTenantCreateFailed("internal error")
	}
	return t, nil
}

func (uc *TenantUsecase) GetBySlug(ctx context.Context, slug string) (*entity.Tenant, error) {
	t, err := uc.repo.GetBySlug(ctx, slug)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, tenantpb.ErrorTenantNotFound("tenant with slug %s not found", slug)
		}
		uc.log.Errorf("get tenant by slug failed: %v", err)
		return nil, tenantpb.ErrorTenantCreateFailed("internal error")
	}
	return t, nil
}

func (uc *TenantUsecase) List(ctx context.Context, userID string, page, pageSize int32) ([]*entity.Tenant, int64, error) {
	if userID == "" {
		return nil, 0, authnpb.ErrorUnauthorized("user not authenticated")
	}
	tenants, total, err := uc.repo.List(ctx, userID, page, pageSize)
	if err != nil {
		uc.log.Errorf("list tenants failed: %v", err)
		return nil, 0, tenantpb.ErrorTenantCreateFailed("internal error")
	}
	return tenants, total, nil
}

func (uc *TenantUsecase) Update(ctx context.Context, t *entity.Tenant) (*entity.Tenant, error) {
	updated, err := uc.repo.Update(ctx, t)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, tenantpb.ErrorTenantNotFound("tenant %s not found", t.ID)
		}
		uc.log.Errorf("update tenant failed: %v", err)
		return nil, tenantpb.ErrorTenantUpdateFailed("failed to update tenant")
	}
	return updated, nil
}

func (uc *TenantUsecase) Delete(ctx context.Context, id string) error {
	t, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return tenantpb.ErrorTenantNotFound("tenant %s not found", id)
		}
		uc.log.Errorf("get tenant failed: %v", err)
		return tenantpb.ErrorTenantCreateFailed("internal error")
	}

	if t.Kind == "personal" {
		return tenantpb.ErrorTenantDeleteFailed("personal tenant cannot be deleted")
	}

	if err := uc.repo.Delete(ctx, id); err != nil {
		uc.log.Errorf("soft delete tenant failed: %v", err)
		return tenantpb.ErrorTenantDeleteFailed("failed to delete tenant")
	}
	return nil
}

// TransferOwnership transfers tenant ownership from the current owner to newOwnerUserID.
// The new owner must already be a member of any organization in this tenant.
// The caller must hold can_transfer_ownership (verified by authz middleware).
func (uc *TenantUsecase) TransferOwnership(ctx context.Context, tenantID, callerID, newOwnerUserID string) error {
	if callerID == newOwnerUserID {
		return tenantpb.ErrorTenantUpdateFailed("new owner must be a different user")
	}

	t, err := uc.repo.GetByID(ctx, tenantID)
	if err != nil {
		return tenantpb.ErrorTenantNotFound("tenant not found")
	}

	currentOwnerID := t.OwnerUserID

	if err := uc.repo.TransferOwnership(ctx, tenantID, newOwnerUserID); err != nil {
		uc.log.Errorf("transfer ownership DB update failed: %v", err)
		return tenantpb.ErrorTenantUpdateFailed("failed to transfer ownership")
	}

	if uc.authz != nil {
		if err := uc.authz.DeleteTuples(ctx,
			Tuple{User: "user:" + currentOwnerID, Relation: "owner", Object: "tenant:" + tenantID},
		); err != nil {
			uc.log.Warnf("delete old owner FGA tuple failed during transfer: %v", err)
		}
		if err := uc.authz.WriteTuples(ctx,
			Tuple{User: "user:" + newOwnerUserID, Relation: "owner", Object: "tenant:" + tenantID},
		); err != nil {
			uc.log.Errorf("write new owner FGA tuple failed during transfer: %v", err)
			return tenantpb.ErrorTenantUpdateFailed("failed to update authorization tuples")
		}
	}

	return nil
}
