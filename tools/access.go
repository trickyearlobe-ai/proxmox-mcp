package tools

import (
	"context"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// -- get_token_permissions --

type GetPermissionsInput struct {
	Host   string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	AuthID string `json:"auth_id,omitempty" jsonschema:"user or token to inspect (e.g. user@pam or user@pam!tokenid). Defaults to the token the MCP server authenticates with."`
	Path   string `json:"path,omitempty" jsonschema:"optional ACL path to restrict the result to (e.g. / or /storage/local)"`
}

type GetPermissionsOutput struct {
	Host        string `json:"host" jsonschema:"Proxmox host that was queried"`
	AuthID      string `json:"auth_id,omitempty" jsonschema:"the auth-id inspected (empty means the server's own token)"`
	Permissions string `json:"permissions" jsonschema:"effective privileges as JSON: an object of {path: {Privilege: 1, ...}}. Look for Sys.Modify, Datastore.* etc. to explain 403s."`
}

func getPermissionsHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, GetPermissionsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input GetPermissionsInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, GetPermissionsOutput{}, err
		}
		q := url.Values{}
		if input.AuthID != "" {
			q.Set("userid", input.AuthID)
		}
		if input.Path != "" {
			q.Set("path", input.Path)
		}
		path := "/access/permissions"
		if len(q) > 0 {
			path += "?" + q.Encode()
		}
		raw, err := client.GetRaw(ctx, path)
		if err != nil {
			return nil, GetPermissionsOutput{}, fmt.Errorf("failed to get permissions: %w", err)
		}
		return nil, GetPermissionsOutput{Host: host, AuthID: input.AuthID, Permissions: raw}, nil
	}
}

// -- list_acl --

type ListACLInput struct {
	Host string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
}

type ACLEntry struct {
	Path      string `json:"path" jsonschema:"ACL path the role applies to"`
	RoleID    string `json:"roleid" jsonschema:"role assigned"`
	Type      string `json:"type" jsonschema:"subject type: user, group, or token"`
	UGID      string `json:"ugid" jsonschema:"the user, group, or token id"`
	Propagate int    `json:"propagate" jsonschema:"1 if the role propagates to sub-paths"`
}

type ListACLOutput struct {
	Host string     `json:"host" jsonschema:"Proxmox host that was queried"`
	ACL  []ACLEntry `json:"acl" jsonschema:"all access-control entries"`
}

func listACLHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, ListACLInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListACLInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ListACLOutput{}, err
		}
		var acl []ACLEntry
		if err := client.Get(ctx, "/access/acl", &acl); err != nil {
			return nil, ListACLOutput{}, fmt.Errorf("failed to list ACL: %w", err)
		}
		return nil, ListACLOutput{Host: host, ACL: acl}, nil
	}
}

// -- list_roles --

type ListRolesInput struct {
	Host string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
}

type RoleInfo struct {
	RoleID  string `json:"roleid" jsonschema:"role name"`
	Privs   string `json:"privs" jsonschema:"comma-separated privileges the role grants"`
	Special int    `json:"special,omitempty" jsonschema:"1 for built-in roles"`
}

type ListRolesOutput struct {
	Host  string     `json:"host" jsonschema:"Proxmox host that was queried"`
	Roles []RoleInfo `json:"roles" jsonschema:"all defined roles and their privileges"`
}

func listRolesHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, ListRolesInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input ListRolesInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, ListRolesOutput{}, err
		}
		var roles []RoleInfo
		if err := client.Get(ctx, "/access/roles", &roles); err != nil {
			return nil, ListRolesOutput{}, fmt.Errorf("failed to list roles: %w", err)
		}
		return nil, ListRolesOutput{Host: host, Roles: roles}, nil
	}
}

func RegisterAccessTools(server *mcp.Server, reg *HostRegistry) {
	mcp.AddTool[GetPermissionsInput, any](server, &mcp.Tool{
		Name: "get_token_permissions",
		Description: "Show the effective privileges of an API token or user (defaults to the token the server uses). " +
			"Use this to explain a 403 — e.g. check for Sys.Modify (needed by download_url_to_storage and send_key) or Datastore privileges.",
	}, getPermissionsHandler(reg))

	mcp.AddTool[ListACLInput, any](server, &mcp.Tool{
		Name:        "list_acl",
		Description: "List all access-control entries (which role is assigned to which user/group/token on which path).",
	}, listACLHandler(reg))

	mcp.AddTool[ListRolesInput, any](server, &mcp.Tool{
		Name:        "list_roles",
		Description: "List all Proxmox roles and the privileges each one grants (e.g. which role includes Sys.Modify).",
	}, listRolesHandler(reg))
}
