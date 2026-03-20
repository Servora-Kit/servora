package data

import (
	"context"
	"fmt"
	"time"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/redis"
)

type otpRepo struct {
	redis *redis.Client
	log   *logger.Helper
}

func NewOTPRepo(redisClient *redis.Client, l logger.Logger) biz.OTPRepo {
	return &otpRepo{
		redis: redisClient,
		log:   logger.For(l, "otp/data/iam"),
	}
}

func otpKey(purpose, hashedToken string) string {
	return fmt.Sprintf("iam:%s:%s", purpose, hashedToken)
}

func (r *otpRepo) SetToken(ctx context.Context, purpose, hashedToken, userID string, ttl time.Duration) error {
	return r.redis.Set(ctx, otpKey(purpose, hashedToken), userID, ttl)
}

func (r *otpRepo) ConsumeToken(ctx context.Context, purpose, hashedToken string) (string, error) {
	key := otpKey(purpose, hashedToken)
	userID, err := r.redis.GetDel(ctx, key)
	if err != nil {
		return "", fmt.Errorf("token not found or expired: %w", err)
	}
	return userID, nil
}
