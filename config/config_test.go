package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromYAML_SingleHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".proxmox.yaml")

	yaml := `
default_host: lab
hosts:
  lab:
    url: https://pve.example.com:8006
    token_id: user@pam!tok
    token_secret: secret-uuid
    tls_insecure: true
`
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFromFile(path, []byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DefaultHost != "lab" {
		t.Errorf("DefaultHost = %q, want %q", cfg.DefaultHost, "lab")
	}
	if len(cfg.Hosts) != 1 {
		t.Fatalf("len(Hosts) = %d, want 1", len(cfg.Hosts))
	}

	h := cfg.Hosts["lab"]
	if h.URL != "https://pve.example.com:8006" {
		t.Errorf("URL = %q", h.URL)
	}
	if h.TokenID != "user@pam!tok" {
		t.Errorf("TokenID = %q", h.TokenID)
	}
	if h.TokenSecret != "secret-uuid" {
		t.Errorf("TokenSecret = %q", h.TokenSecret)
	}
	if !h.TLSInsecure {
		t.Error("TLSInsecure should be true")
	}
}

func TestLoadFromYAML_MultiHost(t *testing.T) {
	yaml := `
default_host: prod
hosts:
  lab:
    url: https://lab:8006
    token_id: user@pam!a
    token_secret: aaa
  prod:
    url: https://prod:8006
    token_id: admin@pam!b
    token_secret: bbb
    tls_insecure: false
`
	cfg, err := loadFromFile("test.yaml", []byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DefaultHost != "prod" {
		t.Errorf("DefaultHost = %q, want %q", cfg.DefaultHost, "prod")
	}
	if len(cfg.Hosts) != 2 {
		t.Fatalf("len(Hosts) = %d, want 2", len(cfg.Hosts))
	}
}

func TestLoadFromYAML_DefaultHostInferred(t *testing.T) {
	yaml := `
hosts:
  only-one:
    url: https://pve:8006
    token_id: user@pam!tok
    token_secret: secret
`
	cfg, err := loadFromFile("test.yaml", []byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DefaultHost != "only-one" {
		t.Errorf("DefaultHost = %q, want %q", cfg.DefaultHost, "only-one")
	}
}

func TestValidate_NoHosts(t *testing.T) {
	cfg := &Config{Hosts: map[string]HostConfig{}}
	if err := validate(cfg); err == nil {
		t.Error("expected error for no hosts")
	}
}

func TestValidate_DefaultHostMissing(t *testing.T) {
	cfg := &Config{
		DefaultHost: "nonexistent",
		Hosts: map[string]HostConfig{
			"lab": {URL: "https://x", TokenID: "a", TokenSecret: "b"},
		},
	}
	err := validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing default_host")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found': %v", err)
	}
}

func TestValidate_MissingURL(t *testing.T) {
	cfg := &Config{
		DefaultHost: "lab",
		Hosts: map[string]HostConfig{
			"lab": {TokenID: "a", TokenSecret: "b"},
		},
	}
	err := validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing url")
	}
	if !contains(err.Error(), "url is required") {
		t.Errorf("error should mention url: %v", err)
	}
}

func TestValidate_MissingTokenID(t *testing.T) {
	cfg := &Config{
		DefaultHost: "lab",
		Hosts: map[string]HostConfig{
			"lab": {URL: "https://x", TokenSecret: "b"},
		},
	}
	err := validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing token_id")
	}
}

func TestValidate_MissingTokenSecret(t *testing.T) {
	cfg := &Config{
		DefaultHost: "lab",
		Hosts: map[string]HostConfig{
			"lab": {URL: "https://x", TokenID: "a"},
		},
	}
	err := validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing token_secret")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("PROXMOX_API_URL", "https://env-host:8006/")
	t.Setenv("PROXMOX_TOKEN_ID", "envuser@pam!tok")
	t.Setenv("PROXMOX_TOKEN_SECRET", "env-secret")
	t.Setenv("PROXMOX_TLS_INSECURE", "TRUE")

	cfg, err := loadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DefaultHost != "default" {
		t.Errorf("DefaultHost = %q", cfg.DefaultHost)
	}

	h := cfg.Hosts["default"]
	if h.URL != "https://env-host:8006" {
		t.Errorf("URL = %q, trailing slash should be trimmed", h.URL)
	}
	if h.TokenID != "envuser@pam!tok" {
		t.Errorf("TokenID = %q", h.TokenID)
	}
	if !h.TLSInsecure {
		t.Error("TLSInsecure should be true")
	}
}

func TestLoadFromEnv_MissingVars(t *testing.T) {
	t.Setenv("PROXMOX_API_URL", "")
	t.Setenv("PROXMOX_TOKEN_ID", "")
	t.Setenv("PROXMOX_TOKEN_SECRET", "")

	_, err := loadFromEnv()
	if err == nil {
		t.Fatal("expected error when env vars are missing")
	}
}

func TestInit_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	if err := Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	path := filepath.Join(dir, ".proxmox.yaml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions = %o, want 0600", info.Mode().Perm())
	}

	data, _ := os.ReadFile(path)
	if !contains(string(data), "default_host") {
		t.Error("template should contain default_host")
	}
	if !contains(string(data), "my-proxmox") {
		t.Error("template should contain my-proxmox")
	}
}

func TestInit_RefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	path := filepath.Join(dir, ".proxmox.yaml")
	os.WriteFile(path, []byte("existing"), 0600)

	err := Init()
	if err == nil {
		t.Fatal("expected error when file already exists")
	}
	if !contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists': %v", err)
	}
}

func TestLoadFromYAML_InvalidYAML(t *testing.T) {
	_, err := loadFromFile("test.yaml", []byte("not: [valid: yaml: {"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
