package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type WaitGuestAgentInput struct {
	Host           string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node           string `json:"node,omitempty" jsonschema:"node name (optional — auto-resolved from VMID if omitted)"`
	VMID           int    `json:"vmid" jsonschema:"VM ID,required"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"how long to wait for the guest agent (default 600, max 1800)"`
}

type WaitGuestAgentOutput struct {
	Host           string `json:"host" jsonschema:"Proxmox host that was queried"`
	Node           string `json:"node" jsonschema:"node hosting the VM"`
	VMID           int    `json:"vmid" jsonschema:"VM ID"`
	Online         bool   `json:"online" jsonschema:"true if the guest agent responded before the timeout"`
	ElapsedSeconds int    `json:"elapsed_seconds" jsonschema:"how long the wait took"`
	Interfaces     string `json:"interfaces,omitempty" jsonschema:"raw network-get-interfaces JSON when online (contains IP addresses)"`
	Detail         string `json:"detail,omitempty" jsonschema:"last error detail if the agent never came online"`
}

func waitGuestAgentHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, WaitGuestAgentInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input WaitGuestAgentInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, WaitGuestAgentOutput{}, err
		}
		node, err := resolveVMNode(ctx, client, input.Node, input.VMID)
		if err != nil {
			return nil, WaitGuestAgentOutput{}, err
		}

		timeout := input.TimeoutSeconds
		if timeout <= 0 {
			timeout = 600
		}
		if timeout > 1800 {
			timeout = 1800
		}

		token := req.Params.GetProgressToken()
		notify := func(elapsed int, msg string) {
			if token == nil {
				return
			}
			_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
				ProgressToken: token,
				Message:       msg,
				Progress:      float64(elapsed),
				Total:         float64(timeout),
			})
		}

		out := WaitGuestAgentOutput{Host: host, Node: node, VMID: input.VMID}
		path := fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", node, input.VMID)
		const interval = 5 * time.Second
		start := time.Now()
		deadline := start.Add(time.Duration(timeout) * time.Second)

		for {
			elapsed := int(time.Since(start).Seconds())
			raw, err := client.GetRaw(ctx, path)
			if err == nil {
				out.Online = true
				out.Interfaces = raw
				out.ElapsedSeconds = elapsed
				notify(elapsed, fmt.Sprintf("guest agent online after %ds", elapsed))
				return nil, out, nil
			}
			out.Detail = err.Error()

			if time.Now().Add(interval).After(deadline) {
				out.Online = false
				out.ElapsedSeconds = int(time.Since(start).Seconds())
				return nil, out, nil
			}
			notify(elapsed, fmt.Sprintf("waiting for guest agent on VM %d (%ds elapsed)", input.VMID, elapsed))

			select {
			case <-ctx.Done():
				out.Online = false
				out.ElapsedSeconds = int(time.Since(start).Seconds())
				out.Detail = ctx.Err().Error()
				return nil, out, nil
			case <-time.After(interval):
			}
		}
	}
}

func RegisterWaitTools(server *mcp.Server, reg *HostRegistry) {
	mcp.AddTool[WaitGuestAgentInput, any](server, &mcp.Tool{
		Name: "wait_for_guest_agent",
		Description: "Block until a VM's QEMU guest agent responds (e.g. after an OS install reboots), then return its network interfaces/IPs. " +
			"Emits progress notifications while waiting. Use this instead of repeatedly polling get_guest_agent_info during installs.",
	}, waitGuestAgentHandler(reg))
}
