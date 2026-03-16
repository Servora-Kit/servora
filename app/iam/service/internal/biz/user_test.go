package biz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
)

// --- fakes ---

type fakeUserRepo struct {
	purgeCascadeErr   error
	purgeCascadeCalls []string
}

func (r *fakeUserRepo) PurgeCascade(_ context.Context, id string) error {
	r.purgeCascadeCalls = append(r.purgeCascadeCalls, id)
	return r.purgeCascadeErr
}

func (r *fakeUserRepo) SaveUser(context.Context, *entity.User) (*entity.User, error)   { return nil, nil }
func (r *fakeUserRepo) GetUserById(context.Context, string) (*entity.User, error)       { return nil, nil }
func (r *fakeUserRepo) DeleteUser(context.Context, *entity.User) (*entity.User, error)  { return nil, nil }
func (r *fakeUserRepo) PurgeUser(context.Context, *entity.User) (*entity.User, error)   { return nil, nil }
func (r *fakeUserRepo) RestoreUser(context.Context, string) (*entity.User, error)       { return nil, nil }
func (r *fakeUserRepo) GetUserByIdIncludingDeleted(context.Context, string) (*entity.User, error) {
	return nil, nil
}
func (r *fakeUserRepo) UpdateUser(context.Context, *entity.User) (*entity.User, error) {
	return nil, nil
}
func (r *fakeUserRepo) ListUsers(context.Context, int32, int32) ([]*entity.User, int64, error) {
	return nil, 0, nil
}

type fakeAuthnRepo struct {
	deleteTokensErr   error
	deleteTokensCalls []string
}

func (r *fakeAuthnRepo) DeleteUserRefreshTokens(_ context.Context, userID string) error {
	r.deleteTokensCalls = append(r.deleteTokensCalls, userID)
	return r.deleteTokensErr
}

func (r *fakeAuthnRepo) SaveUser(context.Context, *entity.User) (*entity.User, error) {
	return nil, nil
}
func (r *fakeAuthnRepo) GetUserByEmail(context.Context, string) (*entity.User, error) {
	return nil, nil
}
func (r *fakeAuthnRepo) GetUserByUserName(context.Context, string) (*entity.User, error) {
	return nil, nil
}
func (r *fakeAuthnRepo) GetUserByID(context.Context, string) (*entity.User, error) {
	return nil, nil
}
func (r *fakeAuthnRepo) UpdatePassword(context.Context, string, string) error   { return nil }
func (r *fakeAuthnRepo) UpdateEmailVerified(context.Context, string, bool) error { return nil }
func (r *fakeAuthnRepo) SaveRefreshToken(context.Context, string, string, time.Duration) error {
	return nil
}
func (r *fakeAuthnRepo) GetRefreshToken(context.Context, string) (string, error) { return "", nil }
func (r *fakeAuthnRepo) DeleteRefreshToken(context.Context, string) error        { return nil }

type fakeOrgRepo struct {
	memberships []*entity.OrganizationMember
}

func (r *fakeOrgRepo) ListMembershipsByUserID(_ context.Context, _ string) ([]*entity.OrganizationMember, error) {
	return r.memberships, nil
}

func (r *fakeOrgRepo) Create(context.Context, *entity.Organization) (*entity.Organization, error) {
	return nil, nil
}
func (r *fakeOrgRepo) GetByID(context.Context, string) (*entity.Organization, error) {
	return nil, nil
}
func (r *fakeOrgRepo) GetByIDs(context.Context, []string, int32, int32) ([]*entity.Organization, int64, error) {
	return nil, 0, nil
}
func (r *fakeOrgRepo) GetBySlug(context.Context, string) (*entity.Organization, error) {
	return nil, nil
}
func (r *fakeOrgRepo) ListByUserID(context.Context, string, int32, int32) ([]*entity.Organization, int64, error) {
	return nil, 0, nil
}
func (r *fakeOrgRepo) Update(context.Context, *entity.Organization) (*entity.Organization, error) {
	return nil, nil
}
func (r *fakeOrgRepo) Delete(context.Context, string) error       { return nil }
func (r *fakeOrgRepo) Purge(context.Context, string) error        { return nil }
func (r *fakeOrgRepo) PurgeCascade(context.Context, string) error { return nil }
func (r *fakeOrgRepo) Restore(context.Context, string) (*entity.Organization, error) {
	return nil, nil
}
func (r *fakeOrgRepo) GetByIDIncludingDeleted(context.Context, string) (*entity.Organization, error) {
	return nil, nil
}
func (r *fakeOrgRepo) AddMember(context.Context, *entity.OrganizationMember) (*entity.OrganizationMember, error) {
	return nil, nil
}
func (r *fakeOrgRepo) RemoveMember(context.Context, string, string) error { return nil }
func (r *fakeOrgRepo) UpdateMemberRole(context.Context, string, string, string) (*entity.OrganizationMember, error) {
	return nil, nil
}
func (r *fakeOrgRepo) ListMembers(context.Context, string, int32, int32) ([]*entity.OrganizationMember, int64, error) {
	return nil, 0, nil
}
func (r *fakeOrgRepo) GetMember(context.Context, string, string) (*entity.OrganizationMember, error) {
	return nil, nil
}
func (r *fakeOrgRepo) ListAllMembers(context.Context, string) ([]*entity.OrganizationMember, error) {
	return nil, nil
}
func (r *fakeOrgRepo) DeleteAllMembers(context.Context, string) (int, error) { return 0, nil }
func (r *fakeOrgRepo) DeleteMembershipsByUserID(context.Context, string) (int, error) {
	return 0, nil
}

type fakeProjRepo struct {
	memberships []*entity.ProjectMember
}

func (r *fakeProjRepo) ListMembershipsByUserID(_ context.Context, _ string) ([]*entity.ProjectMember, error) {
	return r.memberships, nil
}

func (r *fakeProjRepo) Create(context.Context, *entity.Project) (*entity.Project, error) {
	return nil, nil
}
func (r *fakeProjRepo) GetByID(context.Context, string) (*entity.Project, error) { return nil, nil }
func (r *fakeProjRepo) GetByIDs(context.Context, []string, int32, int32) ([]*entity.Project, int64, error) {
	return nil, 0, nil
}
func (r *fakeProjRepo) ListByOrgID(context.Context, string, int32, int32) ([]*entity.Project, int64, error) {
	return nil, 0, nil
}
func (r *fakeProjRepo) Update(context.Context, *entity.Project) (*entity.Project, error) {
	return nil, nil
}
func (r *fakeProjRepo) Delete(context.Context, string) error       { return nil }
func (r *fakeProjRepo) Purge(context.Context, string) error        { return nil }
func (r *fakeProjRepo) PurgeCascade(context.Context, string) error { return nil }
func (r *fakeProjRepo) Restore(context.Context, string) (*entity.Project, error) {
	return nil, nil
}
func (r *fakeProjRepo) GetByIDIncludingDeleted(context.Context, string) (*entity.Project, error) {
	return nil, nil
}
func (r *fakeProjRepo) AddMember(context.Context, *entity.ProjectMember) (*entity.ProjectMember, error) {
	return nil, nil
}
func (r *fakeProjRepo) RemoveMember(context.Context, string, string) error { return nil }
func (r *fakeProjRepo) UpdateMemberRole(context.Context, string, string, string) (*entity.ProjectMember, error) {
	return nil, nil
}
func (r *fakeProjRepo) ListMembers(context.Context, string, int32, int32) ([]*entity.ProjectMember, int64, error) {
	return nil, 0, nil
}
func (r *fakeProjRepo) GetMember(context.Context, string, string) (*entity.ProjectMember, error) {
	return nil, nil
}
func (r *fakeProjRepo) ListAllMembers(context.Context, string) ([]*entity.ProjectMember, error) {
	return nil, nil
}
func (r *fakeProjRepo) DeleteAllMembers(context.Context, string) (int, error) { return 0, nil }
func (r *fakeProjRepo) ListAllByOrgID(context.Context, string) ([]*entity.Project, error) {
	return nil, nil
}
func (r *fakeProjRepo) DeleteMembershipsByUserID(context.Context, string) (int, error) {
	return 0, nil
}

type fakeAuthZRepo struct {
	deleteTuplesCalls [][]Tuple
	deleteTuplesErr   error
}

func (r *fakeAuthZRepo) DeleteTuples(_ context.Context, tuples ...Tuple) error {
	r.deleteTuplesCalls = append(r.deleteTuplesCalls, tuples)
	return r.deleteTuplesErr
}

func (r *fakeAuthZRepo) WriteTuples(context.Context, ...Tuple) error { return nil }
func (r *fakeAuthZRepo) Check(context.Context, string, string, string, string) (bool, error) {
	return false, nil
}
func (r *fakeAuthZRepo) ListObjects(context.Context, string, string, string) ([]string, error) {
	return nil, nil
}
func (r *fakeAuthZRepo) CachedListObjects(context.Context, time.Duration, string, string, string) ([]string, error) {
	return nil, nil
}
func (r *fakeAuthZRepo) InvalidateCheck(context.Context, string, string, string, string) {}
func (r *fakeAuthZRepo) InvalidateListObjects(context.Context, string, string, string)   {}

// --- helpers ---

func newTestPurgeUserUC(userRepo *fakeUserRepo, authnRepo *fakeAuthnRepo, orgRepo *fakeOrgRepo, projRepo *fakeProjRepo, authzRepo *fakeAuthZRepo) *UserUsecase {
	return NewUserUsecase(userRepo, log.DefaultLogger, nil, authnRepo, orgRepo, projRepo, authzRepo, "tenant-root")
}

// --- tests ---

func TestPurgeUser_HappyPath(t *testing.T) {
	ur := &fakeUserRepo{}
	ar := &fakeAuthnRepo{}
	or := &fakeOrgRepo{memberships: []*entity.OrganizationMember{
		{OrganizationID: "org-1", Role: "owner"},
	}}
	pr := &fakeProjRepo{memberships: []*entity.ProjectMember{
		{ProjectID: "proj-1", Role: "member"},
	}}
	az := &fakeAuthZRepo{}

	uc := newTestPurgeUserUC(ur, ar, or, pr, az)
	ok, err := uc.PurgeUser(context.Background(), &entity.User{ID: "user-1"})
	if err != nil {
		t.Fatalf("PurgeUser() unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("PurgeUser() returned false, want true")
	}

	if len(ur.purgeCascadeCalls) != 1 || ur.purgeCascadeCalls[0] != "user-1" {
		t.Errorf("PurgeCascade calls = %v, want [user-1]", ur.purgeCascadeCalls)
	}
	if len(az.deleteTuplesCalls) != 1 {
		t.Errorf("DeleteTuples called %d times, want 1", len(az.deleteTuplesCalls))
	}
	if len(ar.deleteTokensCalls) != 1 || ar.deleteTokensCalls[0] != "user-1" {
		t.Errorf("DeleteUserRefreshTokens calls = %v, want [user-1]", ar.deleteTokensCalls)
	}
}

func TestPurgeUser_CascadeFails_StopsEarly(t *testing.T) {
	ur := &fakeUserRepo{purgeCascadeErr: errors.New("db error")}
	ar := &fakeAuthnRepo{}
	or := &fakeOrgRepo{}
	pr := &fakeProjRepo{}
	az := &fakeAuthZRepo{}

	uc := newTestPurgeUserUC(ur, ar, or, pr, az)
	ok, err := uc.PurgeUser(context.Background(), &entity.User{ID: "user-1"})
	if err == nil {
		t.Fatal("PurgeUser() expected error when PurgeCascade fails")
	}
	if ok {
		t.Fatal("PurgeUser() returned true, want false")
	}

	if len(az.deleteTuplesCalls) != 0 {
		t.Error("FGA should not be called when PurgeCascade fails")
	}
	if len(ar.deleteTokensCalls) != 0 {
		t.Error("Redis should not be called when PurgeCascade fails")
	}
}

func TestPurgeUser_FGAFails_StillSucceeds(t *testing.T) {
	ur := &fakeUserRepo{}
	ar := &fakeAuthnRepo{}
	or := &fakeOrgRepo{memberships: []*entity.OrganizationMember{
		{OrganizationID: "org-1", Role: "owner"},
	}}
	pr := &fakeProjRepo{}
	az := &fakeAuthZRepo{deleteTuplesErr: errors.New("fga error")}

	uc := newTestPurgeUserUC(ur, ar, or, pr, az)
	ok, err := uc.PurgeUser(context.Background(), &entity.User{ID: "user-1"})
	if err != nil {
		t.Fatalf("PurgeUser() should succeed even when FGA fails: %v", err)
	}
	if !ok {
		t.Fatal("PurgeUser() returned false, want true")
	}

	if len(ar.deleteTokensCalls) != 1 {
		t.Error("Redis cleanup should still be called after FGA failure")
	}
}

func TestPurgeUser_RedisFails_StillSucceeds(t *testing.T) {
	ur := &fakeUserRepo{}
	ar := &fakeAuthnRepo{deleteTokensErr: errors.New("redis error")}
	or := &fakeOrgRepo{}
	pr := &fakeProjRepo{}
	az := &fakeAuthZRepo{}

	uc := newTestPurgeUserUC(ur, ar, or, pr, az)
	ok, err := uc.PurgeUser(context.Background(), &entity.User{ID: "user-1"})
	if err != nil {
		t.Fatalf("PurgeUser() should succeed even when Redis fails: %v", err)
	}
	if !ok {
		t.Fatal("PurgeUser() returned false, want true")
	}
}

func TestPurgeUser_ExecutionOrder_DBBeforeFGABeforeRedis(t *testing.T) {
	var order []string

	or := &fakeOrgRepo{}
	pr := &fakeProjRepo{}

	ur := &orderTrackingUserRepo{order: &order}
	ar := &orderTrackingAuthnRepo{order: &order}
	az := &orderTrackingAuthZRepo{order: &order}

	uc := NewUserUsecase(ur, log.DefaultLogger, nil, ar, or, pr, az, "tenant-root")
	_, _ = uc.PurgeUser(context.Background(), &entity.User{ID: "user-1"})

	if len(order) < 3 {
		t.Fatalf("expected at least 3 operations, got %v", order)
	}
	if order[0] != "db" {
		t.Errorf("first operation = %q, want 'db'", order[0])
	}
	if order[1] != "fga" {
		t.Errorf("second operation = %q, want 'fga'", order[1])
	}
	if order[2] != "redis" {
		t.Errorf("third operation = %q, want 'redis'", order[2])
	}
}

// Order-tracking fakes that embed the base fakes

type orderTrackingUserRepo struct {
	fakeUserRepo
	order *[]string
}

func (r *orderTrackingUserRepo) PurgeCascade(_ context.Context, _ string) error {
	*r.order = append(*r.order, "db")
	return nil
}

type orderTrackingAuthnRepo struct {
	fakeAuthnRepo
	order *[]string
}

func (r *orderTrackingAuthnRepo) DeleteUserRefreshTokens(_ context.Context, _ string) error {
	*r.order = append(*r.order, "redis")
	return nil
}

type orderTrackingAuthZRepo struct {
	fakeAuthZRepo
	order *[]string
}

func (r *orderTrackingAuthZRepo) DeleteTuples(_ context.Context, _ ...Tuple) error {
	*r.order = append(*r.order, "fga")
	return nil
}
