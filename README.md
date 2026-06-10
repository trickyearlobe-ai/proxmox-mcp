# proxmox-mcp

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server for [Proxmox VE](https://www.proxmox.com/), written in Go. Lets AI assistants manage your Proxmox infrastructure — VMs, containers, nodes, storage, and cluster resources.

## Features

- **39 tools** covering cluster, node, VM, container, storage, task, provisioning, teardown, serial console, and raw API access
- **VM provisioning & unattended OS install** — create VMs and install an OS via cloud-init, kickstart, Ubuntu autoinstall, or Windows autounattend
- **Serial console access** — read from and type into a VM's serial console (drive serial-only guests like network appliances)
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
| `list_storage_content` | List ISOs, disk images, templates, and backups on a storage |
| `get_task_status` | Track async task completion by UPID |
| `create_vm` | Create a new QEMU/KVM VM (returns task UPID) |
| `update_vm_config` | Set VM config keys — attach disks/CDs, cloud-init, boot order, import disks |
| `clone_vm` | Clone a VM or template into a new VM |
| `resize_vm_disk` | Grow a VM disk |
| `download_url_to_storage` | Download an ISO or cloud image from a URL onto a storage |
| `create_answer_media` | Package an unattended answer file into a labeled ISO and upload it |
| `get_guest_agent_info` | Query the guest agent (detect when an OS install has booted) |
| `wait_for_guest_agent` | Block until the guest agent responds (emits progress), then return its IPs |
| `read_serial_console` | Read output from a VM's serial console (requires a serial device) |
| `send_serial_console` | Type into a VM's serial console and capture the response |
| `send_key` | Press keys at a VM's VGA console (boot prompts, BIOS, ctrl-alt-delete) |
| `delete_vm` | Permanently destroy a stopped VM and its disks |
| `delete_container` | Permanently destroy a stopped LXC container and its disks |
| `delete_storage_content` | Delete a single volume (ISO, image, backup) from a storage |
| `get_token_permissions` | Show a token/user's effective privileges (explain a 403) |
| `list_acl` | List access-control entries (role → user/token → path) |
| `list_roles` | List roles and the privileges each grants |
| `raw_api_request` | Make raw GET/POST/PUT/DELETE requests to the Proxmox API |

All tools accept an optional `host` parameter to target a specific Proxmox host. If omitted, the default host is used.

## Provisioning & unattended OS install

The provisioning tools let an assistant create a VM and bring up an OS end-to-end. Two
approaches are supported, neither of which needs console interaction:

**Cloud images / cloud-init (or template clone).** Boot a prebuilt cloud image and let
cloud-init configure it on first boot:

1. `download_url_to_storage` a cloud image (`content: import`) — or `clone_vm` an existing
   cloud-init template.
2. `create_vm`, then `update_vm_config` to import the disk
   (`scsi0: <storage>:0,import-from=<volid>`), set cloud-init keys
   (`ciuser`, `cipassword`/`sshkeys`, `ipconfig0`), attach a cloud-init drive
   (`ide2: <storage>:cloudinit`), and set `boot: order=scsi0`.
3. `resize_vm_disk` if needed, then `start_vm`.

**Real installer + answer file.** Attach the install ISO plus a generated answer ISO that
the installer auto-discovers by volume label — covers **kickstart** (RHEL/Fedora),
**Ubuntu autoinstall**, and **Windows autounattend**:

1. `download_url_to_storage` the install ISO (`content: iso`).
2. `create_answer_media` — the assistant authors the answer file (`ks.cfg` / cloud-init
   `user-data` / `autounattend.xml`); this packages it into a small ISO with the right
   label (`OEMDRV` / `CIDATA` / removable media) using Rock Ridge + Joliet so the exact
   filenames are preserved.
3. `create_vm`, `update_vm_config` to attach both CDs and set boot order, then `start_vm`.
4. `wait_for_guest_agent` (or poll `get_guest_agent_info`) until the installed OS reboots
   and the guest agent responds — it returns the VM's IP addresses.

Notes:
- `import-from` requires Proxmox VE 8+.
- For a graphical installer ISO (e.g. AlmaLinux/Fedora) keep the default `vga`; do **not**
  set `vga=serial0` — the installer renders to VGA and a serial-only display will appear
  blank. To watch/drive the *installed* system over serial, have the kickstart add
  `console=ttyS0` to the bootloader and give the VM a `serial0: socket` device.
- Put `qemu-guest-agent` in the answer file's package list (it's in AppStream / universe,
  not on minimal install ISOs — add the online repo) so `get_guest_agent_info` works.
- Windows also needs a `virtio-win.iso` attached as a third CD (it carries the storage/net
  drivers and the `qemu-ga` MSI), and shows a "Press any key to boot from CD…" prompt on
  first boot — clear it with `send_key` (`keys: ["ret"]`).
- Debian preseed and SUSE AutoYaST are **not** supported — they require kernel boot-args
  the Proxmox API can't set without remastering the install ISO.

### Serial console

`read_serial_console` and `send_serial_console` connect to a VM's serial device via
Proxmox's term-proxy (the VM needs `serial0: socket`). This is the way to drive
**serial-only guests** — network appliances (Cisco Nexus 9000v, Arista vEOS, etc.) and any
OS configured with a serial console — letting an assistant read boot output, answer
prompts, log in, and run commands without a graphical console.

## Safety

These tools can create, reconfigure, and power VMs. The real, unbypassable access boundary
is the **Proxmox API token's own role/ACL** — not this server — because Proxmox enforces it
regardless of what the client or config requests:

- Use a **`PVEAuditor`** token for read-only access: status/list tools work; every write
  tool gets a `403`.
- Use a **`PVEVMAdmin`** (+ `Datastore.AllocateSpace`) token only when you want the server
  to make changes.
- `download_url_to_storage` and `send_key` need extra privilege (`Sys.Modify` on `/`); a
  privilege-separated token without it gets a `403`. Grant that role or use a non-privsep /
  `root@pam` token if you need URL downloads or console keystrokes.

Optionally set `PROXMOX_CONFIRM_WRITES=true` to require human approval (via MCP elicitation)
before any mutating tool acts. This needs a client that supports elicitation and is a
convenience speed bump, not the security boundary.

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
