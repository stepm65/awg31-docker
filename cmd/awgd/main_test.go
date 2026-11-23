package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePrefixes(t *testing.T) {
	got, err := parsePrefixes("10.0.0.0/8, 192.168.1.1 ,fd00::1")
	if err != nil || len(got) != 3 {
		t.Fatalf("%v %v", got, err)
	}
	if got[1].String() != "192.168.1.1/32" || got[2].String() != "fd00::1/128" {
		t.Fatalf("одиночные адреса должны становиться /32 и /128: %v", got)
	}
	if got, err := parsePrefixes(""); err != nil || len(got) != 0 {
		t.Fatalf("пустая строка: %v %v", got, err)
	}
	if _, err := parsePrefixes("not-an-ip"); err == nil {
		t.Fatal("мусор должен давать ошибку, а не молча игнорироваться")
	}
}

func TestHealthURL(t *testing.T) {
	t.Setenv("UI_LISTEN", "")
	t.Setenv("UI_TLS", "")
	if got := healthURL(); got != "https://127.0.0.1:8443/healthz" {
		t.Errorf("умолчание: %s", got)
	}
	t.Setenv("UI_LISTEN", "0.0.0.0:9000")
	t.Setenv("UI_TLS", "off")
	if got := healthURL(); got != "http://127.0.0.1:9000/healthz" {
		t.Errorf("кастомный порт и без TLS: %s", got)
	}
}

func TestEnvBool(t *testing.T) {
	t.Setenv("X_B", "ON")
	if !envBool("X_B", false) {
		t.Error("ON должно быть true")
	}
	t.Setenv("X_B", "off")
	if envBool("X_B", true) {
		t.Error("off должно быть false")
	}
	t.Setenv("X_B", "maybe")
	if !envBool("X_B", true) || envBool("X_B", false) {
		t.Error("непонятное значение должно давать значение по умолчанию")
	}
}

func TestAdminPasswordFromFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "pw")
	_ = os.WriteFile(f, []byte("secret-from-file\n"), 0o600)
	t.Setenv("ADMIN_PASSWORD_FILE", f)
	t.Setenv("ADMIN_PASSWORD", "ignored")
	pw, err := adminPassword()
	if err != nil || pw != "secret-from-file" {
		t.Fatalf("пароль из файла должен иметь приоритет и очищаться от перевода строки: %q %v", pw, err)
	}
	t.Setenv("ADMIN_PASSWORD_FILE", filepath.Join(t.TempDir(), "missing"))
	if _, err := adminPassword(); err == nil {
		t.Fatal("отсутствующий файл должен давать ошибку")
	}
}
