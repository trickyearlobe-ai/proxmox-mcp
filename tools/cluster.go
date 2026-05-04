package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HostInput is used for tools that only need a host selector.
type HostInput struct {
	Host string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
}

// -- get_cluster_status --

type ClusterStatusOutput struct {
	Host   string               `json:"host" jsonschema:"Proxmox host that was queried"`
	Status []ClusterStatusEntry `json:"status" jsonschema:"list of cluster members and their status"`
}

type ClusterStatusEntry struct {
	Name    string `json:"name" jsonschema:"node or cluster name"`
	Type    string `json:"type" jsonschema:"entry type: cluster or node"`
	NodeID  int    `json:"nodeid" jsonschema:"node ID in the cluster"`
	Online  int    `json:"online" jsonschema:"1 if online, 0 if offline"`
	IP      string `json:"ip,omitempty" jsonschema:"IP address of the node"`
	Level   string `json:"level,omitempty" jsonschema:"support level"`
	Local   int    `json:"local,omitempty" jsonschema:"1 if this is the local node"`
	Version int    `json:"version,omitempty" jsonschema:"cluster config version"`
}

func getClusterStatusHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, HostInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input HostInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ClusterStatusOutput{}, err
		}
		var entries []ClusterStatusEntry
		if err := client.Get(ctx, "/cluster/status", &entries); err != nil {
			return nil, ClusterStatusOutput{}, fmt.Errorf("failed to get cluster status: %w", err)
		}
		return nil, ClusterStatusOutput{Host: host, Status: entries}, nil
	}
}

// -- list_cluster_resources --

type ListClusterResourcesInput struct {
	Host string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Type string `json:"type,omitempty" jsonschema:"filter by resource type: vm, storage, node, sdn, or empty for all"`
}

type ClusterResourcesOutput struct {
	Host      string            `json:"host" jsonschema:"Proxmox host that was queried"`
	Resources []ClusterResource `json:"resources" jsonschema:"list of cluster resources"`
}

type ClusterResource struct {
	ID       string  `json:"id" jsonschema:"resource ID (e.g. qemu/100, lxc/101)"`
	Type     string  `json:"type" jsonschema:"resource type: qemu, lxc, storage, node, sdn"`
	Node     string  `json:"node,omitempty" jsonschema:"node hosting the resource"`
	Name     string  `json:"name,omitempty" jsonschema:"resource name"`
	Status   string  `json:"status" jsonschema:"resource status"`
	VMID     int     `json:"vmid,omitempty" jsonschema:"VM or container ID"`
	MaxMem   int64   `json:"maxmem,omitempty" jsonschema:"maximum memory in bytes"`
	Mem      int64   `json:"mem,omitempty" jsonschema:"current memory usage in bytes"`
	MaxDisk  int64   `json:"maxdisk,omitempty" jsonschema:"maximum disk size in bytes"`
	Disk     int64   `json:"disk,omitempty" jsonschema:"current disk usage in bytes"`
	MaxCPU   int     `json:"maxcpu,omitempty" jsonschema:"number of CPUs"`
	CPU      float64 `json:"cpu,omitempty" jsonschema:"CPU utilization (0.0 to 1.0)"`
	Uptime   int64   `json:"uptime,omitempty" jsonschema:"uptime in seconds"`
	Template int     `json:"template,omitempty" jsonschema:"1 if template, 0 otherwise"`
}

func listClusterResourcesHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, ListClusterResourcesInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListClusterResourcesInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ClusterResourcesOutput{}, err
		}
		path := "/cluster/resources"
		if input.Type != "" {
			path += "?type=" + input.Type
		}
		var resources []ClusterResource
		if err := client.Get(ctx, path, &resources); err != nil {
			return nil, ClusterResourcesOutput{}, fmt.Errorf("failed to list cluster resources: %w", err)
		}
		return nil, ClusterResourcesOutput{Host: host, Resources: resources}, nil
	}
}

func RegisterClusterTools(server *mcp.Server, reg *HostRegistry) {
	mcp.AddTool[HostInput, any](server, &mcp.Tool{
		Name:        "get_cluster_status",
		Description: "Get Proxmox cluster status including node membership and health",
	}, getClusterStatusHandler(reg))

	mcp.AddTool[ListClusterResourcesInput, any](server, &mcp.Tool{
		Name:        "list_cluster_resources",
		Description: "List all resources (VMs, containers, storage, nodes) across the Proxmox cluster. Optionally filter by type.",
	}, listClusterResourcesHandler(reg))
}
