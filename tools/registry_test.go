package tools

import (
	"testing"

	"github.com/trickyearlobe-ai/proxmox-mcp/config"
)

func TestNewHostRegistry_SingleHost(t *testing.T) {
	cfg := &config.Config{
		DefaultHost: "lab",
		Hosts: map[string]config.HostConfig{
			"lab": {
				URL:         "https://pve:8006",
				TokenID:     "user@pam!tok",
				TokenSecret: "secret",
			},
		},
	}

	reg := NewHostRegistry(cfg)

	client, host, err := reg.GetClient("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "lab" {
		t.Errorf("host = %q, want lab", host)
	}
	if client == nil {
		t.Fatal("client should not be nil")
	}
}

func TestNewHostRegistry_MultiHost(t *testing.T) {
	cfg := &config.Config{
		DefaultHost: "prod",
		Hosts: map[string]config.HostConfig{
			"lab": {
				URL:         "https://lab:8006",
				TokenID:     "user@pam!a",
				TokenSecret: "aaa",
			},
			"prod": {
				URL:         "https://prod:8006",
				TokenID:     "admin@pam!b",
				TokenSecret: "bbb",
			},
		},
	}

	reg := NewHostRegistry(cfg)

	// Default host
	_, host, err := reg.GetClient("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "prod" {
		t.Errorf("default host = %q, want prod", host)
	}

	// Explicit host
	_, host, err = reg.GetClient("lab")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "lab" {
		t.Errorf("explicit host = %q, want lab", host)
	}
}

func TestNewHostRegistry_UnknownHost(t *testing.T) {
	cfg := &config.Config{
		DefaultHost: "lab",
		Hosts: map[string]config.HostConfig{
			"lab": {URL: "https://x", TokenID: "a", TokenSecret: "b"},
		},
	}

	reg := NewHostRegistry(cfg)

	_, _, err := reg.GetClient("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown host")
	}
	if !containsSubstr(err.Error(), "nonexistent") {
		t.Errorf("error should mention the host name: %v", err)
	}
	if !containsSubstr(err.Error(), "lab") {
		t.Errorf("error should list available hosts: %v", err)
	}
}

func TestNewHostRegistry_HostNames(t *testing.T) {
	cfg := &config.Config{
		DefaultHost: "a",
		Hosts: map[string]config.HostConfig{
			"a": {URL: "https://a", TokenID: "a", TokenSecret: "a"},
			"b": {URL: "https://b", TokenID: "b", TokenSecret: "b"},
			"c": {URL: "https://c", TokenID: "c", TokenSecret: "c"},
		},
	}

	reg := NewHostRegistry(cfg)
	names := reg.HostNames()

	if len(names) != 3 {
		t.Fatalf("len(HostNames) = %d, want 3", len(names))
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !nameSet[want] {
			t.Errorf("missing host name %q", want)
		}
	}
}

func TestGetClient_ReturnsDistinctClients(t *testing.T) {
	cfg := &config.Config{
		DefaultHost: "a",
		Hosts: map[string]config.HostConfig{
			"a": {URL: "https://host-a", TokenID: "a", TokenSecret: "a"},
			"b": {URL: "https://host-b", TokenID: "b", TokenSecret: "b"},
		},
	}

	reg := NewHostRegistry(cfg)

	clientA, _, _ := reg.GetClient("a")
	clientB, _, _ := reg.GetClient("b")

	if clientA == clientB {
		t.Error("different hosts should return different clients")
	}
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
