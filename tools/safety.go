package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// confirmWrite asks the user to approve a mutating operation through MCP elicitation,
// but only when write confirmation is enabled on the registry. It returns an error
// when the user declines or when the client cannot present the prompt — failing
// closed so an unconfirmable write never silently proceeds.
//
// The real authorization boundary remains the Proxmox API token's own role/ACL; this
// is an optional human-in-the-loop speed bump on top.
func confirmWrite(ctx context.Context, reg *HostRegistry, req *mcp.CallToolRequest, summary string) error {
	if !reg.confirmWrites {
		return nil
	}
	if req == nil || req.Session == nil {
		return fmt.Errorf("write confirmation is enabled but no session is available to prompt the user")
	}
	res, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
		Message: "Approve this Proxmox write operation?\n\n" + summary,
	})
	if err != nil {
		return fmt.Errorf("write confirmation required but the client could not prompt the user (%w); refusing", err)
	}
	if res.Action != "accept" {
		return fmt.Errorf("operation declined by user (action: %s)", res.Action)
	}
	return nil
}
