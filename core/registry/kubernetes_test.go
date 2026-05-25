package registry

import (
	"testing"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"github.com/stretchr/testify/assert"
)

func TestNewKubernetesRegistry(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		reg := NewKubernetesRegistry(nil)
		assert.Nil(t, reg)
	})

	t.Run("enable false returns nil", func(t *testing.T) {
		cfg := &corev1.Kubernetes{Enable: false}
		reg := NewKubernetesRegistry(cfg)
		assert.Nil(t, reg)
	})

	t.Run("enable true creates registry", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping: requires Kubernetes")
		}

		cfg := &corev1.Kubernetes{Enable: true}

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
		cfg := &corev1.Kubernetes{Enable: false}
		disc := NewKubernetesDiscovery(cfg)
		assert.Nil(t, disc)
	})

	t.Run("enable true creates discovery", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping: requires Kubernetes")
		}

		cfg := &corev1.Kubernetes{Enable: true}

		defer func() {
			if r := recover(); r != nil {
				t.Skipf("could not create kubernetes discovery (expected if no kubeconfig): %v", r)
			}
		}()

		disc := NewKubernetesDiscovery(cfg)
		assert.NotNil(t, disc)
	})
}
