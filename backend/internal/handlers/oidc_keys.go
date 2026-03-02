package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"sync"
)

var (
	oidcKeyMu     sync.RWMutex
	oidcPrivate   *rsa.PrivateKey
	oidcPublicDER []byte
	oidcKid       string
)

const oidcKeyFile = "data/oidc_signing_key.pem"

func getOIDCSigningKey() (*rsa.PrivateKey, []byte, string) {
	oidcKeyMu.RLock()
	if oidcPrivate != nil {
		defer oidcKeyMu.RUnlock()
		return oidcPrivate, oidcPublicDER, oidcKid
	}
	oidcKeyMu.RUnlock()

	oidcKeyMu.Lock()
	defer oidcKeyMu.Unlock()
	if oidcPrivate != nil {
		return oidcPrivate, oidcPublicDER, oidcKid
	}

	if key, pubDER, kid, ok := loadOIDCKeyFromFile(); ok {
		oidcPrivate = key
		oidcPublicDER = pubDER
		oidcKid = kid
		return oidcPrivate, oidcPublicDER, oidcKid
	}

	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubDER, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	kid := kidFromDER(pubDER)
	_ = saveOIDCKeyToFile(key)

	oidcPrivate = key
	oidcPublicDER = pubDER
	oidcKid = kid
	return oidcPrivate, oidcPublicDER, oidcKid
}

func loadOIDCKeyFromFile() (*rsa.PrivateKey, []byte, string, bool) {
	data, err := os.ReadFile(oidcKeyFile)
	if err != nil {
		return nil, nil, "", false
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, nil, "", false
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, "", false
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, nil, "", false
	}
	return key, pubDER, kidFromDER(pubDER), true
}

func saveOIDCKeyToFile(key *rsa.PrivateKey) error {
	if err := os.MkdirAll(filepath.Dir(oidcKeyFile), 0o755); err != nil {
		return err
	}
	b := x509.MarshalPKCS1PrivateKey(key)
	p := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: b})
	return os.WriteFile(oidcKeyFile, p, 0o600)
}

func kidFromDER(pubDER []byte) string {
	sum := sha256.Sum256(pubDER)
	return hex.EncodeToString(sum[:8])
}

func base64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}
