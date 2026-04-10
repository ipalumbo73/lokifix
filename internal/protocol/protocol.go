package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// Message types for the JSON-RPC protocol between MCP server and remote agent.
const (
	TypeRequest      = "request"
	TypeResponse     = "response"
	TypePing         = "ping"
	TypePong         = "pong"
	TypeConfirmReq   = "confirm_request"   // Server asks agent user to confirm
	TypeConfirmResp  = "confirm_response"  // Agent user's decision
)

// Tool names exposed by the remote agent.
const (
	ToolShellExec  = "shell_exec"
	ToolFileRead   = "file_read"
	ToolFileWrite  = "file_write"
	ToolFileEdit   = "file_edit"
	ToolFileList   = "file_list"
	ToolFileDelete = "file_delete"
	ToolGlob       = "glob"
	ToolGrep       = "grep"
	ToolSysInfo    = "sys_info"
	ToolProcesses  = "processes"
	ToolServices   = "services"
	ToolRegistry   = "registry_read"
	ToolNetInfo    = "net_info"
	ToolEnvVars    = "env_vars"
	ToolInstalledSoftware = "installed_software"
	ToolEventLog     = "event_log"
	ToolFileUpload   = "file_upload"
	ToolFileDownload = "file_download"
)

// Envelope wraps all messages exchanged over the WebSocket.
type Envelope struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Timestamp int64           `json:"ts"`
	Payload   json.RawMessage `json:"payload"`
}

// NewEnvelope creates a new envelope with the given type and payload.
func NewEnvelope(msgType string, id string, payload any) (Envelope, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal payload: %w", err)
	}
	return Envelope{
		Type:      msgType,
		ID:        id,
		Timestamp: time.Now().UnixMilli(),
		Payload:   data,
	}, nil
}

// Request is sent from MCP server to the remote agent.
type Request struct {
	Tool   string          `json:"tool"`
	Params json.RawMessage `json:"params"`
}

// Response is sent from the remote agent back to MCP server.
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ConfirmRequest is sent to the agent to ask the user for approval.
type ConfirmRequest struct {
	Action  string `json:"action"`
	Detail  string `json:"detail"`
	Reason  string `json:"reason"`
}

// ConfirmResponse is the agent user's decision.
type ConfirmResponse struct {
	Approved bool   `json:"approved"`
	Message  string `json:"message,omitempty"`
}

// AuthHandshake is the first message the remote agent sends.
type AuthHandshake struct {
	Token        string `json:"token"`
	SessionToken string `json:"session_token,omitempty"` // For reconnection
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
}

// AuthResult is the server's response to the handshake.
type AuthResult struct {
	Accepted     bool   `json:"accepted"`
	Message      string `json:"message,omitempty"`
	SessionToken string `json:"session_token,omitempty"` // Issued on first auth for reconnection
}

// --- Tool parameter types ---

type ShellExecParams struct {
	Command     string `json:"command"`
	Shell       string `json:"shell,omitempty"`       // "powershell" (default), "cmd"
	Timeout     int    `json:"timeout,omitempty"`      // seconds, default 120
	Description string `json:"description,omitempty"`  // human-readable description for audit
}

type ShellExecResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

type FileReadParams struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type FileReadResult struct {
	Content  string `json:"content"`
	Size     int64  `json:"size"`
	Lines    int    `json:"lines"`
	Truncated bool  `json:"truncated"`
}

type FileWriteParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type FileEditParams struct {
	Path       string `json:"path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"` // replace all occurrences instead of requiring uniqueness
}

type FileListParams struct {
	Path string `json:"path"`
}

type FileListEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

type FileDeleteParams struct {
	Path string `json:"path"`
}

type GlobParams struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

type GrepParams struct {
	Pattern        string `json:"pattern"`
	Path           string `json:"path,omitempty"`
	Glob           string `json:"glob,omitempty"`
	Type           string `json:"type,omitempty"`             // file type filter: "go", "js", "py", etc.
	OutputMode     string `json:"output_mode,omitempty"`      // "content" (default), "files_with_matches", "count"
	CaseInsensitive bool  `json:"case_insensitive,omitempty"` // case-insensitive search
	ContextBefore  int    `json:"context_before,omitempty"`   // lines before match (-B)
	ContextAfter   int    `json:"context_after,omitempty"`    // lines after match (-A)
	Context        int    `json:"context,omitempty"`          // lines before and after (-C)
	HeadLimit      int    `json:"head_limit,omitempty"`       // max results (default: 250)
	Multiline      bool   `json:"multiline,omitempty"`        // enable multiline matching
}

type GrepMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

type GrepCountEntry struct {
	File  string `json:"file"`
	Count int    `json:"count"`
}

type SysInfoResult struct {
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Version      string `json:"version"`
	Arch         string `json:"arch"`
	CPUs         int    `json:"cpus"`
	MemoryTotalMB int64 `json:"memory_total_mb"`
	MemoryFreeMB  int64 `json:"memory_free_mb"`
	UptimeHours  float64 `json:"uptime_hours"`
}

type ProcessEntry struct {
	PID     int    `json:"pid"`
	Name    string `json:"name"`
	CPU     string `json:"cpu"`
	MemMB   string `json:"mem_mb"`
}

type ServiceEntry struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Start  string `json:"start_type"`
}

type RegistryReadParams struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

type NetInfoResult struct {
	Interfaces []NetInterface `json:"interfaces"`
	Connections int           `json:"connections"`
}

type NetInterface struct {
	Name    string   `json:"name"`
	Addrs   []string `json:"addrs"`
	Status  string   `json:"status"`
}

type EventLogParams struct {
	LogName  string `json:"log_name"`  // "System", "Application", "Security"
	MaxItems int    `json:"max_items,omitempty"`
	Level    string `json:"level,omitempty"` // "Error", "Warning", "Information"
}

type EventLogEntry struct {
	TimeCreated string `json:"time"`
	Level       string `json:"level"`
	Source      string `json:"source"`
	Message     string `json:"message"`
}

type InstalledSoftwareEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Vendor  string `json:"vendor"`
}

// FileUploadParams: operator sends base64 content to be written on the remote machine.
type FileUploadParams struct {
	Path          string `json:"path"`           // destination path on remote
	ContentBase64 string `json:"content_base64"` // file content encoded in base64
	Overwrite     bool   `json:"overwrite,omitempty"` // overwrite if exists (default: false)
}

// FileDownloadParams: operator requests a file from the remote machine.
type FileDownloadParams struct {
	Path string `json:"path"` // source path on remote
}

// FileDownloadResult: remote sends back the file as base64.
type FileDownloadResult struct {
	ContentBase64 string `json:"content_base64"` // file content encoded in base64
	Size          int64  `json:"size"`           // original file size in bytes
	Name          string `json:"name"`           // basename of the file
}
