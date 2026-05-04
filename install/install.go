package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// IDE represents a supported IDE with its config path and format.
type IDE struct {
	Name        string
	ConfigPath  string // relative to home dir
	TopLevelKey string
	ExtraFields bool // Zed requires args+env, Copilot CLI requires type+args+env+tools
	IsJSONC     bool // Zed uses JSONC
	IsCopilot   bool // Copilot CLI has special fields
}

func supportedIDEs() []IDE {
	ides := []IDE{
		{Name: "Claude Desktop", TopLevelKey: "mcpServers"},
		{Name: "Claude Code", ConfigPath: ".claude.json", TopLevelKey: "mcpServers"},
		{Name: "VS Code", ConfigPath: ".vscode/mcp.json", TopLevelKey: "servers"},
		{Name: "Cursor", ConfigPath: ".cursor/mcp.json", TopLevelKey: "servers"},
		{Name: "Windsurf", ConfigPath: ".codeium/windsurf/mcp_config.json", TopLevelKey: "mcpServers"},
		{Name: "Zed", ConfigPath: ".config/zed/settings.json", TopLevelKey: "context_servers", ExtraFields: true, IsJSONC: true},
		{Name: "Copilot CLI", ConfigPath: ".copilot/mcp-config.json", TopLevelKey: "mcpServers", IsCopilot: true},
		{Name: "JetBrains", ConfigPath: ".junie/mcp/mcp.json", TopLevelKey: "mcpServers"},
	}

	// Claude Desktop has platform-specific paths
	switch runtime.GOOS {
	case "darwin":
		ides[0].ConfigPath = "Library/Application Support/Claude/claude_desktop_config.json"
	case "windows":
		// Uses %APPDATA% instead of home dir
		appData := os.Getenv("APPDATA")
		if appData != "" {
			ides[0].ConfigPath = filepath.Join(appData, "Claude", "claude_desktop_config.json")
		}
	default:
		ides[0].ConfigPath = ".config/Claude/claude_desktop_config.json"
	}

	return ides
}

const serverName = "proxmox"

// Install registers the MCP server in all detected IDE configs.
func Install() error {
	binPath, err := resolveExecutable()
	if err != nil {
		return err
	}

	var errors []string
	for _, ide := range supportedIDEs() {
		result := installIDE(ide, binPath)
		fmt.Fprintf(os.Stderr, "  %-16s %s\n", ide.Name+":", result)
		if strings.HasPrefix(result, "ERROR") {
			errors = append(errors, fmt.Sprintf("%s: %s", ide.Name, result))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("%d IDE(s) had errors", len(errors))
	}
	return nil
}

// Uninstall removes the MCP server from all detected IDE configs.
func Uninstall() error {
	var errors []string
	for _, ide := range supportedIDEs() {
		result := uninstallIDE(ide)
		fmt.Fprintf(os.Stderr, "  %-16s %s\n", ide.Name+":", result)
		if strings.HasPrefix(result, "ERROR") {
			errors = append(errors, fmt.Sprintf("%s: %s", ide.Name, result))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("%d IDE(s) had errors", len(errors))
	}
	return nil
}

func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine executable path: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("cannot resolve symlinks for %s: %w", exe, err)
	}

	// Warn about unstable paths
	if strings.Contains(resolved, "go-build") || strings.Contains(resolved, "/tmp/") || strings.Contains(resolved, "/var/folders/") {
		fmt.Fprintf(os.Stderr, "WARNING: Executable path %q looks like a temporary build path.\n", resolved)
		fmt.Fprintf(os.Stderr, "         Consider installing with: go install github.com/trickyearlobe-ai/proxmox-mcp@latest\n")
	}

	return resolved, nil
}

func configFilePath(ide IDE) string {
	if filepath.IsAbs(ide.ConfigPath) {
		return ide.ConfigPath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ide.ConfigPath)
}

func installIDE(ide IDE, binPath string) string {
	cfgPath := configFilePath(ide)

	// Check if the parent directory exists (IDE installed?)
	dir := filepath.Dir(cfgPath)

	// For Claude Code's ~/.claude.json, parent is home — always exists
	// For others, check the specific directory
	if ide.Name != "Claude Code" {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return "skipped (IDE not detected)"
		}
	}

	// Read existing config or start fresh
	var configMap map[string]any
	var preamble string

	if data, err := os.ReadFile(cfgPath); err == nil {
		content := string(data)
		if ide.IsJSONC {
			preamble, content = SplitPreamble(content)
			content = StripJSONC(content)
		}
		if err := json.Unmarshal([]byte(content), &configMap); err != nil {
			return fmt.Sprintf("ERROR: failed to parse %s: %v", cfgPath, err)
		}
	} else {
		configMap = make(map[string]any)
	}

	// Get or create the servers section
	serversRaw, ok := configMap[ide.TopLevelKey]
	if !ok {
		serversRaw = make(map[string]any)
		configMap[ide.TopLevelKey] = serversRaw
	}

	servers, ok := serversRaw.(map[string]any)
	if !ok {
		return fmt.Sprintf("ERROR: %s is not an object in %s", ide.TopLevelKey, cfgPath)
	}

	// Build the server entry
	entry := buildEntry(ide, binPath)

	// Check if already up to date
	if existing, ok := servers[serverName]; ok {
		existingJSON, _ := json.Marshal(existing)
		newJSON, _ := json.Marshal(entry)
		if string(existingJSON) == string(newJSON) {
			return "already up to date"
		}
		servers[serverName] = entry
		return writeConfig(cfgPath, configMap, preamble, "updated")
	}

	servers[serverName] = entry
	return writeConfig(cfgPath, configMap, preamble, "installed")
}

func uninstallIDE(ide IDE) string {
	cfgPath := configFilePath(ide)

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return "not present (no config file)"
	}

	var configMap map[string]any
	var preamble string
	content := string(data)

	if ide.IsJSONC {
		preamble, content = SplitPreamble(content)
		content = StripJSONC(content)
	}

	if err := json.Unmarshal([]byte(content), &configMap); err != nil {
		return fmt.Sprintf("ERROR: failed to parse %s: %v", cfgPath, err)
	}

	serversRaw, ok := configMap[ide.TopLevelKey]
	if !ok {
		return "not present"
	}

	servers, ok := serversRaw.(map[string]any)
	if !ok {
		return "not present"
	}

	if _, ok := servers[serverName]; !ok {
		return "not present"
	}

	delete(servers, serverName)

	// If the servers map is now empty, remove the key entirely
	if len(servers) == 0 {
		delete(configMap, ide.TopLevelKey)
	}

	return writeConfig(cfgPath, configMap, preamble, "removed")
}

func buildEntry(ide IDE, binPath string) map[string]any {
	entry := map[string]any{
		"command": binPath,
	}

	// Zed requires args and env
	if ide.ExtraFields {
		entry["args"] = []string{}
		entry["env"] = map[string]string{}
	}

	// Copilot CLI requires type, args, env, tools
	if ide.IsCopilot {
		entry["type"] = "local"
		entry["args"] = []string{}
		entry["env"] = map[string]string{}
		entry["tools"] = []string{"*"}
	}

	return entry
}

func writeConfig(path string, configMap map[string]any, preamble string, successMsg string) string {
	data, err := json.MarshalIndent(configMap, "", "  ")
	if err != nil {
		return fmt.Sprintf("ERROR: failed to marshal config: %v", err)
	}

	content := preamble + string(data) + "\n"

	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Sprintf("ERROR: failed to create directory: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Sprintf("ERROR: failed to write %s: %v", path, err)
	}

	return successMsg
}
