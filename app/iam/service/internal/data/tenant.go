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
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/organization"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/organizationmember"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/tenant"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type tenantRepo struct {
	data *Data
	log  *logger.Helper
}

func NewTenantRepo(data *Data, l logger.Logger) biz.TenantRepo {
	return &tenantRepo{
		data: data,
		log:  logger.NewHelper(l, logger.WithModule("tenant/data/iam-service")),
	}
}

func (r *tenantRepo) Create(ctx context.Context, t *entity.Tenant) (*entity.Tenant, error) {
	ownerID, err := uuid.Parse(t.OwnerUserID)
	if err != nil {
		return nil, fmt.Errorf("invalid owner user ID: %w", err)
	}
	b := r.data.Ent(ctx).Tenant.Create().
		SetOwnerUserID(ownerID).
		SetSlug(t.Slug).
		SetName(t.Name).
		SetKind(tenant.Kind(t.Kind)).
		SetStatus(tenant.Status(t.Status))
	if t.Domain != "" {
		b.SetDomain(t.Domain)
	}
	if t.DisplayName != "" {
		b.SetDisplayName(t.DisplayName)
	}
	created, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create tenant: %w", err)
	}
	return tenantMapper.Map(created), nil
}

func (r *tenantRepo) GetByID(ctx context.Context, id string) (*entity.Tenant, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}
	t, err := r.data.Ent(ctx).Tenant.Query().
		Where(tenant.IDEQ(uid)).
		Where(tenant.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return tenantMapper.Map(t), nil
}

func (r *tenantRepo) GetBySlug(ctx context.Context, slug string) (*entity.Tenant, error) {
	t, err := r.data.Ent(ctx).Tenant.Query().
		Where(tenant.SlugEQ(slug)).
		Where(tenant.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return tenantMapper.Map(t), nil
}

func (r *tenantRepo) GetByDomain(ctx context.Context, domain string) (*entity.Tenant, error) {
	t, err := r.data.Ent(ctx).Tenant.Query().
		Where(tenant.DomainEQ(domain)).
		Where(tenant.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return tenantMapper.Map(t), nil
}

// List returns all tenants where the user is either the owner or a member
// of any organization under the tenant.
func (r *tenantRepo) List(ctx context.Context, userID string, page, pageSize int32) ([]*entity.Tenant, int64, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid user ID: %w", err)
	}

	// Collect tenant IDs via org membership (user belongs to some org in the tenant)
	orgMembers, err := r.data.Ent(ctx).OrganizationMember.Query().
		Where(organizationmember.UserIDEQ(uid)).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list org memberships: %w", err)
	}

	orgIDs := make([]uuid.UUID, 0, len(orgMembers))
	for _, m := range orgMembers {
		orgIDs = append(orgIDs, m.OrganizationID)
	}

	var tenantIDs []uuid.UUID
	if len(orgIDs) > 0 {
		orgs, err := r.data.Ent(ctx).Organization.Query().
			Where(organization.IDIn(orgIDs...)).
			Where(organization.DeletedAtIsNil()).
			All(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("list organizations: %w", err)
		}
		seen := make(map[uuid.UUID]struct{})
		for _, o := range orgs {
			if _, ok := seen[o.TenantID]; !ok {
				seen[o.TenantID] = struct{}{}
				tenantIDs = append(tenantIDs, o.TenantID)
			}
		}
	}

	// Also include tenants where user is the owner (e.g. personal tenant with no org memberships)
	ownedTenants, err := r.data.Ent(ctx).Tenant.Query().
		Where(tenant.OwnerUserIDEQ(uid)).
		Where(tenant.DeletedAtIsNil()).
		IDs(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list owned tenants: %w", err)
	}
	seen := make(map[uuid.UUID]struct{})
	for _, id := range tenantIDs {
		seen[id] = struct{}{}
	}
	for _, id := range ownedTenants {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			tenantIDs = append(tenantIDs, id)
		}
	}

	if len(tenantIDs) == 0 {
		return nil, 0, nil
	}

	query := r.data.Ent(ctx).Tenant.Query().
		Where(tenant.IDIn(tenantIDs...)).
		Where(tenant.DeletedAtIsNil()).
		Order(tenant.ByCreatedAt(sql.OrderDesc()))

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := int((page - 1) * pageSize)
	tenants, err := query.Offset(offset).Limit(int(pageSize)).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return tenantMapper.MapSlice(tenants), int64(total), nil
}

func (r *tenantRepo) Update(ctx context.Context, t *entity.Tenant) (*entity.Tenant, error) {
	uid, err := uuid.Parse(t.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}
	b := r.data.Ent(ctx).Tenant.UpdateOneID(uid)
	if t.Name != "" {
		b.SetName(t.Name)
	}
	if t.DisplayName != "" {
		b.SetDisplayName(t.DisplayName)
	}
	if t.Domain != "" {
		b.SetDomain(t.Domain)
	}
	if t.Status != "" {
		b.SetStatus(tenant.Status(t.Status))
	}
	updated, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update tenant: %w", err)
	}
	return tenantMapper.Map(updated), nil
}

func (r *tenantRepo) Delete(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid tenant ID: %w", err)
	}
	return r.data.Ent(ctx).Tenant.UpdateOneID(uid).
		SetDeletedAt(time.Now()).
		Exec(ctx)
}

func (r *tenantRepo) Purge(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid tenant ID: %w", err)
	}
	return r.data.RunInEntTx(ctx, func(txCtx context.Context) error {
		c := r.data.Ent(txCtx)

		// 1. 找出租户下所有组织 ID
		orgIDs, err := c.Organization.Query().
			Where(organization.TenantIDEQ(uid)).
			IDs(txCtx)
		if err != nil {
			return fmt.Errorf("query tenant organizations: %w", err)
		}

		// 2. 删除这些组织的所有成员
		if len(orgIDs) > 0 {
			if _, err := c.OrganizationMember.Delete().
				Where(organizationmember.OrganizationIDIn(orgIDs...)).
				Exec(txCtx); err != nil {
				return err
			}
			// 3. 删除这些组织（含软删除记录）
			if _, err := c.Organization.Delete().
				Where(organization.IDIn(orgIDs...)).
				Exec(txCtx); err != nil {
				return err
			}
		}

		// 4. 删除租户下应用（含软删除记录）
		if _, err := c.Application.Delete().
			Where(application.TenantIDEQ(uid)).
			Exec(txCtx); err != nil {
			return err
		}

		// 5. 删除租户本身
		return c.Tenant.DeleteOneID(uid).Exec(txCtx)
	})
}

// TransferOwnership updates the owner_user_id field of the tenant.
func (r *tenantRepo) TransferOwnership(ctx context.Context, tenantID, newOwnerUserID string) error {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return fmt.Errorf("invalid tenant ID: %w", err)
	}
	newOwnerID, err := uuid.Parse(newOwnerUserID)
	if err != nil {
		return fmt.Errorf("invalid new owner user ID: %w", err)
	}
	return r.data.Ent(ctx).Tenant.UpdateOneID(tid).
		SetOwnerUserID(newOwnerID).
		Exec(ctx)
}

// GetPersonalTenantByUserID finds the personal tenant owned by the given user.
func (r *tenantRepo) GetPersonalTenantByUserID(ctx context.Context, userID string) (*entity.Tenant, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	t, err := r.data.Ent(ctx).Tenant.Query().
		Where(tenant.OwnerUserIDEQ(uid)).
		Where(tenant.KindEQ(tenant.KindPersonal)).
		Where(tenant.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return tenantMapper.Map(t), nil
}
