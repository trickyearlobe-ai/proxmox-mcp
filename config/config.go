package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// HostConfig holds connection details for a single Proxmox host.
type HostConfig struct {
	URL         string `yaml:"url"`
	TokenID     string `yaml:"token_id"`
	TokenSecret string `yaml:"token_secret"`
	TLSInsecure bool   `yaml:"tls_insecure"`
}

// Config is the top-level configuration.
type Config struct {
	DefaultHost string                `yaml:"default_host"`
	Hosts       map[string]HostConfig `yaml:"hosts"`
}

// ConfigPath returns the default config file path.
func ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".proxmox.yaml")
}

// Load reads config from ~/.proxmox.yaml, falling back to environment variables.
func Load() (*Config, error) {
	path := ConfigPath()

	if data, err := os.ReadFile(path); err == nil {
		return loadFromFile(path, data)
	}

	return loadFromEnv()
}

func loadFromFile(path string, data []byte) (*Config, error) {
	// Check file permissions
	info, err := os.Stat(path)
	if err == nil {
		perm := info.Mode().Perm()
		if perm&0077 != 0 {
			fmt.Fprintf(os.Stderr, "WARNING: %s has permissions %o — should be 0600. Run: chmod 600 %s\n", path, perm, path)
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}

	return &cfg, nil
}

func loadFromEnv() (*Config, error) {
	url := os.Getenv("PROXMOX_API_URL")
	tokenID := os.Getenv("PROXMOX_TOKEN_ID")
	tokenSecret := os.Getenv("PROXMOX_TOKEN_SECRET")

	if url == "" || tokenID == "" || tokenSecret == "" {
		return nil, fmt.Errorf(
			"no config found at %s and required env vars are missing.\n"+
				"Either create a config file with: proxmox-mcp --init\n"+
				"Or set: PROXMOX_API_URL, PROXMOX_TOKEN_ID, PROXMOX_TOKEN_SECRET",
			ConfigPath(),
		)
	}

	tlsInsecure := strings.EqualFold(os.Getenv("PROXMOX_TLS_INSECURE"), "true")

	return &Config{
		DefaultHost: "default",
		Hosts: map[string]HostConfig{
			"default": {
				URL:         strings.TrimRight(url, "/"),
				TokenID:     tokenID,
				TokenSecret: tokenSecret,
				TLSInsecure: tlsInsecure,
			},
		},
	}, nil
}

func validate(cfg *Config) error {
	if len(cfg.Hosts) == 0 {
		return fmt.Errorf("no hosts defined")
	}

	// Default to first host if not specified
	if cfg.DefaultHost == "" {
		for name := range cfg.Hosts {
			cfg.DefaultHost = name
			break
		}
	}

	if _, ok := cfg.Hosts[cfg.DefaultHost]; !ok {
		names := make([]string, 0, len(cfg.Hosts))
		for name := range cfg.Hosts {
			names = append(names, name)
		}
		return fmt.Errorf("default_host %q not found in hosts (available: %s)", cfg.DefaultHost, strings.Join(names, ", "))
	}

	for name, host := range cfg.Hosts {
		if host.URL == "" {
			return fmt.Errorf("host %q: url is required", name)
		}
		if host.TokenID == "" {
			return fmt.Errorf("host %q: token_id is required", name)
		}
		if host.TokenSecret == "" {
			return fmt.Errorf("host %q: token_secret is required", name)
		}
	}

	return nil
}

const configTemplate = `# Proxmox MCP Server Configuration
# ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
# This file configures the proxmox-mcp server.
# Edit the hosts below to match your Proxmox environment.
#
# SAFETY — the API token IS the access boundary
# ----------------------------------------------
# This server can create, reconfigure, and power VMs (and provision OSes). The
# only gate an AI assistant cannot bypass is the Proxmox token's own privileges,
# because they are enforced by Proxmox, not by this file or the client.
#
#   * Read-only token: assign the built-in 'PVEAuditor' role. Listing/status tools
#     work; every write tool gets a 403 from Proxmox. Use this by default.
#       pveum user token add user@pam mytoken --privsep 1
#       pveum acl modify / --roles PVEAuditor --tokens 'user@pam!mytoken'
#   * Write/provisioning token: assign 'PVEVMAdmin' (+ 'Datastore.AllocateSpace'
#     on storages you provision into). Only hand the server this when you intend
#     for it to make changes.
#
# Optional extra speed bump: set PROXMOX_CONFIRM_WRITES=true in the environment to
# make every mutating tool ask for human approval (via MCP elicitation) before it
# acts. Requires an MCP client that supports elicitation.

# Which host to use when tools don't specify one
default_host: my-proxmox

hosts:
  # Give each Proxmox host/cluster a friendly name
  my-proxmox:
    url: https://pve.example.com:8006
    token_id: user@pam!mytoken
    token_secret: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    tls_insecure: true    # set to false if using valid TLS certificates

  # Add more hosts for multiple clusters, or to separate read-only from read-write
  # production:
  #   url: https://pve-prod.example.com:8006
  #   token_id: automation@pam!readonly   # a PVEAuditor token — cannot mutate prod
  #   token_secret: yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy
  #   tls_insecure: false
`

// Init creates a template config file at ~/.proxmox.yaml.
func Init() error {
	path := ConfigPath()

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists — edit it directly or delete it first", path)
	}

	if err := os.WriteFile(path, []byte(configTemplate), 0600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	fmt.Fprintf(os.Stderr, "Created %s (mode 0600)\nEdit this file with your Proxmox connection details.\n", path)
	return nil
}
