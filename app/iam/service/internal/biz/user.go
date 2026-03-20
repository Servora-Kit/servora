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
}

type UserUsecase struct {
	repo      UserRepo
	log       *logger.Helper
	cfg       *conf.App
	authnRepo AuthnRepo
	authnUC   *AuthnUsecase
	authz     AuthZRepo
}

func NewUserUsecase(
	repo UserRepo,
	l logger.Logger,
	cfg *conf.App,
	authnRepo AuthnRepo,
	authnUC *AuthnUsecase,
	authz AuthZRepo,
) *UserUsecase {
	return &UserUsecase{
		repo:      repo,
		log:       logger.For(l, "user/biz/iam"),
		cfg:       cfg,
		authnRepo: authnRepo,
		authnUC:   authnUC,
		authz:     authz,
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

	if user.Username != "" && user.Username != origUser.Username {
		userWithSameName, err := uc.authnRepo.GetUserByUserName(ctx, user.Username)
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

// CreateUser creates a new user in IAM.
// The created user starts with email_verified=false; a verification email is sent.
func (uc *UserUsecase) CreateUser(ctx context.Context, user *entity.User) (*entity.User, error) {
	if err := uc.checkUserExists(ctx, user); err != nil {
		return nil, err
	}

	if user.Role == "" {
		user.Role = "user"
	}
	user.EmailVerified = false

	savedUser, err := uc.repo.SaveUser(ctx, user)
	if err != nil {
		uc.log.Errorf("create user failed: %v", err)
		return nil, userpb.ErrorCreateUserFailed("failed to create user")
	}

	if uc.authnUC != nil {
		if err := uc.authnUC.SendVerificationEmail(ctx, savedUser); err != nil {
			uc.log.Warnf("send verification email failed for user %s: %v", savedUser.ID, err)
		}
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
	uc.log.Infof("PurgeUser start: user_id=%s", user.ID)

	if err := uc.repo.PurgeCascade(ctx, user.ID); err != nil {
		uc.log.Errorf("PurgeUser PurgeCascade failed: user_id=%s err=%v", user.ID, err)
		return false, userpb.ErrorDeleteUserFailed("failed to delete user")
	}

	if err := uc.authnRepo.DeleteUserRefreshTokens(ctx, user.ID); err != nil {
		uc.log.Warnf("PurgeUser Redis cleanup partial failure: user_id=%s err=%v", user.ID, err)
	}

	uc.log.Infof("PurgeUser complete: user_id=%s", user.ID)
	return true, nil
}

func (uc *UserUsecase) RestoreUser(ctx context.Context, id string) (*entity.User, error) {
	if _, err := uc.repo.GetUserByIdIncludingDeleted(ctx, id); err != nil {
		return nil, userpb.ErrorUserNotFound("user not found")
	}
	return uc.repo.RestoreUser(ctx, id)
}

func (uc *UserUsecase) checkUserExists(ctx context.Context, user *entity.User) error {
	existingUser, err := uc.authnRepo.GetUserByUserName(ctx, user.Username)
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
