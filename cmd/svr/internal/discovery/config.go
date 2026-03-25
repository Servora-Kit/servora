package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	config "github.com/Servora-Kit/servora/pkg/bootstrap/config"
)

// ServiceConfig holds resolved configuration for a service.
type ServiceConfig struct {
	Name      string
	Path      string
	Bootstrap *conf.Bootstrap
}

// LoadServiceConfig loads the bootstrap config for the given service.
func LoadServiceConfig(serviceName string) (*ServiceConfig, error) {
	servicePath := filepath.Join("app", serviceName, "service")
	configPath := filepath.Join(servicePath, "configs", "local")

	bc, krCfg, err := config.LoadBootstrap(configPath, serviceName, false)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	defer func() { _ = krCfg.Close() }()

	return &ServiceConfig{
		Name:      serviceName,
		Path:      servicePath,
		Bootstrap: bc,
	}, nil
}

// ValidateServiceExists checks that the service directory exists.
func ValidateServiceExists(serviceName string) error {
	servicePath := filepath.Join("app", serviceName, "service")
	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		return fmt.Errorf("service '%s' not found at app/%s/service", serviceName, serviceName)
	}
	return nil
}

// ValidateConfigExists checks that configs/local/ directory exists for the service.
func ValidateConfigExists(serviceName string) error {
	configDir := filepath.Join("app", serviceName, "service", "configs", "local")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return fmt.Errorf("config directory not found at app/%s/service/configs/local/", serviceName)
	}
	return nil
}

// ValidateDatabaseConfig checks that the bootstrap has a database config.
func ValidateDatabaseConfig(bc *conf.Bootstrap) error {
	if bc.GetData() == nil || bc.GetData().GetDatabase() == nil {
		return fmt.Errorf("no database config found")
	}
	return nil
}

// ListAvailableServices scans app/*/service directories and returns service names.
func ListAvailableServices() ([]string, error) {
	entries, err := os.ReadDir("app")
	if err != nil {
		return nil, fmt.Errorf("failed to scan app directory: %w", err)
	}

	var services []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		servicePath := filepath.Join("app", entry.Name(), "service")
		if info, err := os.Stat(servicePath); err == nil && info.IsDir() {
			services = append(services, entry.Name())
		}
	}
	return services, nil
}
