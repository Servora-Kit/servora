package biz

import (
	"context"

	projectpb "github.com/Servora-Kit/servora/api/gen/go/project/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	dataent "github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/actor"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/openfga"
	"github.com/Servora-Kit/servora/pkg/redis"
)

type ProjectRepo interface {
	Create(ctx context.Context, p *entity.Project) (*entity.Project, error)
	GetByID(ctx context.Context, id string) (*entity.Project, error)
	GetByIDs(ctx context.Context, ids []string, page, pageSize int32) ([]*entity.Project, int64, error)
	ListByOrgID(ctx context.Context, orgID string, page, pageSize int32) ([]*entity.Project, int64, error)
	Update(ctx context.Context, p *entity.Project) (*entity.Project, error)
	Delete(ctx context.Context, id string) error
	AddMember(ctx context.Context, m *entity.ProjectMember) (*entity.ProjectMember, error)
	RemoveMember(ctx context.Context, projID, userID string) error
	ListMembers(ctx context.Context, projID string, page, pageSize int32) ([]*entity.ProjectMember, int64, error)
	GetMember(ctx context.Context, projID, userID string) (*entity.ProjectMember, error)
	UpdateMemberRole(ctx context.Context, projID, userID, role string) (*entity.ProjectMember, error)
	ListAllMembers(ctx context.Context, projID string) ([]*entity.ProjectMember, error)
	DeleteAllMembers(ctx context.Context, projID string) (int, error)
	ListAllByOrgID(ctx context.Context, orgID string) ([]*entity.Project, error)
	ListMembershipsByUserID(ctx context.Context, userID string) ([]*entity.ProjectMember, error)
	DeleteMembershipsByUserID(ctx context.Context, userID string) (int, error)
}

type ProjectUsecase struct {
	repo    ProjectRepo
	orgRepo OrganizationRepo
	fga     *openfga.Client
	redis   *redis.Client
	log     *logger.Helper
}

func NewProjectUsecase(repo ProjectRepo, orgRepo OrganizationRepo, fga *openfga.Client, rdb *redis.Client, l logger.Logger) *ProjectUsecase {
	return &ProjectUsecase{
		repo:    repo,
		orgRepo: orgRepo,
		fga:     fga,
		redis:   rdb,
		log:     logger.NewHelper(l, logger.WithModule("project/biz/iam-service")),
	}
}

func (uc *ProjectUsecase) Create(ctx context.Context, p *entity.Project) (*entity.Project, error) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.Type() != actor.TypeUser {
		return nil, projectpb.ErrorProjectCreateFailed("user not authenticated")
	}
	userID := a.ID()

	created, err := uc.repo.Create(ctx, p)
	if err != nil {
		return nil, projectpb.ErrorProjectCreateFailed("create: %v", err)
	}

	if uc.fga != nil {
		_ = uc.fga.WriteTuples(ctx,
			openfga.Tuple{User: "organization:" + p.OrganizationID, Relation: "organization", Object: "project:" + created.ID},
			openfga.Tuple{User: "user:" + userID, Relation: "admin", Object: "project:" + created.ID},
		)
		openfga.InvalidateListObjects(ctx, uc.redis, userID, "can_view", "project")
	}

	if _, err := uc.repo.AddMember(ctx, &entity.ProjectMember{
		ProjectID: created.ID,
		UserID:    userID,
		Role:      "admin",
	}); err != nil {
		uc.log.Warnf("auto-add creator as admin failed: %v", err)
	}

	return created, nil
}

func (uc *ProjectUsecase) CreateDefault(ctx context.Context, userID, orgID, name, slug string) (*entity.Project, error) {
	p := &entity.Project{
		OrganizationID: orgID,
		Name:           name,
		Slug:           slug,
		Description:    "Default project",
	}
	created, err := uc.repo.Create(ctx, p)
	if err != nil {
		return nil, err
	}

	if uc.fga != nil {
		_ = uc.fga.WriteTuples(ctx,
			openfga.Tuple{User: "organization:" + orgID, Relation: "organization", Object: "project:" + created.ID},
			openfga.Tuple{User: "user:" + userID, Relation: "admin", Object: "project:" + created.ID},
		)
		openfga.InvalidateListObjects(ctx, uc.redis, userID, "can_view", "project")
	}

	if _, err := uc.repo.AddMember(ctx, &entity.ProjectMember{
		ProjectID: created.ID,
		UserID:    userID,
		Role:      "admin",
	}); err != nil {
		uc.log.Warnf("auto-add admin failed: %v", err)
	}

	return created, nil
}

func (uc *ProjectUsecase) Get(ctx context.Context, id string) (*entity.Project, error) {
	p, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		if dataent.IsNotFound(err) {
			return nil, projectpb.ErrorProjectNotFound("project %s not found", id)
		}
		return nil, err
	}
	return p, nil
}

func (uc *ProjectUsecase) List(ctx context.Context, orgID string, page, pageSize int32) ([]*entity.Project, int64, error) {
	a, ok := actor.FromContext(ctx)
	if !ok {
		return nil, 0, projectpb.ErrorProjectNotFound("user not authenticated")
	}

	if uc.fga != nil {
		ids, err := uc.fga.CachedListObjects(ctx, uc.redis, openfga.DefaultListCacheTTL,
			a.ID(), "can_view", "project")
		if err != nil {
			uc.log.Warnf("ListObjects fallback to DB: %v", err)
			return uc.repo.ListByOrgID(ctx, orgID, page, pageSize)
		}
		return uc.repo.GetByIDs(ctx, ids, page, pageSize)
	}

	return uc.repo.ListByOrgID(ctx, orgID, page, pageSize)
}

func (uc *ProjectUsecase) Update(ctx context.Context, p *entity.Project) (*entity.Project, error) {
	updated, err := uc.repo.Update(ctx, p)
	if err != nil {
		if dataent.IsNotFound(err) {
			return nil, projectpb.ErrorProjectNotFound("project %s not found", p.ID)
		}
		return nil, projectpb.ErrorProjectUpdateFailed("update: %v", err)
	}
	return updated, nil
}

func (uc *ProjectUsecase) Delete(ctx context.Context, id string) error {
	proj, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		if dataent.IsNotFound(err) {
			return projectpb.ErrorProjectNotFound("project %s not found", id)
		}
		return projectpb.ErrorProjectDeleteFailed("get: %v", err)
	}

	members, _ := uc.repo.ListAllMembers(ctx, id)
	if uc.fga != nil {
		var tuples []openfga.Tuple
		for _, m := range members {
			tuples = append(tuples,
				openfga.Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "project:" + id},
			)
		}
		tuples = append(tuples,
			openfga.Tuple{User: "organization:" + proj.OrganizationID, Relation: "organization", Object: "project:" + id},
		)
		if err := uc.fga.DeleteTuples(ctx, tuples...); err != nil {
			uc.log.Warnf("delete project FGA tuples: %v", err)
		}
	}

	if _, err := uc.repo.DeleteAllMembers(ctx, id); err != nil {
		uc.log.Warnf("delete project members: %v", err)
	}

	if err := uc.repo.Delete(ctx, id); err != nil {
		return projectpb.ErrorProjectDeleteFailed("delete: %v", err)
	}
	return nil
}

func (uc *ProjectUsecase) AddMember(ctx context.Context, m *entity.ProjectMember) (*entity.ProjectMember, error) {
	if err := ValidateProjectRole(m.Role); err != nil {
		return nil, projectpb.ErrorProjectCreateFailed("%v", err)
	}

	// Verify user is a member of the parent organization
	proj, err := uc.repo.GetByID(ctx, m.ProjectID)
	if err != nil {
		return nil, projectpb.ErrorProjectNotFound("project not found")
	}
	if _, err := uc.orgRepo.GetMember(ctx, proj.OrganizationID, m.UserID); err != nil {
		return nil, projectpb.ErrorProjectCreateFailed("user must be a member of the parent organization")
	}

	if _, err := uc.repo.GetMember(ctx, m.ProjectID, m.UserID); err == nil {
		return nil, projectpb.ErrorProjectMemberAlreadyExists("user is already a member")
	}

	created, err := uc.repo.AddMember(ctx, m)
	if err != nil {
		return nil, err
	}

	if uc.fga != nil {
		_ = uc.fga.WriteTuples(ctx,
			openfga.Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "project:" + m.ProjectID},
		)
		openfga.InvalidateListObjects(ctx, uc.redis, m.UserID, "can_view", "project")
	}
	return created, nil
}

func (uc *ProjectUsecase) RemoveMember(ctx context.Context, projID, userID string) error {
	member, err := uc.repo.GetMember(ctx, projID, userID)
	if err != nil {
		return projectpb.ErrorProjectMemberNotFound("member not found")
	}

	if err := uc.repo.RemoveMember(ctx, projID, userID); err != nil {
		return err
	}

	if uc.fga != nil {
		_ = uc.fga.DeleteTuples(ctx,
			openfga.Tuple{User: "user:" + userID, Relation: member.Role, Object: "project:" + projID},
		)
		openfga.InvalidateListObjects(ctx, uc.redis, userID, "can_view", "project")
	}
	return nil
}

func (uc *ProjectUsecase) ListMembers(ctx context.Context, projID string, page, pageSize int32) ([]*entity.ProjectMember, int64, error) {
	return uc.repo.ListMembers(ctx, projID, page, pageSize)
}

func (uc *ProjectUsecase) UpdateMemberRole(ctx context.Context, projID, userID, newRole string) (*entity.ProjectMember, error) {
	if err := ValidateProjectRole(newRole); err != nil {
		return nil, projectpb.ErrorProjectCreateFailed("%v", err)
	}

	oldMember, err := uc.repo.GetMember(ctx, projID, userID)
	if err != nil {
		return nil, projectpb.ErrorProjectMemberNotFound("member not found")
	}

	updated, err := uc.repo.UpdateMemberRole(ctx, projID, userID, newRole)
	if err != nil {
		return nil, err
	}

	if uc.fga != nil && oldMember.Role != newRole {
		_ = uc.fga.DeleteTuples(ctx,
			openfga.Tuple{User: "user:" + userID, Relation: oldMember.Role, Object: "project:" + projID},
		)
		_ = uc.fga.WriteTuples(ctx,
			openfga.Tuple{User: "user:" + userID, Relation: newRole, Object: "project:" + projID},
		)
	}
	return updated, nil
}
