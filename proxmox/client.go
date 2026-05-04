package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a Proxmox VE REST API client.
type Client struct {
	baseURL    string
	tokenID    string
	tokenSecret string
	httpClient *http.Client
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

// Put performs a PUT request and unmarshals the "data" field into dest.
func (c *Client) Put(ctx context.Context, path string, dest any) error {
	return c.do(ctx, http.MethodPut, path, dest)
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
