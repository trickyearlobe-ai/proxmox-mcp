package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// termProxyTicket is the response from POST .../termproxy.
type termProxyTicket struct {
	Ticket string      `json:"ticket"`
	Port   json.Number `json:"port"`
	User   string      `json:"user"`
}

// SerialConsoleExchange opens a one-shot serial-console session to a QEMU VM via
// Proxmox's term-proxy, optionally writes input (keystrokes), then captures terminal
// output until the stream is idle for idleGap or the overall window elapses, and
// returns the captured text.
//
// The VM must have a serial device configured (e.g. serial0: socket). This speaks
// Proxmox's term-proxy framing over the vncwebsocket endpoint.
func (c *Client) SerialConsoleExchange(ctx context.Context, node string, vmid int, input string, window, idleGap time.Duration) (string, error) {
	// 1. Acquire a term-proxy ticket.
	var tkt termProxyTicket
	if err := c.Post(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/termproxy", node, vmid), &tkt); err != nil {
		return "", fmt.Errorf("requesting term proxy (is a serial device configured?): %w", err)
	}

	// 2. Build the websocket URL (https->wss) with the proxy port + ticket.
	wsURL := strings.Replace(c.baseURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL += fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/vncwebsocket?port=%s&vncticket=%s",
		node, vmid, tkt.Port.String(), url.QueryEscape(tkt.Ticket))

	// 3. Dial, authenticating the upgrade with the API token. Reuse the client's
	// transport (and its TLS config) but without the 30s request timeout.
	dialClient := &http.Client{Transport: c.httpClient.Transport}
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPClient:   dialClient,
		HTTPHeader:   http.Header{"Authorization": {fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret)}},
		Subprotocols: []string{"binary"},
	})
	if err != nil {
		return "", fmt.Errorf("opening console websocket: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	conn.SetReadLimit(8 << 20)

	// 4. Term-proxy auth handshake: first message is "user:ticket\n".
	authCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := conn.Write(authCtx, websocket.MessageBinary, []byte(tkt.User+":"+tkt.Ticket+"\n")); err != nil {
		return "", fmt.Errorf("sending console auth: %w", err)
	}
	// Nudge the terminal to a known size so a getty repaints.
	_ = conn.Write(authCtx, websocket.MessageBinary, []byte("1:120:40:"))

	// 5. Optionally send input as a "0:<len>:<data>" frame.
	if input != "" {
		frame := fmt.Sprintf("0:%d:%s", len(input), input)
		if err := conn.Write(authCtx, websocket.MessageBinary, []byte(frame)); err != nil {
			return "", fmt.Errorf("sending console input: %w", err)
		}
	}

	// 6. Capture output until idle for idleGap, or the overall window elapses.
	deadline := time.Now().Add(window)
	var buf strings.Builder
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		readCtx, rcancel := context.WithTimeout(ctx, minDuration(idleGap, remaining))
		_, data, err := conn.Read(readCtx)
		rcancel()
		if err != nil {
			// Idle gap or window elapsed (DeadlineExceeded), or peer closed: stop.
			break
		}
		// Skip the term-proxy "OK" auth ack and keepalive pings.
		s := string(data)
		if s == "OK" {
			continue
		}
		buf.Write(data)
	}

	out := buf.String()
	if out == "" {
		return "", nil
	}
	return out, nil
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
