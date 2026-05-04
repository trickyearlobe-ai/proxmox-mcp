package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// testIDE creates an IDE pointing to a temp dir
func testIDE(t *testing.T, name, topKey string, opts ...func(*IDE)) IDE {
	t.Helper()
	ide := IDE{
		Name:        name,
		ConfigPath:  filepath.Join(t.TempDir(), "config.json"),
		TopLevelKey: topKey,
	}
	for _, opt := range opts {
		opt(&ide)
	}
	return ide
}

func withZed(ide *IDE) {
	ide.ExtraFields = true
	ide.IsJSONC = true
}

func withCopilot(ide *IDE) {
	ide.IsCopilot = true
}

func TestBuildEntry_Standard(t *testing.T) {
	ide := IDE{Name: "Test", TopLevelKey: "mcpServers"}
	entry := buildEntry(ide, "/usr/local/bin/proxmox-mcp")

	if entry["command"] != "/usr/local/bin/proxmox-mcp" {
		t.Errorf("command = %v", entry["command"])
	}
	if _, ok := entry["args"]; ok {
		t.Error("standard entry should not have args")
	}
	if _, ok := entry["type"]; ok {
		t.Error("standard entry should not have type")
	}
}

func TestBuildEntry_Zed(t *testing.T) {
	ide := IDE{Name: "Zed", TopLevelKey: "context_servers", ExtraFields: true}
	entry := buildEntry(ide, "/bin/proxmox-mcp")

	if _, ok := entry["args"]; !ok {
		t.Error("Zed entry must have args")
	}
	if _, ok := entry["env"]; !ok {
		t.Error("Zed entry must have env")
	}
}

func TestBuildEntry_CopilotCLI(t *testing.T) {
	ide := IDE{Name: "Copilot CLI", TopLevelKey: "mcpServers", IsCopilot: true}
	entry := buildEntry(ide, "/bin/proxmox-mcp")

	if entry["type"] != "stdio" {
		t.Errorf("Copilot entry type = %v", entry["type"])
	}
	for _, key := range []string{"args", "env", "tools"} {
		if _, ok := entry[key]; !ok {
			t.Errorf("Copilot entry must have %s", key)
		}
	}
}

func TestInstallIDE_NewFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")

	ide := IDE{
		Name:        "Test IDE",
		ConfigPath:  cfgPath,
		TopLevelKey: "mcpServers",
	}

	result := installIDE(ide, "/bin/proxmox-mcp")
	if result != "installed" {
		t.Fatalf("result = %q, want 'installed'", result)
	}

	// Verify the file was created
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	servers := cfg["mcpServers"].(map[string]any)
	entry := servers["proxmox"].(map[string]any)
	if entry["command"] != "/bin/proxmox-mcp" {
		t.Errorf("command = %v", entry["command"])
	}
}

func TestInstallIDE_ExistingFile_PreservesOtherServers(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")

	existing := `{
  "mcpServers": {
    "other-server": {
      "command": "/bin/other"
    }
  },
  "otherKey": "preserved"
}`
	os.WriteFile(cfgPath, []byte(existing), 0644)

	ide := IDE{
		Name:        "Test IDE",
		ConfigPath:  cfgPath,
		TopLevelKey: "mcpServers",
	}

	result := installIDE(ide, "/bin/proxmox-mcp")
	if result != "installed" {
		t.Fatalf("result = %q, want 'installed'", result)
	}

	data, _ := os.ReadFile(cfgPath)
	var cfg map[string]any
	json.Unmarshal(data, &cfg)

	// Other server preserved
	servers := cfg["mcpServers"].(map[string]any)
	if _, ok := servers["other-server"]; !ok {
		t.Error("other-server should be preserved")
	}
	if _, ok := servers["proxmox"]; !ok {
		t.Error("proxmox should be installed")
	}

	// Other top-level key preserved
	if cfg["otherKey"] != "preserved" {
		t.Error("otherKey should be preserved")
	}
}

func TestInstallIDE_Idempotent(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")

	ide := IDE{
		Name:        "Test IDE",
		ConfigPath:  cfgPath,
		TopLevelKey: "mcpServers",
	}

	result1 := installIDE(ide, "/bin/proxmox-mcp")
	if result1 != "installed" {
		t.Fatalf("first install: %q", result1)
	}

	result2 := installIDE(ide, "/bin/proxmox-mcp")
	if result2 != "already up to date" {
		t.Fatalf("second install should be idempotent: %q", result2)
	}
}

func TestInstallIDE_UpdatesExistingEntry(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")

	ide := IDE{
		Name:        "Test IDE",
		ConfigPath:  cfgPath,
		TopLevelKey: "mcpServers",
	}

	installIDE(ide, "/old/path")
	result := installIDE(ide, "/new/path")
	if result != "updated" {
		t.Fatalf("result = %q, want 'updated'", result)
	}

	data, _ := os.ReadFile(cfgPath)
	var cfg map[string]any
	json.Unmarshal(data, &cfg)
	servers := cfg["mcpServers"].(map[string]any)
	entry := servers["proxmox"].(map[string]any)
	if entry["command"] != "/new/path" {
		t.Errorf("command should be updated to /new/path, got %v", entry["command"])
	}
}

func TestInstallIDE_SkipsWhenDirMissing(t *testing.T) {
	ide := IDE{
		Name:        "Missing IDE",
		ConfigPath:  filepath.Join(t.TempDir(), "nonexistent-subdir", "config.json"),
		TopLevelKey: "mcpServers",
	}

	result := installIDE(ide, "/bin/proxmox-mcp")
	if result != "skipped (IDE not detected)" {
		t.Fatalf("result = %q, want 'skipped'", result)
	}
}

func TestInstallIDE_VSCode_ServersKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")

	ide := IDE{
		Name:        "VS Code",
		ConfigPath:  cfgPath,
		TopLevelKey: "servers",
	}

	installIDE(ide, "/bin/proxmox-mcp")

	data, _ := os.ReadFile(cfgPath)
	var cfg map[string]any
	json.Unmarshal(data, &cfg)

	if _, ok := cfg["servers"]; !ok {
		t.Error("VS Code should use 'servers' key")
	}
}

func TestInstallIDE_Zed_JSONC(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "settings.json")

	existing := `// Zed settings
{
  "theme": "One Dark",
  "context_servers": {},
}`
	os.WriteFile(cfgPath, []byte(existing), 0644)

	ide := IDE{
		Name:        "Zed",
		ConfigPath:  cfgPath,
		TopLevelKey: "context_servers",
		ExtraFields: true,
		IsJSONC:     true,
	}

	result := installIDE(ide, "/bin/proxmox-mcp")
	if result != "installed" {
		t.Fatalf("result = %q", result)
	}

	data, _ := os.ReadFile(cfgPath)
	content := string(data)

	// Preamble should be preserved
	if content[:2] != "//" {
		t.Error("preamble should be preserved")
	}

	// Should still be parseable (after stripping preamble)
	_, jsonBody := SplitPreamble(content)
	var cfg map[string]any
	if err := json.Unmarshal([]byte(jsonBody), &cfg); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	servers := cfg["context_servers"].(map[string]any)
	entry := servers["proxmox"].(map[string]any)
	// Zed requires args and env
	if _, ok := entry["args"]; !ok {
		t.Error("Zed entry must have args")
	}
	if _, ok := entry["env"]; !ok {
		t.Error("Zed entry must have env")
	}
}

func TestInstallIDE_CopilotCLI_ExtraFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp-config.json")

	ide := IDE{
		Name:        "Copilot CLI",
		ConfigPath:  cfgPath,
		TopLevelKey: "mcpServers",
		IsCopilot:   true,
	}

	installIDE(ide, "/bin/proxmox-mcp")

	data, _ := os.ReadFile(cfgPath)
	var cfg map[string]any
	json.Unmarshal(data, &cfg)

	servers := cfg["mcpServers"].(map[string]any)
	entry := servers["proxmox"].(map[string]any)

	if entry["type"] != "stdio" {
		t.Errorf("Copilot type = %v", entry["type"])
	}
	for _, key := range []string{"args", "env", "tools"} {
		if _, ok := entry[key]; !ok {
			t.Errorf("Copilot entry must have %s", key)
		}
	}
}

func TestUninstallIDE_Removes(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")

	ide := IDE{
		Name:        "Test IDE",
		ConfigPath:  cfgPath,
		TopLevelKey: "mcpServers",
	}

	// Install first
	installIDE(ide, "/bin/proxmox-mcp")

	// Then uninstall
	result := uninstallIDE(ide)
	if result != "removed" {
		t.Fatalf("result = %q, want 'removed'", result)
	}

	data, _ := os.ReadFile(cfgPath)
	var cfg map[string]any
	json.Unmarshal(data, &cfg)

	// mcpServers key should be removed (was the only server)
	if _, ok := cfg["mcpServers"]; ok {
		t.Error("empty mcpServers key should be removed")
	}
}

func TestUninstallIDE_PreservesOtherServers(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")

	existing := `{
  "mcpServers": {
    "proxmox": {"command": "/bin/proxmox-mcp"},
    "other": {"command": "/bin/other"}
  }
}`
	os.WriteFile(cfgPath, []byte(existing), 0644)

	ide := IDE{
		Name:        "Test IDE",
		ConfigPath:  cfgPath,
		TopLevelKey: "mcpServers",
	}

	result := uninstallIDE(ide)
	if result != "removed" {
		t.Fatalf("result = %q", result)
	}

	data, _ := os.ReadFile(cfgPath)
	var cfg map[string]any
	json.Unmarshal(data, &cfg)

	servers := cfg["mcpServers"].(map[string]any)
	if _, ok := servers["proxmox"]; ok {
		t.Error("proxmox should be removed")
	}
	if _, ok := servers["other"]; !ok {
		t.Error("other server should be preserved")
	}
}

func TestUninstallIDE_NotPresent(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")
	os.WriteFile(cfgPath, []byte(`{"mcpServers": {}}`), 0644)

	ide := IDE{
		Name:        "Test IDE",
		ConfigPath:  cfgPath,
		TopLevelKey: "mcpServers",
	}

	result := uninstallIDE(ide)
	if result != "not present" {
		t.Fatalf("result = %q, want 'not present'", result)
	}
}

func TestUninstallIDE_NoConfigFile(t *testing.T) {
	ide := IDE{
		Name:        "Test IDE",
		ConfigPath:  filepath.Join(t.TempDir(), "nonexistent.json"),
		TopLevelKey: "mcpServers",
	}

	result := uninstallIDE(ide)
	if result != "not present (no config file)" {
		t.Fatalf("result = %q", result)
	}
}

func TestRoundTrip_InstallUninstallInstall(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")

	ide := IDE{
		Name:        "Test IDE",
		ConfigPath:  cfgPath,
		TopLevelKey: "mcpServers",
	}

	// Install
	r1 := installIDE(ide, "/bin/proxmox-mcp")
	if r1 != "installed" {
		t.Fatalf("install: %q", r1)
	}

	// Uninstall
	r2 := uninstallIDE(ide)
	if r2 != "removed" {
		t.Fatalf("uninstall: %q", r2)
	}

	// Uninstall again (idempotent)
	r3 := uninstallIDE(ide)
	if r3 != "not present (no config file)" && r3 != "not present" {
		t.Fatalf("second uninstall: %q", r3)
	}

	// Re-install
	r4 := installIDE(ide, "/bin/proxmox-mcp")
	if r4 != "installed" {
		t.Fatalf("re-install: %q", r4)
	}
}
