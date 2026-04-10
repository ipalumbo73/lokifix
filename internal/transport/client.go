package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/ivanpalumbo/lokifix/internal/protocol"
)

// RequestHandler processes incoming requests from the MCP server.
type RequestHandler func(ctx context.Context, req protocol.Request) protocol.Response

// Client is the WebSocket client that runs on the remote machine.
type Client struct {
	serverURL string
	token     string
	handler   RequestHandler

	mu   sync.Mutex
	conn *websocket.Conn
}

// NewClient creates a new WebSocket client.
func NewClient(serverURL, token string, handler RequestHandler) *Client {
	return &Client{
		serverURL: serverURL,
		token:     token,
		handler:   handler,
	}
}

// Connect establishes a connection to the operator's WebSocket server.
func (c *Client) Connect(ctx context.Context) error {
	wsURL := c.serverURL + "/ws"

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", wsURL, err)
	}

	conn.SetReadLimit(64 * 1024 * 1024) // 64MB

	// Send auth handshake
	hostname, _ := os.Hostname()
	handshake := protocol.AuthHandshake{
		Token:    c.token,
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
	}

	env, err := protocol.NewEnvelope(protocol.TypeResponse, "auth", handshake)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "")
		return fmt.Errorf("create handshake: %w", err)
	}

	data, err := json.Marshal(env)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "")
		return err
	}

	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		return fmt.Errorf("send handshake: %w", err)
	}

	// Wait for auth result
	authCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, respData, err := conn.Read(authCtx)
	if err != nil {
		return fmt.Errorf("read auth result: %w", err)
	}

	var respEnv protocol.Envelope
	if err := json.Unmarshal(respData, &respEnv); err != nil {
		return fmt.Errorf("parse auth result: %w", err)
	}

	var result protocol.AuthResult
	if err := json.Unmarshal(respEnv.Payload, &result); err != nil {
		return fmt.Errorf("parse auth payload: %w", err)
	}

	if !result.Accepted {
		conn.Close(websocket.StatusPolicyViolation, "")
		return fmt.Errorf("authentication rejected: %s", result.Message)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	return nil
}

// Run starts the main loop: reads requests, processes them, sends responses.
func (c *Client) Run(ctx context.Context) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	// Start ping ticker
	go c.pingLoop(ctx)

	// Worker pool: limit concurrent request handlers
	sem := make(chan struct{}, 20)

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		var env protocol.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			log.Printf("invalid message: %v", err)
			continue
		}

		switch env.Type {
		case protocol.TypeRequest:
			sem <- struct{}{}
			go func(e protocol.Envelope) {
				defer func() { <-sem }()
				c.handleRequest(ctx, e)
			}(env)
		case protocol.TypePong:
			// Pong received, connection is alive
		}
	}
}

// Close cleanly closes the connection.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close(websocket.StatusNormalClosure, "agent shutting down")
		c.conn = nil
	}
}

func (c *Client) handleRequest(ctx context.Context, env protocol.Envelope) {
	var req protocol.Request
	if err := json.Unmarshal(env.Payload, &req); err != nil {
		resp := protocol.Response{Success: false, Error: "invalid request payload"}
		c.writeMessage(ctx, env.ID, resp)
		return
	}

	resp := c.handler(ctx, req)
	c.writeMessage(ctx, env.ID, resp)
}

func (c *Client) writeMessage(ctx context.Context, id string, resp protocol.Response) {
	env, err := protocol.NewEnvelope(protocol.TypeResponse, id, resp)
	if err != nil {
		log.Printf("create response envelope: %v", err)
		return
	}

	data, err := json.Marshal(env)
	if err != nil {
		log.Printf("marshal response: %v", err)
		return
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return
	}

	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		log.Printf("write response: %v", err)
	}
}

func (c *Client) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ping, _ := protocol.NewEnvelope(protocol.TypePing, "ping", nil)
			data, _ := json.Marshal(ping)

			c.mu.Lock()
			conn := c.conn
			c.mu.Unlock()

			if conn == nil {
				return
			}
			if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
				return
			}
		}
	}
}
