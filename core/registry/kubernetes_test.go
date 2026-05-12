package registry

import (
	"os"
	"testing"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/stretchr/testify/assert"
)

func shouldRunKubernetesIntegration() bool {
	return os.Getenv("RUN_K8S_INTEGRATION_TESTS") == "1"
}

func TestNewKubernetesRegistry(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		reg := NewKubernetesRegistry(nil)
		assert.Nil(t, reg)
	})

	t.Run("enable false returns nil", func(t *testing.T) {
		cfg := &conf.KubernetesConfig{Enable: false}
		reg := NewKubernetesRegistry(cfg)
		assert.Nil(t, reg)
	})

	t.Run("enable true creates registry", func(t *testing.T) {
		if !shouldRunKubernetesIntegration() {
			t.Skip("set RUN_K8S_INTEGRATION_TESTS=1 to run kubernetes integration test")
		}

		cfg := &conf.KubernetesConfig{Enable: true}

		defer func() {
			if r := recover(); r != nil {
				t.Skipf("could not create kubernetes registry (expected if no kubeconfig): %v", r)
			}
		}()

		reg := NewKubernetesRegistry(cfg)
		assert.NotNil(t, reg)
	})
}

func TestNewKubernetesDiscovery(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		disc := NewKubernetesDiscovery(nil)
		assert.Nil(t, disc)
	})

	t.Run("enable false returns nil", func(t *testing.T) {
		cfg := &conf.KubernetesConfig{Enable: false}
		disc := NewKubernetesDiscovery(cfg)
		assert.Nil(t, disc)
	})

	t.Run("enable true creates discovery", func(t *testing.T) {
		if !shouldRunKubernetesIntegration() {
			t.Skip("set RUN_K8S_INTEGRATION_TESTS=1 to run kubernetes integration test")
		}

		cfg := &conf.KubernetesConfig{Enable: true}

		defer func() {
			if r := recover(); r != nil {
				t.Skipf("could not create kubernetes discovery (expected if no kubeconfig): %v", r)
			}
		}()

		disc := NewKubernetesDiscovery(cfg)
		assert.NotNil(t, disc)
	})
}
