package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/pkg/actor"
)

// requireAuthenticatedUser extracts the authenticated user ID from context.
func requireAuthenticatedUser(ctx context.Context) (userID string, err error) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.Type() != actor.TypeUser {
		return "", errors.Unauthorized("UNAUTHORIZED", "unauthorized")
	}
	return a.ID(), nil
}

// requireOrgScope extracts the authenticated user ID and organization scope
// from context. The organization ID is injected by the ScopeFromHeaders
// middleware from the X-Organization-ID header.
func requireOrgScope(ctx context.Context) (userID, orgID string, err error) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.Type() != actor.TypeUser {
		return "", "", errors.Unauthorized("UNAUTHORIZED", "unauthorized")
	}
	ua, ok := a.(*actor.UserActor)
	if !ok {
		return "", "", errors.Unauthorized("UNAUTHORIZED", "unauthorized")
	}
	if ua.OrganizationID() == "" {
		return "", "", errors.BadRequest("MISSING_ORGANIZATION_SCOPE",
			"missing X-Organization-ID header")
	}
	return ua.ID(), ua.OrganizationID(), nil
}

// requireTenantScope extracts the authenticated user ID and tenant scope
// from context. The tenant ID is injected by the ScopeFromHeaders
// middleware from the X-Tenant-ID header.
func requireTenantScope(ctx context.Context) (userID, tenantID string, err error) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.Type() != actor.TypeUser {
		return "", "", errors.Unauthorized("UNAUTHORIZED", "unauthorized")
	}
	tenantID, hasTenant := actor.TenantIDFromContext(ctx)
	if !hasTenant || tenantID == "" {
		return "", "", errors.BadRequest("MISSING_TENANT_SCOPE",
			"missing X-Tenant-ID header")
	}
	return a.ID(), tenantID, nil
}

// requireTenantScopeOptional returns the tenant ID from context if present,
// or empty string if not set. Does not return an error for missing tenant.
func requireTenantScopeOptional(ctx context.Context) (tenantID string, ok bool) {
	return actor.TenantIDFromContext(ctx)
}

// errForbidden returns a standardized Forbidden error with the given reason.
func errForbidden(reason string) error {
	return errors.Forbidden("FORBIDDEN", reason)
}

// checkPlatformAdmin returns an error if the authenticated user is not a platform admin.
// It is a package-level helper so all services can call it without duplicating logic.
func checkPlatformAdmin(ctx context.Context, userUC *biz.UserUsecase) error {
	callerID, err := requireAuthenticatedUser(ctx)
	if err != nil {
		return err
	}
	user, err := userUC.CurrentUserInfo(ctx, callerID)
	if err != nil {
		return err
	}
	if user.Role != "admin" {
		return errors.Forbidden("FORBIDDEN", "only platform admin can perform this action")
	}
	return nil
}
