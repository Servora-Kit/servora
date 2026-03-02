package data

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/horonlee/servora/app/servora/service/internal/biz"
	"github.com/horonlee/servora/app/servora/service/internal/biz/entity"
	"github.com/horonlee/servora/app/servora/service/internal/data/ent"
	"github.com/horonlee/servora/app/servora/service/internal/data/ent/user"
	"github.com/horonlee/servora/pkg/helpers/hash"
	"github.com/horonlee/servora/pkg/logger"
	"github.com/horonlee/servora/pkg/mapper"
)

type authRepo struct {
	data   *Data
	log    *logger.Helper
	mapper *mapper.CopierMapper[entity.User, ent.User]
}

func NewAuthRepo(data *Data, l logger.Logger) biz.AuthRepo {
	return &authRepo{
		data:   data,
		log:    logger.NewHelper(l, logger.WithModule("auth/data/servora-service")),
		mapper: mapper.New[entity.User, ent.User]().RegisterConverters(mapper.AllBuiltinConverters()),
	}
}

// 数据库操作方法

func (r *authRepo) SaveUser(ctx context.Context, user *entity.User) (*entity.User, error) {
	if !hash.BcryptIsHashed(user.Password) {
		bcryptPassword, err := hash.BcryptHash(user.Password)
		if err != nil {
			return nil, err
		}
		user.Password = bcryptPassword
	}
	entUser := r.mapper.ToEntity(user)
	created, err := r.data.entClient.User.
		Create().
		SetName(entUser.Name).
		SetEmail(entUser.Email).
		SetPassword(entUser.Password).
		SetRole(entUser.Role).
		Save(ctx)
	if err != nil {
		r.log.Errorf("SaveUser failed: %v", err)
		return nil, err
	}
	return r.mapper.ToDomain(created), nil
}

func (r *authRepo) GetUserByUserName(ctx context.Context, name string) (*entity.User, error) {
	entUser, err := r.data.entClient.User.Query().Where(user.NameEQ(name)).Only(ctx)
	if err != nil {
		return nil, err
	}
	return r.mapper.ToDomain(entUser), nil
}

func (r *authRepo) GetUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	entUser, err := r.data.entClient.User.Query().Where(user.EmailEQ(email)).Only(ctx)
	if err != nil {
		return nil, err
	}
	return r.mapper.ToDomain(entUser), nil
}

func (r *authRepo) GetUserByID(ctx context.Context, id int64) (*entity.User, error) {
	entUser, err := r.data.entClient.User.Query().Where(user.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, err
	}
	return r.mapper.ToDomain(entUser), nil
}

// TokenStore methods implementation

// SaveRefreshToken 保存Refresh Token到Redis
func (r *authRepo) SaveRefreshToken(ctx context.Context, userID int64, token string, expiration time.Duration) error {
	// 存储refresh token -> user_id的映射
	tokenKey := fmt.Sprintf("refresh_token:%s", token)
	if err := r.data.redis.Set(ctx, tokenKey, strconv.FormatInt(userID, 10), expiration); err != nil {
		r.log.Errorf("Failed to save refresh token: %v", err)
		return err
	}

	// 将token添加到用户的token集合中，用于批量删除
	userTokensKey := fmt.Sprintf("user_tokens:%d", userID)
	if err := r.data.redis.SAdd(ctx, userTokensKey, token); err != nil {
		r.log.Errorf("Failed to add token to user set: %v", err)
		return err
	}

	// 为用户token集合设置过期时间
	if err := r.data.redis.Expire(ctx, userTokensKey, expiration); err != nil {
		r.log.Errorf("Failed to set expiration for user tokens set: %v", err)
		return err
	}

	return nil
}

// GetRefreshToken 获取Refresh Token关联的用户ID
func (r *authRepo) GetRefreshToken(ctx context.Context, token string) (int64, error) {
	tokenKey := fmt.Sprintf("refresh_token:%s", token)
	userIDStr, err := r.data.redis.Get(ctx, tokenKey)
	if err != nil {
		r.log.Errorf("Failed to get refresh token: %v", err)
		return 0, err
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		r.log.Errorf("Failed to parse user ID: %v", err)
		return 0, err
	}

	return userID, nil
}

// DeleteRefreshToken 删除Refresh Token
func (r *authRepo) DeleteRefreshToken(ctx context.Context, token string) error {
	// 首先获取用户ID，以便从用户token集合中删除
	userID, err := r.GetRefreshToken(ctx, token)
	if err != nil {
		// 如果token不存在，也认为删除成功
		r.log.Warnf("Token not found during deletion: %v", err)
		return nil
	}

	// 删除token -> user_id的映射
	tokenKey := fmt.Sprintf("refresh_token:%s", token)
	if err := r.data.redis.Del(ctx, tokenKey); err != nil {
		r.log.Errorf("Failed to delete refresh token: %v", err)
		return err
	}

	// 从用户token集合中删除该token
	userTokensKey := fmt.Sprintf("user_tokens:%d", userID)
	// 获取集合中的所有token
	tokens, err := r.data.redis.SMembers(ctx, userTokensKey)
	if err != nil {
		r.log.Errorf("Failed to get user tokens: %v", err)
		return err
	}

	// 重新创建集合，排除要删除的token
	if err := r.data.redis.Del(ctx, userTokensKey); err != nil {
		r.log.Errorf("Failed to delete user tokens set: %v", err)
		return err
	}

	// 重新添加除了要删除的token之外的所有token
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

// DeleteUserRefreshTokens 删除用户所有Refresh Token
func (r *authRepo) DeleteUserRefreshTokens(ctx context.Context, userID int64) error {
	userTokensKey := fmt.Sprintf("user_tokens:%d", userID)

	// 获取用户的所有token
	tokens, err := r.data.redis.SMembers(ctx, userTokensKey)
	if err != nil {
		r.log.Errorf("Failed to get user tokens: %v", err)
		return err
	}

	// 删除每个token的映射
	for _, token := range tokens {
		tokenKey := fmt.Sprintf("refresh_token:%s", token)
		if err := r.data.redis.Del(ctx, tokenKey); err != nil {
			r.log.Errorf("Failed to delete token %s: %v", token, err)
			return err
		}
	}

	// 删除用户token集合
	if err := r.data.redis.Del(ctx, userTokensKey); err != nil {
		r.log.Errorf("Failed to delete user tokens set: %v", err)
		return err
	}

	return nil
}
