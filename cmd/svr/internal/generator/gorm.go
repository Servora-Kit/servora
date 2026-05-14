package generator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gen"
	"gorm.io/gorm"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"github.com/Servora-Kit/servora/cmd/svr/internal/ux"
)

// GormGenerator wraps the GORM GEN code generation for a single service.
type GormGenerator struct {
	ServiceName string
	ServicePath string
	DatabaseCfg *corev1.Data_Database
	DryRun      bool
}

// daoPath returns the output path for generated DAO code.
func (g *GormGenerator) daoPath() string {
	return filepath.Join(g.ServicePath, "internal", "data", "gorm", "dao")
}

// poPath returns the output path for generated PO (model) code.
func (g *GormGenerator) poPath() string {
	return filepath.Join(g.ServicePath, "internal", "data", "gorm", "po")
}

// connectDB opens a database connection based on the configured driver.
func (g *GormGenerator) connectDB() (*gorm.DB, error) {
	driver := strings.ToLower(g.DatabaseCfg.GetDriver())
	source := g.DatabaseCfg.GetSource()

	var dialector gorm.Dialector
	switch driver {
	case "mysql":
		dialector = mysql.Open(source)
	case "postgres", "postgresql":
		dialector = postgres.Open(source)
	case "sqlite":
		dialector = sqlite.Open(source)
	default:
		return nil, fmt.Errorf("unsupported db driver: %s", driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect db failed: %w", err)
	}
	return db, nil
}

// Generate runs the GORM GEN code generation.
func (g *GormGenerator) Generate() error {
	daoPath := g.daoPath()
	poPath := g.poPath()

	if g.DryRun {
		ux.PrintDryRun(daoPath, poPath)
		return nil
	}

	db, err := g.connectDB()
	if err != nil {
		return err
	}

	ux.PrintDBConnected(g.DatabaseCfg.GetDriver())

	generator := gen.NewGenerator(gen.Config{
		OutPath:       daoPath,
		ModelPkgPath:  poPath,
		Mode:          gen.WithDefaultQuery | gen.WithQueryInterface,
		FieldNullable: true,
	})

	generator.UseDB(db)
	generator.ApplyBasic(generator.GenerateAllTable()...)
	generator.Execute()

	ux.PrintGenerated(g.ServiceName, daoPath, poPath)
	return nil
}
