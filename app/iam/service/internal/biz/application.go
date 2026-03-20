package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	apppb "github.com/Servora-Kit/servora/api/gen/go/application/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/pkg/helpers"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type ApplicationRepo interface {
	Create(ctx context.Context, app *entity.Application) (*entity.Application, error)
	GetByID(ctx context.Context, id string) (*entity.Application, error)
	GetByClientID(ctx context.Context, clientID string) (*entity.Application, error)
	List(ctx context.Context, page, pageSize int32) ([]*entity.Application, int64, error)
	Update(ctx context.Context, app *entity.Application) (*entity.Application, error)
	Delete(ctx context.Context, id string) error
	UpdateClientSecretHash(ctx context.Context, id string, hash string) error
}

type ApplicationUsecase struct {
	repo ApplicationRepo
	log  *logger.Helper
}

func NewApplicationUsecase(repo ApplicationRepo, l logger.Logger) *ApplicationUsecase {
	return &ApplicationUsecase{
		repo: repo,
		log:  logger.For(l, "application/biz/iam"),
	}
}

func (uc *ApplicationUsecase) Create(ctx context.Context, app *entity.Application) (*entity.Application, string, error) {
	clientID, err := generateRandomHex(16)
	if err != nil {
		uc.log.Errorf("generate client_id: %v", err)
		return nil, "", apppb.ErrorApplicationCreateFailed("failed to create application")
	}
	plainSecret, err := generateRandomHex(32)
	if err != nil {
		uc.log.Errorf("generate client_secret: %v", err)
		return nil, "", apppb.ErrorApplicationCreateFailed("failed to create application")
	}

	hash, err := helpers.BcryptHash(plainSecret)
	if err != nil {
		uc.log.Errorf("hash client_secret: %v", err)
		return nil, "", apppb.ErrorApplicationCreateFailed("failed to create application")
	}

	app.ClientID = clientID
	app.ClientSecretHash = hash

	created, err := uc.repo.Create(ctx, app)
	if err != nil {
		uc.log.Errorf("create application failed: %v", err)
		return nil, "", apppb.ErrorApplicationCreateFailed("%v", err)
	}
	return created, plainSecret, nil
}

func (uc *ApplicationUsecase) Get(ctx context.Context, id string) (*entity.Application, error) {
	return uc.repo.GetByID(ctx, id)
}

func (uc *ApplicationUsecase) GetByClientID(ctx context.Context, clientID string) (*entity.Application, error) {
	return uc.repo.GetByClientID(ctx, clientID)
}

func (uc *ApplicationUsecase) List(ctx context.Context, page, pageSize int32) ([]*entity.Application, int64, error) {
	return uc.repo.List(ctx, page, pageSize)
}

func (uc *ApplicationUsecase) Update(ctx context.Context, app *entity.Application) (*entity.Application, error) {
	return uc.repo.Update(ctx, app)
}

func (uc *ApplicationUsecase) Delete(ctx context.Context, id string) error {
	return uc.repo.Delete(ctx, id)
}

func (uc *ApplicationUsecase) RegenerateClientSecret(ctx context.Context, id string) (string, error) {
	plainSecret, err := generateRandomHex(32)
	if err != nil {
		uc.log.Errorf("generate client_secret: %v", err)
		return "", apppb.ErrorApplicationCreateFailed("failed to regenerate client secret")
	}

	hash, err := helpers.BcryptHash(plainSecret)
	if err != nil {
		uc.log.Errorf("hash client_secret: %v", err)
		return "", apppb.ErrorApplicationCreateFailed("failed to regenerate client secret")
	}

	if err := uc.repo.UpdateClientSecretHash(ctx, id, hash); err != nil {
		uc.log.Errorf("update client secret hash failed: %v", err)
		return "", apppb.ErrorApplicationCreateFailed("%v", err)
	}
	return plainSecret, nil
}

// generateRandomHex returns a hex-encoded string of n random bytes (2*n chars).
func generateRandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
