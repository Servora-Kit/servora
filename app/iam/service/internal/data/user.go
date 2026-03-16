package data

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/organizationmember"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/projectmember"
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
		SetRole(u.Role)

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

		ownedMembers, err := c.OrganizationMember.Query().
			Where(
				organizationmember.UserIDEQ(uid),
				organizationmember.RoleEQ("owner"),
			).
			All(txCtx)
		if err != nil {
			return fmt.Errorf("query owned organizations: %w", err)
		}

		for _, m := range ownedMembers {
			if err := purgeOrganizationInTx(txCtx, c, m.OrganizationID); err != nil {
				return fmt.Errorf("purge owned organization %s: %w", m.OrganizationID, err)
			}
		}

		if _, err := c.OrganizationMember.Delete().
			Where(organizationmember.UserIDEQ(uid)).
			Exec(txCtx); err != nil {
			return err
		}
		if _, err := c.ProjectMember.Delete().
			Where(projectmember.UserIDEQ(uid)).
			Exec(txCtx); err != nil {
			return err
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
