package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
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
		log:  logger.For(l, "user/data/iam"),
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

	profileJSON := profileToJSON(u.Profile)
	b := r.data.Ent(ctx).User.Create().
		SetUsername(u.Username).
		SetEmail(u.Email).
		SetPassword(u.Password).
		SetPhone(u.Phone).
		SetPhoneVerified(u.PhoneVerified).
		SetRole(u.Role).
		SetEmailVerified(u.EmailVerified).
		SetProfile(profileJSON)

	if u.Status != "" {
		b.SetStatus(u.Status)
	}
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
	entUser, err := r.data.Ent(ctx).User.Query().Where(user.IDEQ(uid), user.DeletedAtIsNil()).Only(ctx)
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
	err = r.data.Ent(ctx).User.UpdateOneID(uid).SetDeletedAt(time.Now()).Exec(ctx)
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
	if err := r.data.Ent(ctx).User.DeleteOneID(uid).Exec(ctx); err != nil {
		return nil, err
	}
	return u, nil
}

func (r *userRepo) PurgeCascade(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	return r.data.Ent(ctx).User.DeleteOneID(uid).Exec(ctx)
}

func (r *userRepo) RestoreUser(ctx context.Context, id string) (*entity.User, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	u, err := r.data.Ent(ctx).User.UpdateOneID(uid).ClearDeletedAt().Save(ctx)
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
	if u.Password != "" && !helpers.BcryptIsHashed(u.Password) {
		hashed, err := helpers.BcryptHash(u.Password)
		if err != nil {
			return nil, err
		}
		u.Password = hashed
	}

	profileJSON := profileToJSON(u.Profile)
	upd := r.data.Ent(ctx).User.UpdateOneID(uid).
		SetProfile(profileJSON)

	if u.Username != "" {
		upd.SetUsername(u.Username)
	}
	if u.Email != "" {
		upd.SetEmail(u.Email)
	}
	if u.Password != "" {
		upd.SetPassword(u.Password)
	}
	if u.Phone != "" {
		upd.SetPhone(u.Phone)
	}
	if u.Role != "" {
		upd.SetRole(u.Role)
	}
	if u.Status != "" {
		upd.SetStatus(u.Status)
	}

	updated, err := upd.Save(ctx)
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

// profileToJSON serializes UserProfile into a map[string]interface{} for Ent JSON storage.
func profileToJSON(p entity.UserProfile) map[string]interface{} {
	b, _ := json.Marshal(p)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m
}
