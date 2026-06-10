package proxmox

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a Proxmox VE REST API client.
type Client struct {
	baseURL     string
	tokenID     string
	tokenSecret string
	httpClient  *http.Client
}

// NewClient creates a Client from explicit parameters.
func NewClient(baseURL, tokenID, tokenSecret string, tlsInsecure bool) *Client {
	baseURL = strings.TrimRight(baseURL, "/")

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: tlsInsecure,
		},
	}

	return &Client{
		baseURL:     baseURL,
		tokenID:     tokenID,
		tokenSecret: tokenSecret,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

// apiResponse wraps the Proxmox response envelope {"data": ...}.
type apiResponse struct {
	Data json.RawMessage `json:"data"`
}

// Get performs a GET request and unmarshals the "data" field into dest.
func (c *Client) Get(ctx context.Context, path string, dest any) error {
	return c.do(ctx, http.MethodGet, path, dest)
}

// Post performs a POST request and unmarshals the "data" field into dest.
func (c *Client) Post(ctx context.Context, path string, dest any) error {
	return c.do(ctx, http.MethodPost, path, dest)
}

// Delete performs a DELETE request and unmarshals the "data" field into dest.
func (c *Client) Delete(ctx context.Context, path string, dest any) error {
	return c.do(ctx, http.MethodDelete, path, dest)
}

// MonitorCommand runs a QEMU monitor (HMP) command against a VM and returns its
// text output. Used for low-level actions like sendkey or screendump.
func (c *Client) MonitorCommand(ctx context.Context, node string, vmid int, command string) (string, error) {
	var out string
	path := fmt.Sprintf("/nodes/%s/qemu/%d/monitor", node, vmid)
	if err := c.PostForm(ctx, path, url.Values{"command": {command}}, &out); err != nil {
		return "", err
	}
	return out, nil
}

func (c *Client) do(ctx context.Context, method, path string, dest any) error {
	url := c.baseURL + "/api2/json" + path

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s %s failed: %w", method, path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Proxmox API error: %s %s returned %d: %s", method, path, resp.StatusCode, string(body))
	}

	if dest == nil {
		return nil
	}

	var envelope apiResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("parsing response JSON: %w", err)
	}

	if err := json.Unmarshal(envelope.Data, dest); err != nil {
		return fmt.Errorf("parsing response data: %w", err)
	}

	return nil
}

// PostForm performs a POST with a form-urlencoded body and unmarshals "data" into dest.
func (c *Client) PostForm(ctx context.Context, path string, body url.Values, dest any) error {
	return c.doForm(ctx, http.MethodPost, path, body, dest)
}

// PutForm performs a PUT with a form-urlencoded body and unmarshals "data" into dest.
func (c *Client) PutForm(ctx context.Context, path string, body url.Values, dest any) error {
	return c.doForm(ctx, http.MethodPut, path, body, dest)
}

func (c *Client) doForm(ctx context.Context, method, path string, body url.Values, dest any) error {
	url := c.baseURL + "/api2/json" + path

	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body.Encode()))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s %s failed: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Proxmox API error: %s %s returned %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	if dest == nil {
		return nil
	}

	var envelope apiResponse
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("parsing response JSON: %w", err)
	}

	if err := json.Unmarshal(envelope.Data, dest); err != nil {
		return fmt.Errorf("parsing response data: %w", err)
	}

	return nil
}

// Upload streams data as a multipart file upload to a storage's upload endpoint and
// returns the resulting task UPID. content is the Proxmox content type (e.g. "iso").
func (c *Client) Upload(ctx context.Context, node, storage, content, filename string, data []byte) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("content", content); err != nil {
		return "", fmt.Errorf("writing content field: %w", err)
	}
	fw, err := mw.CreateFormFile("filename", filename)
	if err != nil {
		return "", fmt.Errorf("creating file field: %w", err)
	}
	if _, err := fw.Write(data); err != nil {
		return "", fmt.Errorf("writing file data: %w", err)
	}
	if err := mw.Close(); err != nil {
		return "", fmt.Errorf("finalizing multipart body: %w", err)
	}

	path := fmt.Sprintf("/nodes/%s/storage/%s/upload", node, storage)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api2/json"+path, &buf)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload to %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Proxmox API error: POST %s returned %d: %s", path, resp.StatusCode, string(respBody))
	}

	var envelope apiResponse
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return "", fmt.Errorf("parsing response JSON: %w", err)
	}
	var upid string
	if err := json.Unmarshal(envelope.Data, &upid); err != nil {
		return "", fmt.Errorf("parsing response data: %w", err)
	}
	return upid, nil
}

// GetRaw performs a GET request and returns the raw "data" field as a JSON string.
func (c *Client) GetRaw(ctx context.Context, path string) (string, error) {
	url := c.baseURL + "/api2/json" + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request to GET %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Proxmox API error: GET %s returned %d: %s", path, resp.StatusCode, string(body))
	}

	var envelope apiResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("parsing response JSON: %w", err)
	}

	return string(envelope.Data), nil
}

// DoRaw performs an HTTP request and returns the status code and raw body.
// Used by the raw API tool for exploring the Proxmox API.
func (c *Client) DoRaw(ctx context.Context, method, path, formBody string) (int, string, error) {
	url := c.baseURL + "/api2/json" + path

	var bodyReader io.Reader
	if formBody != "" {
		bodyReader = strings.NewReader(formBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return 0, "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))
	if formBody != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("request to %s %s failed: %w", method, path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, "", fmt.Errorf("reading response body: %w", err)
	}

	return resp.StatusCode, string(body), nil
}

// ResolveNode finds the node hosting a given VMID by querying /cluster/resources.
func (c *Client) ResolveNode(ctx context.Context, vmid int) (string, error) {
	var resources []struct {
		Node string `json:"node"`
		VMID int    `json:"vmid"`
		Type string `json:"type"`
	}
	if err := c.Get(ctx, "/cluster/resources", &resources); err != nil {
		return "", fmt.Errorf("resolving node for VMID %d: %w", vmid, err)
	}

	for _, r := range resources {
		if r.VMID == vmid && (r.Type == "qemu" || r.Type == "lxc") {
			return r.Node, nil
		}
	}

	return "", fmt.Errorf("VMID %d not found in cluster", vmid)
}
