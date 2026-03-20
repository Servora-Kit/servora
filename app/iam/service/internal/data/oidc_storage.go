package data

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/application"
	"github.com/Servora-Kit/servora/pkg/helpers"
	"github.com/Servora-Kit/servora/pkg/jwks"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/redis"
)

const (
	oidcAuthRequestPrefix  = "oidc:auth_request:"
	oidcAuthCodePrefix     = "oidc:auth_code:"
	oidcAccessTokenPrefix  = "oidc:access_token:"
	oidcRefreshTokenPrefix = "oidc:refresh_token:"

	authRequestTTL  = 10 * time.Minute
	authCodeTTL     = 5 * time.Minute
	accessTokenTTL  = 1 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
)

type oidcStorage struct {
	data      *Data
	km        *jwks.KeyManager
	authnRepo biz.AuthnRepo
	env       string
	log       *logger.Helper
	redis     *redis.Client
}

func NewOIDCStorage(
	data *Data,
	km *jwks.KeyManager,
	authnRepo biz.AuthnRepo,
	rdb *redis.Client,
	appCfg *conf.App,
	l logger.Logger,
) op.Storage {
	return &oidcStorage{
		data:      data,
		km:        km,
		authnRepo: authnRepo,
		env:       appCfg.GetEnv(),
		log:       logger.For(l, "oidc-storage/data/iam"),
		redis:     rdb,
	}
}

// ---------------------------------------------------------------------------
// authRequest implements op.AuthRequest
// ---------------------------------------------------------------------------

type authRequest struct {
	ID            string              `json:"id"`
	ClientID      string              `json:"client_id"`
	RedirectURI   string              `json:"redirect_uri"`
	Scopes        []string            `json:"scopes"`
	ResponseType  oidc.ResponseType   `json:"response_type"`
	ResponseMode  oidc.ResponseMode   `json:"response_mode"`
	Nonce         string              `json:"nonce"`
	State         string              `json:"state"`
	CodeChallenge *oidc.CodeChallenge `json:"code_challenge,omitempty"`
	Audience      []string            `json:"audience"`
	UserID        string              `json:"user_id"`
	AuthTime      time.Time           `json:"auth_time"`
	IsDone        bool                `json:"done"`
}

func (a *authRequest) GetID() string                         { return a.ID }
func (a *authRequest) GetACR() string                        { return "" }
func (a *authRequest) GetAMR() []string                      { return nil }
func (a *authRequest) GetAudience() []string                 { return a.Audience }
func (a *authRequest) GetAuthTime() time.Time                { return a.AuthTime }
func (a *authRequest) GetClientID() string                   { return a.ClientID }
func (a *authRequest) GetCodeChallenge() *oidc.CodeChallenge { return a.CodeChallenge }
func (a *authRequest) GetNonce() string                      { return a.Nonce }
func (a *authRequest) GetRedirectURI() string                { return a.RedirectURI }
func (a *authRequest) GetResponseType() oidc.ResponseType    { return a.ResponseType }
func (a *authRequest) GetResponseMode() oidc.ResponseMode    { return a.ResponseMode }
func (a *authRequest) GetScopes() []string                   { return a.Scopes }
func (a *authRequest) GetState() string                      { return a.State }
func (a *authRequest) GetSubject() string                    { return a.UserID }
func (a *authRequest) Done() bool                            { return a.IsDone }

// ---------------------------------------------------------------------------
// AuthStorage implementation
// ---------------------------------------------------------------------------

func (s *oidcStorage) CreateAuthRequest(ctx context.Context, oidcReq *oidc.AuthRequest, userID string) (op.AuthRequest, error) {
	id, _ := uuid.NewV7()
	req := &authRequest{
		ID:           id.String(),
		ClientID:     oidcReq.ClientID,
		RedirectURI:  oidcReq.RedirectURI,
		Scopes:       oidcReq.Scopes,
		ResponseType: oidcReq.ResponseType,
		ResponseMode: oidcReq.ResponseMode,
		Nonce:        oidcReq.Nonce,
		State:        oidcReq.State,
		Audience:     []string{oidcReq.ClientID},
	}
	if oidcReq.CodeChallenge != "" {
		req.CodeChallenge = &oidc.CodeChallenge{
			Challenge: oidcReq.CodeChallenge,
			Method:    oidcReq.CodeChallengeMethod,
		}
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal auth request: %w", err)
	}
	if err := s.redis.Set(ctx, oidcAuthRequestPrefix+req.ID, string(data), authRequestTTL); err != nil {
		return nil, fmt.Errorf("store auth request: %w", err)
	}
	return req, nil
}

func (s *oidcStorage) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	data, err := s.redis.Get(ctx, oidcAuthRequestPrefix+id)
	if err != nil {
		return nil, fmt.Errorf("auth request not found: %w", err)
	}
	var req authRequest
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		return nil, fmt.Errorf("unmarshal auth request: %w", err)
	}
	return &req, nil
}

func (s *oidcStorage) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	reqID, err := s.redis.Get(ctx, oidcAuthCodePrefix+code)
	if err != nil {
		return nil, fmt.Errorf("auth code not found: %w", err)
	}
	return s.AuthRequestByID(ctx, reqID)
}

func (s *oidcStorage) SaveAuthCode(ctx context.Context, id, code string) error {
	return s.redis.Set(ctx, oidcAuthCodePrefix+code, id, authCodeTTL)
}

func (s *oidcStorage) DeleteAuthRequest(ctx context.Context, id string) error {
	return s.redis.Del(ctx, oidcAuthRequestPrefix+id)
}

// tokenMeta is stored in Redis for token introspection and userinfo.
type tokenMeta struct {
	Subject  string   `json:"sub"`
	ClientID string   `json:"client_id"`
	Scopes   []string `json:"scopes"`
	Audience []string `json:"audience"`
}

func (s *oidcStorage) CreateAccessToken(ctx context.Context, request op.TokenRequest) (string, time.Time, error) {
	tokenID, _ := uuid.NewV7()
	expiration := time.Now().Add(accessTokenTTL)

	meta := tokenMeta{
		Subject:  request.GetSubject(),
		Scopes:   request.GetScopes(),
		Audience: request.GetAudience(),
	}
	if ar, ok := request.(op.AuthRequest); ok {
		meta.ClientID = ar.GetClientID()
	}

	data, _ := json.Marshal(meta)
	if err := s.redis.Set(ctx, oidcAccessTokenPrefix+tokenID.String(), string(data), accessTokenTTL); err != nil {
		return "", time.Time{}, fmt.Errorf("store access token: %w", err)
	}
	return tokenID.String(), expiration, nil
}

func (s *oidcStorage) CreateAccessAndRefreshTokens(ctx context.Context, request op.TokenRequest, currentRefreshToken string) (string, string, time.Time, error) {
	accessTokenID, expiration, err := s.CreateAccessToken(ctx, request)
	if err != nil {
		return "", "", time.Time{}, err
	}

	if currentRefreshToken != "" {
		_ = s.redis.Del(ctx, oidcRefreshTokenPrefix+currentRefreshToken)
	}

	refreshToken, _ := uuid.NewV7()
	refreshMeta := refreshTokenMeta{
		TokenMeta: tokenMeta{
			Subject:  request.GetSubject(),
			Scopes:   request.GetScopes(),
			Audience: request.GetAudience(),
		},
		AccessTokenID: accessTokenID,
	}
	if ar, ok := request.(op.AuthRequest); ok {
		refreshMeta.ClientID = ar.GetClientID()
		refreshMeta.AuthTime = ar.GetAuthTime()
		refreshMeta.AMR = ar.GetAMR()
	}

	data, _ := json.Marshal(refreshMeta)
	if err := s.redis.Set(ctx, oidcRefreshTokenPrefix+refreshToken.String(), string(data), refreshTokenTTL); err != nil {
		return "", "", time.Time{}, fmt.Errorf("store refresh token: %w", err)
	}
	return accessTokenID, refreshToken.String(), expiration, nil
}

type refreshTokenMeta struct {
	TokenMeta     tokenMeta `json:"token_meta"`
	ClientID      string    `json:"client_id"`
	AccessTokenID string    `json:"access_token_id"`
	AuthTime      time.Time `json:"auth_time"`
	AMR           []string  `json:"amr"`
}

func (r *refreshTokenMeta) GetAMR() []string                 { return r.AMR }
func (r *refreshTokenMeta) GetAudience() []string            { return r.TokenMeta.Audience }
func (r *refreshTokenMeta) GetAuthTime() time.Time           { return r.AuthTime }
func (r *refreshTokenMeta) GetClientID() string              { return r.ClientID }
func (r *refreshTokenMeta) GetScopes() []string              { return r.TokenMeta.Scopes }
func (r *refreshTokenMeta) GetSubject() string               { return r.TokenMeta.Subject }
func (r *refreshTokenMeta) SetCurrentScopes(scopes []string) { r.TokenMeta.Scopes = scopes }

func (s *oidcStorage) TokenRequestByRefreshToken(ctx context.Context, refreshToken string) (op.RefreshTokenRequest, error) {
	data, err := s.redis.Get(ctx, oidcRefreshTokenPrefix+refreshToken)
	if err != nil {
		return nil, fmt.Errorf("refresh token not found: %w", err)
	}
	var meta refreshTokenMeta
	if err := json.Unmarshal([]byte(data), &meta); err != nil {
		return nil, fmt.Errorf("unmarshal refresh token: %w", err)
	}
	return &meta, nil
}

func (s *oidcStorage) TerminateSession(ctx context.Context, userID, clientID string) error {
	s.log.Debugf("terminate session: userID=%s clientID=%s", userID, clientID)
	return nil
}

func (s *oidcStorage) RevokeToken(ctx context.Context, tokenOrTokenID, userID, clientID string) *oidc.Error {
	_ = s.redis.Del(ctx, oidcAccessTokenPrefix+tokenOrTokenID)
	_ = s.redis.Del(ctx, oidcRefreshTokenPrefix+tokenOrTokenID)
	return nil
}

func (s *oidcStorage) GetRefreshTokenInfo(ctx context.Context, clientID, token string) (string, string, error) {
	data, err := s.redis.Get(ctx, oidcRefreshTokenPrefix+token)
	if err != nil {
		return "", "", op.ErrInvalidRefreshToken
	}
	var meta refreshTokenMeta
	if err := json.Unmarshal([]byte(data), &meta); err != nil {
		return "", "", op.ErrInvalidRefreshToken
	}
	return meta.TokenMeta.Subject, meta.AccessTokenID, nil
}

// ---------------------------------------------------------------------------
// SigningKey / KeySet
// ---------------------------------------------------------------------------

type signingKey struct {
	id  string
	key *rsa.PrivateKey
}

func (k *signingKey) SignatureAlgorithm() jose.SignatureAlgorithm { return jose.RS256 }
func (k *signingKey) Key() any                                    { return k.key }
func (k *signingKey) ID() string                                  { return k.id }

type publicKey struct {
	id  string
	key *rsa.PublicKey
}

func (k *publicKey) ID() string                         { return k.id }
func (k *publicKey) Algorithm() jose.SignatureAlgorithm { return jose.RS256 }
func (k *publicKey) Use() string                        { return "sig" }
func (k *publicKey) Key() any                           { return k.key }

func (s *oidcStorage) SigningKey(_ context.Context) (op.SigningKey, error) {
	signer := s.km.Signer()
	return &signingKey{
		id:  signer.KID(),
		key: signer.PrivateKey(),
	}, nil
}

func (s *oidcStorage) SignatureAlgorithms(_ context.Context) ([]jose.SignatureAlgorithm, error) {
	return []jose.SignatureAlgorithm{jose.RS256}, nil
}

func (s *oidcStorage) KeySet(_ context.Context) ([]op.Key, error) {
	signer := s.km.Signer()
	return []op.Key{
		&publicKey{
			id:  signer.KID(),
			key: signer.PublicKey(),
		},
	}, nil
}

// ---------------------------------------------------------------------------
// OPStorage implementation
// ---------------------------------------------------------------------------

func (s *oidcStorage) GetClientByClientID(ctx context.Context, clientID string) (op.Client, error) {
	entApp, err := s.data.Ent(ctx).Application.Query().
		Where(application.ClientIDEQ(clientID), application.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("client not found: %w", err)
	}
	app := applicationMapper.Map(entApp)
	return newOIDCClient(app, strings.EqualFold(s.env, "dev")), nil
}

func (s *oidcStorage) AuthorizeClientIDSecret(ctx context.Context, clientID, clientSecret string) error {
	entApp, err := s.data.Ent(ctx).Application.Query().
		Where(application.ClientIDEQ(clientID), application.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		return fmt.Errorf("client not found: %w", err)
	}
	if !helpers.BcryptCheck(clientSecret, entApp.ClientSecretHash) {
		return fmt.Errorf("invalid client secret")
	}
	return nil
}

func (s *oidcStorage) SetUserinfoFromScopes(_ context.Context, _ *oidc.UserInfo, _, _ string, _ []string) error {
	return nil
}

func (s *oidcStorage) SetUserinfoFromToken(ctx context.Context, userinfo *oidc.UserInfo, tokenID, subject, origin string) error {
	user, err := s.authnRepo.GetUserByID(ctx, subject)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	userinfo.Subject = user.ID
	userinfo.Name = user.Username
	userinfo.Email = user.Email
	userinfo.EmailVerified = oidc.Bool(user.EmailVerified)
	return nil
}

func (s *oidcStorage) SetIntrospectionFromToken(ctx context.Context, resp *oidc.IntrospectionResponse, tokenID, subject, clientID string) error {
	user, err := s.authnRepo.GetUserByID(ctx, subject)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	resp.Active = true
	resp.Subject = user.ID
	resp.Username = user.Username
	resp.Email = user.Email
	resp.EmailVerified = oidc.Bool(user.EmailVerified)
	resp.ClientID = clientID
	return nil
}

func (s *oidcStorage) GetPrivateClaimsFromScopes(ctx context.Context, userID, clientID string, scopes []string) (map[string]any, error) {
	return nil, nil
}

func (s *oidcStorage) GetKeyByIDAndClientID(_ context.Context, _, _ string) (*jose.JSONWebKey, error) {
	return nil, nil
}

func (s *oidcStorage) ValidateJWTProfileScopes(_ context.Context, _ string, scopes []string) ([]string, error) {
	return scopes, nil
}

// ---------------------------------------------------------------------------
// ClientCredentialsStorage
// ---------------------------------------------------------------------------

func (s *oidcStorage) ClientCredentials(ctx context.Context, clientID, clientSecret string) (op.Client, error) {
	if err := s.AuthorizeClientIDSecret(ctx, clientID, clientSecret); err != nil {
		return nil, err
	}
	return s.GetClientByClientID(ctx, clientID)
}

func (s *oidcStorage) ClientCredentialsTokenRequest(ctx context.Context, clientID string, scopes []string) (op.TokenRequest, error) {
	return &clientCredentialsTokenRequest{
		clientID: clientID,
		scopes:   scopes,
	}, nil
}

type clientCredentialsTokenRequest struct {
	clientID string
	scopes   []string
}

func (r *clientCredentialsTokenRequest) GetSubject() string    { return r.clientID }
func (r *clientCredentialsTokenRequest) GetAudience() []string { return []string{r.clientID} }
func (r *clientCredentialsTokenRequest) GetScopes() []string   { return r.scopes }

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func (s *oidcStorage) Health(ctx context.Context) error {
	return s.redis.Ping(ctx)
}
