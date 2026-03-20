package data

import (
	"context"

	iamconf "github.com/Servora-Kit/servora/api/gen/go/iam/conf/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent"
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/user"
	"github.com/Servora-Kit/servora/pkg/helpers"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/openfga"
)

const platformObjectID = "default"

// Seeder performs one-time data initialization for the IAM service.
// It runs as a Kratos BeforeStart hook, after all Wire DI is complete.
type Seeder struct {
	ec   *ent.Client
	fga  *openfga.Client
	seed *iamconf.Biz_Seed
	log  *logger.Helper
}

func NewSeeder(ec *ent.Client, fga *openfga.Client, bizConf *iamconf.Biz, l logger.Logger) *Seeder {
	return &Seeder{
		ec:   ec,
		fga:  fga,
		seed: bizConf.GetSeed(),
		log:  logger.For(l, "seed/data/iam"),
	}
}

// Run executes all seed steps. Each step is idempotent.
func (s *Seeder) Run(ctx context.Context) error {
	if s.seed == nil || s.seed.AdminEmail == "" {
		s.log.Info("no seed config provided, skipping user seed")
		return nil
	}

	adminUser, err := s.ensureAdminUser(ctx)
	if err != nil {
		return err
	}

	s.ensurePlatformAdmin(ctx, adminUser.ID.String())
	return nil
}

// ensureAdminUser creates the seed admin user if it does not already exist.
func (s *Seeder) ensureAdminUser(ctx context.Context) (*ent.User, error) {
	existing, err := s.ec.User.Query().Where(user.EmailEQ(s.seed.AdminEmail)).Only(ctx)
	if err == nil {
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

	username := s.seed.AdminName
	if username == "" {
		username = "admin"
	}

	created, err := s.ec.User.Create().
		SetUsername(username).
		SetEmail(s.seed.AdminEmail).
		SetPassword(pw).
		SetRole("admin").
		SetStatus("active").
		SetEmailVerified(true).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	s.log.Infof("seeded admin user: %s", s.seed.AdminEmail)
	return created, nil
}

// ensurePlatformAdmin writes the platform admin FGA tuple.
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
}
