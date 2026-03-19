package biz

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/golang-jwt/jwt/v5"

	authnpb "github.com/Servora-Kit/servora/api/gen/go/authn/service/v1"
	"github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/helpers"
	"github.com/Servora-Kit/servora/pkg/jwks"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/mail"
)

type AuthnUsecase struct {
	repo       AuthnRepo
	tokenStore OTPRepo
	mailer     mail.Sender
	mailCfg    *conf.Mail
	log        *logger.Helper
	cfg        *conf.App
	keyManager *jwks.KeyManager
}

func NewAuthnUsecase(
	repo AuthnRepo,
	tokenStore OTPRepo,
	mailer mail.Sender,
	mailCfg *conf.Mail,
	l logger.Logger,
	cfg *conf.App,
	km *jwks.KeyManager,
) *AuthnUsecase {
	return &AuthnUsecase{
		repo:       repo,
		tokenStore: tokenStore,
		mailer:     mailer,
		mailCfg:    mailCfg,
		log:        logger.NewHelper(l, logger.WithModule("authn/biz/iam-service")),
		cfg:        cfg,
		keyManager: km,
	}
}

type UserClaims struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Nonce    string `json:"nonce"`
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
	UpdateEmailVerified(ctx context.Context, userID string, verified bool) error
	TokenStore
}

type OTPRepo interface {
	SetToken(ctx context.Context, purpose, token, userID string, ttl time.Duration) error
	ConsumeToken(ctx context.Context, purpose, token string) (userID string, err error)
}

func (uc *AuthnUsecase) SignupByEmail(ctx context.Context, user *entity.User) (*entity.User, error) {
	existingUser, err := uc.repo.GetUserByUserName(ctx, user.Username)
	if err != nil && !ent.IsNotFound(err) {
		uc.log.Errorf("check username failed: %v", err)
		return nil, kerrors.InternalServer("INTERNAL", "internal error")
	}
	if existingUser != nil {
		return nil, authnpb.ErrorUserAlreadyExists("username already exists")
	}

	existingEmail, err := uc.repo.GetUserByEmail(ctx, user.Email)
	if err != nil && !ent.IsNotFound(err) {
		uc.log.Errorf("check email failed: %v", err)
		return nil, kerrors.InternalServer("INTERNAL", "internal error")
	}
	if existingEmail != nil {
		return nil, authnpb.ErrorUserAlreadyExists("email already exists")
	}

	user.Role = "user"
	createdUser, err := uc.repo.SaveUser(ctx, user)
	if err != nil {
		return nil, err
	}

	// Automatically send verification email; failure is non-fatal.
	if err := uc.sendVerificationEmail(ctx, createdUser); err != nil {
		uc.log.Warnf("auto-send verification email failed for user %s: %v", createdUser.ID, err)
	}

	return createdUser, nil
}

// SendVerificationEmail is the public wrapper used by other usecases (e.g. UserUsecase)
// to trigger email verification for a newly created user.
func (uc *AuthnUsecase) SendVerificationEmail(ctx context.Context, user *entity.User) error {
	return uc.sendVerificationEmail(ctx, user)
}

// sendVerificationEmail generates a verification token and sends the email.
func (uc *AuthnUsecase) sendVerificationEmail(ctx context.Context, user *entity.User) error {
	raw, err := uc.generateOpaqueToken()
	if err != nil {
		return err
	}
	ttl := uc.verifyEmailTTL()
	if err := uc.tokenStore.SetToken(ctx, purposeVerifyEmail, tokenHash(raw), user.ID, ttl); err != nil {
		return err
	}
	link := uc.buildTokenLink(mailPathVerifyEmail, raw)
	subject, html, err := RenderVerifyEmail(uc.mailCfg, link, ttl)
	if err != nil {
		return err
	}
	return uc.mailer.Send(ctx, mail.Email{
		From:    mail.DefaultFrom(uc.mailCfg),
		To:      []string{user.Email},
		Subject: subject,
		HTML:    html,
	})
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
		if ent.IsNotFound(err) {
			return nil, authnpb.ErrorUserNotFound("invalid email or password")
		}
		uc.log.Errorf("get user by email failed: %v", err)
		return nil, kerrors.InternalServer("INTERNAL", "internal error")
	}
	if foundUser == nil {
		return nil, authnpb.ErrorUserNotFound("invalid email or password")
	}
	if !helpers.BcryptCheck(user.Password, foundUser.Password) {
		return nil, authnpb.ErrorIncorrectPassword("invalid email or password")
	}

	if !foundUser.EmailVerified {
		return nil, authnpb.ErrorEmailNotVerified("please verify your email before logging in")
	}

	nonce, err := uc.generateOpaqueToken()
	if err != nil {
		uc.log.Errorf("generate nonce failed: %v", err)
		return nil, kerrors.InternalServer("INTERNAL", "internal error")
	}

	accessClaims := &UserClaims{
		ID:       foundUser.ID,
		Username: foundUser.Username,
		Role:     foundUser.Role,
		Nonce:    nonce,
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
		return nil, kerrors.InternalServer("INTERNAL", "internal error")
	}

	refreshToken, err := uc.generateOpaqueToken()
	if err != nil {
		uc.log.Errorf("generate refresh token failed: %v", err)
		return nil, kerrors.InternalServer("INTERNAL", "internal error")
	}

	refreshExpirationTime := time.Duration(uc.cfg.Jwt.RefreshExpire) * time.Second
	if err := uc.repo.SaveRefreshToken(ctx, foundUser.ID, refreshToken, refreshExpirationTime); err != nil {
		uc.log.Errorf("save refresh token failed: %v", err)
		return nil, kerrors.InternalServer("INTERNAL", "internal error")
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
		return nil, kerrors.InternalServer("INTERNAL", "internal error")
	}

	accessExpirationTime := time.Duration(uc.cfg.Jwt.AccessExpire) * time.Second
	nonce, err := uc.generateOpaqueToken()
	if err != nil {
		uc.log.Errorf("generate nonce failed: %v", err)
		return nil, kerrors.InternalServer("INTERNAL", "internal error")
	}

	accessClaims := &UserClaims{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.Role,
		Nonce:    nonce,
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
		return nil, kerrors.InternalServer("INTERNAL", "internal error")
	}

	newRefreshToken, err := uc.generateOpaqueToken()
	if err != nil {
		uc.log.Errorf("generate refresh token failed: %v", err)
		return nil, kerrors.InternalServer("INTERNAL", "internal error")
	}

	if err := uc.repo.DeleteRefreshToken(ctx, refreshToken); err != nil {
		uc.log.Warnf("Failed to delete old refresh token: %v", err)
	}

	refreshExpirationTime := time.Duration(uc.cfg.Jwt.RefreshExpire) * time.Second
	if err := uc.repo.SaveRefreshToken(ctx, user.ID, newRefreshToken, refreshExpirationTime); err != nil {
		uc.log.Errorf("save refresh token failed: %v", err)
		return nil, kerrors.InternalServer("INTERNAL", "internal error")
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
		return kerrors.InternalServer("INTERNAL", "internal error")
	}

	if !helpers.BcryptCheck(currentPassword, user.Password) {
		return authnpb.ErrorIncorrectPassword("current password is incorrect")
	}

	hashed, err := helpers.BcryptHash(newPassword)
	if err != nil {
		uc.log.Errorf("hash new password failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "internal error")
	}

	if err := uc.repo.UpdatePassword(ctx, userID, hashed); err != nil {
		uc.log.Errorf("save new password failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "internal error")
	}

	if err := uc.repo.DeleteUserRefreshTokens(ctx, userID); err != nil {
		uc.log.Warnf("delete refresh tokens after password change: %v", err)
	}

	return nil
}

func (uc *AuthnUsecase) LogoutAllDevices(ctx context.Context, userID string) error {
	if err := uc.repo.DeleteUserRefreshTokens(ctx, userID); err != nil {
		uc.log.Errorf("delete all refresh tokens failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "internal error")
	}
	return nil
}

// --- Email verification & password reset (token-link flow) ---

const (
	purposeVerifyEmail   = "verify_email"
	purposeResetPassword = "reset_password"

	defaultVerifyEmailTTL    = 24 * time.Hour
	defaultResetPasswordTTL  = 1 * time.Hour

	mailPathVerifyEmail   = "/verify-email"
	mailPathResetPassword = "/reset-password"
)

// verifyEmailTTL 返回邮箱验证链接有效期，优先读配置，未设置则使用默认值 24h。
func (uc *AuthnUsecase) verifyEmailTTL() time.Duration {
	if uc.mailCfg != nil && uc.mailCfg.GetVerifyEmailTtl() != nil {
		if d := uc.mailCfg.GetVerifyEmailTtl().AsDuration(); d > 0 {
			return d
		}
	}
	return defaultVerifyEmailTTL
}

// resetPasswordTTL 返回密码重置链接有效期，优先读配置，未设置则使用默认值 1h。
func (uc *AuthnUsecase) resetPasswordTTL() time.Duration {
	if uc.mailCfg != nil && uc.mailCfg.GetResetPasswordTtl() != nil {
		if d := uc.mailCfg.GetResetPasswordTtl().AsDuration(); d > 0 {
			return d
		}
	}
	return defaultResetPasswordTTL
}

func tokenHash(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func (uc *AuthnUsecase) RequestEmailVerification(ctx context.Context, email string) error {
	user, err := uc.repo.GetUserByEmail(ctx, email)
	if err != nil || user == nil || user.EmailVerified {
		return nil // always succeed to avoid leaking state
	}

	raw, err := uc.generateOpaqueToken()
	if err != nil {
		uc.log.Errorf("generate verify token failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "internal error")
	}

	ttl := uc.verifyEmailTTL()
	if err := uc.tokenStore.SetToken(ctx, purposeVerifyEmail, tokenHash(raw), user.ID, ttl); err != nil {
		uc.log.Errorf("save verify token failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "internal error")
	}

	link := uc.buildTokenLink(mailPathVerifyEmail, raw)
	subject, html, err := RenderVerifyEmail(uc.mailCfg, link, ttl)
	if err != nil {
		uc.log.Errorf("render verify email template failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "internal error")
	}
	if err := uc.mailer.Send(ctx, mail.Email{
		From:    mail.DefaultFrom(uc.mailCfg),
		To:      []string{user.Email},
		Subject: subject,
		HTML:    html,
	}); err != nil {
		uc.log.Errorf("send verify email failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "failed to send email")
	}
	return nil
}

func (uc *AuthnUsecase) VerifyEmail(ctx context.Context, token string) error {
	userID, err := uc.tokenStore.ConsumeToken(ctx, purposeVerifyEmail, tokenHash(token))
	if err != nil {
		return authnpb.ErrorTokenExpired("invalid or expired verification token")
	}
	if err := uc.repo.UpdateEmailVerified(ctx, userID, true); err != nil {
		uc.log.Errorf("update email_verified failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "internal error")
	}
	return nil
}

func (uc *AuthnUsecase) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := uc.repo.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return nil // always succeed to avoid leaking account existence
	}

	raw, err := uc.generateOpaqueToken()
	if err != nil {
		uc.log.Errorf("generate reset token failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "internal error")
	}

	resetTTL := uc.resetPasswordTTL()
	if err := uc.tokenStore.SetToken(ctx, purposeResetPassword, tokenHash(raw), user.ID, resetTTL); err != nil {
		uc.log.Errorf("save reset token failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "internal error")
	}

	link := uc.buildTokenLink(mailPathResetPassword, raw)
	subject, html, err := RenderResetPassword(uc.mailCfg, link, resetTTL)
	if err != nil {
		uc.log.Errorf("render reset password template failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "internal error")
	}
	if err := uc.mailer.Send(ctx, mail.Email{
		From:    mail.DefaultFrom(uc.mailCfg),
		To:      []string{user.Email},
		Subject: subject,
		HTML:    html,
	}); err != nil {
		uc.log.Errorf("send reset email failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "failed to send email")
	}
	return nil
}

func (uc *AuthnUsecase) ResetPassword(ctx context.Context, token, newPassword string) error {
	userID, err := uc.tokenStore.ConsumeToken(ctx, purposeResetPassword, tokenHash(token))
	if err != nil {
		return authnpb.ErrorTokenExpired("invalid or expired reset token")
	}

	hashed, err := helpers.BcryptHash(newPassword)
	if err != nil {
		uc.log.Errorf("hash new password failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "internal error")
	}

	if err := uc.repo.UpdatePassword(ctx, userID, hashed); err != nil {
		uc.log.Errorf("reset password update failed: %v", err)
		return kerrors.InternalServer("INTERNAL", "internal error")
	}

	if err := uc.repo.DeleteUserRefreshTokens(ctx, userID); err != nil {
		uc.log.Warnf("delete refresh tokens after reset: %v", err)
	}
	return nil
}

func (uc *AuthnUsecase) buildTokenLink(path, token string) string {
	return uc.mailCfg.GetBaseUrl() + path + "?token=" + token
}

// VerifyAccessToken 校验 access token（JWT），成功返回用户 ID（sub claim）。
// 供网关 ForwardAuth 的 /v1/auth/verify 端点使用，与 authn 中间件共用同一套 Verifier。
func (uc *AuthnUsecase) VerifyAccessToken(tokenString string) (string, error) {
	claims := jwt.MapClaims{}
	if err := uc.keyManager.Verifier().Verify(tokenString, claims); err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", authnpb.ErrorTokenExpired("token expired")
		}
		return "", authnpb.ErrorInvalidCredentials("invalid token")
	}
	sub, _ := claims.GetSubject()
	if sub == "" {
		return "", authnpb.ErrorInvalidCredentials("token missing subject")
	}
	return sub, nil
}
