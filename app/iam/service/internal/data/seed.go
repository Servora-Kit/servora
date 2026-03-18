package data

import (
	"context"

	iamconf "github.com/Servora-Kit/servora/api/gen/go/iam/conf/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/tenant"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/user"
	"github.com/Servora-Kit/servora/pkg/helpers"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/openfga"
	"github.com/google/uuid"
)

const platformObjectID = "default"

// Seeder performs one-time data initialization for the IAM service.
// It runs as a Kratos BeforeStart hook, after all Wire DI is complete,
// giving it access to biz-layer use cases.
type Seeder struct {
	ec       *ent.Client
	tenantUC *biz.TenantUsecase
	fga      *openfga.Client
	seed     *iamconf.Biz_Seed
	log      *logger.Helper
}

func NewSeeder(ec *ent.Client, tenantUC *biz.TenantUsecase, fga *openfga.Client, bizConf *iamconf.Biz, l logger.Logger) *Seeder {
	return &Seeder{
		ec:       ec,
		tenantUC: tenantUC,
		fga:      fga,
		seed:     bizConf.GetSeed(),
		log:      logger.NewHelper(l, logger.WithModule("seed/data/iam-service")),
	}
}

// Run executes all seed steps. Each step is idempotent.
func (s *Seeder) Run(ctx context.Context) error {
	// RBAC seed always runs regardless of admin seed config
	if err := s.SeedRBAC(ctx); err != nil {
		return err
	}

	if s.seed == nil || s.seed.AdminEmail == "" {
		s.log.Info("no seed config provided, skipping user seed")
		return nil
	}

	adminUser, err := s.ensureAdminUser(ctx)
	if err != nil {
		return err
	}
	userID := adminUser.ID.String()

	if _, err := s.tenantUC.EnsurePersonalTenant(ctx, userID, adminUser.Name); err != nil {
		s.log.Warnf("ensure personal tenant: %v", err)
	}

	s.ensurePlatformAdmin(ctx, userID)
	return nil
}

// ensureAdminUser creates the seed admin user if it does not already exist.
func (s *Seeder) ensureAdminUser(ctx context.Context) (*ent.User, error) {
	existing, err := s.ec.User.Query().Where(user.EmailEQ(s.seed.AdminEmail)).Only(ctx)
	if err == nil {
		// 补偿修复：若 seed 管理员的 email_verified 为 false（历史数据），强制更新为 true。
		if !existing.EmailVerified {
			updated, uerr := s.ec.User.UpdateOneID(existing.ID).SetEmailVerified(true).Save(ctx)
			if uerr != nil {
				s.log.Warnf("fix admin email_verified: %v", uerr)
			} else {
				s.log.Infof("fixed email_verified=true for admin user %s", existing.Email)
				return updated, nil
			}
		}
		return existing, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}

	pw, err := helpers.BcryptHash(s.seed.AdminPassword)
	if err != nil {
		return nil, err
	}

	name := s.seed.AdminName
	if name == "" {
		name = "admin"
	}

	created, err := s.ec.User.Create().
		SetName(name).
		SetEmail(s.seed.AdminEmail).
		SetPassword(pw).
		SetRole("admin").
		SetEmailVerified(true). // seed 管理员无需邮箱验证流程
		Save(ctx)
	if err != nil {
		return nil, err
	}
	s.log.Infof("seeded admin user: %s", s.seed.AdminEmail)
	return created, nil
}

// ensurePlatformAdmin writes the platform admin FGA tuple if not already present.
// It also ensures all tenants owned by this user have the platform:default → tenant
// inheritance tuple, so platform admins transitively inherit tenant admin rights.
func (s *Seeder) ensurePlatformAdmin(ctx context.Context, userID string) {
	if s.fga == nil {
		return
	}

	if err := s.fga.EnsureTuples(ctx, openfga.Tuple{
		User:     "user:" + userID,
		Relation: "admin",
		Object:   "platform:" + platformObjectID,
	}); err != nil {
		s.log.Warnf("FGA ensure platform admin tuple: %v", err)
	} else {
		s.log.Infof("seeded platform admin FGA tuple for user %s", userID)
	}

	// Ensure every tenant this user owns has the platform:default → tenant tuple,
	// enabling the platform admin → tenant effective_admin inheritance chain.
	uid, err := uuid.Parse(userID)
	if err != nil {
		s.log.Warnf("invalid userID for platform-tenant tuple: %v", err)
		return
	}
	ownedTenants, err := s.ec.Tenant.Query().
		Where(tenant.OwnerUserIDEQ(uid)).
		All(ctx)
	if err != nil {
		s.log.Warnf("list owned tenants for platform admin setup: %v", err)
		return
	}
	for _, t := range ownedTenants {
		tid := t.ID.String()
		if err := s.fga.EnsureTuples(ctx, openfga.Tuple{
			User:     "platform:" + platformObjectID,
			Relation: "platform",
			Object:   "tenant:" + tid,
		}); err != nil {
			s.log.Warnf("FGA ensure platform-tenant tuple for tenant %s: %v", tid, err)
		}
	}
}
