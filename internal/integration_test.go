package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ivanpalumbo/lokifix/internal/agent"
	"github.com/ivanpalumbo/lokifix/internal/auth"
	"github.com/ivanpalumbo/lokifix/internal/protocol"
	"github.com/ivanpalumbo/lokifix/internal/transport"
)

func TestFullConnectionFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start server
	authMgr := auth.NewManager()
	server := transport.NewServer(authMgr)

	port, err := server.Start(ctx)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop()

	// Generate connection code
	cc, err := authMgr.GenerateCode()
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}

	serverURL := "ws://localhost:" + itoa(port)

	// Track connection
	connected := make(chan string, 1)
	server.OnAgentConnected = func(hostname string) {
		connected <- hostname
	}

	// Connect agent with handler (no logging in tests)
	handler := agent.NewHandler(agent.HandlerConfig{})
	client := transport.NewClient(serverURL, cc.Token, handler)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	go client.Run(ctx)

	// Wait for connection
	select {
	case hostname := <-connected:
		t.Logf("Agent connected: %s", hostname)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for agent connection")
	}

	// Test shell_exec
	params, _ := json.Marshal(protocol.ShellExecParams{
		Command: "echo hello-lokifix",
		Shell:   "cmd",
		Timeout: 10,
	})

	resp, err := server.SendRequest(ctx, "test-1", protocol.Request{
		Tool:   protocol.ToolShellExec,
		Params: params,
	})
	if err != nil {
		t.Fatalf("send request: %v", err)
	}

	if !resp.Success {
		t.Fatalf("shell_exec failed: %s", resp.Error)
	}

	// Verify output contains our string
	dataBytes, _ := json.Marshal(resp.Data)
	if !contains(string(dataBytes), "hello-lokifix") {
		t.Errorf("expected 'hello-lokifix' in output, got: %s", string(dataBytes))
	}

	// Test sysinfo
	resp2, err := server.SendRequest(ctx, "test-2", protocol.Request{
		Tool:   protocol.ToolSysInfo,
		Params: json.RawMessage("{}"),
	})
	if err != nil {
		t.Fatalf("sysinfo request: %v", err)
	}
	if !resp2.Success {
		t.Fatalf("sysinfo failed: %s", resp2.Error)
	}

	t.Logf("SysInfo: %+v", resp2.Data)
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
