package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"

	projectpb "github.com/Servora-Kit/servora/api/gen/go/project/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/actor"
	"github.com/Servora-Kit/servora/pkg/helpers"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type ProjectRepo interface {
	Create(ctx context.Context, p *entity.Project) (*entity.Project, error)
	GetByID(ctx context.Context, orgID, id string) (*entity.Project, error)
	GetByIDs(ctx context.Context, orgID string, ids []string, page, pageSize int32) ([]*entity.Project, int64, error)
	ListByOrgID(ctx context.Context, orgID string, page, pageSize int32) ([]*entity.Project, int64, error)
	Update(ctx context.Context, orgID string, p *entity.Project) (*entity.Project, error)
	Delete(ctx context.Context, orgID, id string) error
	Purge(ctx context.Context, orgID, id string) error
	PurgeCascade(ctx context.Context, id string) error
	Restore(ctx context.Context, orgID, id string) (*entity.Project, error)
	GetByIDIncludingDeleted(ctx context.Context, orgID, id string) (*entity.Project, error)
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

	if p.Slug == "" {
		p.Slug = helpers.Slugify(p.Name)
	}

	created, err := uc.repo.Create(ctx, p)
	if err != nil {
		uc.log.Errorf("create project failed: %v", err)
		return nil, projectpb.ErrorProjectCreateFailed("failed to create project")
	}

	if _, err := uc.repo.AddMember(ctx, &entity.ProjectMember{
		ProjectID: created.ID,
		UserID:    userID,
		Role:      "admin",
	}); err != nil {
		uc.log.Errorf("add admin member failed, rolling back project: %v", err)
		if delErr := uc.repo.PurgeCascade(ctx, created.ID); delErr != nil {
			uc.log.Errorf("rollback purge project failed: %v", delErr)
		}
		return nil, projectpb.ErrorProjectCreateFailed("failed to add admin member")
	}

	if uc.authz != nil {
		if err := uc.authz.WriteTuples(ctx,
			Tuple{User: "organization:" + p.OrganizationID, Relation: "organization", Object: "project:" + created.ID},
			Tuple{User: "user:" + userID, Relation: "admin", Object: "project:" + created.ID},
		); err != nil {
			uc.log.Errorf("write FGA tuples failed, rolling back project: %v", err)
			if delErr := uc.repo.PurgeCascade(ctx, created.ID); delErr != nil {
				uc.log.Errorf("rollback purge project failed: %v", delErr)
			}
			return nil, projectpb.ErrorProjectCreateFailed("failed to write authorization tuples")
		}
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
		uc.log.Errorf("create project failed: %v", err)
		return nil, projectpb.ErrorProjectCreateFailed("%v", err)
	}

	if _, err := uc.repo.AddMember(ctx, &entity.ProjectMember{
		ProjectID: created.ID,
		UserID:    userID,
		Role:      "admin",
	}); err != nil {
		uc.log.Errorf("add admin member failed, rolling back project: %v", err)
		if delErr := uc.repo.PurgeCascade(ctx, created.ID); delErr != nil {
			uc.log.Errorf("rollback purge project failed: %v", delErr)
		}
		return nil, projectpb.ErrorProjectCreateFailed("failed to add admin member")
	}

	if uc.authz != nil {
		if err := uc.authz.WriteTuples(ctx,
			Tuple{User: "organization:" + orgID, Relation: "organization", Object: "project:" + created.ID},
			Tuple{User: "user:" + userID, Relation: "admin", Object: "project:" + created.ID},
		); err != nil {
			uc.log.Errorf("write FGA tuples failed, rolling back project: %v", err)
			if delErr := uc.repo.PurgeCascade(ctx, created.ID); delErr != nil {
				uc.log.Errorf("rollback purge project failed: %v", delErr)
			}
			return nil, projectpb.ErrorProjectCreateFailed("failed to write authorization tuples")
		}
	}

	return created, nil
}

func (uc *ProjectUsecase) Get(ctx context.Context, id string) (*entity.Project, error) {
	orgID, _ := actor.OrganizationIDFromContext(ctx)
	p, err := uc.repo.GetByID(ctx, orgID, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, projectpb.ErrorProjectNotFound("project %s not found", id)
		}
		uc.log.Errorf("get project failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
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
			projects, total, err := uc.repo.ListByOrgID(ctx, orgID, page, pageSize)
			if err != nil {
				uc.log.Errorf("list projects failed: %v", err)
				return nil, 0, errors.InternalServer("INTERNAL", "internal error")
			}
			return projects, total, nil
		}
		projects, total, err := uc.repo.GetByIDs(ctx, orgID, ids, page, pageSize)
		if err != nil {
			uc.log.Errorf("list projects by ids failed: %v", err)
			return nil, 0, errors.InternalServer("INTERNAL", "internal error")
		}
		return projects, total, nil
	}

	projects, total, err := uc.repo.ListByOrgID(ctx, orgID, page, pageSize)
	if err != nil {
		uc.log.Errorf("list projects failed: %v", err)
		return nil, 0, errors.InternalServer("INTERNAL", "internal error")
	}
	return projects, total, nil
}

func (uc *ProjectUsecase) Update(ctx context.Context, p *entity.Project) (*entity.Project, error) {
	orgID, _ := actor.OrganizationIDFromContext(ctx)
	updated, err := uc.repo.Update(ctx, orgID, p)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, projectpb.ErrorProjectNotFound("project %s not found", p.ID)
		}
		uc.log.Errorf("update project failed: %v", err)
		return nil, projectpb.ErrorProjectUpdateFailed("failed to update project")
	}
	return updated, nil
}

func (uc *ProjectUsecase) Delete(ctx context.Context, id string) error {
	orgID, _ := actor.OrganizationIDFromContext(ctx)
	if _, err := uc.repo.GetByID(ctx, orgID, id); err != nil {
		if ent.IsNotFound(err) {
			return projectpb.ErrorProjectNotFound("project %s not found", id)
		}
		uc.log.Errorf("get project failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}
	if err := uc.repo.Delete(ctx, orgID, id); err != nil {
		uc.log.Errorf("soft delete project failed: %v", err)
		return projectpb.ErrorProjectDeleteFailed("failed to delete project")
	}
	return nil
}

func (uc *ProjectUsecase) Purge(ctx context.Context, id string) error {
	proj, err := uc.repo.GetByID(ctx, "", id)
	if err != nil {
		if ent.IsNotFound(err) {
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
	if _, err := uc.repo.GetByIDIncludingDeleted(ctx, "", id); err != nil {
		if ent.IsNotFound(err) {
			return nil, projectpb.ErrorProjectNotFound("project %s not found", id)
		}
		uc.log.Errorf("get project failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}
	p, err := uc.repo.Restore(ctx, "", id)
	if err != nil {
		uc.log.Errorf("restore project failed: %v", err)
		return nil, projectpb.ErrorProjectUpdateFailed("%v", err)
	}
	return p, nil
}

func (uc *ProjectUsecase) AddMember(ctx context.Context, m *entity.ProjectMember) (*entity.ProjectMember, error) {
	if err := ValidateProjectRole(m.Role); err != nil {
		return nil, projectpb.ErrorProjectCreateFailed("%v", err)
	}

	orgID, _ := actor.OrganizationIDFromContext(ctx)
	proj, err := uc.repo.GetByID(ctx, orgID, m.ProjectID)
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
		uc.log.Errorf("add member failed: %v", err)
		return nil, projectpb.ErrorProjectCreateFailed("%v", err)
	}

	if uc.authz != nil {
		if err := uc.authz.WriteTuples(ctx,
			Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "project:" + m.ProjectID},
		); err != nil {
			uc.log.Errorf("write FGA tuple failed, rolling back member: %v", err)
			if rbErr := uc.repo.RemoveMember(ctx, m.ProjectID, m.UserID); rbErr != nil {
				uc.log.Errorf("rollback remove member failed: %v", rbErr)
			}
			return nil, projectpb.ErrorProjectCreateFailed("failed to write authorization tuple")
		}
	}
	return created, nil
}

func (uc *ProjectUsecase) RemoveMember(ctx context.Context, projID, userID string) error {
	member, err := uc.repo.GetMember(ctx, projID, userID)
	if err != nil {
		return projectpb.ErrorProjectMemberNotFound("member not found")
	}

	if err := uc.repo.RemoveMember(ctx, projID, userID); err != nil {
		uc.log.Errorf("remove member failed: %v", err)
		return projectpb.ErrorProjectDeleteFailed("%v", err)
	}

	if uc.authz != nil {
		if err := uc.authz.DeleteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: member.Role, Object: "project:" + projID},
		); err != nil {
			uc.log.Errorf("delete FGA tuple failed, rolling back member removal: %v", err)
			if _, rbErr := uc.repo.AddMember(ctx, &entity.ProjectMember{
				ProjectID: projID,
				UserID:    userID,
				Role:      member.Role,
			}); rbErr != nil {
				uc.log.Errorf("rollback re-add member failed: %v", rbErr)
			}
			return projectpb.ErrorProjectDeleteFailed("failed to delete authorization tuple")
		}
	}
	return nil
}

func (uc *ProjectUsecase) ListMembers(ctx context.Context, projID string, page, pageSize int32) ([]*entity.ProjectMember, int64, error) {
	members, total, err := uc.repo.ListMembers(ctx, projID, page, pageSize)
	if err != nil {
		uc.log.Errorf("list members failed: %v", err)
		return nil, 0, errors.InternalServer("INTERNAL", "internal error")
	}
	return members, total, nil
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
		uc.log.Errorf("update member role failed: %v", err)
		return nil, projectpb.ErrorProjectUpdateFailed("%v", err)
	}

	if uc.authz != nil && oldMember.Role != newRole {
		if err := uc.authz.DeleteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: oldMember.Role, Object: "project:" + projID},
		); err != nil {
			uc.log.Errorf("delete old FGA tuple failed, rolling back role: %v", err)
			if _, rbErr := uc.repo.UpdateMemberRole(ctx, projID, userID, oldMember.Role); rbErr != nil {
				uc.log.Errorf("rollback role update failed: %v", rbErr)
			}
			return nil, projectpb.ErrorProjectUpdateFailed("failed to update authorization")
		}
		if err := uc.authz.WriteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: newRole, Object: "project:" + projID},
		); err != nil {
			uc.log.Errorf("write new FGA tuple failed, rolling back role: %v", err)
			// Best-effort restore old FGA tuple; caller already gets "failed to update authorization"
			_ = uc.authz.WriteTuples(ctx,
				Tuple{User: "user:" + userID, Relation: oldMember.Role, Object: "project:" + projID},
			)
			if _, rbErr := uc.repo.UpdateMemberRole(ctx, projID, userID, oldMember.Role); rbErr != nil {
				uc.log.Errorf("rollback role update failed: %v", rbErr)
			}
			return nil, projectpb.ErrorProjectUpdateFailed("failed to update authorization")
		}
	}
	return updated, nil
}

func (uc *ProjectUsecase) InviteMember(ctx context.Context, orgID string, m *entity.ProjectMember) (*entity.ProjectMember, error) {
	if err := ValidateProjectRole(m.Role); err != nil {
		return nil, projectpb.ErrorProjectCreateFailed("%v", err)
	}

	if _, err := uc.repo.GetMember(ctx, m.ProjectID, m.UserID); err == nil {
		return nil, projectpb.ErrorProjectMemberAlreadyExists("user is already a member")
	}

	m.Status = "invited"
	created, err := uc.repo.AddMember(ctx, m)
	if err != nil {
		uc.log.Errorf("invite member failed: %v", err)
		return nil, projectpb.ErrorProjectCreateFailed("failed to invite member")
	}

	if uc.authz != nil {
		if err := uc.authz.WriteTuples(ctx,
			Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "project:" + m.ProjectID},
		); err != nil {
			uc.log.Errorf("write FGA tuple failed, rolling back invite: %v", err)
			if rbErr := uc.repo.RemoveMember(ctx, m.ProjectID, m.UserID); rbErr != nil {
				uc.log.Errorf("rollback remove member failed: %v", rbErr)
			}
			return nil, projectpb.ErrorProjectCreateFailed("failed to write authorization tuple")
		}
	}
	return created, nil
}

func (uc *ProjectUsecase) AcceptInvitation(ctx context.Context, projID, userID string) error {
	member, err := uc.repo.GetMember(ctx, projID, userID)
	if err != nil {
		return projectpb.ErrorProjectMemberNotFound("invitation not found")
	}
	if member.Status == "active" {
		return nil
	}
	if _, err := uc.repo.UpdateMemberRole(ctx, projID, userID, member.Role); err != nil {
		uc.log.Errorf("accept invitation failed: %v", err)
		return projectpb.ErrorProjectUpdateFailed("failed to accept invitation")
	}
	return nil
}

func (uc *ProjectUsecase) RejectInvitation(ctx context.Context, projID, userID string) error {
	member, err := uc.repo.GetMember(ctx, projID, userID)
	if err != nil {
		return projectpb.ErrorProjectMemberNotFound("invitation not found")
	}
	if member.Status != "invited" {
		return projectpb.ErrorProjectUpdateFailed("can only reject pending invitations")
	}

	if err := uc.repo.RemoveMember(ctx, projID, userID); err != nil {
		uc.log.Errorf("reject invitation - remove member failed: %v", err)
		return projectpb.ErrorProjectDeleteFailed("failed to reject invitation")
	}

	if uc.authz != nil {
		if err := uc.authz.DeleteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: member.Role, Object: "project:" + projID},
		); err != nil {
			uc.log.Errorf("delete FGA tuple on reject failed, rolling back: %v", err)
			if _, rbErr := uc.repo.AddMember(ctx, &entity.ProjectMember{
				ProjectID: projID,
				UserID:    userID,
				Role:      member.Role,
			}); rbErr != nil {
				uc.log.Errorf("rollback re-add member failed: %v", rbErr)
			}
			return projectpb.ErrorProjectDeleteFailed("failed to delete authorization tuple")
		}
	}
	return nil
}
