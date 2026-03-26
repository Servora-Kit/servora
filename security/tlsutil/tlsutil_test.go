package tlsutil

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
)

func TestNewServerConfig(t *testing.T) {
	tmp := t.TempDir()
	certPath, keyPath := writeSelfSignedPair(t, tmp)

	cfg, err := NewServerConfig(ServerConfigOptions{
		CertPath: certPath,
		KeyPath:  keyPath,
	})
	if err != nil {
		t.Fatalf("NewServerConfig() error = %v", err)
	}
	if cfg.MinVersion != defaultMinVersion {
		t.Fatalf("expected MinVersion=%d, got %d", defaultMinVersion, cfg.MinVersion)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(cfg.Certificates))
	}
}

func TestNewServerConfig_MissingFiles(t *testing.T) {
	_, err := NewServerConfig(ServerConfigOptions{})
	if err == nil {
		t.Fatal("expected missing cert/key to return error")
	}
}

func TestNewClientConfig_Defaults(t *testing.T) {
	cfg, err := NewClientConfig(ClientConfigOptions{})
	if err != nil {
		t.Fatalf("NewClientConfig() error = %v", err)
	}
	if cfg.MinVersion != defaultMinVersion {
		t.Fatalf("expected MinVersion=%d, got %d", defaultMinVersion, cfg.MinVersion)
	}
	if cfg.RootCAs != nil {
		t.Fatal("expected nil RootCAs when ca_path is empty")
	}
	if len(cfg.Certificates) != 0 {
		t.Fatalf("expected no client certificates, got %d", len(cfg.Certificates))
	}
}

func TestNewClientConfig_WithCAAndClientCert(t *testing.T) {
	tmp := t.TempDir()
	certPath, keyPath := writeSelfSignedPair(t, tmp)
	caPath := filepath.Join(tmp, "ca.pem")
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read cert file: %v", err)
	}
	if err := os.WriteFile(caPath, certPEM, 0o600); err != nil {
		t.Fatalf("write ca file: %v", err)
	}

	cfg, err := NewClientConfig(ClientConfigOptions{
		CAPath:   caPath,
		CertPath: certPath,
		KeyPath:  keyPath,
	})
	if err != nil {
		t.Fatalf("NewClientConfig() error = %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("expected RootCAs to be loaded")
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("expected 1 client certificate, got %d", len(cfg.Certificates))
	}
}

func TestNewClientConfig_CertKeyMustAppearTogether(t *testing.T) {
	_, err := NewClientConfig(ClientConfigOptions{CertPath: "/tmp/client.crt"})
	if err == nil {
		t.Fatal("expected mismatched cert/key to return error")
	}
}

func TestLoadCertPoolFromPEMFile_InvalidPEM(t *testing.T) {
	tmp := t.TempDir()
	invalidPath := filepath.Join(tmp, "invalid.pem")
	if err := os.WriteFile(invalidPath, []byte("not pem"), 0o600); err != nil {
		t.Fatalf("write invalid pem file: %v", err)
	}

	_, err := LoadCertPoolFromPEMFile(invalidPath)
	if err == nil {
		t.Fatal("expected invalid PEM to return error")
	}
}

func writeSelfSignedPair(t *testing.T, dir string) (string, string) {
	t.Helper()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key: %v", err)
	}

	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "servora.test",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"servora.test"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &privKey.PublicKey, privKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("write cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	return certPath, keyPath
}
