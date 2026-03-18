package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"

	orgpb "github.com/Servora-Kit/servora/api/gen/go/organization/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz/entity"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/pkg/helpers"
	"github.com/Servora-Kit/servora/pkg/logger"
)

type OrganizationRepo interface {
	Create(ctx context.Context, org *entity.Organization) (*entity.Organization, error)
	GetByID(ctx context.Context, id string) (*entity.Organization, error)
	GetByIDs(ctx context.Context, tenantID string, ids []string, page, pageSize int32) ([]*entity.Organization, int64, error)
	GetBySlug(ctx context.Context, tenantID, slug string) (*entity.Organization, error)
	ListByUserID(ctx context.Context, userID, tenantID string, page, pageSize int32) ([]*entity.Organization, int64, error)
	Update(ctx context.Context, org *entity.Organization) (*entity.Organization, error)
	Delete(ctx context.Context, id string) error
	Purge(ctx context.Context, id string) error
	PurgeCascade(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) (*entity.Organization, error)
	GetByIDIncludingDeleted(ctx context.Context, id string) (*entity.Organization, error)
	HasChildren(ctx context.Context, id string) (bool, error)
	ListAllByTenant(ctx context.Context, tenantID string) ([]*entity.Organization, error)
	AddMember(ctx context.Context, m *entity.OrganizationMember) (*entity.OrganizationMember, error)
	RemoveMember(ctx context.Context, orgID, userID string) error
	ListMembers(ctx context.Context, orgID string, page, pageSize int32) ([]*entity.OrganizationMember, int64, error)
	GetMember(ctx context.Context, orgID, userID string) (*entity.OrganizationMember, error)
	UpdateMemberRole(ctx context.Context, orgID, userID, role string) (*entity.OrganizationMember, error)
	ListAllMembers(ctx context.Context, orgID string) ([]*entity.OrganizationMember, error)
	DeleteAllMembers(ctx context.Context, orgID string) (int, error)
	ListMembershipsByUserID(ctx context.Context, userID string) ([]*entity.OrganizationMember, error)
	DeleteMembershipsByUserID(ctx context.Context, userID string) (int, error)
	// ListOrgMembershipsByUserIDs returns a map of userID -> []orgID for the given users
	// scoped to the organizations belonging to tenantID.
	ListOrgMembershipsByUserIDs(ctx context.Context, tenantID string, userIDs []string) (map[string][]string, error)
}

type OrganizationUsecase struct {
	repo  OrganizationRepo
	authz AuthZRepo
	log   *logger.Helper
}

func NewOrganizationUsecase(repo OrganizationRepo, authz AuthZRepo, l logger.Logger) *OrganizationUsecase {
	return &OrganizationUsecase{
		repo:  repo,
		authz: authz,
		log:   logger.NewHelper(l, logger.WithModule("organization/biz/iam-service")),
	}
}

func (uc *OrganizationUsecase) Create(ctx context.Context, userID string, org *entity.Organization) (*entity.Organization, error) {
	if userID == "" {
		return nil, orgpb.ErrorOrganizationCreateFailed("user not authenticated")
	}

	if org.TenantID == "" {
		return nil, orgpb.ErrorOrganizationCreateFailed("tenant_id is required")
	}

	if org.Slug == "" {
		org.Slug = helpers.Slugify(org.Name)
	}
	if org.DisplayName == "" {
		org.DisplayName = org.Name
	}

	if _, err := uc.repo.GetBySlug(ctx, org.TenantID, org.Slug); err == nil {
		return nil, orgpb.ErrorOrganizationAlreadyExists("slug '%s' already taken", org.Slug)
	} else if !ent.IsNotFound(err) {
		uc.log.Errorf("check slug failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}
	created, err := uc.repo.Create(ctx, org)
	if err != nil {
		uc.log.Errorf("create organization failed: %v", err)
		return nil, orgpb.ErrorOrganizationCreateFailed("failed to create organization")
	}

	if _, err := uc.repo.AddMember(ctx, &entity.OrganizationMember{
		OrganizationID: created.ID,
		UserID:         userID,
		Role:           string(RoleAdmin),
	}); err != nil {
		uc.log.Errorf("add admin member failed, rolling back org: %v", err)
		if delErr := uc.repo.Purge(ctx, created.ID); delErr != nil {
			uc.log.Errorf("rollback purge org failed: %v", delErr)
		}
		return nil, orgpb.ErrorOrganizationCreateFailed("failed to add admin member")
	}

	if uc.authz != nil {
		if err := uc.authz.WriteTuples(ctx,
			Tuple{User: "tenant:" + created.TenantID, Relation: "tenant", Object: "organization:" + created.ID},
			Tuple{User: "user:" + userID, Relation: string(RoleAdmin), Object: "organization:" + created.ID},
		); err != nil {
			uc.log.Errorf("write FGA tuples failed, rolling back org: %v", err)
			_ = uc.repo.RemoveMember(ctx, created.ID, userID)
			if delErr := uc.repo.Purge(ctx, created.ID); delErr != nil {
				uc.log.Errorf("rollback purge org failed: %v", delErr)
			}
			return nil, orgpb.ErrorOrganizationCreateFailed("failed to write authorization tuples")
		}
	}

	return created, nil
}

// CreateDefault creates the default organization for a tenant. displayName is the
// user-facing name; name and slug are used as the internal/URL identifier.
func (uc *OrganizationUsecase) CreateDefault(ctx context.Context, userID, name, slug, displayName, tenantID string) (*entity.Organization, error) {
	if displayName == "" {
		displayName = name
	}
	org := &entity.Organization{
		TenantID:    tenantID,
		Name:        name,
		Slug:        slug,
		DisplayName: displayName,
	}
	created, err := uc.repo.Create(ctx, org)
	if err != nil {
		uc.log.Errorf("create organization failed: %v", err)
		return nil, orgpb.ErrorOrganizationCreateFailed("%v", err)
	}

	if _, err := uc.repo.AddMember(ctx, &entity.OrganizationMember{
		OrganizationID: created.ID,
		UserID:         userID,
		Role:           string(RoleAdmin),
	}); err != nil {
		uc.log.Errorf("add admin member failed, rolling back org: %v", err)
		if delErr := uc.repo.Purge(ctx, created.ID); delErr != nil {
			uc.log.Errorf("rollback purge org failed: %v", delErr)
		}
		return nil, orgpb.ErrorOrganizationCreateFailed("failed to add admin member")
	}

	if uc.authz != nil {
		if err := uc.authz.WriteTuples(ctx,
			Tuple{User: "tenant:" + tenantID, Relation: "tenant", Object: "organization:" + created.ID},
			Tuple{User: "user:" + userID, Relation: string(RoleAdmin), Object: "organization:" + created.ID},
		); err != nil {
			uc.log.Errorf("write FGA tuples failed, rolling back org: %v", err)
			_ = uc.repo.RemoveMember(ctx, created.ID, userID)
			if delErr := uc.repo.Purge(ctx, created.ID); delErr != nil {
				uc.log.Errorf("rollback purge org failed: %v", delErr)
			}
			return nil, orgpb.ErrorOrganizationCreateFailed("failed to write authorization tuples")
		}
	}

	return created, nil
}

func (uc *OrganizationUsecase) Get(ctx context.Context, id string) (*entity.Organization, error) {
	org, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, orgpb.ErrorOrganizationNotFound("organization %s not found", id)
		}
		uc.log.Errorf("get organization failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}
	return org, nil
}

func (uc *OrganizationUsecase) List(ctx context.Context, userID, tenantID string, page, pageSize int32) ([]*entity.Organization, int64, error) {
	if userID == "" {
		return nil, 0, orgpb.ErrorOrganizationNotFound("user not authenticated")
	}

	if uc.authz != nil {
		ids, err := uc.authz.CachedListObjects(ctx, DefaultListCacheTTL, userID, "can_view", "organization")
		if err != nil {
			uc.log.Warnf("ListObjects fallback to DB: %v", err)
			orgs, total, err := uc.repo.ListByUserID(ctx, userID, tenantID, page, pageSize)
			if err != nil {
				uc.log.Errorf("list organizations failed: %v", err)
				return nil, 0, errors.InternalServer("INTERNAL", "internal error")
			}
			return orgs, total, nil
		}
		orgs, total, err := uc.repo.GetByIDs(ctx, tenantID, ids, page, pageSize)
		if err != nil {
			uc.log.Errorf("list organizations by ids failed: %v", err)
			return nil, 0, errors.InternalServer("INTERNAL", "internal error")
		}
		return orgs, total, nil
	}

	orgs, total, err := uc.repo.ListByUserID(ctx, userID, tenantID, page, pageSize)
	if err != nil {
		uc.log.Errorf("list organizations failed: %v", err)
		return nil, 0, errors.InternalServer("INTERNAL", "internal error")
	}
	return orgs, total, nil
}

func (uc *OrganizationUsecase) Update(ctx context.Context, org *entity.Organization) (*entity.Organization, error) {
	updated, err := uc.repo.Update(ctx, org)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, orgpb.ErrorOrganizationNotFound("organization %s not found", org.ID)
		}
		uc.log.Errorf("update organization failed: %v", err)
		return nil, orgpb.ErrorOrganizationUpdateFailed("failed to update organization")
	}
	return updated, nil
}

func (uc *OrganizationUsecase) Delete(ctx context.Context, id string) error {
	if _, err := uc.repo.GetByID(ctx, id); err != nil {
		if ent.IsNotFound(err) {
			return orgpb.ErrorOrganizationNotFound("organization %s not found", id)
		}
		uc.log.Errorf("get organization failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}
	hasChildren, err := uc.repo.HasChildren(ctx, id)
	if err != nil {
		uc.log.Errorf("check children failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}
	if hasChildren {
		return orgpb.ErrorOrganizationHasChildren("cannot delete organization with child organizations")
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		uc.log.Errorf("soft delete organization failed: %v", err)
		return orgpb.ErrorOrganizationDeleteFailed("failed to delete organization")
	}
	return nil
}

// ListTree returns all tenant organizations as a tree (in-memory construction).
func (uc *OrganizationUsecase) ListTree(ctx context.Context, tenantID string) ([]*entity.Organization, error) {
	all, err := uc.repo.ListAllByTenant(ctx, tenantID)
	if err != nil {
		uc.log.Errorf("list all orgs failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}
	return buildOrgTree(all), nil
}

// buildOrgTree converts a flat list of organizations into a tree by ID linkage.
func buildOrgTree(orgs []*entity.Organization) []*entity.Organization {
	byID := make(map[string]*entity.Organization, len(orgs))
	for _, o := range orgs {
		byID[o.ID] = o
	}
	var roots []*entity.Organization
	for _, o := range orgs {
		if o.ParentID == nil || *o.ParentID == "" {
			roots = append(roots, o)
		} else if parent, ok := byID[*o.ParentID]; ok {
			parent.Children = append(parent.Children, o)
		} else {
			roots = append(roots, o)
		}
	}
	return roots
}

func (uc *OrganizationUsecase) Purge(ctx context.Context, id string) error {
	org, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return orgpb.ErrorOrganizationNotFound("organization %s not found", id)
		}
		uc.log.Errorf("get organization failed: %v", err)
		return errors.InternalServer("INTERNAL", "internal error")
	}

	uc.purgeOrgFGA(ctx, id, org.TenantID)

	if err := uc.repo.PurgeCascade(ctx, id); err != nil {
		uc.log.Errorf("purge organization failed: %v", err)
		return orgpb.ErrorOrganizationDeleteFailed("failed to delete organization")
	}
	return nil
}

func (uc *OrganizationUsecase) Restore(ctx context.Context, id string) (*entity.Organization, error) {
	if _, err := uc.repo.GetByIDIncludingDeleted(ctx, id); err != nil {
		if ent.IsNotFound(err) {
			return nil, orgpb.ErrorOrganizationNotFound("organization %s not found", id)
		}
		uc.log.Errorf("get organization failed: %v", err)
		return nil, errors.InternalServer("INTERNAL", "internal error")
	}
	org, err := uc.repo.Restore(ctx, id)
	if err != nil {
		uc.log.Errorf("restore organization failed: %v", err)
		return nil, orgpb.ErrorOrganizationUpdateFailed("%v", err)
	}
	return org, nil
}

func (uc *OrganizationUsecase) purgeOrgFGA(ctx context.Context, orgID, tenantID string) {
	if uc.authz == nil {
		return
	}
	var tuples []Tuple

	members, _ := uc.repo.ListAllMembers(ctx, orgID)
	for _, m := range members {
		tuples = append(tuples,
			Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "organization:" + orgID},
		)
	}
	if tenantID != "" {
		tuples = append(tuples,
			Tuple{User: "tenant:" + tenantID, Relation: "tenant", Object: "organization:" + orgID},
		)
	}

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
		uc.log.Errorf("add member failed: %v", err)
		return nil, orgpb.ErrorOrganizationCreateFailed("%v", err)
	}

	if uc.authz != nil {
		if err := uc.authz.WriteTuples(ctx,
			Tuple{User: "user:" + m.UserID, Relation: m.Role, Object: "organization:" + m.OrganizationID},
		); err != nil {
			uc.log.Errorf("write FGA tuple failed, rolling back member: %v", err)
			if rbErr := uc.repo.RemoveMember(ctx, m.OrganizationID, m.UserID); rbErr != nil {
				uc.log.Errorf("rollback remove member failed: %v", rbErr)
			}
			return nil, orgpb.ErrorOrganizationCreateFailed("failed to write authorization tuple")
		}
	}
	return created, nil
}

func (uc *OrganizationUsecase) RemoveMember(ctx context.Context, orgID, userID string) error {
	member, err := uc.repo.GetMember(ctx, orgID, userID)
	if err != nil {
		return orgpb.ErrorOrganizationMemberNotFound("member not found")
	}

	if err := uc.repo.RemoveMember(ctx, orgID, userID); err != nil {
		uc.log.Errorf("remove member failed: %v", err)
		return orgpb.ErrorOrganizationDeleteFailed("%v", err)
	}

	if uc.authz != nil {
		if err := uc.authz.DeleteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: member.Role, Object: "organization:" + orgID},
		); err != nil {
			uc.log.Errorf("delete FGA tuple failed, rolling back member removal: %v", err)
			if _, rbErr := uc.repo.AddMember(ctx, &entity.OrganizationMember{
				OrganizationID: orgID,
				UserID:         userID,
				Role:           member.Role,
			}); rbErr != nil {
				uc.log.Errorf("rollback re-add member failed: %v", rbErr)
			}
			return orgpb.ErrorOrganizationDeleteFailed("failed to delete authorization tuple")
		}
	}
	return nil
}

func (uc *OrganizationUsecase) ListMembers(ctx context.Context, orgID string, page, pageSize int32) ([]*entity.OrganizationMember, int64, error) {
	members, total, err := uc.repo.ListMembers(ctx, orgID, page, pageSize)
	if err != nil {
		uc.log.Errorf("list members failed: %v", err)
		return nil, 0, errors.InternalServer("INTERNAL", "internal error")
	}
	return members, total, nil
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
		uc.log.Errorf("update member role failed: %v", err)
		return nil, orgpb.ErrorOrganizationUpdateFailed("%v", err)
	}

	if uc.authz != nil && oldMember.Role != newRole {
		if err := uc.authz.DeleteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: oldMember.Role, Object: "organization:" + orgID},
		); err != nil {
			uc.log.Errorf("delete old FGA tuple failed, rolling back role: %v", err)
			if _, rbErr := uc.repo.UpdateMemberRole(ctx, orgID, userID, oldMember.Role); rbErr != nil {
				uc.log.Errorf("rollback role update failed: %v", rbErr)
			}
			return nil, orgpb.ErrorOrganizationUpdateFailed("failed to update authorization")
		}
		if err := uc.authz.WriteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: newRole, Object: "organization:" + orgID},
		); err != nil {
			uc.log.Errorf("write new FGA tuple failed, rolling back role: %v", err)
			_ = uc.authz.WriteTuples(ctx,
				Tuple{User: "user:" + userID, Relation: oldMember.Role, Object: "organization:" + orgID},
			)
			if _, rbErr := uc.repo.UpdateMemberRole(ctx, orgID, userID, oldMember.Role); rbErr != nil {
				uc.log.Errorf("rollback role update failed: %v", rbErr)
			}
			return nil, orgpb.ErrorOrganizationUpdateFailed("failed to update authorization")
		}
	}
	return updated, nil
}

func (uc *OrganizationUsecase) InviteMember(ctx context.Context, orgID, userID, role string) (*entity.OrganizationMember, error) {
	if err := ValidateOrganizationRole(role); err != nil {
		return nil, orgpb.ErrorOrganizationCreateFailed("%v", err)
	}

	if _, err := uc.repo.GetMember(ctx, orgID, userID); err == nil {
		return nil, orgpb.ErrorOrganizationMemberAlreadyExists("user is already a member")
	}

	created, err := uc.repo.AddMember(ctx, &entity.OrganizationMember{
		OrganizationID: orgID,
		UserID:         userID,
		Role:           role,
	})
	if err != nil {
		uc.log.Errorf("invite member failed: %v", err)
		return nil, orgpb.ErrorOrganizationCreateFailed("failed to invite member")
	}

	if uc.authz != nil {
		if err := uc.authz.WriteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: role, Object: "organization:" + orgID},
		); err != nil {
			uc.log.Errorf("write FGA tuple failed, rolling back invite: %v", err)
			if rbErr := uc.repo.RemoveMember(ctx, orgID, userID); rbErr != nil {
				uc.log.Errorf("rollback remove member failed: %v", rbErr)
			}
			return nil, orgpb.ErrorOrganizationCreateFailed("failed to write authorization tuple")
		}
	}
	return created, nil
}

func (uc *OrganizationUsecase) AcceptInvitation(ctx context.Context, orgID, userID string) error {
	member, err := uc.repo.GetMember(ctx, orgID, userID)
	if err != nil {
		return orgpb.ErrorOrganizationMemberNotFound("invitation not found")
	}
	if member.Status == "active" {
		return nil
	}
	if _, err := uc.repo.UpdateMemberRole(ctx, orgID, userID, member.Role); err != nil {
		uc.log.Errorf("accept invitation failed: %v", err)
		return orgpb.ErrorOrganizationUpdateFailed("failed to accept invitation")
	}
	return nil
}

func (uc *OrganizationUsecase) GetUserMemberships(ctx context.Context, userID string) ([]*entity.OrganizationMember, error) {
	return uc.repo.ListMembershipsByUserID(ctx, userID)
}

func (uc *OrganizationUsecase) RejectInvitation(ctx context.Context, orgID, userID string) error {
	member, err := uc.repo.GetMember(ctx, orgID, userID)
	if err != nil {
		return orgpb.ErrorOrganizationMemberNotFound("invitation not found")
	}
	if member.Status != "invited" {
		return orgpb.ErrorOrganizationUpdateFailed("can only reject pending invitations")
	}

	if err := uc.repo.RemoveMember(ctx, orgID, userID); err != nil {
		uc.log.Errorf("reject invitation - remove member failed: %v", err)
		return orgpb.ErrorOrganizationDeleteFailed("failed to reject invitation")
	}

	if uc.authz != nil {
		if err := uc.authz.DeleteTuples(ctx,
			Tuple{User: "user:" + userID, Relation: member.Role, Object: "organization:" + orgID},
		); err != nil {
			uc.log.Errorf("delete FGA tuple on reject failed, rolling back: %v", err)
			if _, rbErr := uc.repo.AddMember(ctx, &entity.OrganizationMember{
				OrganizationID: orgID,
				UserID:         userID,
				Role:           member.Role,
			}); rbErr != nil {
				uc.log.Errorf("rollback re-add member failed: %v", rbErr)
			}
			return orgpb.ErrorOrganizationDeleteFailed("failed to delete authorization tuple")
		}
	}
	return nil
}
