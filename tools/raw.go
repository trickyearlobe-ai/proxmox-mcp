package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type RawAPIInput struct {
	Host   string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Method string `json:"method" jsonschema:"HTTP method: GET, POST, PUT, or DELETE,required"`
	Path   string `json:"path" jsonschema:"API path starting with / (e.g. /nodes or /nodes/pve1/qemu),required"`
	Body   string `json:"body,omitempty" jsonschema:"form-encoded body for POST/PUT requests (e.g. key1=value1&key2=value2)"`
}

type RawAPIOutput struct {
	Host       string `json:"host" jsonschema:"Proxmox host that was queried"`
	StatusCode int    `json:"status_code" jsonschema:"HTTP status code"`
	Body       string `json:"body" jsonschema:"raw JSON response body"`
}

func rawAPIHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, RawAPIInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input RawAPIInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, RawAPIOutput{}, err
		}

		method := strings.ToUpper(input.Method)
		if method != "GET" && method != "POST" && method != "PUT" && method != "DELETE" {
			return nil, RawAPIOutput{}, fmt.Errorf("method must be GET, POST, PUT, or DELETE, got %q", input.Method)
		}

		if !strings.HasPrefix(input.Path, "/") {
			return nil, RawAPIOutput{}, fmt.Errorf("path must start with /, got %q", input.Path)
		}

		statusCode, body, err := client.DoRaw(ctx, method, input.Path, input.Body)
		if err != nil {
			return nil, RawAPIOutput{}, fmt.Errorf("raw API request failed: %w", err)
		}

		return nil, RawAPIOutput{
			Host:       host,
			StatusCode: statusCode,
			Body:       body,
		}, nil
	}
}

func RegisterRawTools(server *mcp.Server, reg *HostRegistry) {
	mcp.AddTool[RawAPIInput, any](server, &mcp.Tool{
		Name:        "raw_api_request",
		Description: "Make a raw API request to the Proxmox REST API. Use this to explore endpoints not covered by other tools. Path is relative to /api2/json (e.g. /nodes, /cluster/resources). Supports GET, POST, PUT, and DELETE methods.",
	}, rawAPIHandler(reg))
}
