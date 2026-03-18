package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"

	authnpb "github.com/Servora-Kit/servora/api/gen/go/authn/service/v1"
	"github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	userpb "github.com/Servora-Kit/servora/api/gen/go/user/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type UserRepo interface {
	SaveUser(context.Context, *entity.User) (*entity.User, error)
	GetUserById(context.Context, string) (*entity.User, error)
	DeleteUser(context.Context, *entity.User) (*entity.User, error)
	PurgeUser(context.Context, *entity.User) (*entity.User, error)
	PurgeCascade(ctx context.Context, id string) error
	RestoreUser(context.Context, string) (*entity.User, error)
	GetUserByIdIncludingDeleted(context.Context, string) (*entity.User, error)
	UpdateUser(context.Context, *entity.User) (*entity.User, error)
	ListUsers(context.Context, int32, int32) ([]*entity.User, int64, error)
	ListByTenantID(context.Context, string, int32, int32) ([]*entity.User, int64, error)
}

type UserUsecase struct {
	repo       UserRepo
	log        *logger.Helper
	cfg        *conf.App
	authnRepo  AuthnRepo
	orgRepo    OrganizationRepo
	tenantRepo TenantRepo
	authz      AuthZRepo
}

func NewUserUsecase(
	repo UserRepo,
	l logger.Logger,
	cfg *conf.App,
	authnRepo AuthnRepo,
	orgRepo OrganizationRepo,
	tenantRepo TenantRepo,
	authz AuthZRepo,
) *UserUsecase {
	return &UserUsecase{
		repo:       repo,
		log:        logger.NewHelper(l, logger.WithModule("user/biz/iam-service")),
		cfg:        cfg,
		authnRepo:  authnRepo,
		orgRepo:    orgRepo,
		tenantRepo: tenantRepo,
		authz:      authz,
	}
}

func (uc *UserUsecase) CurrentUserInfo(ctx context.Context, callerID string) (*entity.User, error) {
	if callerID == "" {
		return nil, authnpb.ErrorUnauthorized("user not authenticated")
	}
	u, err := uc.repo.GetUserById(ctx, callerID)
	if err != nil {
		return nil, userpb.ErrorUserNotFound("user not found")
	}
	return u, nil
}

func (uc *UserUsecase) GetUser(ctx context.Context, id string) (*entity.User, error) {
	u, err := uc.repo.GetUserById(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, userpb.ErrorUserNotFound("user not found")
		}
		uc.log.Errorf("get user by id failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}
	return u, nil
}

func (uc *UserUsecase) UpdateUser(ctx context.Context, callerID string, user *entity.User) (*entity.User, error) {
	if callerID == "" {
		return nil, authnpb.ErrorUnauthorized("user not authenticated")
	}

	origUser, err := uc.repo.GetUserById(ctx, user.ID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, userpb.ErrorUserNotFound("user not found")
		}
		uc.log.Errorf("get user failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}

	if callerID != user.ID {
		return nil, authnpb.ErrorUnauthorized("you can only update your own information")
	}

	if user.Name != "" && user.Name != origUser.Name {
		userWithSameName, err := uc.authnRepo.GetUserByUserName(ctx, user.Name)
		if err != nil && !ent.IsNotFound(err) {
			uc.log.Errorf("check username failed: %v", err)
			return nil, errors.InternalServer("INTERNAL", "internal error")
		}
		if userWithSameName != nil {
			return nil, authnpb.ErrorUserAlreadyExists("username already exists")
		}
	}

	if user.Email != "" && user.Email != origUser.Email {
		userWithSameEmail, err := uc.authnRepo.GetUserByEmail(ctx, user.Email)
		if err != nil && !ent.IsNotFound(err) {
			uc.log.Errorf("check email failed: %v", err)
			return nil, errors.InternalServer("INTERNAL", "internal error")
		}
		if userWithSameEmail != nil {
			return nil, authnpb.ErrorUserAlreadyExists("email already exists")
		}
	}

	updatedUser, err := uc.repo.UpdateUser(ctx, user)
	if err != nil {
		uc.log.Errorf("update user failed: %v", err)
		return nil, userpb.ErrorUpdateUserFailed("failed to update user")
	}
	return updatedUser, nil
}

// CreateUser creates a new user and adds them to the specified tenant and organization.
// The user is also given a personal tenant/org space.
func (uc *UserUsecase) CreateUser(ctx context.Context, tenantID, organizationID string, user *entity.User) (*entity.User, error) {
	if tenantID == "" {
		return nil, userpb.ErrorCreateUserFailed("tenant_id is required")
	}
	if organizationID == "" {
		return nil, userpb.ErrorCreateUserFailed("organization_id is required")
	}

	org, err := uc.orgRepo.GetByID(ctx, organizationID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, userpb.ErrorCreateUserFailed("organization not found")
		}
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}
	if org.TenantID != tenantID {
		return nil, userpb.ErrorCreateUserFailed("organization does not belong to the specified tenant")
	}

	if err := uc.checkUserExists(ctx, user); err != nil {
		return nil, err
	}

	user.EmailVerified = true

	savedUser, err := uc.repo.SaveUser(ctx, user)
	if err != nil {
		uc.log.Errorf("create user failed: %v", err)
		return nil, userpb.ErrorCreateUserFailed("failed to create user")
	}

	if _, err := uc.tenantRepo.AddMember(ctx, &entity.TenantMember{
		TenantID: tenantID,
		UserID:   savedUser.ID,
		Role:     string(RoleMember),
		Status:   "active",
	}); err != nil {
		uc.log.Errorf("add user to tenant failed: %v", err)
	}

	if _, err := uc.orgRepo.AddMember(ctx, &entity.OrganizationMember{
		OrganizationID: organizationID,
		UserID:         savedUser.ID,
		Role:           string(RoleMember),
	}); err != nil {
		uc.log.Errorf("add user to organization failed: %v", err)
	}

	if uc.authz != nil {
		_ = uc.authz.WriteTuples(ctx,
			Tuple{User: "user:" + savedUser.ID, Relation: string(RoleMember), Object: "tenant:" + tenantID},
			Tuple{User: "user:" + savedUser.ID, Relation: string(RoleMember), Object: "organization:" + organizationID},
		)
	}

	return savedUser, nil
}

func (uc *UserUsecase) ListUsers(ctx context.Context, tenantID string, page, pageSize int32) ([]*entity.User, int64, error) {
	var users []*entity.User
	var total int64
	var err error

	if tenantID != "" {
		users, total, err = uc.repo.ListByTenantID(ctx, tenantID, page, pageSize)
	} else {
		users, total, err = uc.repo.ListUsers(ctx, page, pageSize)
	}
	if err != nil {
		uc.log.Errorf("list users failed: %v", err)
		return nil, 0, errors.InternalServer("INTERNAL", "internal error")
	}

	// Enrich users with their org memberships within the current tenant scope.
	if tenantID != "" && len(users) > 0 {
		userIDs := make([]string, len(users))
		for i, u := range users {
			userIDs[i] = u.ID
		}
		orgMap, enrichErr := uc.orgRepo.ListOrgMembershipsByUserIDs(ctx, tenantID, userIDs)
		if enrichErr != nil {
			uc.log.Warnf("enrich user org memberships failed: %v", enrichErr)
		} else {
			for _, u := range users {
				u.OrganizationIDs = orgMap[u.ID]
			}
		}
	}

	return users, total, nil
}

func (uc *UserUsecase) DeleteUser(ctx context.Context, user *entity.User) (bool, error) {
	if _, err := uc.repo.GetUserById(ctx, user.ID); err != nil {
		return false, userpb.ErrorUserNotFound("user not found")
	}
	if _, err := uc.repo.DeleteUser(ctx, user); err != nil {
		uc.log.Errorf("soft delete user failed: %v", err)
		return false, userpb.ErrorDeleteUserFailed("failed to delete user")
	}
	return true, nil
}

func (uc *UserUsecase) PurgeUser(ctx context.Context, user *entity.User) (bool, error) {
	uc.log.Infof("PurgeUser start: user_id=%s", user.ID)

	tuples := uc.collectUserFGATuples(ctx, user.ID)
	uc.log.Infof("PurgeUser collected %d FGA tuples: user_id=%s", len(tuples), user.ID)

	if err := uc.repo.PurgeCascade(ctx, user.ID); err != nil {
		uc.log.Errorf("PurgeUser PurgeCascade failed: user_id=%s err=%v", user.ID, err)
		return false, userpb.ErrorDeleteUserFailed("failed to delete user")
	}
	uc.log.Infof("PurgeUser PurgeCascade done: user_id=%s", user.ID)

	uc.deleteUserFGATuples(ctx, user.ID, tuples)
	uc.log.Infof("PurgeUser FGA cleanup done: user_id=%s", user.ID)

	if err := uc.authnRepo.DeleteUserRefreshTokens(ctx, user.ID); err != nil {
		uc.log.Warnf("PurgeUser Redis cleanup partial failure: user_id=%s err=%v", user.ID, err)
	} else {
		uc.log.Infof("PurgeUser Redis cleanup done: user_id=%s", user.ID)
	}

	uc.log.Infof("PurgeUser complete: user_id=%s", user.ID)
	return true, nil
}

// collectUserFGATuples builds the FGA tuple list from current DB memberships.
// Must be called BEFORE PurgeCascade so the membership rows still exist.
func (uc *UserUsecase) collectUserFGATuples(ctx context.Context, userID string) []Tuple {
	var tuples []Tuple

	tenantMemberships, _ := uc.tenantRepo.ListMembershipsByUserID(ctx, userID)
	for _, m := range tenantMemberships {
		tuples = append(tuples,
			Tuple{User: "user:" + userID, Relation: m.Role, Object: "tenant:" + m.TenantID},
		)
	}

	orgMemberships, _ := uc.orgRepo.ListMembershipsByUserID(ctx, userID)
	for _, m := range orgMemberships {
		tuples = append(tuples,
			Tuple{User: "user:" + userID, Relation: m.Role, Object: "organization:" + m.OrganizationID},
		)
	}

	return tuples
}

func (uc *UserUsecase) deleteUserFGATuples(ctx context.Context, userID string, tuples []Tuple) {
	if uc.authz == nil || len(tuples) == 0 {
		return
	}
	if err := uc.authz.DeleteTuples(ctx, tuples...); err != nil {
		uc.log.Warnf("purge user %s FGA tuples: %v", userID, err)
	}
}

// CompensateUserPurge cleans up residual FGA tuples and Redis refresh tokens
// for a user whose DB records have already been deleted by PurgeCascade.
// It queries FGA directly (via ListObjects) to discover remaining tuples.
func (uc *UserUsecase) CompensateUserPurge(ctx context.Context, userID string) error {
	uc.log.Infof("CompensateUserPurge start: user_id=%s", userID)

	var tuples []Tuple

	if uc.authz != nil {
		orgRelations := []string{"owner", "admin", "member", "viewer"}

		for _, rel := range orgRelations {
			objects, err := uc.authz.ListObjects(ctx, userID, rel, "organization")
			if err != nil {
				uc.log.Warnf("CompensateUserPurge ListObjects(organization/%s) failed: %v", rel, err)
				continue
			}
			for _, obj := range objects {
				tuples = append(tuples, Tuple{User: "user:" + userID, Relation: rel, Object: obj})
			}
		}

		tenantRelations := []string{"owner", "admin", "member"}
		for _, rel := range tenantRelations {
			objects, err := uc.authz.ListObjects(ctx, userID, rel, "tenant")
			if err != nil {
				uc.log.Warnf("CompensateUserPurge ListObjects(tenant/%s) failed: %v", rel, err)
				continue
			}
			for _, obj := range objects {
				tuples = append(tuples, Tuple{User: "user:" + userID, Relation: rel, Object: obj})
			}
		}

		if len(tuples) > 0 {
			if err := uc.authz.DeleteTuples(ctx, tuples...); err != nil {
				uc.log.Errorf("CompensateUserPurge DeleteTuples failed: user_id=%s err=%v", userID, err)
				return err
			}
			uc.log.Infof("CompensateUserPurge deleted %d FGA tuples: user_id=%s", len(tuples), userID)
		} else {
			uc.log.Infof("CompensateUserPurge no FGA tuples found: user_id=%s", userID)
		}
	}

	if err := uc.authnRepo.DeleteUserRefreshTokens(ctx, userID); err != nil {
		uc.log.Warnf("CompensateUserPurge Redis cleanup failed: user_id=%s err=%v", userID, err)
		return err
	}
	uc.log.Infof("CompensateUserPurge Redis cleanup done: user_id=%s", userID)

	uc.log.Infof("CompensateUserPurge complete: user_id=%s", userID)
	return nil
}

func (uc *UserUsecase) RestoreUser(ctx context.Context, id string) (*entity.User, error) {
	if _, err := uc.repo.GetUserByIdIncludingDeleted(ctx, id); err != nil {
		return nil, userpb.ErrorUserNotFound("user not found")
	}
	return uc.repo.RestoreUser(ctx, id)
}

func (uc *UserUsecase) checkUserExists(ctx context.Context, user *entity.User) error {
	existingUser, err := uc.authnRepo.GetUserByUserName(ctx, user.Name)
	if err != nil && !ent.IsNotFound(err) {
		uc.log.Errorf("check username failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}
	if existingUser != nil {
		return authnpb.ErrorUserAlreadyExists("username already exists")
	}

	existingEmail, err := uc.authnRepo.GetUserByEmail(ctx, user.Email)
	if err != nil && !ent.IsNotFound(err) {
		uc.log.Errorf("check email failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}
	if existingEmail != nil {
		return authnpb.ErrorUserAlreadyExists("email already exists")
	}
	return nil
}
