package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/golang-jwt/jwt/v5"

	authnpb "github.com/Servora-Kit/servora/api/gen/go/authn/service/v1"
	"github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	dataent "github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/helpers"
	"github.com/Servora-Kit/servora/pkg/jwks"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type AuthnUsecase struct {
	repo       AuthnRepo
	log        *logger.Helper
	cfg        *conf.App
	keyManager *jwks.KeyManager
	orgUC      *OrganizationUsecase
	projUC     *ProjectUsecase
}

func NewAuthnUsecase(repo AuthnRepo, l logger.Logger, cfg *conf.App, km *jwks.KeyManager, orgUC *OrganizationUsecase, projUC *ProjectUsecase) *AuthnUsecase {
	return &AuthnUsecase{
		repo:       repo,
		log:        logger.NewHelper(l, logger.WithModule("authn/biz/iam-service")),
		cfg:        cfg,
		keyManager: km,
		orgUC:      orgUC,
		projUC:     projUC,
	}
}

type UserClaims struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Nonce string `json:"nonce"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

type TokenStore interface {
	SaveRefreshToken(ctx context.Context, userID string, token string, expiration time.Duration) error
	GetRefreshToken(ctx context.Context, token string) (string, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteUserRefreshTokens(ctx context.Context, userID string) error
}

type AuthnRepo interface {
	SaveUser(context.Context, *entity.User) (*entity.User, error)
	GetUserByEmail(context.Context, string) (*entity.User, error)
	GetUserByUserName(context.Context, string) (*entity.User, error)
	GetUserByID(context.Context, string) (*entity.User, error)
	UpdatePassword(ctx context.Context, userID string, hashedPassword string) error
	TokenStore
}

func (uc *AuthnUsecase) SignupByEmail(ctx context.Context, user *entity.User) (*entity.User, error) {
	existingUser, err := uc.repo.GetUserByUserName(ctx, user.Name)
	if err != nil && !dataent.IsNotFound(err) {
		uc.log.Errorf("check username failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}
	if existingUser != nil {
		return nil, authnpb.ErrorUserAlreadyExists("username already exists")
	}

	existingEmail, err := uc.repo.GetUserByEmail(ctx, user.Email)
	if err != nil && !dataent.IsNotFound(err) {
		uc.log.Errorf("check email failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}
	if existingEmail != nil {
		return nil, authnpb.ErrorUserAlreadyExists("email already exists")
	}

	user.Role = "user"
	createdUser, err := uc.repo.SaveUser(ctx, user)
	if err != nil {
		return nil, err
	}

	slug := helpers.Slugify(createdUser.Name)
	org, err := uc.orgUC.CreateDefault(ctx, createdUser.ID, createdUser.Name+"'s Organization", slug+"-org")
	if err != nil {
		uc.log.Warnf("auto-create default org failed for user %s: %v", createdUser.ID, err)
	} else {
		if _, err := uc.projUC.CreateDefault(ctx, createdUser.ID, org.ID, "Default Project", "default"); err != nil {
			uc.log.Warnf("auto-create default project failed for user %s: %v", createdUser.ID, err)
		}
	}

	return createdUser, nil
}

func (uc *AuthnUsecase) generateAccessToken(claims *UserClaims) (string, error) {
	return uc.keyManager.Signer().Sign(claims)
}

func (uc *AuthnUsecase) generateOpaqueToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (uc *AuthnUsecase) LoginByEmailPassword(ctx context.Context, user *entity.User) (*TokenPair, error) {
	foundUser, err := uc.repo.GetUserByEmail(ctx, user.Email)
	if err != nil {
		if dataent.IsNotFound(err) {
			return nil, authnpb.ErrorUserNotFound("invalid email or password")
		}
		uc.log.Errorf("get user by email failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}
	if foundUser == nil {
		return nil, authnpb.ErrorUserNotFound("invalid email or password")
	}
	if !helpers.BcryptCheck(user.Password, foundUser.Password) {
		return nil, authnpb.ErrorIncorrectPassword("invalid email or password")
	}

	nonce, err := uc.generateOpaqueToken()
	if err != nil {
		uc.log.Errorf("generate nonce failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}

	accessClaims := &UserClaims{
		ID:    foundUser.ID,
		Name:  foundUser.Name,
		Role:  foundUser.Role,
		Nonce: nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   foundUser.ID,
			Audience:  jwt.ClaimStrings{uc.cfg.Jwt.Audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(uc.cfg.Jwt.AccessExpire) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    uc.cfg.Jwt.Issuer,
		},
	}

	accessToken, err := uc.generateAccessToken(accessClaims)
	if err != nil {
		uc.log.Errorf("generate access token failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}

	refreshToken, err := uc.generateOpaqueToken()
	if err != nil {
		uc.log.Errorf("generate refresh token failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}

	refreshExpirationTime := time.Duration(uc.cfg.Jwt.RefreshExpire) * time.Second
	if err := uc.repo.SaveRefreshToken(ctx, foundUser.ID, refreshToken, refreshExpirationTime); err != nil {
		uc.log.Errorf("save refresh token failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(uc.cfg.Jwt.AccessExpire),
	}, nil
}

func (uc *AuthnUsecase) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	userID, err := uc.repo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		uc.log.Warnf("invalid refresh token: %v", err)
		return nil, authnpb.ErrorInvalidRefreshToken("invalid or expired refresh token")
	}

	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		uc.log.Errorf("get user by ID failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}

	accessExpirationTime := time.Duration(uc.cfg.Jwt.AccessExpire) * time.Second
	nonce, err := uc.generateOpaqueToken()
	if err != nil {
		uc.log.Errorf("generate nonce failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}

	accessClaims := &UserClaims{
		ID:    user.ID,
		Name:  user.Name,
		Role:  user.Role,
		Nonce: nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			Audience:  jwt.ClaimStrings{uc.cfg.Jwt.Audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessExpirationTime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    uc.cfg.Jwt.Issuer,
		},
	}

	accessToken, err := uc.generateAccessToken(accessClaims)
	if err != nil {
		uc.log.Errorf("generate access token failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}

	newRefreshToken, err := uc.generateOpaqueToken()
	if err != nil {
		uc.log.Errorf("generate refresh token failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}

	if err := uc.repo.DeleteRefreshToken(ctx, refreshToken); err != nil {
		uc.log.Warnf("Failed to delete old refresh token: %v", err)
	}

	refreshExpirationTime := time.Duration(uc.cfg.Jwt.RefreshExpire) * time.Second
	if err := uc.repo.SaveRefreshToken(ctx, user.ID, newRefreshToken, refreshExpirationTime); err != nil {
		uc.log.Errorf("save refresh token failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(uc.cfg.Jwt.AccessExpire),
	}, nil
}

func (uc *AuthnUsecase) Logout(ctx context.Context, refreshToken string) error {
	if err := uc.repo.DeleteRefreshToken(ctx, refreshToken); err != nil {
		uc.log.Warnf("Failed to delete refresh token during logout: %v", err)
	}
	return nil
}

func (uc *AuthnUsecase) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		uc.log.Errorf("get user for change password failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}

	if !helpers.BcryptCheck(currentPassword, user.Password) {
		return authnpb.ErrorIncorrectPassword("current password is incorrect")
	}

	hashed, err := helpers.BcryptHash(newPassword)
	if err != nil {
		uc.log.Errorf("hash new password failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}

	if err := uc.repo.UpdatePassword(ctx, userID, hashed); err != nil {
		uc.log.Errorf("save new password failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}

	if err := uc.repo.DeleteUserRefreshTokens(ctx, userID); err != nil {
		uc.log.Warnf("delete refresh tokens after password change: %v", err)
	}

	return nil
}

func (uc *AuthnUsecase) LogoutAllDevices(ctx context.Context, userID string) error {
	if err := uc.repo.DeleteUserRefreshTokens(ctx, userID); err != nil {
		uc.log.Errorf("delete all refresh tokens failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}
	return nil
}
