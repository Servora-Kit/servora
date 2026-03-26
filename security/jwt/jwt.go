package jwt

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

type Signer struct {
	key *rsa.PrivateKey
	kid string
}

func NewSigner(privateKeyPEM []byte) (*Signer, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("jwt: failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		pkcs8Key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("jwt: failed to parse private key: %w", err)
		}
		var ok bool
		key, ok = pkcs8Key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("jwt: PKCS#8 key is not RSA")
		}
	}

	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("jwt: failed to marshal public key: %w", err)
	}
	hash := sha256.Sum256(der)
	kid := hex.EncodeToString(hash[:])[:8]

	return &Signer{key: key, kid: kid}, nil
}

func (s *Signer) Sign(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.kid
	return token.SignedString(s.key)
}

func (s *Signer) PrivateKey() *rsa.PrivateKey {
	return s.key
}

func (s *Signer) PublicKey() *rsa.PublicKey {
	return &s.key.PublicKey
}

func (s *Signer) KID() string {
	return s.kid
}

type Verifier struct {
	publicKeys map[string]*rsa.PublicKey
}

func NewVerifier() *Verifier {
	return &Verifier{publicKeys: make(map[string]*rsa.PublicKey)}
}

func (v *Verifier) AddKey(kid string, pub *rsa.PublicKey) {
	v.publicKeys[kid] = pub
}

func (v *Verifier) Verify(tokenString string, claims jwt.Claims) error {
	_, err := jwt.ParseWithClaims(tokenString, claims, v.keyFunc)
	return err
}

func (v *Verifier) keyFunc(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, fmt.Errorf("jwt: unexpected signing method: %v", token.Header["alg"])
	}

	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("jwt: missing kid in token header")
	}

	pub, exists := v.publicKeys[kid]
	if !exists {
		return nil, fmt.Errorf("jwt: unknown kid: %s", kid)
	}

	return pub, nil
}

type authKey struct{}

func NewContext[T any](ctx context.Context, claims *T) context.Context {
	return context.WithValue(ctx, authKey{}, claims)
}

func FromContext[T any](ctx context.Context) (*T, bool) {
	claims, ok := ctx.Value(authKey{}).(*T)
	return claims, ok
}
