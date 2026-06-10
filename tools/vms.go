package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trickyearlobe-ai/proxmox-mcp/proxmox"
)

// -- list_vms --

type ListVMsInput struct {
	Host string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node string `json:"node" jsonschema:"name of the Proxmox node,required"`
}

type ListVMsOutput struct {
	Host string      `json:"host" jsonschema:"Proxmox host that was queried"`
	VMs  []VMSummary `json:"vms" jsonschema:"list of VMs on the node"`
}

type VMSummary struct {
	VMID   int     `json:"vmid" jsonschema:"VM ID"`
	Name   string  `json:"name" jsonschema:"VM name"`
	Status string  `json:"status" jsonschema:"VM status: running, stopped, etc."`
	CPU    float64 `json:"cpu" jsonschema:"CPU utilization (0.0 to 1.0)"`
	MaxCPU int     `json:"cpus" jsonschema:"number of CPUs allocated"`
	Mem    int64   `json:"mem" jsonschema:"current memory usage in bytes"`
	MaxMem int64   `json:"maxmem" jsonschema:"maximum memory in bytes"`
	Disk   int64   `json:"disk" jsonschema:"current disk usage in bytes"`
	Uptime int64   `json:"uptime" jsonschema:"uptime in seconds"`
}

func listVMsHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, ListVMsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListVMsInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ListVMsOutput{}, err
		}
		var vms []VMSummary
		if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu", input.Node), &vms); err != nil {
			return nil, ListVMsOutput{}, fmt.Errorf("failed to list VMs: %w", err)
		}
		return nil, ListVMsOutput{Host: host, VMs: vms}, nil
	}
}

// -- get_vm_status --

type VMIdentifier struct {
	Host string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node string `json:"node,omitempty" jsonschema:"node name (optional — auto-resolved from VMID if omitted)"`
	VMID int    `json:"vmid" jsonschema:"VM ID,required"`
}

type VMStatusOutput struct {
	Host      string  `json:"host" jsonschema:"Proxmox host that was queried"`
	VMID      int     `json:"vmid" jsonschema:"VM ID"`
	Name      string  `json:"name" jsonschema:"VM name"`
	Status    string  `json:"status" jsonschema:"VM status: running, stopped, paused"`
	Node      string  `json:"node" jsonschema:"node hosting the VM"`
	CPU       float64 `json:"cpu" jsonschema:"CPU utilization"`
	MaxCPU    int     `json:"cpus" jsonschema:"number of CPUs"`
	Mem       int64   `json:"mem" jsonschema:"current memory in bytes"`
	MaxMem    int64   `json:"maxmem" jsonschema:"max memory in bytes"`
	Uptime    int64   `json:"uptime" jsonschema:"uptime in seconds"`
	PID       int     `json:"pid,omitempty" jsonschema:"QEMU process ID if running"`
	QMPStatus string  `json:"qmpstatus,omitempty" jsonschema:"QMP status"`
}

func resolveVMNode(ctx context.Context, client *proxmox.Client, node string, vmid int) (string, error) {
	if node != "" {
		return node, nil
	}
	return client.ResolveNode(ctx, vmid)
}

func getVMStatusHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, VMIdentifier) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input VMIdentifier) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, VMStatusOutput{}, err
		}
		node, err := resolveVMNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, VMStatusOutput{}, err
		}
		var status VMStatusOutput
		if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/current", node, input.VMID), &status); err != nil {
			return nil, VMStatusOutput{}, fmt.Errorf("failed to get VM status: %w", err)
		}
		status.Host = host
		status.Node = node
		return nil, status, nil
	}
}

// -- get_vm_config --

type VMConfigOutput struct {
	Host   string `json:"host" jsonschema:"Proxmox host that was queried"`
	VMID   int    `json:"vmid" jsonschema:"VM ID"`
	Node   string `json:"node" jsonschema:"node hosting the VM"`
	Config string `json:"config" jsonschema:"VM configuration as JSON (contains dynamic keys like net0, scsi0, etc.)"`
}

func getVMConfigHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, VMIdentifier) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input VMIdentifier) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, VMConfigOutput{}, err
		}
		node, err := resolveVMNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, VMConfigOutput{}, err
		}
		raw, err := client.GetRaw(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", node, input.VMID))
		if err != nil {
			return nil, VMConfigOutput{}, fmt.Errorf("failed to get VM config: %w", err)
		}
		return nil, VMConfigOutput{
			Host:   host,
			VMID:   input.VMID,
			Node:   node,
			Config: raw,
		}, nil
	}
}

// -- VM lifecycle tools (start, stop, shutdown) --

type VMActionOutput struct {
	Host   string `json:"host" jsonschema:"Proxmox host that was queried"`
	VMID   int    `json:"vmid" jsonschema:"VM ID"`
	Node   string `json:"node" jsonschema:"node hosting the VM"`
	Action string `json:"action" jsonschema:"action performed"`
	UPID   string `json:"upid" jsonschema:"Proxmox task ID — use get_task_status to track completion"`
}

func vmActionHandler(reg *HostRegistry, action string) func(context.Context, *mcp.CallToolRequest, VMIdentifier) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input VMIdentifier) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, VMActionOutput{}, err
		}
		node, err := resolveVMNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, VMActionOutput{}, err
		}
		if err := confirmWrite(ctx, reg, req, fmt.Sprintf("%s_vm: %s VM %d on %s/%s", action, action, input.VMID, host, node)); err != nil {
			return nil, VMActionOutput{}, err
		}
		var upid string
		if err := client.Post(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/%s", node, input.VMID, action), &upid); err != nil {
			return nil, VMActionOutput{}, fmt.Errorf("failed to %s VM %d: %w", action, input.VMID, err)
		}
		return nil, VMActionOutput{
			Host:   host,
			VMID:   input.VMID,
			Node:   node,
			Action: action,
			UPID:   upid,
		}, nil
	}
}

func RegisterVMTools(server *mcp.Server, reg *HostRegistry) {
	mcp.AddTool[ListVMsInput, any](server, &mcp.Tool{
		Name:        "list_vms",
		Description: "List all QEMU/KVM virtual machines on a specific Proxmox node",
	}, listVMsHandler(reg))

	mcp.AddTool[VMIdentifier, any](server, &mcp.Tool{
		Name:        "get_vm_status",
		Description: "Get current status of a VM. Node is auto-resolved if omitted.",
	}, getVMStatusHandler(reg))

	mcp.AddTool[VMIdentifier, any](server, &mcp.Tool{
		Name:        "get_vm_config",
		Description: "Get full configuration of a VM as JSON. Node is auto-resolved if omitted.",
	}, getVMConfigHandler(reg))

	mcp.AddTool[VMIdentifier, any](server, &mcp.Tool{
		Name:        "start_vm",
		Description: "Start a VM. Returns a task UPID for tracking. Node is auto-resolved if omitted.",
	}, vmActionHandler(reg, "start"))

	mcp.AddTool[VMIdentifier, any](server, &mcp.Tool{
		Name:        "stop_vm",
		Description: "Hard stop a VM (like pulling the power cord). Returns a task UPID. Node is auto-resolved if omitted.",
	}, vmActionHandler(reg, "stop"))

	mcp.AddTool[VMIdentifier, any](server, &mcp.Tool{
		Name:        "shutdown_vm",
		Description: "Gracefully shutdown a VM via ACPI. Returns a task UPID. Node is auto-resolved if omitted.",
	}, vmActionHandler(reg, "shutdown"))
}
