package biz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
)

// richTenantRepo is an in-memory TenantRepo that supports TransferOwnership
// for testing the new simplified owner_user_id model.
type richTenantRepo struct {
	fakeTenantRepo
	tenants map[string]*entity.Tenant // key: tenantID
}

func newRichTenantRepo() *richTenantRepo {
	return &richTenantRepo{
		tenants: make(map[string]*entity.Tenant),
	}
}

func (r *richTenantRepo) GetByID(_ context.Context, id string) (*entity.Tenant, error) {
	t, ok := r.tenants[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *t
	return &cp, nil
}

func (r *richTenantRepo) TransferOwnership(_ context.Context, tenantID, newOwnerUserID string) error {
	t, ok := r.tenants[tenantID]
	if !ok {
		return ErrNotFound
	}
	t.OwnerUserID = newOwnerUserID
	return nil
}

var ErrNotFound = &notFoundError{}

type notFoundError struct{}

func (e *notFoundError) Error() string { return "not found" }

func newTenantUCWithRepo(repo TenantRepo) *TenantUsecase {
	orgUC := &OrganizationUsecase{}
	return NewTenantUsecase(repo, orgUC, nil, log.DefaultLogger)
}

func TestTenantTransferOwnership_HappyPath(t *testing.T) {
	repo := newRichTenantRepo()
	ctx := context.Background()

	repo.tenants["t1"] = &entity.Tenant{ID: "t1", OwnerUserID: "owner-user"}

	uc := newTenantUCWithRepo(repo)
	err := uc.TransferOwnership(ctx, "t1", "owner-user", "admin-user")
	if err != nil {
		t.Fatalf("TransferOwnership() unexpected error: %v", err)
	}

	if repo.tenants["t1"].OwnerUserID != "admin-user" {
		t.Errorf("owner_user_id = %q, want admin-user", repo.tenants["t1"].OwnerUserID)
	}
}

func TestTenantTransferOwnership_SameUser(t *testing.T) {
	repo := newRichTenantRepo()
	ctx := context.Background()

	repo.tenants["t1"] = &entity.Tenant{ID: "t1", OwnerUserID: "owner-user"}

	uc := newTenantUCWithRepo(repo)
	err := uc.TransferOwnership(ctx, "t1", "owner-user", "owner-user")
	if err == nil {
		t.Fatal("TransferOwnership() should fail when caller and new owner are the same")
	}
}
