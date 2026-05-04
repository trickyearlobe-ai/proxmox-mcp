package proxmox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	client := NewClient(ts.URL, "user@pam!tok", "test-secret", false)
	return ts, client
}

func TestClient_AuthHeader(t *testing.T) {
	var gotAuth string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"data": null}`))
	})

	client.Get(context.Background(), "/test", nil)

	want := "PVEAPIToken=user@pam!tok=test-secret"
	if gotAuth != want {
		t.Errorf("Auth header = %q, want %q", gotAuth, want)
	}
}

func TestClient_Get_UnmarshalData(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"node": "pve1", "status": "online"},
				{"node": "pve2", "status": "offline"},
			},
		})
	})

	var nodes []struct {
		Node   string `json:"node"`
		Status string `json:"status"`
	}
	err := client.Get(context.Background(), "/nodes", &nodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("len(nodes) = %d, want 2", len(nodes))
	}
	if nodes[0].Node != "pve1" {
		t.Errorf("nodes[0].Node = %q", nodes[0].Node)
	}
}

func TestClient_Get_NilDest(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data": "ignored"}`))
	})

	err := client.Get(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Get_HTTPError(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"errors":{"user":"permission denied"}}`))
	})

	var dest any
	err := client.Get(context.Background(), "/test", &dest)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !containsStr(err.Error(), "403") {
		t.Errorf("error should contain status code: %v", err)
	}
}

func TestClient_Post(t *testing.T) {
	var gotMethod string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		json.NewEncoder(w).Encode(map[string]any{
			"data": "UPID:pve1:00001234:12345678:663E1234:qmstart:100:root@pam:",
		})
	})

	var upid string
	err := client.Post(context.Background(), "/nodes/pve1/qemu/100/status/start", &upid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if upid == "" {
		t.Error("upid should not be empty")
	}
}

func TestClient_Delete(t *testing.T) {
	var gotMethod string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		json.NewEncoder(w).Encode(map[string]any{
			"data": "UPID:pve1:00005678:ABCDEF01:663E5678:qmdestroy:100:root@pam:",
		})
	})

	var upid string
	err := client.Delete(context.Background(), "/nodes/pve1/qemu/100", &upid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != "DELETE" {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	if upid == "" {
		t.Error("upid should not be empty")
	}
}

func TestClient_Put(t *testing.T) {
	var gotMethod string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		json.NewEncoder(w).Encode(map[string]any{
			"data": nil,
		})
	})

	err := client.Put(context.Background(), "/nodes/pve1/qemu/100/config", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != "PUT" {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
}

func TestClient_GetRaw(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"cpu":    0.5,
				"uptime": 12345,
				"nested": map[string]any{"deep": true},
			},
		})
	})

	raw, err := client.GetRaw(context.Background(), "/nodes/pve1/status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("raw response is not valid JSON: %v", err)
	}
	if parsed["uptime"].(float64) != 12345 {
		t.Errorf("uptime = %v", parsed["uptime"])
	}
}

func TestClient_GetRaw_HTTPError(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`internal error`))
	})

	_, err := client.GetRaw(context.Background(), "/test")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestClient_DoRaw_GET(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s", r.Method)
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"data": [1, 2, 3]}`))
	})

	status, body, err := client.DoRaw(context.Background(), "GET", "/test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d", status)
	}
	if body == "" {
		t.Error("body should not be empty")
	}
}

func TestClient_DoRaw_POST_WithBody(t *testing.T) {
	var gotBody string
	var gotContentType string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(200)
		w.Write([]byte(`{"data": "ok"}`))
	})

	status, _, err := client.DoRaw(context.Background(), "POST", "/test", "key=value&other=123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d", status)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", gotContentType)
	}
	if gotBody != "key=value&other=123" {
		t.Errorf("body = %q", gotBody)
	}
}

func TestClient_DoRaw_POST_NoBody(t *testing.T) {
	var gotContentType string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(200)
		w.Write([]byte(`{"data": "ok"}`))
	})

	_, _, err := client.DoRaw(context.Background(), "POST", "/test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotContentType != "" {
		t.Errorf("Content-Type should be empty for no body, got %q", gotContentType)
	}
}

func TestClient_DoRaw_ReturnsErrorStatus(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`not found`))
	})

	status, body, err := client.DoRaw(context.Background(), "GET", "/missing", "")
	if err != nil {
		t.Fatalf("DoRaw should not error on non-2xx: %v", err)
	}
	if status != 404 {
		t.Errorf("status = %d", status)
	}
	if body != "not found" {
		t.Errorf("body = %q", body)
	}
}

func TestClient_DoRaw_DELETE(t *testing.T) {
	var gotMethod string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"data": "UPID:pve1:00005678:ABCDEF01:663E5678:qmdestroy:100:root@pam:",
		})
	})

	status, body, err := client.DoRaw(context.Background(), "DELETE", "/nodes/pve1/qemu/100", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
	if body == "" {
		t.Error("body should not be empty")
	}
}

func TestClient_DoRaw_PUT_WithBody(t *testing.T) {
	var gotMethod string
	var gotBody string
	var gotContentType string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(200)
		w.Write([]byte(`{"data": null}`))
	})

	status, _, err := client.DoRaw(context.Background(), "PUT", "/nodes/pve1/qemu/100/config", "memory=4096&cores=2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "PUT" {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", gotContentType)
	}
	if gotBody != "memory=4096&cores=2" {
		t.Errorf("body = %q", gotBody)
	}
}

func TestClient_RequestPath(t *testing.T) {
	var gotPath string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"data": null}`))
	})

	client.Get(context.Background(), "/nodes/pve1/qemu", nil)

	if gotPath != "/api2/json/nodes/pve1/qemu" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestClient_ResolveNode(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"node": "pve1", "vmid": 100, "type": "qemu"},
				{"node": "pve2", "vmid": 200, "type": "lxc"},
				{"node": "pve1", "vmid": 300, "type": "qemu"},
			},
		})
	})

	node, err := client.ResolveNode(context.Background(), 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node != "pve2" {
		t.Errorf("node = %q, want pve2", node)
	}
}

func TestClient_ResolveNode_NotFound(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"node": "pve1", "vmid": 100, "type": "qemu"},
			},
		})
	})

	_, err := client.ResolveNode(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for VMID not found")
	}
	if !containsStr(err.Error(), "999") {
		t.Errorf("error should mention VMID: %v", err)
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data": null}`))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := client.Get(ctx, "/test", nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	client := NewClient("https://pve.example.com:8006/", "tok", "secret", false)
	if client.baseURL != "https://pve.example.com:8006" {
		t.Errorf("baseURL = %q, trailing slash should be trimmed", client.baseURL)
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
