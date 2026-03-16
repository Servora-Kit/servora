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
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/organization"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/organizationmember"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/user"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type organizationRepo struct {
	data *Data
	log  *logger.Helper
}

func NewOrganizationRepo(data *Data, l logger.Logger) biz.OrganizationRepo {
	return &organizationRepo{
		data: data,
		log:  logger.NewHelper(l, logger.WithModule("organization/data/iam-service")),
	}
}

func (r *organizationRepo) Create(ctx context.Context, org *entity.Organization) (*entity.Organization, error) {
	tenantID, err := uuid.Parse(org.TenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}
	b := r.data.Ent(ctx).Organization.Create().
		SetTenantID(tenantID).
		SetName(org.Name).
		SetSlug(org.Slug)
	if org.DisplayName != "" {
		b.SetDisplayName(org.DisplayName)
	}
	created, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}
	return orgMapper.Map(created), nil
}

func (r *organizationRepo) GetByID(ctx context.Context, id string) (*entity.Organization, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid organization ID: %w", err)
	}
	org, err := r.data.Ent(ctx).Organization.Query().
		Where(organization.IDEQ(uid)).
		Where(organization.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return orgMapper.Map(org), nil
}

func (r *organizationRepo) GetBySlug(ctx context.Context, slug string) (*entity.Organization, error) {
	org, err := r.data.Ent(ctx).Organization.Query().
		Where(organization.SlugEQ(slug)).
		Where(organization.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return orgMapper.Map(org), nil
}

func (r *organizationRepo) GetByIDs(ctx context.Context, ids []string, page, pageSize int32) ([]*entity.Organization, int64, error) {
	uuids := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if uid, e := uuid.Parse(id); e == nil {
			uuids = append(uuids, uid)
		}
	}

	query := r.data.Ent(ctx).Organization.Query().
		Where(organization.IDIn(uuids...)).
		Where(organization.DeletedAtIsNil()).
		Order(organization.ByCreatedAt(sql.OrderDesc()))

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := int((page - 1) * pageSize)
	orgs, err := query.Offset(offset).Limit(int(pageSize)).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return orgMapper.MapSlice(orgs), int64(total), nil
}

func (r *organizationRepo) ListByUserID(ctx context.Context, userID string, page, pageSize int32) ([]*entity.Organization, int64, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid user ID: %w", err)
	}

	memberOrgIDs, err := r.data.Ent(ctx).OrganizationMember.Query().
		Where(organizationmember.UserIDEQ(uid)).
		Select(organizationmember.FieldOrganizationID).
		Strings(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list member orgs: %w", err)
	}

	orgUUIDs := make([]uuid.UUID, 0, len(memberOrgIDs))
	for _, idStr := range memberOrgIDs {
		if oid, e := uuid.Parse(idStr); e == nil {
			orgUUIDs = append(orgUUIDs, oid)
		}
	}

	query := r.data.Ent(ctx).Organization.Query().
		Where(organization.IDIn(orgUUIDs...)).
		Where(organization.DeletedAtIsNil()).
		Order(organization.ByCreatedAt(sql.OrderDesc()))

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := int((page - 1) * pageSize)
	orgs, err := query.Offset(offset).Limit(int(pageSize)).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return orgMapper.MapSlice(orgs), int64(total), nil
}

func (r *organizationRepo) Update(ctx context.Context, org *entity.Organization) (*entity.Organization, error) {
	uid, err := uuid.Parse(org.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid organization ID: %w", err)
	}
	b := r.data.Ent(ctx).Organization.UpdateOneID(uid)
	if org.Name != "" {
		b.SetName(org.Name)
	}
	if org.DisplayName != "" {
		b.SetDisplayName(org.DisplayName)
	}
	updated, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update organization: %w", err)
	}
	return orgMapper.Map(updated), nil
}

func (r *organizationRepo) Delete(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid organization ID: %w", err)
	}
	return r.data.Ent(ctx).Organization.UpdateOneID(uid).
		SetDeletedAt(time.Now()).
		Exec(ctx)
}

func (r *organizationRepo) Purge(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid organization ID: %w", err)
	}
	return r.data.Ent(ctx).Organization.DeleteOneID(uid).Exec(ctx)
}

func (r *organizationRepo) PurgeCascade(ctx context.Context, id string) error {
	oid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid organization ID: %w", err)
	}
	return r.data.RunInEntTx(ctx, func(txCtx context.Context) error {
		return purgeOrganizationInTx(txCtx, r.data.Ent(txCtx), oid)
	})
}

func (r *organizationRepo) Restore(ctx context.Context, id string) (*entity.Organization, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid organization ID: %w", err)
	}
	org, err := r.data.Ent(ctx).Organization.UpdateOneID(uid).
		ClearDeletedAt().
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return orgMapper.Map(org), nil
}

func (r *organizationRepo) GetByIDIncludingDeleted(ctx context.Context, id string) (*entity.Organization, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid organization ID: %w", err)
	}
	org, err := r.data.Ent(ctx).Organization.Query().
		Where(organization.IDEQ(uid)).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return orgMapper.Map(org), nil
}

func (r *organizationRepo) AddMember(ctx context.Context, m *entity.OrganizationMember) (*entity.OrganizationMember, error) {
	orgID, err := uuid.Parse(m.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("invalid organization ID: %w", err)
	}
	userID, err := uuid.Parse(m.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	created, err := r.data.Ent(ctx).OrganizationMember.Create().
		SetOrganizationID(orgID).
		SetUserID(userID).
		SetRole(m.Role).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("add organization member: %w", err)
	}
	return r.enrichMember(ctx, created)
}

func (r *organizationRepo) RemoveMember(ctx context.Context, orgID, userID string) error {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return fmt.Errorf("invalid organization ID: %w", err)
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	_, err = r.data.Ent(ctx).OrganizationMember.Delete().
		Where(
			organizationmember.OrganizationIDEQ(oid),
			organizationmember.UserIDEQ(uid),
		).Exec(ctx)
	return err
}

func (r *organizationRepo) ListMembers(ctx context.Context, orgID string, page, pageSize int32) ([]*entity.OrganizationMember, int64, error) {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid organization ID: %w", err)
	}
	query := r.data.Ent(ctx).OrganizationMember.Query().
		Where(organizationmember.OrganizationIDEQ(oid)).
		Order(organizationmember.ByCreatedAt(sql.OrderDesc()))

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	offset := int((page - 1) * pageSize)
	members, err := query.Offset(offset).Limit(int(pageSize)).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	// Batch user lookup (avoids N+1)
	userIDs := make([]uuid.UUID, len(members))
	for i, m := range members {
		userIDs[i] = m.UserID
	}
	users, _ := r.data.Ent(ctx).User.Query().Where(user.IDIn(userIDs...)).All(ctx)
	userMap := make(map[uuid.UUID]*ent.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	result := make([]*entity.OrganizationMember, 0, len(members))
	for _, m := range members {
		em := &entity.OrganizationMember{
			ID:             m.ID.String(),
			OrganizationID: m.OrganizationID.String(),
			UserID:         m.UserID.String(),
			Role:           m.Role,
			CreatedAt:      m.CreatedAt,
		}
		if u, ok := userMap[m.UserID]; ok {
			em.UserName = u.Name
			em.UserEmail = u.Email
		}
		result = append(result, em)
	}
	return result, int64(total), nil
}

func (r *organizationRepo) GetMember(ctx context.Context, orgID, userID string) (*entity.OrganizationMember, error) {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("invalid organization ID: %w", err)
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	m, err := r.data.Ent(ctx).OrganizationMember.Query().
		Where(
			organizationmember.OrganizationIDEQ(oid),
			organizationmember.UserIDEQ(uid),
		).Only(ctx)
	if err != nil {
		return nil, err
	}
	return r.enrichMember(ctx, m)
}

func (r *organizationRepo) UpdateMemberRole(ctx context.Context, orgID, userID, role string) (*entity.OrganizationMember, error) {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("invalid organization ID: %w", err)
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	affected, err := r.data.Ent(ctx).OrganizationMember.Update().
		Where(
			organizationmember.OrganizationIDEQ(oid),
			organizationmember.UserIDEQ(uid),
		).
		SetRole(role).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update member role: %w", err)
	}
	if affected == 0 {
		return nil, fmt.Errorf("member not found")
	}
	return r.GetMember(ctx, orgID, userID)
}

func (r *organizationRepo) ListAllMembers(ctx context.Context, orgID string) ([]*entity.OrganizationMember, error) {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("invalid organization ID: %w", err)
	}
	members, err := r.data.Ent(ctx).OrganizationMember.Query().
		Where(organizationmember.OrganizationIDEQ(oid)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*entity.OrganizationMember, len(members))
	for i, m := range members {
		result[i] = &entity.OrganizationMember{
			ID:             m.ID.String(),
			OrganizationID: m.OrganizationID.String(),
			UserID:         m.UserID.String(),
			Role:           m.Role,
			CreatedAt:      m.CreatedAt,
		}
	}
	return result, nil
}

func (r *organizationRepo) DeleteAllMembers(ctx context.Context, orgID string) (int, error) {
	oid, err := uuid.Parse(orgID)
	if err != nil {
		return 0, fmt.Errorf("invalid organization ID: %w", err)
	}
	return r.data.Ent(ctx).OrganizationMember.Delete().
		Where(organizationmember.OrganizationIDEQ(oid)).Exec(ctx)
}

func (r *organizationRepo) ListMembershipsByUserID(ctx context.Context, userID string) ([]*entity.OrganizationMember, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	members, err := r.data.Ent(ctx).OrganizationMember.Query().
		Where(organizationmember.UserIDEQ(uid)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*entity.OrganizationMember, len(members))
	for i, m := range members {
		result[i] = &entity.OrganizationMember{
			ID:             m.ID.String(),
			OrganizationID: m.OrganizationID.String(),
			UserID:         m.UserID.String(),
			Role:           m.Role,
			CreatedAt:      m.CreatedAt,
		}
	}
	return result, nil
}

func (r *organizationRepo) DeleteMembershipsByUserID(ctx context.Context, userID string) (int, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID: %w", err)
	}
	return r.data.Ent(ctx).OrganizationMember.Delete().
		Where(organizationmember.UserIDEQ(uid)).Exec(ctx)
}

func (r *organizationRepo) enrichMember(ctx context.Context, m *ent.OrganizationMember) (*entity.OrganizationMember, error) {
	u, err := r.data.Ent(ctx).User.Query().Where(user.IDEQ(m.UserID)).Only(ctx)
	if err != nil {
		return &entity.OrganizationMember{
			ID:             m.ID.String(),
			OrganizationID: m.OrganizationID.String(),
			UserID:         m.UserID.String(),
			Role:           m.Role,
			CreatedAt:      m.CreatedAt,
		}, nil
	}
	return &entity.OrganizationMember{
		ID:             m.ID.String(),
		OrganizationID: m.OrganizationID.String(),
		UserID:         m.UserID.String(),
		UserName:       u.Name,
		UserEmail:      u.Email,
		Role:           m.Role,
		CreatedAt:      m.CreatedAt,
	}, nil
}
