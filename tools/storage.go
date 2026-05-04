package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// -- list_storage --

type ListStorageInput struct {
	Host string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node string `json:"node" jsonschema:"name of the Proxmox node,required"`
}

type ListStorageOutput struct {
	Host    string           `json:"host" jsonschema:"Proxmox host that was queried"`
	Storage []StorageSummary `json:"storage" jsonschema:"list of storage pools on the node"`
}

type StorageSummary struct {
	Storage string  `json:"storage" jsonschema:"storage pool name"`
	Type    string  `json:"type" jsonschema:"storage type (dir, lvm, zfs, nfs, ceph, etc.)"`
	Content string  `json:"content" jsonschema:"allowed content types (images, rootdir, iso, backup, etc.)"`
	Status  string  `json:"status,omitempty" jsonschema:"storage status"`
	Active  int     `json:"active" jsonschema:"1 if active, 0 if inactive"`
	Shared  int     `json:"shared" jsonschema:"1 if shared across nodes, 0 if local"`
	Total   int64   `json:"total" jsonschema:"total space in bytes"`
	Used    int64   `json:"used" jsonschema:"used space in bytes"`
	Avail   int64   `json:"avail" jsonschema:"available space in bytes"`
	UsedPct float64 `json:"used_fraction" jsonschema:"used space as fraction (0.0 to 1.0)"`
}

func listStorageHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, ListStorageInput) (*mcp.CallToolResult, ListStorageOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListStorageInput) (*mcp.CallToolResult, ListStorageOutput, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ListStorageOutput{}, err
		}
		var storage []StorageSummary
		if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/storage", input.Node), &storage); err != nil {
			return nil, ListStorageOutput{}, fmt.Errorf("failed to list storage: %w", err)
		}
		return nil, ListStorageOutput{Host: host, Storage: storage}, nil
	}
}

// -- get_task_status --

type GetTaskStatusInput struct {
	Host string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node string `json:"node" jsonschema:"node where the task is running,required"`
	UPID string `json:"upid" jsonschema:"Proxmox task UPID returned by action tools,required"`
}

type TaskStatusOutput struct {
	Host       string `json:"host" jsonschema:"Proxmox host that was queried"`
	UPID       string `json:"upid" jsonschema:"task UPID"`
	Node       string `json:"node" jsonschema:"node where the task ran"`
	Status     string `json:"status" jsonschema:"task status: running, stopped, OK, or error message"`
	Type       string `json:"type" jsonschema:"task type"`
	ExitStatus string `json:"exitstatus,omitempty" jsonschema:"exit status when completed (OK or error description)"`
	PID        int    `json:"pid" jsonschema:"process ID of the task"`
	StartTime  int64  `json:"starttime" jsonschema:"task start time as Unix timestamp"`
	EndTime    int64  `json:"endtime,omitempty" jsonschema:"task end time as Unix timestamp (0 if still running)"`
}

func getTaskStatusHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, GetTaskStatusInput) (*mcp.CallToolResult, TaskStatusOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetTaskStatusInput) (*mcp.CallToolResult, TaskStatusOutput, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, TaskStatusOutput{}, err
		}
		var status TaskStatusOutput
		if err := client.Get(ctx, fmt.Sprintf("/nodes/%s/tasks/%s/status", input.Node, input.UPID), &status); err != nil {
			return nil, TaskStatusOutput{}, fmt.Errorf("failed to get task status: %w", err)
		}
		status.Host = host
		status.Node = input.Node
		return nil, status, nil
	}
}

func RegisterStorageTools(server *mcp.Server, reg *HostRegistry) {
	mcp.AddTool[ListStorageInput, ListStorageOutput](server, &mcp.Tool{
		Name:        "list_storage",
		Description: "List storage pools on a specific Proxmox node with capacity info",
	}, listStorageHandler(reg))

	mcp.AddTool[GetTaskStatusInput, TaskStatusOutput](server, &mcp.Tool{
		Name:        "get_task_status",
		Description: "Get status of a Proxmox background task by its UPID. Use this to check if start/stop/shutdown operations have completed.",
	}, getTaskStatusHandler(reg))
}
