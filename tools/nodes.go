package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// -- list_nodes --

type ListNodesOutput struct {
	Host  string        `json:"host" jsonschema:"Proxmox host that was queried"`
	Nodes []NodeSummary `json:"nodes" jsonschema:"list of cluster nodes"`
}

type NodeSummary struct {
	Node    string  `json:"node" jsonschema:"node name"`
	Status  string  `json:"status" jsonschema:"node status: online or offline"`
	CPU     float64 `json:"cpu" jsonschema:"CPU utilization (0.0 to 1.0)"`
	MaxCPU  int     `json:"maxcpu" jsonschema:"number of CPU cores"`
	Mem     int64   `json:"mem" jsonschema:"used memory in bytes"`
	MaxMem  int64   `json:"maxmem" jsonschema:"total memory in bytes"`
	Disk    int64   `json:"disk" jsonschema:"used local disk in bytes"`
	MaxDisk int64   `json:"maxdisk" jsonschema:"total local disk in bytes"`
	Uptime  int64   `json:"uptime" jsonschema:"uptime in seconds"`
}

func listNodesHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, HostInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input HostInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ListNodesOutput{}, err
		}
		var nodes []NodeSummary
		if err := client.Get(ctx, "/nodes", &nodes); err != nil {
			return nil, ListNodesOutput{}, fmt.Errorf("failed to list nodes: %w", err)
		}
		return nil, ListNodesOutput{Host: host, Nodes: nodes}, nil
	}
}

// -- get_node_status --

type GetNodeStatusInput struct {
	Host string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node string `json:"node" jsonschema:"name of the Proxmox node,required"`
}

type NodeStatusOutput struct {
	Host       string `json:"host" jsonschema:"Proxmox host that was queried"`
	Node       string `json:"node" jsonschema:"node name"`
	Status     string `json:"status" jsonschema:"raw JSON status (contains deeply nested dynamic fields)"`
}

func getNodeStatusHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, GetNodeStatusInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetNodeStatusInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, NodeStatusOutput{}, err
		}
		raw, err := client.GetRaw(ctx, fmt.Sprintf("/nodes/%s/status", input.Node))
		if err != nil {
			return nil, NodeStatusOutput{}, fmt.Errorf("failed to get node status: %w", err)
		}
		return nil, NodeStatusOutput{
			Host:   host,
			Node:   input.Node,
			Status: raw,
		}, nil
	}
}

func RegisterNodeTools(server *mcp.Server, reg *HostRegistry) {
	mcp.AddTool[HostInput, any](server, &mcp.Tool{
		Name:        "list_nodes",
		Description: "List all nodes in the Proxmox cluster with status, CPU, memory, and disk usage",
	}, listNodesHandler(reg))

	mcp.AddTool[GetNodeStatusInput, any](server, &mcp.Tool{
		Name:        "get_node_status",
		Description: "Get detailed status of a specific Proxmox node including CPU, memory, load, and version info. Returns raw JSON due to deeply nested dynamic fields.",
	}, getNodeStatusHandler(reg))
}
