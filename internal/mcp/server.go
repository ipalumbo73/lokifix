package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/ivanpalumbo/lokifix/internal/audit"
	"github.com/ivanpalumbo/lokifix/internal/protocol"
	"github.com/ivanpalumbo/lokifix/internal/transport"
)

// JSONRPC message types for MCP protocol
type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const inactivityTimeout = 30 * time.Minute

// MCPServer implements the MCP protocol over stdio, proxying to the remote agent.
type MCPServer struct {
	wsServer  *transport.Server
	requestID atomic.Int64
	Logger    *audit.Logger
}

// NewMCPServer creates a new MCP server that proxies to the remote agent.
func NewMCPServer(wsServer *transport.Server, logger *audit.Logger) *MCPServer {
	return &MCPServer{wsServer: wsServer, Logger: logger}
}

// Run starts the MCP server, reading from stdin and writing to stdout.
func (s *MCPServer) Run(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read stdin: %w", err)
		}

		var msg jsonrpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("invalid json-rpc: %v", err)
			continue
		}

		var response jsonrpcMessage
		switch msg.Method {
		case "initialize":
			response = s.handleInitialize(msg)
		case "tools/list":
			response = s.handleToolsList(msg)
		case "tools/call":
			response = s.handleToolsCall(ctx, msg)
		case "notifications/initialized":
			continue // no response needed
		default:
			response = jsonrpcMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Error:   &jsonrpcError{Code: -32601, Message: "method not found: " + msg.Method},
			}
		}

		data, err := json.Marshal(response)
		if err != nil {
			log.Printf("marshal response: %v", err)
			continue
		}
		data = append(data, '\n')
		writer.Write(data)
	}
}

func (s *MCPServer) handleInitialize(msg jsonrpcMessage) jsonrpcMessage {
	result := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "lokifix-remote",
			"version": "1.0.0",
		},
	}

	data, _ := json.Marshal(result)
	return jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  data,
	}
}

func (s *MCPServer) handleToolsList(msg jsonrpcMessage) jsonrpcMessage {
	tools := []map[string]any{
		{
			"name":        "remote_shell",
			"description": "Execute a shell command on the remote Windows machine. Uses PowerShell by default. Use 'cmd' shell for CMD commands.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{"type": "string", "description": "The command to execute"},
					"shell":   map[string]any{"type": "string", "enum": []string{"powershell", "cmd"}, "description": "Shell to use (default: powershell)"},
					"timeout": map[string]any{"type": "integer", "description": "Timeout in seconds (default: 120)"},
				},
				"required": []string{"command"},
			},
		},
		{
			"name":        "remote_file_read",
			"description": "Read a file from the remote machine. Returns content with line numbers.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":   map[string]any{"type": "string", "description": "Absolute path to the file"},
					"offset": map[string]any{"type": "integer", "description": "Start from this line number"},
					"limit":  map[string]any{"type": "integer", "description": "Max lines to read (default: 2000)"},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "remote_file_write",
			"description": "Write content to a file on the remote machine. Creates directories if needed.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string", "description": "Absolute path to the file"},
					"content": map[string]any{"type": "string", "description": "Content to write"},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			"name":        "remote_file_edit",
			"description": "Edit a file on the remote machine by replacing a unique string.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":       map[string]any{"type": "string", "description": "Absolute path to the file"},
					"old_string": map[string]any{"type": "string", "description": "Exact string to find (must be unique)"},
					"new_string": map[string]any{"type": "string", "description": "Replacement string"},
				},
				"required": []string{"path", "old_string", "new_string"},
			},
		},
		{
			"name":        "remote_file_list",
			"description": "List files and directories at the given path on the remote machine.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Directory path to list"},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "remote_file_delete",
			"description": "Delete a file on the remote machine.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Path to delete"},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "remote_glob",
			"description": "Find files matching a glob pattern on the remote machine.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{"type": "string", "description": "Glob pattern (e.g. *.log, *.txt)"},
					"path":    map[string]any{"type": "string", "description": "Base directory (default: current dir)"},
				},
				"required": []string{"pattern"},
			},
		},
		{
			"name":        "remote_grep",
			"description": "Search for a text pattern in files on the remote machine.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{"type": "string", "description": "Text pattern to search for"},
					"path":    map[string]any{"type": "string", "description": "Directory to search in"},
					"glob":    map[string]any{"type": "string", "description": "Filter files by glob pattern"},
				},
				"required": []string{"pattern"},
			},
		},
		{
			"name":        "remote_sysinfo",
			"description": "Get system information from the remote machine (OS, CPU, memory, uptime).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "remote_processes",
			"description": "List running processes on the remote machine (top 50 by CPU).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "remote_services",
			"description": "List Windows services on the remote machine.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "remote_registry",
			"description": "Read a Windows registry key or value from the remote machine.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key":   map[string]any{"type": "string", "description": "Registry key path (e.g. HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion)"},
					"value": map[string]any{"type": "string", "description": "Specific value name (optional, reads all if omitted)"},
				},
				"required": []string{"key"},
			},
		},
		{
			"name":        "remote_netinfo",
			"description": "Get network interface information from the remote machine.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "remote_env_vars",
			"description": "Get all environment variables from the remote machine.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "remote_installed_software",
			"description": "List all installed software on the remote Windows machine.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "remote_event_log",
			"description": "Read Windows Event Log entries from the remote machine.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"log_name":  map[string]any{"type": "string", "enum": []string{"System", "Application", "Security"}, "description": "Event log name (default: System)"},
					"max_items": map[string]any{"type": "integer", "description": "Max entries to return (default: 50)"},
					"level":     map[string]any{"type": "string", "enum": []string{"Error", "Warning", "Information"}, "description": "Filter by severity level"},
				},
			},
		},
	}

	result := map[string]any{"tools": tools}
	data, _ := json.Marshal(result)
	return jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  data,
	}
}

func (s *MCPServer) handleToolsCall(ctx context.Context, msg jsonrpcMessage) jsonrpcMessage {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.errorResponse(msg.ID, -32602, "invalid params")
	}

	if !s.wsServer.IsConnected() {
		return s.toolResult(msg.ID, false, "Error: no remote agent connected. Wait for the agent to connect.")
	}

	// Map MCP tool names to protocol tool names
	toolMap := map[string]string{
		"remote_shell":       protocol.ToolShellExec,
		"remote_file_read":   protocol.ToolFileRead,
		"remote_file_write":  protocol.ToolFileWrite,
		"remote_file_edit":   protocol.ToolFileEdit,
		"remote_file_list":   protocol.ToolFileList,
		"remote_file_delete": protocol.ToolFileDelete,
		"remote_glob":        protocol.ToolGlob,
		"remote_grep":        protocol.ToolGrep,
		"remote_sysinfo":     protocol.ToolSysInfo,
		"remote_processes":   protocol.ToolProcesses,
		"remote_services":    protocol.ToolServices,
		"remote_registry":    protocol.ToolRegistry,
		"remote_netinfo":     protocol.ToolNetInfo,
		"remote_env_vars":    protocol.ToolEnvVars,
		"remote_installed_software": protocol.ToolInstalledSoftware,
		"remote_event_log":   protocol.ToolEventLog,
	}

	remoteTool, ok := toolMap[params.Name]
	if !ok {
		return s.toolResult(msg.ID, false, fmt.Sprintf("Unknown tool: %s", params.Name))
	}

	reqID := fmt.Sprintf("req-%d", s.requestID.Add(1))
	req := protocol.Request{
		Tool:   remoteTool,
		Params: params.Arguments,
	}

	// Log the outgoing request
	if s.Logger != nil {
		detail := fmt.Sprintf("%s args=%s", remoteTool, truncateStr(string(params.Arguments), 150))
		s.Logger.Log("SEND_"+remoteTool, detail, "OK")
	}

	resp, err := s.wsServer.SendRequest(ctx, reqID, req)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Log("RECV_"+remoteTool, err.Error(), "ERROR")
		}
		return s.toolResult(msg.ID, false, fmt.Sprintf("Remote agent error: %v", err))
	}

	// Log the response
	if s.Logger != nil {
		result := "OK"
		if !resp.Success {
			result = "ERROR"
		}
		detail := remoteTool
		if !resp.Success {
			detail += ": " + resp.Error
		}
		s.Logger.Log("RECV_"+remoteTool, detail, result)
	}

	if !resp.Success {
		return s.toolResult(msg.ID, false, resp.Error)
	}

	// Marshal data back to string for MCP
	dataStr, err := json.MarshalIndent(resp.Data, "", "  ")
	if err != nil {
		return s.toolResult(msg.ID, true, fmt.Sprintf("%v", resp.Data))
	}
	return s.toolResult(msg.ID, true, string(dataStr))
}

func (s *MCPServer) toolResult(id any, success bool, text string) jsonrpcMessage {
	content := []map[string]any{
		{"type": "text", "text": text},
	}
	result := map[string]any{
		"content": content,
		"isError": !success,
	}
	data, _ := json.Marshal(result)
	return jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  data,
	}
}

func (s *MCPServer) errorResponse(id any, code int, message string) jsonrpcMessage {
	return jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonrpcError{Code: code, Message: message},
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
