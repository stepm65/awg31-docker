package tlsutil

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateIsStable(t *testing.T) {
	dir := t.TempDir()
	sans := []string{"localhost", "127.0.0.1", "router.lan"}
	c1, fp1, err := LoadOrCreate(dir, sans)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(c1.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sans {
		if err := leaf.VerifyHostname(s); err != nil {
			t.Errorf("SAN %s не покрыт: %v", s, err)
		}
	}
	info, err := os.Stat(filepath.Join(dir, "key.pem"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("ключ должен иметь права 0600: %v %v", info, err)
	}
	// повторная загрузка возвращает тот же сертификат
	_, fp2, err := LoadOrCreate(dir, sans)
	if err != nil || fp1 != fp2 {
		t.Fatalf("сертификат должен переиспользоваться: %v %s %s", err, fp1, fp2)
	}
	// новый SAN приводит к перевыпуску
	_, fp3, err := LoadOrCreate(dir, append(sans, "new.example"))
	if err != nil || fp3 == fp1 {
		t.Fatalf("при смене SAN сертификат должен перевыпускаться: %v", err)
	}
}

func TestFingerprintFormat(t *testing.T) {
	fp := Fingerprint([]byte("x"))
	if len(fp) != 95 || fp[2] != ':' {
		t.Fatalf("неожиданный формат отпечатка: %s", fp)
	}
}
