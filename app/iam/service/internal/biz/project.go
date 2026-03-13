package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"

	projectpb "github.com/Servora-Kit/servora/api/gen/go/project/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	dataent "github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/actor"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type ProjectRepo interface {
	Create(ctx context.Context, p *entity.Project) (*entity.Project, error)
	GetByID(ctx context.Context, id string) (*entity.Project, error)
	GetByIDs(ctx context.Context, ids []string, page, pageSize int32) ([]*entity.Project, int64, error)
	ListByOrgID(ctx context.Context, orgID string, page, pageSize int32) ([]*entity.Project, int64, error)
	Update(ctx context.Context, p *entity.Project) (*entity.Project, error)
	Delete(ctx context.Context, id string) error
	Purge(ctx context.Context, id string) error
	PurgeCascade(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) (*entity.Project, error)
	GetByIDIncludingDeleted(ctx context.Context, id string) (*entity.Project, error)
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
	authz   AuthZRepo
	log     *logger.Helper
}

func NewProjectUsecase(repo ProjectRepo, orgRepo OrganizationRepo, authz AuthZRepo, l logger.Logger) *ProjectUsecase {
	return &ProjectUsecase{
		repo:    repo,
		orgRepo: orgRepo,
		authz:   authz,
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
		uc.log.Errorf("create project failed: %v", err)
		return nil, projectpb.ErrorProjectCreateFailed("failed to create project")
	}

	if uc.authz != nil {
		_ = uc.authz.WriteTuples(ctx,
			Tuple{User: "organization:" + p.OrganizationID, Relation: "organization", Object: "project:" + created.ID},
			Tuple{User: "user:" + userID, Relation: "admin", Object: "project:" + created.ID},
		)
		uc.authz.InvalidateListObjects(ctx, userID, "can_view", "project")
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

	if uc.authz != nil {
		_ = uc.authz.WriteTuples(ctx,
			Tuple{User: "organization:" + orgID, Relation: "organization", Object: "project:" + created.ID},
			Tuple{User: "user:" + userID, Relation: "admin", Object: "project:" + created.ID},
		)
		uc.authz.InvalidateListObjects(ctx, userID, "can_view", "project")
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

	if uc.authz != nil {
		ids, err := uc.authz.CachedListObjects(ctx, DefaultListCacheTTL, a.ID(), "can_view", "project")
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
		uc.log.Errorf("update project failed: %v", err)
		return nil, projectpb.ErrorProjectUpdateFailed("failed to update project")
	}
	return updated, nil
}

func (uc *ProjectUsecase) Delete(ctx context.Context, id string) error {
	if _, err := uc.repo.GetByID(ctx, id); err != nil {
		if dataent.IsNotFound(err) {
			return projectpb.ErrorProjectNotFound("project %s not found", id)
		}
		uc.log.Errorf("get project failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		uc.log.Errorf("soft delete project failed: %v", err)
		return projectpb.ErrorProjectDeleteFailed("failed to delete project")
	}
	return nil
}

func (uc *ProjectUsecase) Purge(ctx context.Context, id string) error {
	proj, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		if dataent.IsNotFound(err) {
			return projectpb.ErrorProjectNotFound("project %s not found", id)
		}
		uc.log.Errorf("get project failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}

	uc.purgeProjectFGA(ctx, id, proj.OrganizationID)

	if err := uc.repo.PurgeCascade(ctx, id); err != nil {
		uc.log.Errorf("purge project failed: %v", err)
		return projectpb.ErrorProjectDeleteFailed("failed to delete project")
	}
	return nil
}

func (uc *ProjectUsecase) purgeProjectFGA(ctx context.Context, projID, orgID string) {
	if uc.authz == nil {
		return
	}
	var tuples []Tuple
	members, _ := uc.repo.ListAllMembers(ctx, projID)
	for _, m := range members {
		tuples = append(tuples,
			Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "project:" + projID},
		)
	}
	tuples = append(tuples,
		Tuple{User: "organization:" + orgID, Relation: "organization", Object: "project:" + projID},
	)
	if err := uc.authz.DeleteTuples(ctx, tuples...); err != nil {
		uc.log.Warnf("purge project %s FGA tuples: %v", projID, err)
	}
}

func (uc *ProjectUsecase) Restore(ctx context.Context, id string) (*entity.Project, error) {
	if _, err := uc.repo.GetByIDIncludingDeleted(ctx, id); err != nil {
		if dataent.IsNotFound(err) {
			return nil, projectpb.ErrorProjectNotFound("project %s not found", id)
		}
		return nil, err
	}
	return uc.repo.Restore(ctx, id)
}

func (uc *ProjectUsecase) AddMember(ctx context.Context, m *entity.ProjectMember) (*entity.ProjectMember, error) {
	if err := ValidateProjectRole(m.Role); err != nil {
		return nil, projectpb.ErrorProjectCreateFailed("%v", err)
	}

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

	if uc.authz != nil {
		_ = uc.authz.WriteTuples(ctx,
			Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "project:" + m.ProjectID},
		)
		uc.authz.InvalidateListObjects(ctx, m.UserID, "can_view", "project")
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

	if uc.authz != nil {
		_ = uc.authz.DeleteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: member.Role, Object: "project:" + projID},
		)
		uc.authz.InvalidateListObjects(ctx, userID, "can_view", "project")
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

	if uc.authz != nil && oldMember.Role != newRole {
		_ = uc.authz.DeleteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: oldMember.Role, Object: "project:" + projID},
		)
		_ = uc.authz.WriteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: newRole, Object: "project:" + projID},
		)
	}
	return updated, nil
}
