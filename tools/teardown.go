package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// -- delete_vm --

type DeleteVMInput struct {
	Host                string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node                string `json:"node,omitempty" jsonschema:"node name (optional — auto-resolved from VMID if omitted)"`
	VMID                int    `json:"vmid" jsonschema:"VM ID to delete,required"`
	Purge               bool   `json:"purge,omitempty" jsonschema:"also remove the VM from backup jobs and HA resource configuration"`
	DestroyUnreferenced bool   `json:"destroy_unreferenced_disks,omitempty" jsonschema:"also destroy disks on the VM's storages that are not referenced in the config"`
}

type DeleteVMOutput struct {
	Host string `json:"host" jsonschema:"Proxmox host that was queried"`
	Node string `json:"node" jsonschema:"node the VM was on"`
	VMID int    `json:"vmid" jsonschema:"VM ID"`
	UPID string `json:"upid" jsonschema:"task UPID — track with get_task_status"`
}

func deleteVMHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, DeleteVMInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DeleteVMInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, DeleteVMOutput{}, err
		}
		node, err := resolveVMNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, DeleteVMOutput{}, err
		}
		if err := confirmWrite(ctx, reg, req, fmt.Sprintf("delete_vm: DESTROY VM %d on %s/%s", input.VMID, host, node)); err != nil {
			return nil, DeleteVMOutput{}, err
		}

		q := url.Values{}
		if input.Purge {
			q.Set("purge", "1")
		}
		if input.DestroyUnreferenced {
			q.Set("destroy-unreferenced-disks", "1")
		}
		path := fmt.Sprintf("/nodes/%s/qemu/%d", node, input.VMID)
		if len(q) > 0 {
			path += "?" + q.Encode()
		}

		var upid string
		if err := client.Delete(ctx, path, &upid); err != nil {
			return nil, DeleteVMOutput{}, fmt.Errorf("failed to delete VM %d: %w", input.VMID, err)
		}
		return nil, DeleteVMOutput{Host: host, Node: node, VMID: input.VMID, UPID: upid}, nil
	}
}

// -- delete_container --

type DeleteContainerInput struct {
	Host                string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node                string `json:"node,omitempty" jsonschema:"node name (optional — auto-resolved from VMID if omitted)"`
	VMID                int    `json:"vmid" jsonschema:"container ID to delete,required"`
	Purge               bool   `json:"purge,omitempty" jsonschema:"also remove from backup jobs and HA configuration"`
	DestroyUnreferenced bool   `json:"destroy_unreferenced_disks,omitempty" jsonschema:"also destroy unreferenced disks on the container's storages"`
}

func deleteContainerHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, DeleteContainerInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DeleteContainerInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, DeleteVMOutput{}, err
		}
		node, err := resolveContainerNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, DeleteVMOutput{}, err
		}
		if err := confirmWrite(ctx, reg, req, fmt.Sprintf("delete_container: DESTROY container %d on %s/%s", input.VMID, host, node)); err != nil {
			return nil, DeleteVMOutput{}, err
		}

		q := url.Values{}
		if input.Purge {
			q.Set("purge", "1")
		}
		if input.DestroyUnreferenced {
			q.Set("destroy-unreferenced-disks", "1")
		}
		path := fmt.Sprintf("/nodes/%s/lxc/%d", node, input.VMID)
		if len(q) > 0 {
			path += "?" + q.Encode()
		}

		var upid string
		if err := client.Delete(ctx, path, &upid); err != nil {
			return nil, DeleteVMOutput{}, fmt.Errorf("failed to delete container %d: %w", input.VMID, err)
		}
		return nil, DeleteVMOutput{Host: host, Node: node, VMID: input.VMID, UPID: upid}, nil
	}
}

// -- delete_storage_content --

type DeleteStorageContentInput struct {
	Host  string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node  string `json:"node" jsonschema:"node that can access the storage,required"`
	VolID string `json:"volid" jsonschema:"full volume ID to delete, e.g. SharedNFS:iso/x.iso or local-lvm:vm-9000-disk-0,required"`
}

type DeleteStorageContentOutput struct {
	Host  string `json:"host" jsonschema:"Proxmox host that was queried"`
	Node  string `json:"node" jsonschema:"node used"`
	VolID string `json:"volid" jsonschema:"volume that was deleted"`
}

func deleteStorageContentHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, DeleteStorageContentInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input DeleteStorageContentInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, DeleteStorageContentOutput{}, err
		}
		storage, _, ok := strings.Cut(input.VolID, ":")
		if !ok || storage == "" {
			return nil, DeleteStorageContentOutput{}, fmt.Errorf("volid %q must be in the form storage:path (e.g. SharedNFS:iso/x.iso)", input.VolID)
		}
		if err := confirmWrite(ctx, reg, req, fmt.Sprintf("delete_storage_content: DELETE volume %s on %s/%s", input.VolID, host, input.Node)); err != nil {
			return nil, DeleteStorageContentOutput{}, err
		}

		path := fmt.Sprintf("/nodes/%s/storage/%s/content/%s", input.Node, storage, url.PathEscape(input.VolID))
		if err := client.Delete(ctx, path, nil); err != nil {
			return nil, DeleteStorageContentOutput{}, fmt.Errorf("failed to delete volume %s: %w", input.VolID, err)
		}
		return nil, DeleteStorageContentOutput{Host: host, Node: input.Node, VolID: input.VolID}, nil
	}
}

func RegisterTeardownTools(server *mcp.Server, reg *HostRegistry) {
	mcp.AddTool[DeleteVMInput, any](server, &mcp.Tool{
		Name:        "delete_vm",
		Description: "Permanently delete (destroy) a VM and its disks. The VM must be stopped first. Returns a task UPID. Node is auto-resolved if omitted. This is irreversible.",
	}, deleteVMHandler(reg))

	mcp.AddTool[DeleteContainerInput, any](server, &mcp.Tool{
		Name:        "delete_container",
		Description: "Permanently delete (destroy) an LXC container and its disks. The container must be stopped first. Returns a task UPID. Node is auto-resolved if omitted. This is irreversible.",
	}, deleteContainerHandler(reg))

	mcp.AddTool[DeleteStorageContentInput, any](server, &mcp.Tool{
		Name:        "delete_storage_content",
		Description: "Delete a single volume from a storage (ISO, uploaded image, disk image, backup) by its volume ID. This is irreversible.",
	}, deleteStorageContentHandler(reg))
}
