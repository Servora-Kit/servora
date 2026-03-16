package data

import (
	"context"

	"github.com/google/uuid"

	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/application"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/organizationmember"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/project"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/projectmember"
)

// purgeOrganizationInTx deletes an Organization and all its dependent
// resources (Application, ProjectMember, Project, OrganizationMember)
// within the caller's existing transaction. The ent.Client must come
// from a transaction context (via Data.Ent(txCtx)).
func purgeOrganizationInTx(ctx context.Context, c *ent.Client, orgID uuid.UUID) error {
	if _, err := c.Application.Delete().
		Where(application.OrganizationIDEQ(orgID)).
		Exec(ctx); err != nil {
		return err
	}

	projIDs, err := c.Project.Query().
		Where(project.OrganizationIDEQ(orgID)).
		IDs(ctx)
	if err != nil {
		return err
	}
	if len(projIDs) > 0 {
		if _, err := c.ProjectMember.Delete().
			Where(projectmember.ProjectIDIn(projIDs...)).
			Exec(ctx); err != nil {
			return err
		}
		if _, err := c.Project.Delete().
			Where(project.IDIn(projIDs...)).
			Exec(ctx); err != nil {
			return err
		}
	}

	if _, err := c.OrganizationMember.Delete().
		Where(organizationmember.OrganizationIDEQ(orgID)).
		Exec(ctx); err != nil {
		return err
	}

	return c.Organization.DeleteOneID(orgID).Exec(ctx)
}
