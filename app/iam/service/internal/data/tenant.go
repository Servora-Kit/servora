package data

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/application"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/organization"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/organizationmember"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/tenant"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/tenantmember"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/user"
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
	b := r.data.Ent(ctx).Tenant.Create().
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

func (r *tenantRepo) List(ctx context.Context, userID string, page, pageSize int32) ([]*entity.Tenant, int64, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid user ID: %w", err)
	}

	memberTenantIDs, err := r.data.Ent(ctx).TenantMember.Query().
		Where(tenantmember.UserIDEQ(uid)).
		Select(tenantmember.FieldTenantID).
		Strings(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list member tenants: %w", err)
	}

	tenantUUIDs := make([]uuid.UUID, 0, len(memberTenantIDs))
	for _, idStr := range memberTenantIDs {
		if tid, e := uuid.Parse(idStr); e == nil {
			tenantUUIDs = append(tenantUUIDs, tid)
		}
	}

	query := r.data.Ent(ctx).Tenant.Query().
		Where(tenant.IDIn(tenantUUIDs...)).
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

		// 4. 删除租户成员
		if _, err := c.TenantMember.Delete().
			Where(tenantmember.TenantIDEQ(uid)).
			Exec(txCtx); err != nil {
			return err
		}

		// 5. 删除租户下应用（含软删除记录）
		if _, err := c.Application.Delete().
			Where(application.TenantIDEQ(uid)).
			Exec(txCtx); err != nil {
			return err
		}

		// 6. 删除租户本身
		return c.Tenant.DeleteOneID(uid).Exec(txCtx)
	})
}

func (r *tenantRepo) AddMember(ctx context.Context, m *entity.TenantMember) (*entity.TenantMember, error) {
	tid, err := uuid.Parse(m.TenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}
	uid, err := uuid.Parse(m.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	b := r.data.Ent(ctx).TenantMember.Create().
		SetTenantID(tid).
		SetUserID(uid).
		SetRole(tenantmember.Role(m.Role)).
		SetStatus(tenantmember.Status(m.Status))
	if m.JoinedAt != nil {
		b.SetJoinedAt(*m.JoinedAt)
	}
	created, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("add tenant member: %w", err)
	}
	return r.enrichMember(ctx, created)
}

func (r *tenantRepo) RemoveMember(ctx context.Context, tenantID, userID string) error {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return fmt.Errorf("invalid tenant ID: %w", err)
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	_, err = r.data.Ent(ctx).TenantMember.Delete().
		Where(
			tenantmember.TenantIDEQ(tid),
			tenantmember.UserIDEQ(uid),
		).Exec(ctx)
	return err
}

func (r *tenantRepo) GetMember(ctx context.Context, tenantID, userID string) (*entity.TenantMember, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	m, err := r.data.Ent(ctx).TenantMember.Query().
		Where(
			tenantmember.TenantIDEQ(tid),
			tenantmember.UserIDEQ(uid),
		).Only(ctx)
	if err != nil {
		return nil, err
	}
	return r.enrichMember(ctx, m)
}

func (r *tenantRepo) GetOwnerMember(ctx context.Context, tenantID string) (*entity.TenantMember, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}
	m, err := r.data.Ent(ctx).TenantMember.Query().
		Where(
			tenantmember.TenantIDEQ(tid),
			tenantmember.RoleEQ(tenantmember.RoleOwner),
		).Only(ctx)
	if err != nil {
		return nil, err
	}
	return r.enrichMember(ctx, m)
}

func (r *tenantRepo) ListMembers(ctx context.Context, tenantID string, page, pageSize int32) ([]*entity.TenantMember, int64, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid tenant ID: %w", err)
	}
	query := r.data.Ent(ctx).TenantMember.Query().
		Where(tenantmember.TenantIDEQ(tid)).
		Order(tenantmember.ByCreatedAt(sql.OrderDesc()))

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := int((page - 1) * pageSize)
	members, err := query.Offset(offset).Limit(int(pageSize)).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	userIDs := make([]uuid.UUID, len(members))
	for i, m := range members {
		userIDs[i] = m.UserID
	}
	users, _ := r.data.Ent(ctx).User.Query().Where(user.IDIn(userIDs...)).All(ctx)
	userMap := make(map[uuid.UUID]*ent.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	result := make([]*entity.TenantMember, 0, len(members))
	for _, m := range members {
		em := &entity.TenantMember{
			ID:        m.ID.String(),
			TenantID:  m.TenantID.String(),
			UserID:    m.UserID.String(),
			Role:      string(m.Role),
			Status:    string(m.Status),
			JoinedAt:  m.JoinedAt,
			CreatedAt: m.CreatedAt,
		}
		if u, ok := userMap[m.UserID]; ok {
			em.UserName = u.Name
			em.UserEmail = u.Email
		}
		result = append(result, em)
	}
	return result, int64(total), nil
}

func (r *tenantRepo) UpdateMemberRole(ctx context.Context, tenantID, userID, role string) (*entity.TenantMember, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	affected, err := r.data.Ent(ctx).TenantMember.Update().
		Where(
			tenantmember.TenantIDEQ(tid),
			tenantmember.UserIDEQ(uid),
		).
		SetRole(tenantmember.Role(role)).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update member role: %w", err)
	}
	if affected == 0 {
		return nil, fmt.Errorf("member not found")
	}
	return r.GetMember(ctx, tenantID, userID)
}

func (r *tenantRepo) UpdateMemberStatus(ctx context.Context, tenantID, userID, status string) (*entity.TenantMember, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	b := r.data.Ent(ctx).TenantMember.Update().
		Where(
			tenantmember.TenantIDEQ(tid),
			tenantmember.UserIDEQ(uid),
		).
		SetStatus(tenantmember.Status(status))
	if status == "active" {
		now := time.Now()
		b.SetJoinedAt(now)
	}
	affected, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update member status: %w", err)
	}
	if affected == 0 {
		return nil, fmt.Errorf("member not found")
	}
	return r.GetMember(ctx, tenantID, userID)
}

func (r *tenantRepo) ListMembershipsByUserID(ctx context.Context, userID string) ([]*entity.TenantMember, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	members, err := r.data.Ent(ctx).TenantMember.Query().
		Where(tenantmember.UserIDEQ(uid)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*entity.TenantMember, len(members))
	for i, m := range members {
		result[i] = &entity.TenantMember{
			ID:        m.ID.String(),
			TenantID:  m.TenantID.String(),
			UserID:    m.UserID.String(),
			Role:      string(m.Role),
			Status:    string(m.Status),
			JoinedAt:  m.JoinedAt,
			CreatedAt: m.CreatedAt,
		}
	}
	return result, nil
}

func (r *tenantRepo) GetPersonalTenantByUserID(ctx context.Context, userID string) (*entity.Tenant, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	memberEntries, err := r.data.Ent(ctx).TenantMember.Query().
		Where(tenantmember.UserIDEQ(uid)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	for _, m := range memberEntries {
		t, err := r.data.Ent(ctx).Tenant.Query().
			Where(tenant.IDEQ(m.TenantID)).
			Where(tenant.KindEQ(tenant.KindPersonal)).
			Where(tenant.DeletedAtIsNil()).
			Only(ctx)
		if err != nil {
			continue
		}
		return tenantMapper.Map(t), nil
	}

	return nil, &ent.NotFoundError{}
}

func (r *tenantRepo) enrichMember(ctx context.Context, m *ent.TenantMember) (*entity.TenantMember, error) {
	em := &entity.TenantMember{
		ID:        m.ID.String(),
		TenantID:  m.TenantID.String(),
		UserID:    m.UserID.String(),
		Role:      string(m.Role),
		Status:    string(m.Status),
		JoinedAt:  m.JoinedAt,
		CreatedAt: m.CreatedAt,
	}
	u, err := r.data.Ent(ctx).User.Query().Where(user.IDEQ(m.UserID)).Only(ctx)
	if err == nil {
		em.UserName = u.Name
		em.UserEmail = u.Email
	}
	return em, nil
}
