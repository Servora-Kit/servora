package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"

	authnpb "github.com/Servora-Kit/servora/api/gen/go/authn/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/pkg/actor"
)

type AuthnService struct {
	authnpb.UnimplementedAuthnServiceServer

	uc *biz.AuthnUsecase
}

func NewAuthnService(uc *biz.AuthnUsecase) *AuthnService {
	return &AuthnService{uc: uc}
}

func (s *AuthnService) SignupByEmail(ctx context.Context, req *authnpb.SignupByEmailRequest) (*authnpb.SignupByEmailResponse, error) {
	if req.Password != req.PasswordConfirm {
		return nil, errors.BadRequest("INVALID_REQUEST", "password and confirm password do not match")
	}
	user, err := s.uc.SignupByEmail(ctx, &entity.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	return &authnpb.SignupByEmailResponse{
		Id:    user.ID,
		Name:  user.Name,
		Email: user.Email,
		Role:  user.Role,
	}, nil
}

func (s *AuthnService) LoginByEmailPassword(ctx context.Context, req *authnpb.LoginByEmailPasswordRequest) (*authnpb.LoginByEmailPasswordResponse, error) {
	user := &entity.User{
		Email:    req.Email,
		Password: req.Password,
	}
	tokenPair, err := s.uc.LoginByEmailPassword(ctx, user)
	if err != nil {
		return nil, err
	}
	return &authnpb.LoginByEmailPasswordResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

func (s *AuthnService) RefreshToken(ctx context.Context, req *authnpb.RefreshTokenRequest) (*authnpb.RefreshTokenResponse, error) {
	tokenPair, err := s.uc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	return &authnpb.RefreshTokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

func (s *AuthnService) Logout(ctx context.Context, req *authnpb.LogoutRequest) (*authnpb.LogoutResponse, error) {
	if err := s.uc.Logout(ctx, req.RefreshToken); err != nil {
		return nil, err
	}
	return &authnpb.LogoutResponse{
		Success: true,
	}, nil
}

func (s *AuthnService) ChangePassword(ctx context.Context, req *authnpb.ChangePasswordRequest) (*authnpb.ChangePasswordResponse, error) {
	if req.NewPassword != req.NewPasswordConfirm {
		return nil, errors.BadRequest("INVALID_REQUEST", "new password and confirm password do not match")
	}

	a, ok := actor.FromContext(ctx)
	if !ok || a.Type() != actor.TypeUser {
		return nil, errors.Unauthorized("UNAUTHORIZED", "user not authenticated")
	}

	if err := s.uc.ChangePassword(ctx, a.ID(), req.CurrentPassword, req.NewPassword); err != nil {
		return nil, err
	}
	return &authnpb.ChangePasswordResponse{Success: true}, nil
}

func (s *AuthnService) LogoutAllDevices(ctx context.Context, _ *authnpb.LogoutAllDevicesRequest) (*authnpb.LogoutAllDevicesResponse, error) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.Type() != actor.TypeUser {
		return nil, errors.Unauthorized("UNAUTHORIZED", "user not authenticated")
	}

	if err := s.uc.LogoutAllDevices(ctx, a.ID()); err != nil {
		return nil, err
	}
	return &authnpb.LogoutAllDevicesResponse{Success: true}, nil
}
