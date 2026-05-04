package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trickyearlobe-ai/proxmox-mcp/config"
	"github.com/trickyearlobe-ai/proxmox-mcp/proxmox"
)

func newTestRegistry(t *testing.T, handler http.HandlerFunc) *HostRegistry {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	cfg := &config.Config{
		DefaultHost: "test",
		Hosts: map[string]config.HostConfig{
			"test": {
				URL:         ts.URL,
				TokenID:     "user@pam!tok",
				TokenSecret: "secret",
			},
		},
	}
	return NewHostRegistry(cfg)
}

func TestRawAPIHandler_DELETE(t *testing.T) {
	var gotMethod string
	reg := newTestRegistry(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"data": "UPID:pve1:00005678:ABCDEF01:663E5678:qmdestroy:100:root@pam:",
		})
	})

	handler := rawAPIHandler(reg)
	input := RawAPIInput{Method: "DELETE", Path: "/nodes/pve1/qemu/100"}
	_, output, err := handler(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	result := output.(RawAPIOutput)
	if result.StatusCode != 200 {
		t.Errorf("status = %d, want 200", result.StatusCode)
	}
}

func TestRawAPIHandler_PUT(t *testing.T) {
	var gotMethod string
	var gotBody string
	reg := newTestRegistry(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(200)
		w.Write([]byte(`{"data": null}`))
	})

	handler := rawAPIHandler(reg)
	input := RawAPIInput{Method: "PUT", Path: "/nodes/pve1/qemu/100/config", Body: "memory=4096"}
	_, output, err := handler(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "PUT" {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if gotBody != "memory=4096" {
		t.Errorf("body = %q, want memory=4096", gotBody)
	}
	result := output.(RawAPIOutput)
	if result.StatusCode != 200 {
		t.Errorf("status = %d, want 200", result.StatusCode)
	}
}

func TestRawAPIHandler_InvalidMethod(t *testing.T) {
	reg := newTestRegistry(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	handler := rawAPIHandler(reg)
	input := RawAPIInput{Method: "PATCH", Path: "/test"}
	_, _, err := handler(context.Background(), nil, input)
	if err == nil {
		t.Fatal("expected error for PATCH method")
	}
	if !containsSubstr(err.Error(), "PATCH") {
		t.Errorf("error should mention PATCH: %v", err)
	}
}

func TestRawAPIHandler_InvalidPath(t *testing.T) {
	reg := newTestRegistry(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	handler := rawAPIHandler(reg)
	input := RawAPIInput{Method: "GET", Path: "no-slash"}
	_, _, err := handler(context.Background(), nil, input)
	if err == nil {
		t.Fatal("expected error for path without /")
	}
}

func TestDeleteVMHandler(t *testing.T) {
	var gotMethod string
	var gotPath string

	// First call resolves node, second call is the delete
	callCount := 0
	client := proxmox.NewClient("http://unused", "tok", "secret", false)
	_ = client

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		gotMethod = r.Method
		gotPath = r.URL.Path
		if r.URL.Path == "/api2/json/cluster/resources" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"node": "pve1", "vmid": 100, "type": "qemu"},
				},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": "UPID:pve1:00005678:ABCDEF01:663E5678:qmdestroy:100:root@pam:",
		})
	}))
	defer ts.Close()

	cfg := &config.Config{
		DefaultHost: "test",
		Hosts: map[string]config.HostConfig{
			"test": {URL: ts.URL, TokenID: "user@pam!tok", TokenSecret: "secret"},
		},
	}
	reg := NewHostRegistry(cfg)

	handler := deleteVMHandler(reg)
	input := DeleteVMInput{VMID: 100}
	_, output, err := handler(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	if gotPath != "/api2/json/nodes/pve1/qemu/100" {
		t.Errorf("path = %s", gotPath)
	}
	result := output.(VMActionOutput)
	if result.Action != "delete" {
		t.Errorf("action = %s, want delete", result.Action)
	}
	if result.Node != "pve1" {
		t.Errorf("node = %s, want pve1", result.Node)
	}
	if result.UPID == "" {
		t.Error("UPID should not be empty")
	}
}

func TestDeleteVMHandler_ExplicitNode(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]any{
			"data": "UPID:pve2:00005678:ABCDEF01:663E5678:qmdestroy:200:root@pam:",
		})
	}))
	defer ts.Close()

	cfg := &config.Config{
		DefaultHost: "test",
		Hosts: map[string]config.HostConfig{
			"test": {URL: ts.URL, TokenID: "user@pam!tok", TokenSecret: "secret"},
		},
	}
	reg := NewHostRegistry(cfg)

	handler := deleteVMHandler(reg)
	input := DeleteVMInput{Node: "pve2", VMID: 200}
	_, _, err := handler(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should NOT call /cluster/resources when node is explicitly provided
	if gotPath != "/api2/json/nodes/pve2/qemu/200" {
		t.Errorf("path = %s, expected direct delete without resolve", gotPath)
	}
}

func TestDeleteVMHandler_DefaultParams(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/cluster/resources" {
			gotQuery = r.URL.RawQuery
		}
		if r.URL.Path == "/api2/json/cluster/resources" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"node": "pve1", "vmid": 100, "type": "qemu"},
				},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": "UPID:pve1:task",
		})
	}))
	defer ts.Close()

	cfg := &config.Config{
		DefaultHost: "test",
		Hosts: map[string]config.HostConfig{
			"test": {URL: ts.URL, TokenID: "tok", TokenSecret: "secret"},
		},
	}
	reg := NewHostRegistry(cfg)

	handler := deleteVMHandler(reg)
	// No DestroyDisks or Purge set — should default to 1
	input := DeleteVMInput{VMID: 100}
	_, _, err := handler(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "destroy-unreferenced-disks=1&purge=1" {
		t.Errorf("query = %q, want defaults of 1&1", gotQuery)
	}
}

func TestDeleteVMHandler_ExplicitFalseParams(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/cluster/resources" {
			gotQuery = r.URL.RawQuery
		}
		if r.URL.Path == "/api2/json/cluster/resources" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"node": "pve1", "vmid": 100, "type": "qemu"},
				},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": "UPID:pve1:task",
		})
	}))
	defer ts.Close()

	cfg := &config.Config{
		DefaultHost: "test",
		Hosts: map[string]config.HostConfig{
			"test": {URL: ts.URL, TokenID: "tok", TokenSecret: "secret"},
		},
	}
	reg := NewHostRegistry(cfg)

	handler := deleteVMHandler(reg)
	f := false
	input := DeleteVMInput{VMID: 100, DestroyDisks: &f, Purge: &f}
	_, _, err := handler(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "destroy-unreferenced-disks=0&purge=0" {
		t.Errorf("query = %q, want 0&0 when explicitly false", gotQuery)
	}
}

func TestDeleteContainerHandler(t *testing.T) {
	var gotMethod string
	var gotPath string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if r.URL.Path == "/api2/json/cluster/resources" {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"node": "pve3", "vmid": 500, "type": "lxc"},
				},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": "UPID:pve3:00009999:12345678:663E9999:vzdestroy:500:root@pam:",
		})
	}))
	defer ts.Close()

	cfg := &config.Config{
		DefaultHost: "test",
		Hosts: map[string]config.HostConfig{
			"test": {URL: ts.URL, TokenID: "user@pam!tok", TokenSecret: "secret"},
		},
	}
	reg := NewHostRegistry(cfg)

	handler := deleteContainerHandler(reg)
	input := DeleteContainerInput{VMID: 500}
	_, output, err := handler(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	if gotPath != "/api2/json/nodes/pve3/lxc/500" {
		t.Errorf("path = %s", gotPath)
	}
	result := output.(ContainerActionOutput)
	if result.Action != "delete" {
		t.Errorf("action = %s, want delete", result.Action)
	}
	if result.Node != "pve3" {
		t.Errorf("node = %s, want pve3", result.Node)
	}
}

func TestBoolParamDefault(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name     string
		input    *bool
		defVal   bool
		expected string
	}{
		{"nil default true", nil, true, "1"},
		{"nil default false", nil, false, "0"},
		{"explicit true", &trueVal, false, "1"},
		{"explicit false", &falseVal, true, "0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := boolParamDefault(tc.input, tc.defVal)
			if got != tc.expected {
				t.Errorf("boolParamDefault = %q, want %q", got, tc.expected)
			}
		})
	}
}
