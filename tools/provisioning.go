package tools

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// addInt sets a form value only when non-zero, so omitted optional fields don't
// override Proxmox defaults.
func addInt(v url.Values, key string, val int) {
	if val != 0 {
		v.Set(key, strconv.Itoa(val))
	}
}

// addStr sets a form value only when non-empty.
func addStr(v url.Values, key, val string) {
	if val != "" {
		v.Set(key, val)
	}
}

// -- create_vm --

type CreateVMInput struct {
	Host    string            `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node    string            `json:"node" jsonschema:"node to create the VM on,required"`
	VMID    int               `json:"vmid" jsonschema:"unique VM ID to assign,required"`
	Name    string            `json:"name,omitempty" jsonschema:"VM name"`
	Cores   int               `json:"cores,omitempty" jsonschema:"number of CPU cores per socket (default 1)"`
	Sockets int               `json:"sockets,omitempty" jsonschema:"number of CPU sockets (default 1)"`
	Memory  int               `json:"memory,omitempty" jsonschema:"memory in MiB (e.g. 2048)"`
	OSType  string            `json:"ostype,omitempty" jsonschema:"guest OS type hint, e.g. l26 (Linux 2.6+), win11, win10"`
	BIOS    string            `json:"bios,omitempty" jsonschema:"firmware: seabios (default) or ovmf (UEFI, required for Windows 11)"`
	Machine string            `json:"machine,omitempty" jsonschema:"machine type, e.g. q35"`
	SCSIHW  string            `json:"scsihw,omitempty" jsonschema:"SCSI controller, e.g. virtio-scsi-single"`
	Net0    string            `json:"net0,omitempty" jsonschema:"primary NIC spec, e.g. virtio,bridge=vmbr0"`
	Agent   string            `json:"agent,omitempty" jsonschema:"QEMU guest agent, e.g. 1 to enable"`
	Config  map[string]string `json:"config,omitempty" jsonschema:"additional raw VM config key/value pairs (e.g. scsi0, ide2, efidisk0, boot, ipconfig0). Values use Proxmox option syntax like 'local-lvm:32'"`
}

type VMCreateOutput struct {
	Host string `json:"host" jsonschema:"Proxmox host that was queried"`
	Node string `json:"node" jsonschema:"node the VM was created on"`
	VMID int    `json:"vmid" jsonschema:"VM ID"`
	UPID string `json:"upid" jsonschema:"task UPID — track with get_task_status"`
}

func createVMHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, CreateVMInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateVMInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, VMCreateOutput{}, err
		}
		if err := confirmWrite(ctx, reg, req, fmt.Sprintf("create_vm: create VM %d on %s/%s", input.VMID, host, input.Node)); err != nil {
			return nil, VMCreateOutput{}, err
		}

		form := url.Values{}
		form.Set("vmid", strconv.Itoa(input.VMID))
		addStr(form, "name", input.Name)
		addInt(form, "cores", input.Cores)
		addInt(form, "sockets", input.Sockets)
		addInt(form, "memory", input.Memory)
		addStr(form, "ostype", input.OSType)
		addStr(form, "bios", input.BIOS)
		addStr(form, "machine", input.Machine)
		addStr(form, "scsihw", input.SCSIHW)
		addStr(form, "net0", input.Net0)
		addStr(form, "agent", input.Agent)
		for k, val := range input.Config {
			form.Set(k, val)
		}

		var upid string
		if err := client.PostForm(ctx, fmt.Sprintf("/nodes/%s/qemu", input.Node), form, &upid); err != nil {
			return nil, VMCreateOutput{}, fmt.Errorf("failed to create VM %d: %w", input.VMID, err)
		}
		return nil, VMCreateOutput{Host: host, Node: input.Node, VMID: input.VMID, UPID: upid}, nil
	}
}

// -- update_vm_config --

type UpdateVMConfigInput struct {
	Host   string            `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node   string            `json:"node,omitempty" jsonschema:"node name (optional — auto-resolved from VMID if omitted)"`
	VMID   int               `json:"vmid" jsonschema:"VM ID,required"`
	Config map[string]string `json:"config" jsonschema:"VM config key/value pairs to set. Examples: attach a CD 'ide2'='local:iso/x.iso,media=cdrom'; import a disk 'scsi0'='local-lvm:0,import-from=local:import/img.qcow2'; cloud-init 'ciuser'='root','sshkeys'=<urlencoded>,'ipconfig0'='ip=dhcp'; boot order 'boot'='order=scsi0',required"`
	Delete string            `json:"delete,omitempty" jsonschema:"comma-separated config keys to remove (e.g. 'ide2,unused0')"`
}

type VMConfigUpdateOutput struct {
	Host    string   `json:"host" jsonschema:"Proxmox host that was queried"`
	Node    string   `json:"node" jsonschema:"node hosting the VM"`
	VMID    int      `json:"vmid" jsonschema:"VM ID"`
	Applied []string `json:"applied" jsonschema:"config keys that were set"`
	UPID    string   `json:"upid,omitempty" jsonschema:"task UPID if the change is asynchronous"`
}

func updateVMConfigHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, UpdateVMConfigInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input UpdateVMConfigInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, VMConfigUpdateOutput{}, err
		}
		node, err := resolveVMNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, VMConfigUpdateOutput{}, err
		}
		if err := confirmWrite(ctx, reg, req, fmt.Sprintf("update_vm_config: change VM %d on %s/%s", input.VMID, host, node)); err != nil {
			return nil, VMConfigUpdateOutput{}, err
		}

		form := url.Values{}
		applied := make([]string, 0, len(input.Config))
		for k, val := range input.Config {
			form.Set(k, val)
			applied = append(applied, k)
		}
		addStr(form, "delete", input.Delete)

		var upid string
		if err := client.PostForm(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", node, input.VMID), form, &upid); err != nil {
			return nil, VMConfigUpdateOutput{}, fmt.Errorf("failed to update config for VM %d: %w", input.VMID, err)
		}
		return nil, VMConfigUpdateOutput{Host: host, Node: node, VMID: input.VMID, Applied: applied, UPID: upid}, nil
	}
}

// -- clone_vm --

type CloneVMInput struct {
	Host    string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node    string `json:"node,omitempty" jsonschema:"node hosting the source VM (optional — auto-resolved from VMID if omitted)"`
	VMID    int    `json:"vmid" jsonschema:"source VM/template ID to clone from,required"`
	NewID   int    `json:"newid" jsonschema:"VM ID to assign to the clone,required"`
	Name    string `json:"name,omitempty" jsonschema:"name for the new VM"`
	Full    bool   `json:"full,omitempty" jsonschema:"full clone (independent copy) instead of linked clone"`
	Storage string `json:"storage,omitempty" jsonschema:"target storage for a full clone"`
	Target  string `json:"target,omitempty" jsonschema:"target node (for cross-node clone)"`
}

type VMCloneOutput struct {
	Host  string `json:"host" jsonschema:"Proxmox host that was queried"`
	Node  string `json:"node" jsonschema:"source node"`
	VMID  int    `json:"vmid" jsonschema:"source VM ID"`
	NewID int    `json:"newid" jsonschema:"new VM ID"`
	UPID  string `json:"upid" jsonschema:"task UPID — track with get_task_status"`
}

func cloneVMHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, CloneVMInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CloneVMInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, VMCloneOutput{}, err
		}
		node, err := resolveVMNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, VMCloneOutput{}, err
		}
		if err := confirmWrite(ctx, reg, req, fmt.Sprintf("clone_vm: clone VM %d to new VM %d on %s/%s", input.VMID, input.NewID, host, node)); err != nil {
			return nil, VMCloneOutput{}, err
		}

		form := url.Values{}
		form.Set("newid", strconv.Itoa(input.NewID))
		addStr(form, "name", input.Name)
		addStr(form, "storage", input.Storage)
		addStr(form, "target", input.Target)
		if input.Full {
			form.Set("full", "1")
		}

		var upid string
		if err := client.PostForm(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/clone", node, input.VMID), form, &upid); err != nil {
			return nil, VMCloneOutput{}, fmt.Errorf("failed to clone VM %d: %w", input.VMID, err)
		}
		return nil, VMCloneOutput{Host: host, Node: node, VMID: input.VMID, NewID: input.NewID, UPID: upid}, nil
	}
}

// -- resize_vm_disk --

type ResizeDiskInput struct {
	Host string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node string `json:"node,omitempty" jsonschema:"node name (optional — auto-resolved from VMID if omitted)"`
	VMID int    `json:"vmid" jsonschema:"VM ID,required"`
	Disk string `json:"disk" jsonschema:"disk to resize, e.g. scsi0,required"`
	Size string `json:"size" jsonschema:"new size or increment, e.g. '+10G' to grow by 10GiB or '32G' for absolute,required"`
}

type DiskResizeOutput struct {
	Host string `json:"host" jsonschema:"Proxmox host that was queried"`
	Node string `json:"node" jsonschema:"node hosting the VM"`
	VMID int    `json:"vmid" jsonschema:"VM ID"`
	Disk string `json:"disk" jsonschema:"disk that was resized"`
	Size string `json:"size" jsonschema:"requested size"`
}

func resizeDiskHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, ResizeDiskInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ResizeDiskInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, DiskResizeOutput{}, err
		}
		node, err := resolveVMNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, DiskResizeOutput{}, err
		}
		if err := confirmWrite(ctx, reg, req, fmt.Sprintf("resize_vm_disk: resize %s on VM %d to %s (%s/%s)", input.Disk, input.VMID, input.Size, host, node)); err != nil {
			return nil, DiskResizeOutput{}, err
		}

		form := url.Values{}
		form.Set("disk", input.Disk)
		form.Set("size", input.Size)
		if err := client.PutForm(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/resize", node, input.VMID), form, nil); err != nil {
			return nil, DiskResizeOutput{}, fmt.Errorf("failed to resize disk %s on VM %d: %w", input.Disk, input.VMID, err)
		}
		return nil, DiskResizeOutput{Host: host, Node: node, VMID: input.VMID, Disk: input.Disk, Size: input.Size}, nil
	}
}

// -- download_url_to_storage --

type DownloadURLInput struct {
	Host     string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node     string `json:"node" jsonschema:"node to download on,required"`
	Storage  string `json:"storage" jsonschema:"target storage (must allow the chosen content type),required"`
	URL      string `json:"url" jsonschema:"source URL to download from,required"`
	Content  string `json:"content" jsonschema:"content type: 'iso' for install ISOs, 'import' for disk images (qcow2/raw),required"`
	Filename string `json:"filename" jsonschema:"destination filename on the storage,required"`
	Checksum string `json:"checksum,omitempty" jsonschema:"optional expected checksum"`
	Algo     string `json:"checksum_algorithm,omitempty" jsonschema:"checksum algorithm, e.g. sha256 (required if checksum given)"`
}

type DownloadURLOutput struct {
	Host     string `json:"host" jsonschema:"Proxmox host that was queried"`
	Node     string `json:"node" jsonschema:"node performing the download"`
	Storage  string `json:"storage" jsonschema:"target storage"`
	Filename string `json:"filename" jsonschema:"destination filename"`
	UPID     string `json:"upid" jsonschema:"task UPID — track with get_task_status"`
}

func downloadURLHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, DownloadURLInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DownloadURLInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, DownloadURLOutput{}, err
		}
		if err := confirmWrite(ctx, reg, req, fmt.Sprintf("download_url_to_storage: download %s to %s/%s as %s", input.URL, host, input.Storage, input.Filename)); err != nil {
			return nil, DownloadURLOutput{}, err
		}

		form := url.Values{}
		form.Set("url", input.URL)
		form.Set("content", input.Content)
		form.Set("filename", input.Filename)
		addStr(form, "checksum", input.Checksum)
		addStr(form, "checksum-algorithm", input.Algo)

		var upid string
		path := fmt.Sprintf("/nodes/%s/storage/%s/download-url", input.Node, input.Storage)
		if err := client.PostForm(ctx, path, form, &upid); err != nil {
			return nil, DownloadURLOutput{}, fmt.Errorf("failed to download %s: %w", input.URL, err)
		}
		return nil, DownloadURLOutput{Host: host, Node: input.Node, Storage: input.Storage, Filename: input.Filename, UPID: upid}, nil
	}
}

// -- list_storage_content --

type ListStorageContentInput struct {
	Host    string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node    string `json:"node" jsonschema:"node name,required"`
	Storage string `json:"storage" jsonschema:"storage pool name,required"`
	Content string `json:"content,omitempty" jsonschema:"filter by content type: iso, images, import, vztmpl, backup"`
}

type StorageContentItem struct {
	VolID   string `json:"volid" jsonschema:"volume ID to reference this item (e.g. local:iso/x.iso)"`
	Format  string `json:"format" jsonschema:"format, e.g. iso, qcow2, raw"`
	Size    int64  `json:"size" jsonschema:"size in bytes"`
	Content string `json:"content,omitempty" jsonschema:"content type"`
	VMID    int    `json:"vmid,omitempty" jsonschema:"owning VM ID if applicable"`
}

type ListStorageContentOutput struct {
	Host    string               `json:"host" jsonschema:"Proxmox host that was queried"`
	Storage string               `json:"storage" jsonschema:"storage pool name"`
	Items   []StorageContentItem `json:"items" jsonschema:"content items on the storage"`
}

func listStorageContentHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, ListStorageContentInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListStorageContentInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ListStorageContentOutput{}, err
		}
		path := fmt.Sprintf("/nodes/%s/storage/%s/content", input.Node, input.Storage)
		if input.Content != "" {
			path += "?content=" + url.QueryEscape(input.Content)
		}
		var items []StorageContentItem
		if err := client.Get(ctx, path, &items); err != nil {
			return nil, ListStorageContentOutput{}, fmt.Errorf("failed to list storage content: %w", err)
		}
		return nil, ListStorageContentOutput{Host: host, Storage: input.Storage, Items: items}, nil
	}
}

// -- get_guest_agent_info --

type GuestAgentInput struct {
	Host string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node string `json:"node,omitempty" jsonschema:"node name (optional — auto-resolved from VMID if omitted)"`
	VMID int    `json:"vmid" jsonschema:"VM ID,required"`
}

type GuestAgentOutput struct {
	Host       string `json:"host" jsonschema:"Proxmox host that was queried"`
	Node       string `json:"node" jsonschema:"node hosting the VM"`
	VMID       int    `json:"vmid" jsonschema:"VM ID"`
	Online     bool   `json:"online" jsonschema:"true if the QEMU guest agent responded (a good signal the OS has booted)"`
	Detail     string `json:"detail,omitempty" jsonschema:"error detail when the agent did not respond"`
	Interfaces string `json:"interfaces,omitempty" jsonschema:"raw network-get-interfaces JSON when online (contains IP addresses)"`
}

func guestAgentHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, GuestAgentInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GuestAgentInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, GuestAgentOutput{}, err
		}
		node, err := resolveVMNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, GuestAgentOutput{}, err
		}
		out := GuestAgentOutput{Host: host, Node: node, VMID: input.VMID}
		raw, err := client.GetRaw(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", node, input.VMID))
		if err != nil {
			out.Online = false
			out.Detail = err.Error()
			return nil, out, nil
		}
		out.Online = true
		out.Interfaces = raw
		return nil, out, nil
	}
}

func RegisterProvisioningTools(server *mcp.Server, reg *HostRegistry) {
	mcp.AddTool[CreateVMInput, any](server, &mcp.Tool{
		Name:        "create_vm",
		Description: "Create a new QEMU/KVM virtual machine. Returns a task UPID. Use 'config' for any options not in the typed fields (disks, cloud-init, boot order). Disks must be attached separately or via 'config' (e.g. scsi0=local-lvm:32 for a 32GiB disk).",
	}, createVMHandler(reg))

	mcp.AddTool[UpdateVMConfigInput, any](server, &mcp.Tool{
		Name:        "update_vm_config",
		Description: "Set or change VM configuration keys. The workhorse for OS install: attach install/answer CDs, import disk images, configure cloud-init, set boot order. Node is auto-resolved if omitted.",
	}, updateVMConfigHandler(reg))

	mcp.AddTool[CloneVMInput, any](server, &mcp.Tool{
		Name:        "clone_vm",
		Description: "Clone an existing VM or template into a new VM. Fastest path to a working OS when a cloud-init template exists. Returns a task UPID. Node is auto-resolved if omitted.",
	}, cloneVMHandler(reg))

	mcp.AddTool[ResizeDiskInput, any](server, &mcp.Tool{
		Name:        "resize_vm_disk",
		Description: "Grow a VM disk. Use '+10G' to add space or '32G' for an absolute size. Cloud images often need growing after import. Node is auto-resolved if omitted.",
	}, resizeDiskHandler(reg))

	mcp.AddTool[DownloadURLInput, any](server, &mcp.Tool{
		Name:        "download_url_to_storage",
		Description: "Download a file (install ISO, cloud image, virtio-win.iso) from a URL directly onto a Proxmox storage. Use content='iso' for ISOs, content='import' for disk images. Returns a task UPID.",
	}, downloadURLHandler(reg))

	mcp.AddTool[ListStorageContentInput, any](server, &mcp.Tool{
		Name:        "list_storage_content",
		Description: "List the contents of a storage pool (ISOs, disk images, templates, backups). Use this to find volume IDs to attach or import.",
	}, listStorageContentHandler(reg))

	mcp.AddTool[GuestAgentInput, any](server, &mcp.Tool{
		Name:        "get_guest_agent_info",
		Description: "Query the QEMU guest agent for network interfaces. Returns online=false if the agent isn't responding yet — poll this to detect when an OS install has finished and the guest has booted. Requires qemu-guest-agent installed in the guest.",
	}, guestAgentHandler(reg))
}
