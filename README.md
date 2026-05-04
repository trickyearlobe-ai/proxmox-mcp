# proxmox-mcp

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server for [Proxmox VE](https://www.proxmox.com/), written in Go. Lets AI assistants manage your Proxmox infrastructure — VMs, containers, nodes, storage, and cluster resources.

## Features

- **19 tools** covering cluster, node, VM, container, storage, task, and raw API access
- **Multi-host support** — manage multiple Proxmox clusters from a single server
- **Auto node resolution** — VM/container tools resolve the hosting node from VMID automatically
- **Task tracking** — lifecycle actions return UPIDs that can be tracked with `get_task_status`
- **One-command IDE install** — `--install` registers in all detected IDEs
- **Raw API tool** — explore the Proxmox API directly

## Quick Start

```bash
# Install
go install github.com/trickyearlobe-ai/proxmox-mcp@latest

# Create config
proxmox-mcp --init
# Edit ~/.proxmox.yaml with your Proxmox connection details

# Install in all your IDEs
proxmox-mcp --install
```

## Tools

| Tool | Description |
|------|-------------|
| `get_cluster_status` | Cluster health and node membership |
| `list_cluster_resources` | All resources across the cluster (filterable by type) |
| `list_nodes` | All nodes with CPU, memory, disk usage |
| `get_node_status` | Detailed status of a specific node |
| `list_vms` | VMs on a node |
| `get_vm_status` | Current VM status (node auto-resolved) |
| `get_vm_config` | Full VM configuration as JSON |
| `start_vm` | Start a VM (returns task UPID) |
| `stop_vm` | Hard stop a VM |
| `shutdown_vm` | Graceful ACPI shutdown |
| `list_containers` | LXC containers on a node |
| `get_container_status` | Current container status (node auto-resolved) |
| `get_container_config` | Full container configuration as JSON |
| `start_container` | Start a container (returns task UPID) |
| `stop_container` | Hard stop a container |
| `shutdown_container` | Graceful shutdown |
| `list_storage` | Storage pools on a node with capacity |
| `get_task_status` | Track async task completion by UPID |
| `raw_api_request` | Make raw GET/POST requests to the Proxmox API |

All tools accept an optional `host` parameter to target a specific Proxmox host. If omitted, the default host is used.

## Configuration

### Config file (~/.proxmox.yaml)

Generate a template with `proxmox-mcp --init`:

```yaml
# Default host when tools don't specify one
default_host: my-proxmox

hosts:
  my-proxmox:
    url: https://pve.example.com:8006
    token_id: user@pam!mytoken
    token_secret: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    tls_insecure: true

  # Multiple hosts for failover or separate clusters
  production:
    url: https://pve-prod.example.com:8006
    token_id: admin@pam!automation
    token_secret: yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy
    tls_insecure: false
```

The config file is created with `0600` permissions. If broader permissions are detected, a warning is printed.

### Environment variables (fallback)

If `~/.proxmox.yaml` doesn't exist, these env vars are used to create a single "default" host:

| Variable | Required | Description |
|----------|----------|-------------|
| `PROXMOX_API_URL` | Yes | Base URL, e.g. `https://pve.example.com:8006` |
| `PROXMOX_TOKEN_ID` | Yes | API token ID, e.g. `user@pam!mytoken` |
| `PROXMOX_TOKEN_SECRET` | Yes | API token secret (UUID) |
| `PROXMOX_TLS_INSECURE` | No | Set to `true` to skip TLS certificate verification |

## IDE Installation

Install in all detected IDEs with a single command:

```bash
proxmox-mcp --install
```

Remove from all IDEs:

```bash
proxmox-mcp --uninstall
```

### Supported IDEs

| IDE | Config Path | Notes |
|-----|-------------|-------|
| Claude Desktop | Platform-specific | macOS, Windows, Linux |
| Claude Code | ~/.claude.json | CLI: `claude mcp add` |
| VS Code | ~/.vscode/mcp.json | Global config |
| Cursor | ~/.cursor/mcp.json | VS Code fork |
| Windsurf | ~/.codeium/windsurf/mcp_config.json | |
| Zed | ~/.config/zed/settings.json | JSONC format handled |
| Copilot CLI | ~/.copilot/mcp-config.json | All required fields added |
| JetBrains | ~/.junie/mcp/mcp.json | IntelliJ, PyCharm, etc. |

IDEs that aren't installed are automatically skipped.

### Manual configuration

If you prefer to configure manually, add to your IDE's MCP config:

```json
{
  "mcpServers": {
    "proxmox": {
      "command": "/path/to/proxmox-mcp"
    }
  }
}
```

## Creating a Proxmox API Token

1. Log into the Proxmox web UI
2. Go to **Datacenter → Permissions → API Tokens**
3. Click **Add** and configure:
   - **User**: Select an existing user (e.g. `root@pam`)
   - **Token ID**: Choose a name (e.g. `mcp`)
   - **Privilege Separation**: Uncheck for full access, or leave checked and assign specific permissions
4. Copy the **Token ID** (format: `user@realm!tokenid`) and **Secret**

## CLI Flags

```
--init        Create template config at ~/.proxmox.yaml
--install     Register in all detected IDE MCP configs
--uninstall   Remove from all IDE MCP configs
--version     Print version
```

## License

Apache License 2.0 — see [LICENSE](LICENSE).
