package data

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/user"
	"github.com/Servora-Kit/servora/pkg/helpers"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type authnRepo struct {
	data *Data
	log  *logger.Helper
}

func NewAuthnRepo(data *Data, l logger.Logger) biz.AuthnRepo {
	return &authnRepo{
		data: data,
		log:  logger.NewHelper(l, logger.WithModule("authn/data/iam-service")),
	}
}

func (r *authnRepo) SaveUser(ctx context.Context, u *entity.User) (*entity.User, error) {
	if !helpers.BcryptIsHashed(u.Password) {
		bcryptPassword, err := helpers.BcryptHash(u.Password)
		if err != nil {
			return nil, err
		}
		u.Password = bcryptPassword
	}
	created, err := r.data.Ent(ctx).User.
		Create().
		SetName(u.Name).
		SetEmail(u.Email).
		SetPassword(u.Password).
		SetRole(u.Role).
		Save(ctx)
	if err != nil {
		r.log.Errorf("SaveUser failed: %v", err)
		return nil, err
	}
	return userMapper.Map(created), nil
}

func (r *authnRepo) GetUserByUserName(ctx context.Context, name string) (*entity.User, error) {
	entUser, err := r.data.Ent(ctx).User.Query().Where(user.NameEQ(name)).Only(ctx)
	if err != nil {
		return nil, err
	}
	return userMapper.Map(entUser), nil
}

func (r *authnRepo) GetUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	entUser, err := r.data.Ent(ctx).User.Query().Where(user.EmailEQ(email)).Only(ctx)
	if err != nil {
		return nil, err
	}
	return userMapper.Map(entUser), nil
}

func (r *authnRepo) GetUserByID(ctx context.Context, id string) (*entity.User, error) {
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

func (r *authnRepo) UpdatePassword(ctx context.Context, userID string, hashedPassword string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	_, err = r.data.Ent(ctx).User.UpdateOneID(uid).SetPassword(hashedPassword).Save(ctx)
	return err
}

func (r *authnRepo) SaveRefreshToken(ctx context.Context, userID string, token string, expiration time.Duration) error {
	tokenKey := fmt.Sprintf("refresh_token:%s", token)
	if err := r.data.redis.Set(ctx, tokenKey, userID, expiration); err != nil {
		r.log.Errorf("Failed to save refresh token: %v", err)
		return err
	}

	userTokensKey := fmt.Sprintf("user_tokens:%s", userID)
	if err := r.data.redis.SAdd(ctx, userTokensKey, token); err != nil {
		r.log.Errorf("Failed to add token to user set: %v", err)
		return err
	}

	if err := r.data.redis.Expire(ctx, userTokensKey, expiration); err != nil {
		r.log.Errorf("Failed to set expiration for user tokens set: %v", err)
		return err
	}

	return nil
}

func (r *authnRepo) GetRefreshToken(ctx context.Context, token string) (string, error) {
	tokenKey := fmt.Sprintf("refresh_token:%s", token)
	userID, err := r.data.redis.Get(ctx, tokenKey)
	if err != nil {
		r.log.Errorf("Failed to get refresh token: %v", err)
		return "", err
	}
	return userID, nil
}

func (r *authnRepo) DeleteRefreshToken(ctx context.Context, token string) error {
	userID, err := r.GetRefreshToken(ctx, token)
	if err != nil {
		r.log.Warnf("Token not found during deletion: %v", err)
		return nil
	}

	tokenKey := fmt.Sprintf("refresh_token:%s", token)
	if err := r.data.redis.Del(ctx, tokenKey); err != nil {
		r.log.Errorf("Failed to delete refresh token: %v", err)
		return err
	}

	userTokensKey := fmt.Sprintf("user_tokens:%s", userID)
	tokens, err := r.data.redis.SMembers(ctx, userTokensKey)
	if err != nil {
		r.log.Errorf("Failed to get user tokens: %v", err)
		return err
	}

	if err := r.data.redis.Del(ctx, userTokensKey); err != nil {
		r.log.Errorf("Failed to delete user tokens set: %v", err)
		return err
	}

	for _, t := range tokens {
		if t != token {
			if err := r.data.redis.SAdd(ctx, userTokensKey, t); err != nil {
				r.log.Errorf("Failed to re-add token to user set: %v", err)
				return err
			}
		}
	}

	return nil
}

func (r *authnRepo) DeleteUserRefreshTokens(ctx context.Context, userID string) error {
	userTokensKey := fmt.Sprintf("user_tokens:%s", userID)

	tokens, err := r.data.redis.SMembers(ctx, userTokensKey)
	if err != nil {
		r.log.Errorf("Failed to get user tokens: %v", err)
		return err
	}

	for _, token := range tokens {
		tokenKey := fmt.Sprintf("refresh_token:%s", token)
		if err := r.data.redis.Del(ctx, tokenKey); err != nil {
			r.log.Errorf("Failed to delete token %s: %v", token, err)
			return err
		}
	}

	if err := r.data.redis.Del(ctx, userTokensKey); err != nil {
		r.log.Errorf("Failed to delete user tokens set: %v", err)
		return err
	}

	return nil
}
