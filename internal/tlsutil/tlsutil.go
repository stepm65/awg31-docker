// Package tlsutil создаёт и обновляет самоподписанный сертификат для WebUI,
// чтобы интерфейс по умолчанию работал по HTTPS, а не по открытому HTTP.
package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	validity  = 365 * 24 * time.Hour
	renewLeft = 30 * 24 * time.Hour
)

// LoadOrCreate загружает сертификат из dir или создаёт новый. Сертификат
// пересоздаётся, если до окончания срока осталось меньше 30 дней или
// не совпадает набор SAN. Возвращает SHA-256 отпечаток для сверки в браузере.
func LoadOrCreate(dir string, sans []string) (tls.Certificate, string, error) {
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		if leaf, err := x509.ParseCertificate(cert.Certificate[0]); err == nil &&
			time.Until(leaf.NotAfter) > renewLeft && coversAll(leaf, sans) {
			return cert, Fingerprint(cert.Certificate[0]), nil
		}
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tls.Certificate{}, "", err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		return tls.Certificate{}, "", err
	}
	tpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "awg31"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, s := range sans {
		if ip := net.ParseIP(s); ip != nil {
			tpl.IPAddresses = append(tpl.IPAddresses, ip)
		} else {
			tpl.DNSNames = append(tpl.DNSNames, s)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return tls.Certificate{}, "", err
	}
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return tls.Certificate{}, "", err
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	return cert, Fingerprint(der), nil
}

func coversAll(leaf *x509.Certificate, sans []string) bool {
	for _, s := range sans {
		if leaf.VerifyHostname(s) != nil {
			return false
		}
	}
	return true
}

// Fingerprint возвращает SHA-256 DER-сертификата в виде AA:BB:CC.
func Fingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	h := hex.EncodeToString(sum[:])
	out := make([]byte, 0, len(h)+31)
	for i := 0; i < len(h); i += 2 {
		if i > 0 {
			out = append(out, ':')
		}
		out = append(out, h[i], h[i+1])
	}
	return string(out)
}

// ServerConfig возвращает tls.Config с современными параметрами.
func ServerConfig(cert tls.Certificate) *tls.Config {
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
}

// DefaultSANs — имена, которые всегда попадают в сертификат.
func DefaultSANs(extra ...string) []string {
	sans := []string{"localhost", "127.0.0.1", "::1"}
	if h, err := os.Hostname(); err == nil && h != "" {
		sans = append(sans, h)
	}
	for _, e := range extra {
		if e != "" {
			sans = append(sans, e)
		}
	}
	return sans
}

// Describe — строка для журнала.
func Describe(fp string) string {
	return fmt.Sprintf("самоподписанный сертификат, SHA-256: %s", fp)
}
