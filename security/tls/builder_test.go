package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
)

func TestBuildServerTLS_DisabledReturnsNil(t *testing.T) {
	cfg, err := BuildServerTLS(nil)
	if err != nil {
		t.Fatalf("build server tls: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil tls config, got %v", cfg)
	}
}

func TestBuildClientTLS_LoadsCA(t *testing.T) {
	tmp := t.TempDir()
	caPath := writeCACert(t, tmp)

	c := &corev1.TLSConfig{
		Enable: true,
		CaPath: caPath,
	}
	cfg, err := BuildClientTLS(c)
	if err != nil {
		t.Fatalf("build client tls: %v", err)
	}
	if cfg == nil || cfg.RootCAs == nil {
		t.Fatalf("invalid tls cfg: %+v", cfg)
	}
}

func TestMustBuildServerTLS_DisabledReturnsNil(t *testing.T) {
	cfg := MustBuildServerTLS(nil)
	if cfg != nil {
		t.Fatalf("expected nil tls config, got %v", cfg)
	}
}

func TestMustBuildServerTLS_PanicsOnInvalidConfig(t *testing.T) {
	t.Run("panic on missing cert files", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic, got nil")
			}
		}()

		_ = MustBuildServerTLS(&corev1.TLSConfig{
			Enable:   true,
			CertPath: "/missing-cert.pem",
			KeyPath:  "/missing-key.pem",
		})
	})
}

func writeCACert(t *testing.T, dir string) string {
	t.Helper()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key: %v", err)
	}

	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "servora-ca.test",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &privKey.PublicKey, privKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certPath := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("write cert file: %v", err)
	}
	return certPath
}
