package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trickyearlobe-ai/proxmox-mcp/proxmox"
)

// -- list_containers --

type ListContainersInput struct {
	Host string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node string `json:"node" jsonschema:"name of the Proxmox node,required"`
}

type ListContainersOutput struct {
	Host       string             `json:"host" jsonschema:"Proxmox host that was queried"`
	Containers []ContainerSummary `json:"containers" jsonschema:"list of LXC containers on the node"`
}

type ContainerSummary struct {
	VMID   int     `json:"vmid" jsonschema:"container ID"`
	Name   string  `json:"name" jsonschema:"container name"`
	Status string  `json:"status" jsonschema:"container status: running, stopped, etc."`
	CPU    float64 `json:"cpu" jsonschema:"CPU utilization (0.0 to 1.0)"`
	MaxCPU int     `json:"cpus" jsonschema:"number of CPUs allocated"`
	Mem    int64   `json:"mem" jsonschema:"current memory usage in bytes"`
	MaxMem int64   `json:"maxmem" jsonschema:"maximum memory in bytes"`
	Disk   int64   `json:"disk" jsonschema:"current disk usage in bytes"`
	Uptime int64   `json:"uptime" jsonschema:"uptime in seconds"`
}

func listContainersHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, ListContainersInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListContainersInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ListContainersOutput{}, err
		}
		var containers []ContainerSummary
		if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/lxc", input.Node), &containers); err != nil {
			return nil, ListContainersOutput{}, fmt.Errorf("failed to list containers: %w", err)
		}
		return nil, ListContainersOutput{Host: host, Containers: containers}, nil
	}
}

// -- get_container_status --

type ContainerIdentifier struct {
	Host string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node string `json:"node,omitempty" jsonschema:"node name (optional — auto-resolved from VMID if omitted)"`
	VMID int    `json:"vmid" jsonschema:"container ID,required"`
}

type ContainerStatusOutput struct {
	Host   string  `json:"host" jsonschema:"Proxmox host that was queried"`
	VMID   int     `json:"vmid" jsonschema:"container ID"`
	Name   string  `json:"name" jsonschema:"container name"`
	Status string  `json:"status" jsonschema:"container status: running, stopped"`
	Node   string  `json:"node" jsonschema:"node hosting the container"`
	CPU    float64 `json:"cpu" jsonschema:"CPU utilization"`
	MaxCPU int     `json:"cpus" jsonschema:"number of CPUs"`
	Mem    int64   `json:"mem" jsonschema:"current memory in bytes"`
	MaxMem int64   `json:"maxmem" jsonschema:"max memory in bytes"`
	Uptime int64   `json:"uptime" jsonschema:"uptime in seconds"`
}

func resolveContainerNode(ctx context.Context, client *proxmox.Client, node string, vmid int) (string, error) {
	if node != "" {
		return node, nil
	}
	return client.ResolveNode(ctx, vmid)
}

func getContainerStatusHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, ContainerIdentifier) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ContainerIdentifier) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ContainerStatusOutput{}, err
		}
		node, err := resolveContainerNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, ContainerStatusOutput{}, err
		}
		var status ContainerStatusOutput
		if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/status/current", node, input.VMID), &status); err != nil {
			return nil, ContainerStatusOutput{}, fmt.Errorf("failed to get container status: %w", err)
		}
		status.Host = host
		status.Node = node
		return nil, status, nil
	}
}

// -- get_container_config --

type ContainerConfigOutput struct {
	Host   string `json:"host" jsonschema:"Proxmox host that was queried"`
	VMID   int    `json:"vmid" jsonschema:"container ID"`
	Node   string `json:"node" jsonschema:"node hosting the container"`
	Config string `json:"config" jsonschema:"container configuration as JSON (contains dynamic keys like net0, mp0, etc.)"`
}

func getContainerConfigHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, ContainerIdentifier) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ContainerIdentifier) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ContainerConfigOutput{}, err
		}
		node, err := resolveContainerNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, ContainerConfigOutput{}, err
		}
		raw, err := client.GetRaw(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/config", node, input.VMID))
		if err != nil {
			return nil, ContainerConfigOutput{}, fmt.Errorf("failed to get container config: %w", err)
		}
		return nil, ContainerConfigOutput{
			Host:   host,
			VMID:   input.VMID,
			Node:   node,
			Config: raw,
		}, nil
	}
}

// -- Container lifecycle tools (start, stop, shutdown) --

type ContainerActionOutput struct {
	Host   string `json:"host" jsonschema:"Proxmox host that was queried"`
	VMID   int    `json:"vmid" jsonschema:"container ID"`
	Node   string `json:"node" jsonschema:"node hosting the container"`
	Action string `json:"action" jsonschema:"action performed"`
	UPID   string `json:"upid" jsonschema:"Proxmox task ID — use get_task_status to track completion"`
}

func containerActionHandler(reg *HostRegistry, action string) func(context.Context, *mcp.CallToolRequest, ContainerIdentifier) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ContainerIdentifier) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ContainerActionOutput{}, err
		}
		node, err := resolveContainerNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, ContainerActionOutput{}, err
		}
		if err := confirmWrite(ctx, reg, req, fmt.Sprintf("%s_container: %s container %d on %s/%s", action, action, input.VMID, host, node)); err != nil {
			return nil, ContainerActionOutput{}, err
		}
		var upid string
		if err := client.Post(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/status/%s", node, input.VMID, action), &upid); err != nil {
			return nil, ContainerActionOutput{}, fmt.Errorf("failed to %s container %d: %w", action, input.VMID, err)
		}
		return nil, ContainerActionOutput{
			Host:   host,
			VMID:   input.VMID,
			Node:   node,
			Action: action,
			UPID:   upid,
		}, nil
	}
}

func RegisterContainerTools(server *mcp.Server, reg *HostRegistry) {
	mcp.AddTool[ListContainersInput, any](server, &mcp.Tool{
		Name:        "list_containers",
		Description: "List all LXC containers on a specific Proxmox node",
	}, listContainersHandler(reg))

	mcp.AddTool[ContainerIdentifier, any](server, &mcp.Tool{
		Name:        "get_container_status",
		Description: "Get current status of an LXC container. Node is auto-resolved if omitted.",
	}, getContainerStatusHandler(reg))

	mcp.AddTool[ContainerIdentifier, any](server, &mcp.Tool{
		Name:        "get_container_config",
		Description: "Get full configuration of an LXC container as JSON. Node is auto-resolved if omitted.",
	}, getContainerConfigHandler(reg))

	mcp.AddTool[ContainerIdentifier, any](server, &mcp.Tool{
		Name:        "start_container",
		Description: "Start an LXC container. Returns a task UPID for tracking. Node is auto-resolved if omitted.",
	}, containerActionHandler(reg, "start"))

	mcp.AddTool[ContainerIdentifier, any](server, &mcp.Tool{
		Name:        "stop_container",
		Description: "Hard stop an LXC container. Returns a task UPID. Node is auto-resolved if omitted.",
	}, containerActionHandler(reg, "stop"))

	mcp.AddTool[ContainerIdentifier, any](server, &mcp.Tool{
		Name:        "shutdown_container",
		Description: "Gracefully shutdown an LXC container. Returns a task UPID. Node is auto-resolved if omitted.",
	}, containerActionHandler(reg, "shutdown"))
}
