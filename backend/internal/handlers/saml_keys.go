package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	samlKeyMu   sync.RWMutex
	samlKey     *rsa.PrivateKey
	samlCertDER []byte
)

const (
	samlKeyFile  = "data/saml_idp_key.pem"
	samlCertFile = "data/saml_idp_cert.pem"
)

func getSAMLKeypair() (*rsa.PrivateKey, []byte) {
	samlKeyMu.RLock()
	if samlKey != nil {
		defer samlKeyMu.RUnlock()
		return samlKey, samlCertDER
	}
	samlKeyMu.RUnlock()

	samlKeyMu.Lock()
	defer samlKeyMu.Unlock()
	if samlKey != nil {
		return samlKey, samlCertDER
	}

	if key, certDER, ok := loadSAMLKeypairFromFile(); ok {
		samlKey = key
		samlCertDER = certDER
		return samlKey, samlCertDER
	}

	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	certDER, _ := generateSelfSignedCert(key)
	_ = saveSAMLKeypairToFile(key, certDER)
	samlKey = key
	samlCertDER = certDER
	return samlKey, samlCertDER
}

func loadSAMLKeypairFromFile() (*rsa.PrivateKey, []byte, bool) {
	keyPEM, err := os.ReadFile(samlKeyFile)
	if err != nil {
		return nil, nil, false
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, false
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, false
	}

	certPEM, err := os.ReadFile(samlCertFile)
	if err != nil {
		return nil, nil, false
	}
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, nil, false
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, false
	}
	return key, cert.Raw, true
}

func saveSAMLKeypairToFile(key *rsa.PrivateKey, certDER []byte) error {
	if err := os.MkdirAll(filepath.Dir(samlKeyFile), 0o755); err != nil {
		return err
	}
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	if err := os.WriteFile(samlKeyFile, keyPEM, 0o600); err != nil {
		return err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	return os.WriteFile(samlCertFile, certPEM, 0o644)
}

func generateSelfSignedCert(key *rsa.PrivateKey) ([]byte, error) {
	now := time.Now()
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "Go-SyncFlow SAML IdP",
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(3650 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	return x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
}
