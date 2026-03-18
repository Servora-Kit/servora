package data

import (
	"time"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/mapper"
)

var userMapper = mapper.NewForwardMapper(func(u *ent.User) *entity.User {
	return &entity.User{
		ID:              u.ID.String(),
		Name:            u.Name,
		Email:           u.Email,
		Password:        u.Password,
		Role:            u.Role,
		EmailVerified:   u.EmailVerified,
		EmailVerifiedAt: u.EmailVerifiedAt,
	}
})

var orgMapper = mapper.NewForwardMapper(func(o *ent.Organization) *entity.Organization {
	e := &entity.Organization{
		ID:        o.ID.String(),
		TenantID:  o.TenantID.String(),
		Name:      o.Name,
		Slug:      o.Slug,
		Type:      string(o.Type),
		Sort:      o.Sort,
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
	}
	if o.DisplayName != nil {
		e.DisplayName = *o.DisplayName
	}
	if o.ParentID != nil {
		s := o.ParentID.String()
		e.ParentID = &s
	}
	if o.LeaderUserID != nil {
		s := o.LeaderUserID.String()
		e.LeaderUserID = &s
	}
	return e
})


var tenantMapper = mapper.NewForwardMapper(func(t *ent.Tenant) *entity.Tenant {
	e := &entity.Tenant{
		ID:          t.ID.String(),
		OwnerUserID: t.OwnerUserID.String(),
		Slug:        t.Slug,
		Name:        t.Name,
		Kind:        string(t.Kind),
		Status:      string(t.Status),
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
	if t.Domain != nil {
		e.Domain = *t.Domain
	}
	if t.DisplayName != nil {
		e.DisplayName = *t.DisplayName
	}
	return e
})

var applicationMapper = mapper.NewForwardMapper(func(a *ent.Application) *entity.Application {
	return &entity.Application{
		ID:               a.ID.String(),
		ClientID:         a.ClientID,
		ClientSecretHash: a.ClientSecretHash,
		Name:             a.Name,
		RedirectURIs:     a.RedirectUris,
		Scopes:           a.Scopes,
		GrantTypes:       a.GrantTypes,
		ApplicationType:  a.ApplicationType,
		AccessTokenType:  a.AccessTokenType,
		TenantID:         a.TenantID.String(),
		IDTokenLifetime:  time.Duration(a.IDTokenLifetime) * time.Second,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
})
