package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/ivanpalumbo/lokifix/internal/auth"
	lokicrypto "github.com/ivanpalumbo/lokifix/internal/crypto"
	"github.com/ivanpalumbo/lokifix/internal/protocol"
)

// ConnectionInfo holds details about the currently connected remote agent.
type ConnectionInfo struct {
	Hostname    string `json:"hostname"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	ConnectedAt time.Time `json:"connected_at"`
}

// Server is the WebSocket server that runs on the operator's machine.
type Server struct {
	authMgr    *auth.Manager
	listener   net.Listener
	httpServer *http.Server

	mu        sync.Mutex
	conn      *websocket.Conn
	agentCtx  context.Context
	agentInfo *ConnectionInfo
	cipher    *lokicrypto.Cipher

	// Pending requests waiting for responses
	pending   map[string]chan protocol.Envelope
	pendingMu sync.Mutex

	// Callbacks
	OnAgentConnected    func(hostname string)
	OnAgentDisconnected func()
}

// NewServer creates a new WebSocket server.
func NewServer(authMgr *auth.Manager) *Server {
	return &Server{
		authMgr: authMgr,
		pending: make(map[string]chan protocol.Envelope),
	}
}

// Start begins listening on a random available port.
func (s *Server) Start(ctx context.Context) (int, error) {
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return 0, fmt.Errorf("listen: %w", err)
	}
	s.listener = listener

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	s.httpServer = &http.Server{Handler: mux}

	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	return port, nil
}

// Port returns the listening port.
func (s *Server) Port() int {
	if s.listener == nil {
		return 0
	}
	return s.listener.Addr().(*net.TCPAddr).Port
}

// IsConnected returns true if a remote agent is connected.
func (s *Server) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn != nil
}

// GetConnectionInfo returns info about the connected agent, or nil if none.
func (s *Server) GetConnectionInfo() *ConnectionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.agentInfo == nil {
		return nil
	}
	info := *s.agentInfo
	return &info
}

// SendRequest sends a request to the remote agent and waits for a response.
func (s *Server) SendRequest(ctx context.Context, id string, req protocol.Request) (protocol.Response, error) {
	env, err := protocol.NewEnvelope(protocol.TypeRequest, id, req)
	if err != nil {
		return protocol.Response{}, err
	}

	data, err := json.Marshal(env)
	if err != nil {
		return protocol.Response{}, err
	}

	// Create response channel before sending
	respCh := make(chan protocol.Envelope, 1)
	s.pendingMu.Lock()
	s.pending[id] = respCh
	s.pendingMu.Unlock()

	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
	}()

	// Encrypt if cipher is available
	msgType := websocket.MessageText
	if s.cipher != nil {
		encrypted, err := s.cipher.Encrypt(data)
		if err != nil {
			return protocol.Response{}, fmt.Errorf("encrypt: %w", err)
		}
		data = encrypted
		msgType = websocket.MessageBinary
	}

	// Hold lock during connection check and write to prevent race
	s.mu.Lock()
	conn := s.conn
	if conn == nil {
		s.mu.Unlock()
		return protocol.Response{}, fmt.Errorf("no agent connected")
	}
	writeErr := conn.Write(ctx, msgType, data)
	s.mu.Unlock()

	if writeErr != nil {
		return protocol.Response{}, fmt.Errorf("write: %w", writeErr)
	}

	select {
	case <-ctx.Done():
		return protocol.Response{}, ctx.Err()
	case respEnv := <-respCh:
		var resp protocol.Response
		if err := json.Unmarshal(respEnv.Payload, &resp); err != nil {
			return protocol.Response{}, fmt.Errorf("unmarshal response: %w", err)
		}
		return resp, nil
	}
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"}, // Agents connect from various origins; auth is token-based
	})
	if err != nil {
		log.Printf("websocket accept: %v", err)
		return
	}

	conn.SetReadLimit(64 * 1024 * 1024) // 64MB for large file transfers

	ctx := r.Context()

	// Read auth handshake (5 second timeout)
	authCtx, authCancel := context.WithTimeout(ctx, 5*time.Second)
	defer authCancel()

	_, data, err := conn.Read(authCtx)
	if err != nil {
		conn.Close(websocket.StatusPolicyViolation, "auth timeout")
		return
	}

	var env protocol.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		conn.Close(websocket.StatusPolicyViolation, "invalid message")
		return
	}

	var handshake protocol.AuthHandshake
	if err := json.Unmarshal(env.Payload, &handshake); err != nil {
		conn.Close(websocket.StatusPolicyViolation, "invalid handshake")
		return
	}

	// Try session token first (reconnection), then one-time code token
	authenticated := false
	if handshake.SessionToken != "" {
		authenticated = s.authMgr.ValidateSessionToken(handshake.SessionToken)
	}
	if !authenticated {
		authenticated = s.authMgr.ValidateToken(handshake.Token)
	}

	if !authenticated {
		result := protocol.AuthResult{Accepted: false, Message: "invalid or expired token"}
		respEnv, _ := protocol.NewEnvelope(protocol.TypeResponse, env.ID, result)
		respData, _ := json.Marshal(respEnv)
		conn.Write(ctx, websocket.MessageText, respData)
		conn.Close(websocket.StatusPolicyViolation, "auth failed")
		return
	}

	// Generate session token for reconnection
	sessionToken, _ := s.authMgr.GenerateSessionToken()

	// Auth success — send result before enabling encryption
	result := protocol.AuthResult{Accepted: true, Message: "connected", SessionToken: sessionToken}
	respEnv, _ := protocol.NewEnvelope(protocol.TypeResponse, env.ID, result)
	respData, _ := json.Marshal(respEnv)
	conn.Write(ctx, websocket.MessageText, respData)

	// Derive encryption key from the token that was used for auth
	authSecret := handshake.Token
	if handshake.SessionToken != "" {
		authSecret = handshake.SessionToken
	}
	e2eCipher, err := lokicrypto.NewCipherFromSecret(authSecret)
	if err != nil {
		log.Printf("e2e cipher setup failed: %v", err)
		conn.Close(websocket.StatusInternalError, "crypto setup failed")
		return
	}

	s.mu.Lock()
	s.conn = conn
	s.agentCtx = ctx
	s.cipher = e2eCipher
	s.agentInfo = &ConnectionInfo{
		Hostname:    handshake.Hostname,
		OS:          handshake.OS,
		Arch:        handshake.Arch,
		ConnectedAt: time.Now(),
	}
	s.mu.Unlock()

	if s.OnAgentConnected != nil {
		s.OnAgentConnected(handshake.Hostname)
	}

	// Read loop
	s.readLoop(ctx, conn)

	s.mu.Lock()
	s.conn = nil
	s.cipher = nil
	s.agentInfo = nil
	s.mu.Unlock()

	if s.OnAgentDisconnected != nil {
		s.OnAgentDisconnected()
	}
}

func (s *Server) readLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		// Decrypt binary messages
		if msgType == websocket.MessageBinary && s.cipher != nil {
			data, err = s.cipher.Decrypt(data)
			if err != nil {
				log.Printf("e2e decrypt error: %v", err)
				continue
			}
		}

		var env protocol.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}

		switch env.Type {
		case protocol.TypeResponse:
			s.pendingMu.Lock()
			ch, ok := s.pending[env.ID]
			s.pendingMu.Unlock()
			if ok {
				ch <- env
			}
		case protocol.TypePing:
			pong, _ := protocol.NewEnvelope(protocol.TypePong, env.ID, nil)
			pongData, _ := json.Marshal(pong)
			pongMsgType := websocket.MessageText
			if s.cipher != nil {
				if enc, err := s.cipher.Encrypt(pongData); err == nil {
					pongData = enc
					pongMsgType = websocket.MessageBinary
				}
			}
			conn.Write(ctx, pongMsgType, pongData)
		}
	}
}
