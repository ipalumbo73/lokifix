package mcp

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	writer    io.Writer
	writerMu  sync.Mutex
}

// NewMCPServer creates a new MCP server that proxies to the remote agent.
func NewMCPServer(wsServer *transport.Server, logger *audit.Logger) *MCPServer {
	return &MCPServer{wsServer: wsServer, Logger: logger}
}

// NotifyAgentConnected sends an MCP log notification when a remote agent connects.
func (s *MCPServer) NotifyAgentConnected(hostname string) {
	s.sendNotification("notifications/message", map[string]any{
		"level":  "info",
		"logger": "lokifix",
		"data":   fmt.Sprintf("◈ LokiFix — Remote agent connected: %s", hostname),
	})
}

// NotifyAgentDisconnected sends an MCP log notification when a remote agent disconnects.
func (s *MCPServer) NotifyAgentDisconnected() {
	s.sendNotification("notifications/message", map[string]any{
		"level":  "warning",
		"logger": "lokifix",
		"data":   "◈ LokiFix — Remote agent disconnected",
	})
}

func (s *MCPServer) sendNotification(method string, params any) {
	s.writerMu.Lock()
	defer s.writerMu.Unlock()

	if s.writer == nil {
		return
	}

	paramsData, _ := json.Marshal(params)
	msg := jsonrpcMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsData,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	data = append(data, '\n')
	s.writer.Write(data)
}

// Run starts the MCP server, reading from stdin and writing to stdout.
func (s *MCPServer) Run(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	s.writerMu.Lock()
	s.writer = writer
	s.writerMu.Unlock()

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
			"tools":   map[string]any{},
			"logging": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "lokifix-remote",
			"version": "1.1.0",
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
					"command":     map[string]any{"type": "string", "description": "The command to execute"},
					"shell":       map[string]any{"type": "string", "enum": []string{"powershell", "cmd"}, "description": "Shell to use (default: powershell)"},
					"timeout":     map[string]any{"type": "integer", "description": "Timeout in seconds (default: 120)"},
					"description": map[string]any{"type": "string", "description": "Human-readable description of what this command does (for audit logging)"},
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
			"description": "Edit a file on the remote machine by replacing a string. By default old_string must be unique; use replace_all to replace every occurrence.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":        map[string]any{"type": "string", "description": "Absolute path to the file"},
					"old_string":  map[string]any{"type": "string", "description": "Exact string to find"},
					"new_string":  map[string]any{"type": "string", "description": "Replacement string"},
					"replace_all": map[string]any{"type": "boolean", "description": "Replace all occurrences (default: false, requires unique match)"},
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
			"description": "Delete a file or directory (recursively) on the remote machine. Always requires user confirmation.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Path to file or directory to delete"},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "remote_glob",
			"description": "Find files matching a glob pattern on the remote machine. Supports ** for recursive matching (e.g. src/**/*.go). Results sorted by modification time.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{"type": "string", "description": "Glob pattern (e.g. *.log, src/**/*.go, **/*.txt)"},
					"path":    map[string]any{"type": "string", "description": "Base directory to search in (default: current dir)"},
				},
				"required": []string{"pattern"},
			},
		},
		{
			"name":        "remote_grep",
			"description": "Search for a regex pattern in files on the remote machine. Supports multiple output modes, context lines, case-insensitive and multiline matching.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern":          map[string]any{"type": "string", "description": "Regex pattern to search for"},
					"path":             map[string]any{"type": "string", "description": "File or directory to search in (default: current dir)"},
					"glob":             map[string]any{"type": "string", "description": "Glob pattern to filter files (e.g. *.js, *.{ts,tsx})"},
					"type":             map[string]any{"type": "string", "description": "File type filter: go, js, ts, py, java, rust, c, cpp, etc."},
					"output_mode":      map[string]any{"type": "string", "enum": []string{"content", "files_with_matches", "count"}, "description": "Output mode (default: content)"},
					"case_insensitive": map[string]any{"type": "boolean", "description": "Case-insensitive search"},
					"context_before":   map[string]any{"type": "integer", "description": "Lines to show before each match (-B)"},
					"context_after":    map[string]any{"type": "integer", "description": "Lines to show after each match (-A)"},
					"context":          map[string]any{"type": "integer", "description": "Lines to show before and after each match (-C)"},
					"head_limit":       map[string]any{"type": "integer", "description": "Max results to return (default: 250)"},
					"multiline":        map[string]any{"type": "boolean", "description": "Enable multiline matching where . matches newlines"},
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
		{
			"name":        "remote_file_upload",
			"description": "Upload a file from the operator machine to the remote machine. Reads a local file, encodes it as base64, and writes it on the remote. Supports binary files up to 50MB.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"local_path":  map[string]any{"type": "string", "description": "Absolute path of the file on the OPERATOR machine to upload"},
					"remote_path": map[string]any{"type": "string", "description": "Absolute path where the file will be written on the REMOTE machine"},
					"overwrite":   map[string]any{"type": "boolean", "description": "Overwrite if file already exists on remote (default: false)"},
				},
				"required": []string{"local_path", "remote_path"},
			},
		},
		{
			"name":        "remote_file_download",
			"description": "Download a file from the remote machine to the operator machine. Reads the remote file, encodes it as base64, and writes it locally. Supports binary files up to 50MB.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"remote_path": map[string]any{"type": "string", "description": "Absolute path of the file on the REMOTE machine to download"},
					"local_path":  map[string]any{"type": "string", "description": "Absolute path where the file will be saved on the OPERATOR machine"},
				},
				"required": []string{"remote_path", "local_path"},
			},
		},
		{
			"name":        "remote_connection_status",
			"description": "Check the connection status of the remote agent. Returns hostname, OS, architecture, connection time, and duration.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{},
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

	// Handle local tools (no remote agent needed)
	if params.Name == "remote_connection_status" {
		return s.handleConnectionStatus(msg.ID)
	}

	if !s.wsServer.IsConnected() {
		return s.toolResult(msg.ID, false, "Error: no remote agent connected. Wait for the agent to connect.")
	}

	// Handle file transfer tools (require local I/O on operator side)
	if params.Name == "remote_file_upload" {
		return s.handleFileUpload(ctx, msg.ID, params.Arguments)
	}
	if params.Name == "remote_file_download" {
		return s.handleFileDownload(ctx, msg.ID, params.Arguments)
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

const transferChunkSize = 1 * 1024 * 1024 // 1MB chunks

// handleFileUpload reads a local file, splits it into chunks, and sends them to the remote agent with progress notifications.
func (s *MCPServer) handleFileUpload(ctx context.Context, id any, arguments json.RawMessage) jsonrpcMessage {
	var args struct {
		LocalPath  string `json:"local_path"`
		RemotePath string `json:"remote_path"`
		Overwrite  bool   `json:"overwrite"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.toolResult(id, false, "invalid params: "+err.Error())
	}

	// Read local file
	data, err := os.ReadFile(args.LocalPath)
	if err != nil {
		return s.toolResult(id, false, fmt.Sprintf("failed to read local file: %v", err))
	}

	const maxSize = 50 * 1024 * 1024
	if len(data) > maxSize {
		return s.toolResult(id, false, fmt.Sprintf("file too large: %d bytes (max 50MB)", len(data)))
	}

	totalSize := len(data)
	totalChunks := (totalSize + transferChunkSize - 1) / transferChunkSize
	if totalChunks == 0 {
		totalChunks = 1
	}

	s.sendProgressNotification("upload", args.LocalPath, args.RemotePath, 0, totalChunks, int64(totalSize))

	if s.Logger != nil {
		s.Logger.Log("SEND_"+protocol.ToolFileUpload, fmt.Sprintf("%s -> %s (%d bytes, %d chunks)", args.LocalPath, args.RemotePath, totalSize, totalChunks), "OK")
	}

	for i := 0; i < totalChunks; i++ {
		start := i * transferChunkSize
		end := start + transferChunkSize
		if end > totalSize {
			end = totalSize
		}
		chunk := data[start:end]

		chunkParams := protocol.FileUploadChunkParams{
			Path:          args.RemotePath,
			ChunkIndex:    i,
			TotalChunks:   totalChunks,
			TotalSize:     int64(totalSize),
			ContentBase64: base64.StdEncoding.EncodeToString(chunk),
			Overwrite:     args.Overwrite,
		}
		paramsData, _ := json.Marshal(chunkParams)

		reqID := fmt.Sprintf("req-%d", s.requestID.Add(1))
		req := protocol.Request{
			Tool:   protocol.ToolFileUploadChunk,
			Params: paramsData,
		}

		resp, err := s.wsServer.SendRequest(ctx, reqID, req)
		if err != nil {
			if s.Logger != nil {
				s.Logger.Log("RECV_"+protocol.ToolFileUploadChunk, fmt.Sprintf("chunk %d failed: %v", i, err), "ERROR")
			}
			return s.toolResult(id, false, fmt.Sprintf("upload failed at chunk %d/%d: %v", i+1, totalChunks, err))
		}
		if !resp.Success {
			return s.toolResult(id, false, fmt.Sprintf("upload failed at chunk %d/%d: %s", i+1, totalChunks, resp.Error))
		}

		s.sendProgressNotification("upload", args.LocalPath, args.RemotePath, i+1, totalChunks, int64(totalSize))
	}

	if s.Logger != nil {
		s.Logger.Log("RECV_"+protocol.ToolFileUpload, fmt.Sprintf("complete: %s (%d bytes)", args.RemotePath, totalSize), "OK")
	}

	return s.toolResult(id, true, fmt.Sprintf("File uploaded: %s -> %s (%s, %d chunks)",
		args.LocalPath, args.RemotePath, formatBytes(int64(totalSize)), totalChunks))
}

// handleFileDownload requests file chunks from the remote agent, assembles them, and saves locally with progress notifications.
func (s *MCPServer) handleFileDownload(ctx context.Context, id any, arguments json.RawMessage) jsonrpcMessage {
	var args struct {
		RemotePath string `json:"remote_path"`
		LocalPath  string `json:"local_path"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.toolResult(id, false, "invalid params: "+err.Error())
	}

	if s.Logger != nil {
		s.Logger.Log("SEND_"+protocol.ToolFileDownload, args.RemotePath, "OK")
	}

	// First chunk to get total file size
	firstChunkResult, err := s.requestDownloadChunk(ctx, args.RemotePath, 0, transferChunkSize)
	if err != nil {
		return s.toolResult(id, false, fmt.Sprintf("download failed: %v", err))
	}

	totalSize := firstChunkResult.TotalSize
	totalChunks := int((totalSize + int64(transferChunkSize) - 1) / int64(transferChunkSize))
	if totalChunks == 0 {
		totalChunks = 1
	}

	s.sendProgressNotification("download", args.RemotePath, args.LocalPath, 0, totalChunks, totalSize)

	// Decode first chunk
	firstData, err := base64.StdEncoding.DecodeString(firstChunkResult.ContentBase64)
	if err != nil {
		return s.toolResult(id, false, "failed to decode first chunk: "+err.Error())
	}

	// Create local file
	dir := filepath.Dir(args.LocalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return s.toolResult(id, false, "failed to create local directory: "+err.Error())
	}
	if err := os.WriteFile(args.LocalPath, firstData, 0644); err != nil {
		return s.toolResult(id, false, "failed to write local file: "+err.Error())
	}

	s.sendProgressNotification("download", args.RemotePath, args.LocalPath, 1, totalChunks, totalSize)

	// Request remaining chunks
	for i := 1; i < totalChunks; i++ {
		offset := int64(i) * int64(transferChunkSize)

		chunkResult, err := s.requestDownloadChunk(ctx, args.RemotePath, offset, transferChunkSize)
		if err != nil {
			return s.toolResult(id, false, fmt.Sprintf("download failed at chunk %d/%d: %v", i+1, totalChunks, err))
		}

		chunkData, err := base64.StdEncoding.DecodeString(chunkResult.ContentBase64)
		if err != nil {
			return s.toolResult(id, false, fmt.Sprintf("failed to decode chunk %d: %v", i+1, err))
		}

		f, err := os.OpenFile(args.LocalPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return s.toolResult(id, false, fmt.Sprintf("failed to append chunk %d: %v", i+1, err))
		}
		_, err = f.Write(chunkData)
		f.Close()
		if err != nil {
			return s.toolResult(id, false, fmt.Sprintf("failed to write chunk %d: %v", i+1, err))
		}

		s.sendProgressNotification("download", args.RemotePath, args.LocalPath, i+1, totalChunks, totalSize)
	}

	if s.Logger != nil {
		s.Logger.Log("RECV_"+protocol.ToolFileDownload, fmt.Sprintf("%s -> %s (%d bytes)", args.RemotePath, args.LocalPath, totalSize), "OK")
	}

	return s.toolResult(id, true, fmt.Sprintf("File downloaded: %s -> %s (%s, %d chunks)",
		args.RemotePath, args.LocalPath, formatBytes(totalSize), totalChunks))
}

// requestDownloadChunk sends a single chunk download request to the remote agent.
func (s *MCPServer) requestDownloadChunk(ctx context.Context, remotePath string, offset int64, chunkSize int) (protocol.FileDownloadChunkResult, error) {
	chunkParams := protocol.FileDownloadChunkParams{
		Path:      remotePath,
		Offset:    offset,
		ChunkSize: chunkSize,
	}
	paramsData, _ := json.Marshal(chunkParams)

	reqID := fmt.Sprintf("req-%d", s.requestID.Add(1))
	req := protocol.Request{
		Tool:   protocol.ToolFileDownloadChunk,
		Params: paramsData,
	}

	resp, err := s.wsServer.SendRequest(ctx, reqID, req)
	if err != nil {
		return protocol.FileDownloadChunkResult{}, err
	}
	if !resp.Success {
		return protocol.FileDownloadChunkResult{}, fmt.Errorf("%s", resp.Error)
	}

	respData, err := json.Marshal(resp.Data)
	if err != nil {
		return protocol.FileDownloadChunkResult{}, fmt.Errorf("marshal response: %w", err)
	}

	var result protocol.FileDownloadChunkResult
	if err := json.Unmarshal(respData, &result); err != nil {
		return protocol.FileDownloadChunkResult{}, fmt.Errorf("unmarshal chunk result: %w", err)
	}

	return result, nil
}

// sendProgressNotification sends a transfer progress notification via MCP.
func (s *MCPServer) sendProgressNotification(direction, src, dst string, current, total int, totalBytes int64) {
	pct := 0
	if total > 0 {
		pct = current * 100 / total
	}

	// Build a visual progress bar
	barLen := 20
	filled := barLen * pct / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)

	var msg string
	if current == 0 {
		msg = fmt.Sprintf("📦 %s starting: %s -> %s (%s)",
			direction, src, dst, formatBytes(totalBytes))
	} else if current == total {
		msg = fmt.Sprintf("✅ %s complete: %s -> %s (%s) [%s] 100%%",
			direction, src, dst, formatBytes(totalBytes), bar)
	} else {
		msg = fmt.Sprintf("📤 %s [%s] %d%% — chunk %d/%d (%s)",
			direction, bar, pct, current, total, formatBytes(totalBytes))
	}

	s.sendNotification("notifications/message", map[string]any{
		"level":  "info",
		"logger": "lokifix-transfer",
		"data":   msg,
	})
}

// formatBytes formats bytes to human-readable string.
func formatBytes(b int64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func (s *MCPServer) handleConnectionStatus(id any) jsonrpcMessage {
	info := s.wsServer.GetConnectionInfo()
	if info == nil {
		result := map[string]any{
			"connected": false,
			"message":   "No remote agent connected",
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		return s.toolResult(id, true, string(data))
	}

	duration := time.Since(info.ConnectedAt)
	result := map[string]any{
		"connected":    true,
		"hostname":     info.Hostname,
		"os":           info.OS,
		"arch":         info.Arch,
		"connected_at": info.ConnectedAt.Format("2006-01-02 15:04:05"),
		"duration":     formatDuration(duration),
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return s.toolResult(id, true, string(data))
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
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
