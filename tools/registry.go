package tools

import (
	"fmt"
	"strings"

	"github.com/trickyearlobe-ai/proxmox-mcp/config"
	"github.com/trickyearlobe-ai/proxmox-mcp/proxmox"
)

// HostRegistry maps host names to Proxmox API clients.
type HostRegistry struct {
	clients       map[string]*proxmox.Client
	defaultHost   string
	confirmWrites bool
}

// SetConfirmWrites enables runtime human confirmation (via MCP elicitation) before
// any mutating tool acts. See confirmWrite.
func (r *HostRegistry) SetConfirmWrites(v bool) {
	r.confirmWrites = v
}

// NewHostRegistry creates a registry from the loaded config.
func NewHostRegistry(cfg *config.Config) *HostRegistry {
	clients := make(map[string]*proxmox.Client, len(cfg.Hosts))
	for name, host := range cfg.Hosts {
		clients[name] = proxmox.NewClient(host.URL, host.TokenID, host.TokenSecret, host.TLSInsecure)
	}
	return &HostRegistry{
		clients:     clients,
		defaultHost: cfg.DefaultHost,
	}
}

// GetClient returns the client for the given host name.
// If host is empty, returns the default host's client.
func (r *HostRegistry) GetClient(host string) (*proxmox.Client, string, error) {
	if host == "" {
		host = r.defaultHost
	}
	c, ok := r.clients[host]
	if !ok {
		return nil, "", fmt.Errorf("unknown host %q (available: %s)", host, strings.Join(r.HostNames(), ", "))
	}
	return c, host, nil
}

// HostNames returns all configured host names.
func (r *HostRegistry) HostNames() []string {
	names := make([]string, 0, len(r.clients))
	for name := range r.clients {
		names = append(names, name)
	}
	return names
}
