package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"

	authnpb "github.com/Servora-Kit/servora/api/gen/go/authn/service/v1"
	"github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	userpb "github.com/Servora-Kit/servora/api/gen/go/user/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	dataent "github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/actor"
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
}

type UserUsecase struct {
	repo      UserRepo
	log       *logger.Helper
	cfg       *conf.App
	authnRepo AuthnRepo
	orgRepo   OrganizationRepo
	projRepo  ProjectRepo
	authz     AuthZRepo
	platID    string
}

func NewUserUsecase(
	repo UserRepo,
	l logger.Logger,
	cfg *conf.App,
	authnRepo AuthnRepo,
	orgRepo OrganizationRepo,
	projRepo ProjectRepo,
	authz AuthZRepo,
	platID PlatformRootID,
) *UserUsecase {
	return &UserUsecase{
		repo:      repo,
		log:       logger.NewHelper(l, logger.WithModule("user/biz/iam-service")),
		cfg:       cfg,
		authnRepo: authnRepo,
		orgRepo:   orgRepo,
		projRepo:  projRepo,
		authz:     authz,
		platID:    string(platID),
	}
}

func (uc *UserUsecase) CurrentUserInfo(ctx context.Context) (*entity.User, error) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.Type() != actor.TypeUser {
		return nil, authnpb.ErrorUnauthorized("user not authenticated")
	}

	u, err := uc.repo.GetUserById(ctx, a.ID())
	if err != nil {
		return nil, userpb.ErrorUserNotFound("user not found")
	}
	return u, nil
}

func (uc *UserUsecase) UpdateUser(ctx context.Context, user *entity.User) (*entity.User, error) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.Type() != actor.TypeUser {
		return nil, authnpb.ErrorUnauthorized("user not authenticated")
	}

	origUser, err := uc.repo.GetUserById(ctx, user.ID)
	if err != nil {
		if dataent.IsNotFound(err) {
			return nil, userpb.ErrorUserNotFound("user not found")
		}
		uc.log.Errorf("get user failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}

	callerUser, err := uc.repo.GetUserById(ctx, a.ID())
	if err != nil {
		uc.log.Errorf("get caller user failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}
	isAdmin := callerUser.Role == "admin" || callerUser.Role == "operator"
	if !isAdmin && a.ID() != user.ID {
		return nil, authnpb.ErrorUnauthorized("you can only update your own information")
	}
	if !isAdmin && user.Role != "" && user.Role != callerUser.Role {
		return nil, authnpb.ErrorUnauthorized("you do not have permission to change your role")
	}

	if user.Name != "" && user.Name != origUser.Name {
		userWithSameName, err := uc.authnRepo.GetUserByUserName(ctx, user.Name)
		if err != nil && !dataent.IsNotFound(err) {
			uc.log.Errorf("check username failed: %v", err)
			return nil, errors.InternalServer("INTERNAL", "internal error")
		}
		if userWithSameName != nil {
			return nil, authnpb.ErrorUserAlreadyExists("username already exists")
		}
	}

	if user.Email != "" && user.Email != origUser.Email {
		userWithSameEmail, err := uc.authnRepo.GetUserByEmail(ctx, user.Email)
		if err != nil && !dataent.IsNotFound(err) {
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

func (uc *UserUsecase) SaveUser(ctx context.Context, user *entity.User) (*entity.User, error) {
	if err := uc.checkUserExists(ctx, user); err != nil {
		return nil, err
	}

	savedUser, err := uc.repo.SaveUser(ctx, user)
	if err != nil {
		uc.log.Errorf("save user failed: %v", err)
		return nil, userpb.ErrorSaveUserFailed("failed to save user")
	}
	return savedUser, nil
}

func (uc *UserUsecase) ListUsers(ctx context.Context, page, pageSize int32) ([]*entity.User, int64, error) {
	users, total, err := uc.repo.ListUsers(ctx, page, pageSize)
	if err != nil {
		uc.log.Errorf("list users failed: %v", err)
		return nil, 0, errors.InternalServer("INTERNAL", "internal error")
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
	uc.purgeUserFGA(ctx, user.ID)

	if err := uc.authnRepo.DeleteUserRefreshTokens(ctx, user.ID); err != nil {
		uc.log.Warnf("delete user refresh tokens: %v", err)
	}

	if err := uc.repo.PurgeCascade(ctx, user.ID); err != nil {
		uc.log.Errorf("purge user failed: %v", err)
		return false, userpb.ErrorDeleteUserFailed("failed to delete user")
	}
	return true, nil
}

func (uc *UserUsecase) purgeUserFGA(ctx context.Context, userID string) {
	if uc.authz == nil {
		return
	}
	var tuples []Tuple

	orgMemberships, _ := uc.orgRepo.ListMembershipsByUserID(ctx, userID)
	for _, m := range orgMemberships {
		tuples = append(tuples,
			Tuple{User: "user:" + userID, Relation: m.Role, Object: "organization:" + m.OrganizationID},
			Tuple{User: "user:" + userID, Relation: "member", Object: "organization:" + m.OrganizationID},
		)
	}

	projMemberships, _ := uc.projRepo.ListMembershipsByUserID(ctx, userID)
	for _, m := range projMemberships {
		tuples = append(tuples,
			Tuple{User: "user:" + userID, Relation: m.Role, Object: "project:" + m.ProjectID},
		)
	}

	if uc.platID != "" {
		tuples = append(tuples,
			Tuple{User: "user:" + userID, Relation: "admin", Object: "platform:" + uc.platID},
		)
	}

	if len(tuples) > 0 {
		if err := uc.authz.DeleteTuples(ctx, tuples...); err != nil {
			uc.log.Warnf("purge user %s FGA tuples: %v", userID, err)
		}
	}
}

func (uc *UserUsecase) RestoreUser(ctx context.Context, id string) (*entity.User, error) {
	if _, err := uc.repo.GetUserByIdIncludingDeleted(ctx, id); err != nil {
		return nil, userpb.ErrorUserNotFound("user not found")
	}
	return uc.repo.RestoreUser(ctx, id)
}

func (uc *UserUsecase) checkUserExists(ctx context.Context, user *entity.User) error {
	existingUser, err := uc.authnRepo.GetUserByUserName(ctx, user.Name)
	if err != nil && !dataent.IsNotFound(err) {
		uc.log.Errorf("check username failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}
	if existingUser != nil {
		return authnpb.ErrorUserAlreadyExists("username already exists")
	}

	existingEmail, err := uc.authnRepo.GetUserByEmail(ctx, user.Email)
	if err != nil && !dataent.IsNotFound(err) {
		uc.log.Errorf("check email failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}
	if existingEmail != nil {
		return authnpb.ErrorUserAlreadyExists("email already exists")
	}
	return nil
}
