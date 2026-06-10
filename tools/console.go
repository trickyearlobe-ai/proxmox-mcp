package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func clampWindow(seconds, def, max int) time.Duration {
	if seconds <= 0 {
		seconds = def
	}
	if seconds > max {
		seconds = max
	}
	return time.Duration(seconds) * time.Second
}

// -- read_serial_console --

type ReadConsoleInput struct {
	Host          string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node          string `json:"node,omitempty" jsonschema:"node name (optional — auto-resolved from VMID if omitted)"`
	VMID          int    `json:"vmid" jsonschema:"VM ID,required"`
	WindowSeconds int    `json:"window_seconds,omitempty" jsonschema:"max seconds to capture output (default 6, max 30)"`
}

type ConsoleOutput struct {
	Host   string `json:"host" jsonschema:"Proxmox host that was queried"`
	Node   string `json:"node" jsonschema:"node hosting the VM"`
	VMID   int    `json:"vmid" jsonschema:"VM ID"`
	Output string `json:"output" jsonschema:"captured serial console text (may be empty if the guest emitted nothing in the window)"`
}

func readConsoleHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, ReadConsoleInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ReadConsoleInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ConsoleOutput{}, err
		}
		node, err := resolveVMNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, ConsoleOutput{}, err
		}
		window := clampWindow(input.WindowSeconds, 6, 30)
		out, err := client.SerialConsoleExchange(ctx, node, input.VMID, "", window, 1500*time.Millisecond)
		if err != nil {
			return nil, ConsoleOutput{}, fmt.Errorf("reading serial console: %w", err)
		}
		return nil, ConsoleOutput{Host: host, Node: node, VMID: input.VMID, Output: out}, nil
	}
}

// -- send_serial_console --

type SendConsoleInput struct {
	Host          string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node          string `json:"node,omitempty" jsonschema:"node name (optional — auto-resolved from VMID if omitted)"`
	VMID          int    `json:"vmid" jsonschema:"VM ID,required"`
	Input         string `json:"input" jsonschema:"text to type into the serial console. Include a trailing newline (\\n) to press Enter; control characters are sent as-is,required"`
	WindowSeconds int    `json:"window_seconds,omitempty" jsonschema:"max seconds to capture the response after sending (default 6, max 30)"`
}

func sendConsoleHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, SendConsoleInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input SendConsoleInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ConsoleOutput{}, err
		}
		node, err := resolveVMNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, ConsoleOutput{}, err
		}
		if err := confirmWrite(ctx, reg, req, fmt.Sprintf("send_serial_console: type into VM %d console on %s/%s", input.VMID, host, node)); err != nil {
			return nil, ConsoleOutput{}, err
		}
		window := clampWindow(input.WindowSeconds, 6, 30)
		out, err := client.SerialConsoleExchange(ctx, node, input.VMID, input.Input, window, 1500*time.Millisecond)
		if err != nil {
			return nil, ConsoleOutput{}, fmt.Errorf("sending to serial console: %w", err)
		}
		return nil, ConsoleOutput{Host: host, Node: node, VMID: input.VMID, Output: out}, nil
	}
}

func RegisterConsoleTools(server *mcp.Server, reg *HostRegistry) {
	mcp.AddTool[ReadConsoleInput, any](server, &mcp.Tool{
		Name: "read_serial_console",
		Description: "Read recent output from a VM's serial console (requires a serial device, e.g. serial0: socket). " +
			"Useful for serial-only guests like network appliances (Cisco Nexus, Arista, etc.) and for reading boot/login prompts.",
	}, readConsoleHandler(reg))

	mcp.AddTool[SendConsoleInput, any](server, &mcp.Tool{
		Name: "send_serial_console",
		Description: "Type input into a VM's serial console (requires a serial device) and capture the response. " +
			"Use a trailing newline to press Enter. Lets you answer prompts, log in, and run commands on serial-only guests.",
	}, sendConsoleHandler(reg))
}
