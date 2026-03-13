package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"

	orgpb "github.com/Servora-Kit/servora/api/gen/go/organization/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	dataent "github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/actor"
	"github.com/Servora-Kit/servora/pkg/logger"
)

// PlatformRootID is the UUID string of the root platform record, used for Wire injection.
type PlatformRootID string

type OrganizationRepo interface {
	Create(ctx context.Context, org *entity.Organization) (*entity.Organization, error)
	GetByID(ctx context.Context, id string) (*entity.Organization, error)
	GetByIDs(ctx context.Context, ids []string, page, pageSize int32) ([]*entity.Organization, int64, error)
	GetBySlug(ctx context.Context, slug string) (*entity.Organization, error)
	ListByUserID(ctx context.Context, userID string, page, pageSize int32) ([]*entity.Organization, int64, error)
	Update(ctx context.Context, org *entity.Organization) (*entity.Organization, error)
	Delete(ctx context.Context, id string) error
	Purge(ctx context.Context, id string) error
	PurgeCascade(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) (*entity.Organization, error)
	GetByIDIncludingDeleted(ctx context.Context, id string) (*entity.Organization, error)
	AddMember(ctx context.Context, m *entity.OrganizationMember) (*entity.OrganizationMember, error)
	RemoveMember(ctx context.Context, orgID, userID string) error
	ListMembers(ctx context.Context, orgID string, page, pageSize int32) ([]*entity.OrganizationMember, int64, error)
	GetMember(ctx context.Context, orgID, userID string) (*entity.OrganizationMember, error)
	UpdateMemberRole(ctx context.Context, orgID, userID, role string) (*entity.OrganizationMember, error)
	ListAllMembers(ctx context.Context, orgID string) ([]*entity.OrganizationMember, error)
	DeleteAllMembers(ctx context.Context, orgID string) (int, error)
	ListMembershipsByUserID(ctx context.Context, userID string) ([]*entity.OrganizationMember, error)
	DeleteMembershipsByUserID(ctx context.Context, userID string) (int, error)
}

type OrganizationUsecase struct {
	repo     OrganizationRepo
	projRepo ProjectRepo
	authz    AuthZRepo
	log      *logger.Helper
	platID   string
}

func NewOrganizationUsecase(repo OrganizationRepo, projRepo ProjectRepo, authz AuthZRepo, l logger.Logger, platID PlatformRootID) *OrganizationUsecase {
	return &OrganizationUsecase{
		repo:     repo,
		projRepo: projRepo,
		authz:    authz,
		log:      logger.NewHelper(l, logger.WithModule("organization/biz/iam-service")),
		platID:   string(platID),
	}
}

func (uc *OrganizationUsecase) Create(ctx context.Context, org *entity.Organization) (*entity.Organization, error) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.Type() != actor.TypeUser {
		return nil, orgpb.ErrorOrganizationCreateFailed("user not authenticated")
	}
	userID := a.ID()

	if _, err := uc.repo.GetBySlug(ctx, org.Slug); err == nil {
		return nil, orgpb.ErrorOrganizationAlreadyExists("slug '%s' already taken", org.Slug)
	} else if !dataent.IsNotFound(err) {
		uc.log.Errorf("check slug failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}

	org.PlatformID = uc.platID
	created, err := uc.repo.Create(ctx, org)
	if err != nil {
		uc.log.Errorf("create organization failed: %v", err)
		return nil, orgpb.ErrorOrganizationCreateFailed("failed to create organization")
	}

	if uc.authz != nil {
		_ = uc.authz.WriteTuples(ctx,
			Tuple{User: "platform:" + uc.platID, Relation: "platform", Object: "organization:" + created.ID},
			Tuple{User: "user:" + userID, Relation: "owner", Object: "organization:" + created.ID},
		)
		uc.authz.InvalidateListObjects(ctx, userID, "can_view", "organization")
	}

	if _, err := uc.repo.AddMember(ctx, &entity.OrganizationMember{
		OrganizationID: created.ID,
		UserID:         userID,
		Role:           "owner",
	}); err != nil {
		uc.log.Warnf("auto-add creator as owner failed: %v", err)
	}

	return created, nil
}

func (uc *OrganizationUsecase) CreateDefault(ctx context.Context, userID, name, slug string) (*entity.Organization, error) {
	org := &entity.Organization{
		PlatformID:  uc.platID,
		Name:        name,
		Slug:        slug,
		DisplayName: name,
	}
	created, err := uc.repo.Create(ctx, org)
	if err != nil {
		return nil, err
	}

	if uc.authz != nil {
		_ = uc.authz.WriteTuples(ctx,
			Tuple{User: "platform:" + uc.platID, Relation: "platform", Object: "organization:" + created.ID},
			Tuple{User: "user:" + userID, Relation: "owner", Object: "organization:" + created.ID},
		)
		uc.authz.InvalidateListObjects(ctx, userID, "can_view", "organization")
	}

	if _, err := uc.repo.AddMember(ctx, &entity.OrganizationMember{
		OrganizationID: created.ID,
		UserID:         userID,
		Role:           "owner",
	}); err != nil {
		uc.log.Warnf("auto-add owner failed: %v", err)
	}

	return created, nil
}

func (uc *OrganizationUsecase) Get(ctx context.Context, id string) (*entity.Organization, error) {
	org, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		if dataent.IsNotFound(err) {
			return nil, orgpb.ErrorOrganizationNotFound("organization %s not found", id)
		}
		return nil, err
	}
	return org, nil
}

func (uc *OrganizationUsecase) List(ctx context.Context, page, pageSize int32) ([]*entity.Organization, int64, error) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.Type() != actor.TypeUser {
		return nil, 0, orgpb.ErrorOrganizationNotFound("user not authenticated")
	}

	if uc.authz != nil {
		ids, err := uc.authz.CachedListObjects(ctx, DefaultListCacheTTL, a.ID(), "can_view", "organization")
		if err != nil {
			uc.log.Warnf("ListObjects fallback to DB: %v", err)
			return uc.repo.ListByUserID(ctx, a.ID(), page, pageSize)
		}
		return uc.repo.GetByIDs(ctx, ids, page, pageSize)
	}

	return uc.repo.ListByUserID(ctx, a.ID(), page, pageSize)
}

func (uc *OrganizationUsecase) Update(ctx context.Context, org *entity.Organization) (*entity.Organization, error) {
	updated, err := uc.repo.Update(ctx, org)
	if err != nil {
		if dataent.IsNotFound(err) {
			return nil, orgpb.ErrorOrganizationNotFound("organization %s not found", org.ID)
		}
		uc.log.Errorf("update organization failed: %v", err)
		return nil, orgpb.ErrorOrganizationUpdateFailed("failed to update organization")
	}
	return updated, nil
}

func (uc *OrganizationUsecase) Delete(ctx context.Context, id string) error {
	if _, err := uc.repo.GetByID(ctx, id); err != nil {
		if dataent.IsNotFound(err) {
			return orgpb.ErrorOrganizationNotFound("organization %s not found", id)
		}
		uc.log.Errorf("get organization failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		uc.log.Errorf("soft delete organization failed: %v", err)
		return orgpb.ErrorOrganizationDeleteFailed("failed to delete organization")
	}
	return nil
}

func (uc *OrganizationUsecase) Purge(ctx context.Context, id string) error {
	if _, err := uc.repo.GetByID(ctx, id); err != nil {
		if dataent.IsNotFound(err) {
			return orgpb.ErrorOrganizationNotFound("organization %s not found", id)
		}
		uc.log.Errorf("get organization failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}

	uc.purgeOrgFGA(ctx, id)

	if err := uc.repo.PurgeCascade(ctx, id); err != nil {
		uc.log.Errorf("purge organization failed: %v", err)
		return orgpb.ErrorOrganizationDeleteFailed("failed to delete organization")
	}
	return nil
}

func (uc *OrganizationUsecase) Restore(ctx context.Context, id string) (*entity.Organization, error) {
	if _, err := uc.repo.GetByIDIncludingDeleted(ctx, id); err != nil {
		if dataent.IsNotFound(err) {
			return nil, orgpb.ErrorOrganizationNotFound("organization %s not found", id)
		}
		return nil, err
	}
	return uc.repo.Restore(ctx, id)
}

func (uc *OrganizationUsecase) purgeOrgFGA(ctx context.Context, orgID string) {
	if uc.authz == nil {
		return
	}
	var tuples []Tuple

	projects, _ := uc.projRepo.ListAllByOrgID(ctx, orgID)
	for _, p := range projects {
		projMembers, _ := uc.projRepo.ListAllMembers(ctx, p.ID)
		for _, m := range projMembers {
			tuples = append(tuples,
				Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "project:" + p.ID},
			)
		}
		tuples = append(tuples,
			Tuple{User: "organization:" + p.OrganizationID, Relation: "organization", Object: "project:" + p.ID},
		)
	}

	members, _ := uc.repo.ListAllMembers(ctx, orgID)
	for _, m := range members {
		tuples = append(tuples,
			Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "organization:" + orgID},
		)
	}
	tuples = append(tuples,
		Tuple{User: "platform:" + uc.platID, Relation: "platform", Object: "organization:" + orgID},
	)

	if err := uc.authz.DeleteTuples(ctx, tuples...); err != nil {
		uc.log.Warnf("purge org %s FGA tuples: %v", orgID, err)
	}
}

func (uc *OrganizationUsecase) AddMember(ctx context.Context, m *entity.OrganizationMember) (*entity.OrganizationMember, error) {
	if err := ValidateOrganizationRole(m.Role); err != nil {
		return nil, orgpb.ErrorOrganizationCreateFailed("%v", err)
	}

	if _, err := uc.repo.GetMember(ctx, m.OrganizationID, m.UserID); err == nil {
		return nil, orgpb.ErrorOrganizationMemberAlreadyExists("user is already a member")
	}

	created, err := uc.repo.AddMember(ctx, m)
	if err != nil {
		return nil, err
	}

	if uc.authz != nil {
		_ = uc.authz.WriteTuples(ctx,
			Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "organization:" + m.OrganizationID},
		)
		uc.authz.InvalidateListObjects(ctx, m.UserID, "can_view", "organization")
	}
	return created, nil
}

func (uc *OrganizationUsecase) RemoveMember(ctx context.Context, orgID, userID string) error {
	member, err := uc.repo.GetMember(ctx, orgID, userID)
	if err != nil {
		return orgpb.ErrorOrganizationMemberNotFound("member not found")
	}

	if err := uc.repo.RemoveMember(ctx, orgID, userID); err != nil {
		return err
	}

	if uc.authz != nil {
		_ = uc.authz.DeleteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: member.Role, Object: "organization:" + orgID},
		)
		uc.authz.InvalidateListObjects(ctx, userID, "can_view", "organization")
	}
	return nil
}

func (uc *OrganizationUsecase) ListMembers(ctx context.Context, orgID string, page, pageSize int32) ([]*entity.OrganizationMember, int64, error) {
	return uc.repo.ListMembers(ctx, orgID, page, pageSize)
}

func (uc *OrganizationUsecase) UpdateMemberRole(ctx context.Context, orgID, userID, newRole string) (*entity.OrganizationMember, error) {
	if err := ValidateOrganizationRole(newRole); err != nil {
		return nil, orgpb.ErrorOrganizationCreateFailed("%v", err)
	}

	oldMember, err := uc.repo.GetMember(ctx, orgID, userID)
	if err != nil {
		return nil, orgpb.ErrorOrganizationMemberNotFound("member not found")
	}

	updated, err := uc.repo.UpdateMemberRole(ctx, orgID, userID, newRole)
	if err != nil {
		return nil, err
	}

	if uc.authz != nil && oldMember.Role != newRole {
		_ = uc.authz.DeleteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: oldMember.Role, Object: "organization:" + orgID},
		)
		_ = uc.authz.WriteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: newRole, Object: "organization:" + orgID},
		)
	}
	return updated, nil
}
