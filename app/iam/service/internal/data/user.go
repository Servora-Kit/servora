package data

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/organization"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/organizationmember"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/tenant"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/user"
	"github.com/Servora-Kit/servora/pkg/helpers"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type userRepo struct {
	data *Data
	log  *logger.Helper
}

func NewUserRepo(data *Data, l logger.Logger) biz.UserRepo {
	return &userRepo{
		data: data,
		log:  logger.NewHelper(l, logger.WithModule("user/data/iam-service")),
	}
}

func (r *userRepo) SaveUser(ctx context.Context, u *entity.User) (*entity.User, error) {
	if !helpers.BcryptIsHashed(u.Password) {
		bcryptPassword, err := helpers.BcryptHash(u.Password)
		if err != nil {
			return nil, err
		}
		u.Password = bcryptPassword
	}
	b := r.data.Ent(ctx).User.Create().
		SetName(u.Name).
		SetEmail(u.Email).
		SetPassword(u.Password).
		SetRole(u.Role).
		SetEmailVerified(u.EmailVerified)

	if u.EmailVerifiedAt != nil {
		b.SetEmailVerifiedAt(*u.EmailVerifiedAt)
	}

	if u.ID != "" {
		uid, err := uuid.Parse(u.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID: %w", err)
		}
		b.SetID(uid)
	}

	created, err := b.Save(ctx)
	if err != nil {
		r.log.Errorf("SaveUser failed: %v", err)
		return nil, err
	}
	return userMapper.Map(created), nil
}

func (r *userRepo) GetUserById(ctx context.Context, id string) (*entity.User, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	entUser, err := r.data.Ent(ctx).User.Query().Where(user.IDEQ(uid)).Where(user.DeletedAtIsNil()).Only(ctx)
	if err != nil {
		return nil, err
	}
	return userMapper.Map(entUser), nil
}

func (r *userRepo) DeleteUser(ctx context.Context, u *entity.User) (*entity.User, error) {
	uid, err := uuid.Parse(u.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	err = r.data.Ent(ctx).User.UpdateOneID(uid).
		SetDeletedAt(time.Now()).
		Exec(ctx)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *userRepo) PurgeUser(ctx context.Context, u *entity.User) (*entity.User, error) {
	uid, err := uuid.Parse(u.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	err = r.data.Ent(ctx).User.DeleteOneID(uid).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *userRepo) PurgeCascade(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	return r.data.RunInEntTx(ctx, func(txCtx context.Context) error {
		c := r.data.Ent(txCtx)

		// 删除用户在所有组织的成员关系（ON DELETE CASCADE 会清理 org_memberships 行，
		// 但为了与 FGA 保持同步，这里显式删除）
		if _, err := c.OrganizationMember.Delete().
			Where(organizationmember.UserIDEQ(uid)).
			Exec(txCtx); err != nil {
			return err
		}

		// 检查并更新用户 owned_tenants：若用户是某个租户的 owner，设置 owner_user_id 为 nil 会违反 NOT NULL 约束
		// 因此在 PurgeCascade 中直接删除这些租户（级联删除）
		// 通常调用方应提前转移所有权再调用 Purge；此处做兜底处理。
		ownedTenantIDs, _ := c.Tenant.Query().
			Where(tenant.OwnerUserIDEQ(uid)).
			IDs(txCtx)
		for _, tid := range ownedTenantIDs {
			orgIDs, _ := c.Organization.Query().
				Where(organization.TenantIDEQ(tid)).
				IDs(txCtx)
			if len(orgIDs) > 0 {
				_, _ = c.OrganizationMember.Delete().
					Where(organizationmember.OrganizationIDIn(orgIDs...)).
					Exec(txCtx)
				_, _ = c.Organization.Delete().
					Where(organization.IDIn(orgIDs...)).
					Exec(txCtx)
			}
			_ = c.Tenant.DeleteOneID(tid).Exec(txCtx)
		}

		return c.User.DeleteOneID(uid).Exec(txCtx)
	})
}

func (r *userRepo) RestoreUser(ctx context.Context, id string) (*entity.User, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	u, err := r.data.Ent(ctx).User.UpdateOneID(uid).
		ClearDeletedAt().
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return userMapper.Map(u), nil
}

func (r *userRepo) GetUserByIdIncludingDeleted(ctx context.Context, id string) (*entity.User, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	entUser, err := r.data.Ent(ctx).User.Query().Where(user.IDEQ(uid)).Only(ctx)
	if err != nil {
		return nil, err
	}
	return userMapper.Map(entUser), nil
}

func (r *userRepo) UpdateUser(ctx context.Context, u *entity.User) (*entity.User, error) {
	uid, err := uuid.Parse(u.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	if !helpers.BcryptIsHashed(u.Password) {
		bcryptPassword, err := helpers.BcryptHash(u.Password)
		if err != nil {
			return nil, err
		}
		u.Password = bcryptPassword
	}
	updated, err := r.data.Ent(ctx).User.UpdateOneID(uid).
		SetName(u.Name).
		SetEmail(u.Email).
		SetPassword(u.Password).
		SetRole(u.Role).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return userMapper.Map(updated), nil
}

func (r *userRepo) ListUsers(ctx context.Context, page int32, pageSize int32) ([]*entity.User, int64, error) {
	offset := int((page - 1) * pageSize)
	limit := int(pageSize)

	query := r.data.Ent(ctx).User.Query().Where(user.DeletedAtIsNil()).Order(user.ByID(sql.OrderDesc()))
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	entUsers, err := query.Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return userMapper.MapSlice(entUsers), int64(total), nil
}

// ListByTenantID returns distinct users who belong to any organization under the given tenant.
func (r *userRepo) ListByTenantID(ctx context.Context, tenantID string, page int32, pageSize int32) ([]*entity.User, int64, error) {
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid tenant ID: %w", err)
	}

	offset := int((page - 1) * pageSize)
	limit := int(pageSize)

	// Derive users from organization_members JOIN organizations WHERE tenant_id = ?
	query := r.data.Ent(ctx).User.Query().
		Where(
			user.DeletedAtIsNil(),
			user.HasOrgMembershipsWith(
				organizationmember.HasOrganizationWith(
					organization.TenantIDEQ(tid),
					organization.DeletedAtIsNil(),
				),
			),
		).
		Order(user.ByID(sql.OrderDesc()))

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	entUsers, err := query.Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return userMapper.MapSlice(entUsers), int64(total), nil
}
