package biz

import (
	"context"

	orgpb "github.com/Servora-Kit/servora/api/gen/go/organization/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	dataent "github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/actor"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/openfga"
	"github.com/Servora-Kit/servora/pkg/redis"
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
	fga      *openfga.Client
	redis    *redis.Client
	log      *logger.Helper
	platID   string
}

func NewOrganizationUsecase(repo OrganizationRepo, projRepo ProjectRepo, fga *openfga.Client, rdb *redis.Client, l logger.Logger, platID PlatformRootID) *OrganizationUsecase {
	return &OrganizationUsecase{
		repo:     repo,
		projRepo: projRepo,
		fga:      fga,
		redis:    rdb,
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
		return nil, orgpb.ErrorOrganizationCreateFailed("check slug: %v", err)
	}

	org.PlatformID = uc.platID
	created, err := uc.repo.Create(ctx, org)
	if err != nil {
		return nil, orgpb.ErrorOrganizationCreateFailed("create: %v", err)
	}

	if uc.fga != nil {
		_ = uc.fga.WriteTuples(ctx,
			openfga.Tuple{User: "platform:" + uc.platID, Relation: "platform", Object: "organization:" + created.ID},
			openfga.Tuple{User: "user:" + userID, Relation: "owner", Object: "organization:" + created.ID},
			openfga.Tuple{User: "user:" + userID, Relation: "member", Object: "organization:" + created.ID},
		)
		openfga.InvalidateListObjects(ctx, uc.redis, userID, "can_view", "organization")
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

	if uc.fga != nil {
		_ = uc.fga.WriteTuples(ctx,
			openfga.Tuple{User: "platform:" + uc.platID, Relation: "platform", Object: "organization:" + created.ID},
			openfga.Tuple{User: "user:" + userID, Relation: "owner", Object: "organization:" + created.ID},
			openfga.Tuple{User: "user:" + userID, Relation: "member", Object: "organization:" + created.ID},
		)
		openfga.InvalidateListObjects(ctx, uc.redis, userID, "can_view", "organization")
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

	if uc.fga != nil {
		ids, err := uc.fga.CachedListObjects(ctx, uc.redis, openfga.DefaultListCacheTTL,
			a.ID(), "can_view", "organization")
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
		return nil, orgpb.ErrorOrganizationUpdateFailed("update: %v", err)
	}
	return updated, nil
}

func (uc *OrganizationUsecase) Delete(ctx context.Context, id string) error {
	if _, err := uc.repo.GetByID(ctx, id); err != nil {
		if dataent.IsNotFound(err) {
			return orgpb.ErrorOrganizationNotFound("organization %s not found", id)
		}
		return orgpb.ErrorOrganizationDeleteFailed("get: %v", err)
	}

	// Cascade: delete child projects (each handles its own FGA cleanup)
	projects, err := uc.projRepo.ListAllByOrgID(ctx, id)
	if err == nil {
		for _, p := range projects {
			uc.deleteProjectCascade(ctx, p)
		}
	}

	// Clean up organization member FGA tuples
	members, _ := uc.repo.ListAllMembers(ctx, id)
	if uc.fga != nil && len(members) > 0 {
		var tuples []openfga.Tuple
		for _, m := range members {
			tuples = append(tuples,
				openfga.Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "organization:" + id},
				openfga.Tuple{User: "user:" + m.UserID, Relation: "member", Object: "organization:" + id},
			)
		}
		tuples = append(tuples,
			openfga.Tuple{User: "platform:" + uc.platID, Relation: "platform", Object: "organization:" + id},
		)
		if err := uc.fga.DeleteTuples(ctx, tuples...); err != nil {
			uc.log.Warnf("delete org FGA tuples: %v", err)
		}
	}

	if _, err := uc.repo.DeleteAllMembers(ctx, id); err != nil {
		uc.log.Warnf("delete org members: %v", err)
	}

	if err := uc.repo.Delete(ctx, id); err != nil {
		return orgpb.ErrorOrganizationDeleteFailed("delete: %v", err)
	}
	return nil
}

// deleteProjectCascade handles FGA + member cleanup for a single project during org cascade delete.
func (uc *OrganizationUsecase) deleteProjectCascade(ctx context.Context, proj *entity.Project) {
	projMembers, _ := uc.projRepo.ListAllMembers(ctx, proj.ID)
	if uc.fga != nil {
		var tuples []openfga.Tuple
		for _, m := range projMembers {
			tuples = append(tuples,
				openfga.Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "project:" + proj.ID},
			)
		}
		tuples = append(tuples,
			openfga.Tuple{User: "organization:" + proj.OrganizationID, Relation: "organization", Object: "project:" + proj.ID},
		)
		if err := uc.fga.DeleteTuples(ctx, tuples...); err != nil {
			uc.log.Warnf("cascade delete project %s FGA tuples: %v", proj.ID, err)
		}
	}
	if _, err := uc.projRepo.DeleteAllMembers(ctx, proj.ID); err != nil {
		uc.log.Warnf("cascade delete project %s members: %v", proj.ID, err)
	}
	if err := uc.projRepo.Delete(ctx, proj.ID); err != nil {
		uc.log.Warnf("cascade delete project %s: %v", proj.ID, err)
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

	if uc.fga != nil {
		_ = uc.fga.WriteTuples(ctx,
			openfga.Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "organization:" + m.OrganizationID},
			openfga.Tuple{User: "user:" + m.UserID, Relation: "member", Object: "organization:" + m.OrganizationID},
		)
		openfga.InvalidateListObjects(ctx, uc.redis, m.UserID, "can_view", "organization")
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

	if uc.fga != nil {
		_ = uc.fga.DeleteTuples(ctx,
			openfga.Tuple{User: "user:" + userID, Relation: member.Role, Object: "organization:" + orgID},
			openfga.Tuple{User: "user:" + userID, Relation: "member", Object: "organization:" + orgID},
		)
		openfga.InvalidateListObjects(ctx, uc.redis, userID, "can_view", "organization")
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

	if uc.fga != nil && oldMember.Role != newRole {
		_ = uc.fga.DeleteTuples(ctx,
			openfga.Tuple{User: "user:" + userID, Relation: oldMember.Role, Object: "organization:" + orgID},
		)
		_ = uc.fga.WriteTuples(ctx,
			openfga.Tuple{User: "user:" + userID, Relation: newRole, Object: "organization:" + orgID},
		)
	}
	return updated, nil
}
