package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	authnpb "github.com/Servora-Kit/servora/api/gen/go/authn/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/pkg/jwks"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/redis"
	kErrors "github.com/go-kratos/kratos/v2/errors"
)

// LoginHandler 处理 OIDC 登录流程（GET 重定向到 SPA）。
type LoginHandler struct {
	redis        *redis.Client
	log          *logger.Helper
	loginBaseURL string // 登录页基地址，如 http://localhost:3000
}

// NewLoginHandler 构建 handler，从 *conf.App 读取 login_base_url。
// login_base_url 为空时 panic，属于启动必选配置。
func NewLoginHandler(app *conf.App, rdb *redis.Client, l logger.Logger) *LoginHandler {
	loginBaseURL := app.GetOidc().GetLoginBaseUrl()
	if loginBaseURL == "" {
		panic("oidc: app.oidc.login_base_url is required but not configured")
	}
	loginBaseURL = strings.TrimRight(loginBaseURL, "/")

	return &LoginHandler{
		redis:        rdb,
		log:          logger.For(l, "oidc/login/iam"),
		loginBaseURL: loginBaseURL,
	}
}

// ServeHTTP 仅处理 GET：将 OIDC 授权请求重定向到 SPA 登录页。
func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.renderLogin(w, r)
}

func (h *LoginHandler) renderLogin(w http.ResponseWriter, r *http.Request) {
	authRequestID := r.URL.Query().Get("authRequestID")
	if authRequestID == "" {
		http.Error(w, "missing authRequestID", http.StatusBadRequest)
		return
	}
	spaLoginURL := fmt.Sprintf(
		"%s/login?authRequestID=%s",
		h.loginBaseURL,
		url.QueryEscape(authRequestID),
	)
	http.Redirect(w, r, spaLoginURL, http.StatusFound)
}

// completeOIDCRequest 将 auth request 标记为完成（写入 user_id + auth_time + done），
// 并返回下游 callback URL。
func (h *LoginHandler) completeOIDCRequest(ctx context.Context, authRequestID, userID string) (string, error) {
	if authRequestID == "" || userID == "" {
		return "", kErrors.BadRequest("MISSING_FIELDS", "authRequestID and userID are required")
	}

	key := "oidc:auth_request:" + authRequestID
	data, err := h.redis.Get(ctx, key)
	if err != nil {
		return "", authnpb.ErrorTokenExpired("auth request not found or expired")
	}

	var reqData map[string]any
	if err := json.Unmarshal([]byte(data), &reqData); err != nil {
		h.log.Errorf("unmarshal auth request data: %v", err)
		return "", kErrors.InternalServer("INTERNAL_ERROR", "failed to process auth request")
	}
	reqData["user_id"] = userID
	reqData["auth_time"] = time.Now().UTC().Format(time.RFC3339Nano)
	reqData["done"] = true

	updated, err := json.Marshal(reqData)
	if err != nil {
		h.log.Errorf("marshal auth request data: %v", err)
		return "", kErrors.InternalServer("INTERNAL_ERROR", "failed to serialize auth request")
	}
	if err := h.redis.Set(ctx, key, string(updated), 10*time.Minute); err != nil {
		h.log.Errorf("save auth request: %v", err)
		return "", kErrors.InternalServer("INTERNAL_ERROR", "failed to save auth request")
	}

	return fmt.Sprintf("/authorize/callback?id=%s", authRequestID), nil
}

// LoginCompleteHandler 提供 POST /login/complete JSON API。
// 前端先通过 IAM API 完成认证获得 access token，再调此接口完成 OIDC 授权流程。
type LoginCompleteHandler struct {
	lh         *LoginHandler
	keyManager *jwks.KeyManager
	authnRepo  biz.AuthnRepo
}

// NewLoginCompleteHandler 构建 LoginCompleteHandler。
func NewLoginCompleteHandler(
	lh *LoginHandler,
	km *jwks.KeyManager,
	authnRepo biz.AuthnRepo,
) *LoginCompleteHandler {
	return &LoginCompleteHandler{
		lh:         lh,
		keyManager: km,
		authnRepo:  authnRepo,
	}
}

func (h *LoginCompleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		AuthRequestID string `json:"authRequestID"`
		AccessToken   string `json:"accessToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, kErrors.BadRequest("BAD_REQUEST", "invalid request body"))
		return
	}

	if req.AuthRequestID == "" || req.AccessToken == "" {
		writeJSONError(w, http.StatusBadRequest, kErrors.BadRequest("MISSING_FIELDS", "authRequestID and accessToken are required"))
		return
	}

	// 验证 access token 并提取用户 ID
	claims := &biz.UserClaims{}
	if err := h.keyManager.Verifier().Verify(req.AccessToken, claims); err != nil {
		writeJSONError(w, http.StatusUnauthorized, authnpb.ErrorInvalidCredentials("invalid or expired access token"))
		return
	}
	userID, err := claims.GetSubject()
	if err != nil || userID == "" {
		writeJSONError(w, http.StatusUnauthorized, authnpb.ErrorInvalidCredentials("token missing subject"))
		return
	}

	callbackURL, err := h.lh.completeOIDCRequest(r.Context(), req.AuthRequestID, userID)
	if err != nil {
		se := kErrors.FromError(err)
		writeJSONError(w, int(se.Code), se)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if encErr := json.NewEncoder(w).Encode(map[string]string{"callbackURL": callbackURL}); encErr != nil {
		h.lh.log.Errorf("encode response: %v", encErr)
	}
}

// writeJSONError 将 Kratos 错误以 JSON 格式写回响应。
func writeJSONError(w http.ResponseWriter, statusCode int, err error) {
	se := kErrors.FromError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(se)
}
