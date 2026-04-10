package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ivanpalumbo/lokifix/internal/audit"
	"github.com/ivanpalumbo/lokifix/internal/executor"
	"github.com/ivanpalumbo/lokifix/internal/fileops"
	"github.com/ivanpalumbo/lokifix/internal/protocol"
	"github.com/ivanpalumbo/lokifix/internal/sysinfo"
)

// toolActionMap maps protocol tool names to audit action names.
var toolActionMap = map[string]string{
	protocol.ToolShellExec:        audit.ActionShellExec,
	protocol.ToolFileRead:         audit.ActionFileRead,
	protocol.ToolFileWrite:        audit.ActionFileWrite,
	protocol.ToolFileEdit:         audit.ActionFileEdit,
	protocol.ToolFileList:         audit.ActionFileList,
	protocol.ToolFileDelete:       audit.ActionFileDelete,
	protocol.ToolGlob:             audit.ActionGlob,
	protocol.ToolGrep:             audit.ActionGrep,
	protocol.ToolSysInfo:          audit.ActionSysInfo,
	protocol.ToolProcesses:        audit.ActionProcesses,
	protocol.ToolServices:         audit.ActionServices,
	protocol.ToolRegistry:         audit.ActionRegistry,
	protocol.ToolNetInfo:          audit.ActionNetInfo,
	protocol.ToolEnvVars:          audit.ActionEnvVars,
	protocol.ToolInstalledSoftware: audit.ActionInstalledSW,
	protocol.ToolEventLog:         audit.ActionEventLog,
	protocol.ToolFileUpload:        audit.ActionFileUpload,
	protocol.ToolFileDownload:      audit.ActionFileDownload,
	protocol.ToolFileUploadChunk:   audit.ActionFileUploadChunk,
	protocol.ToolFileDownloadChunk: audit.ActionFileDownloadChunk,
}

// HandlerConfig holds dependencies for the request handler.
type HandlerConfig struct {
	Logger         *audit.Logger
	ConfirmFunc    func(action, detail, reason string) bool // asks user for confirmation
}

// NewHandler creates a request handler with logging and confirmation support.
func NewHandler(cfg HandlerConfig) func(ctx context.Context, req protocol.Request) protocol.Response {
	return func(ctx context.Context, req protocol.Request) protocol.Response {
		action := toolActionMap[req.Tool]
		if action == "" {
			action = req.Tool
		}

		detail := extractDetail(req)

		// Check if dangerous and needs confirmation
		if needsConfirmation(req) {
			reason := getDangerReason(req)
			if cfg.ConfirmFunc != nil && !cfg.ConfirmFunc(action, detail, reason) {
				if cfg.Logger != nil {
					cfg.Logger.Log(audit.ActionDenied, detail, "DENIED")
				}
				return protocol.Response{
					Success: false,
					Error:   "Operazione negata dall'utente remoto: " + detail,
				}
			}
			if cfg.Logger != nil {
				cfg.Logger.Log(audit.ActionApproved, detail, "OK")
			}
		}

		// Execute the tool
		resp := dispatch(ctx, req)

		// Log the action
		if cfg.Logger != nil {
			result := "OK"
			if !resp.Success {
				result = "ERROR"
			}
			cfg.Logger.Log(action, detail, result)
		}

		return resp
	}
}

// dispatch routes the request to the appropriate handler.
func dispatch(ctx context.Context, req protocol.Request) protocol.Response {
	switch req.Tool {
	case protocol.ToolShellExec:
		return handleShellExec(ctx, req.Params)
	case protocol.ToolFileRead:
		return handleFileRead(req.Params)
	case protocol.ToolFileWrite:
		return handleFileWrite(req.Params)
	case protocol.ToolFileEdit:
		return handleFileEdit(req.Params)
	case protocol.ToolFileList:
		return handleFileList(req.Params)
	case protocol.ToolFileDelete:
		return handleFileDelete(req.Params)
	case protocol.ToolGlob:
		return handleGlob(req.Params)
	case protocol.ToolGrep:
		return handleGrep(req.Params)
	case protocol.ToolSysInfo:
		return handleSysInfo(ctx)
	case protocol.ToolProcesses:
		return handleProcesses(ctx)
	case protocol.ToolServices:
		return handleServices(ctx)
	case protocol.ToolRegistry:
		return handleRegistry(ctx, req.Params)
	case protocol.ToolNetInfo:
		return handleNetInfo()
	case protocol.ToolEnvVars:
		return handleEnvVars()
	case protocol.ToolInstalledSoftware:
		return handleInstalledSoftware(ctx)
	case protocol.ToolEventLog:
		return handleEventLog(ctx, req.Params)
	case protocol.ToolFileUpload:
		return handleFileUpload(req.Params)
	case protocol.ToolFileDownload:
		return handleFileDownload(req.Params)
	case protocol.ToolFileUploadChunk:
		return handleFileUploadChunk(req.Params)
	case protocol.ToolFileDownloadChunk:
		return handleFileDownloadChunk(req.Params)
	default:
		return protocol.Response{Success: false, Error: "unknown tool: " + req.Tool}
	}
}

// needsConfirmation checks if the request requires user confirmation.
func needsConfirmation(req protocol.Request) bool {
	switch req.Tool {
	case protocol.ToolShellExec:
		var p protocol.ShellExecParams
		if err := json.Unmarshal(req.Params, &p); err == nil {
			dangerous, _ := audit.IsDangerousCommand(p.Command)
			return dangerous
		}
	case protocol.ToolFileWrite, protocol.ToolFileEdit, protocol.ToolFileUpload:
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(req.Params, &p); err == nil {
			dangerous, _ := audit.IsDangerousFilePath(p.Path)
			return dangerous
		}
	case protocol.ToolFileUploadChunk:
		var p protocol.FileUploadChunkParams
		if err := json.Unmarshal(req.Params, &p); err == nil && p.ChunkIndex == 0 {
			dangerous, _ := audit.IsDangerousFilePath(p.Path)
			return dangerous
		}
	case protocol.ToolFileDelete:
		return true // Always confirm deletions
	}
	return false
}

// getDangerReason returns a human-readable reason for the danger.
func getDangerReason(req protocol.Request) string {
	switch req.Tool {
	case protocol.ToolShellExec:
		var p protocol.ShellExecParams
		if err := json.Unmarshal(req.Params, &p); err == nil {
			_, pattern := audit.IsDangerousCommand(p.Command)
			return "Comando contiene pattern pericoloso: " + pattern
		}
	case protocol.ToolFileWrite, protocol.ToolFileEdit, protocol.ToolFileUpload, protocol.ToolFileUploadChunk:
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(req.Params, &p); err == nil {
			_, pattern := audit.IsDangerousFilePath(p.Path)
			return "File in area protetta: " + pattern
		}
	case protocol.ToolFileDelete:
		return "Eliminazione file"
	}
	return "Operazione potenzialmente pericolosa"
}

// extractDetail extracts a human-readable detail from the request.
func extractDetail(req protocol.Request) string {
	switch req.Tool {
	case protocol.ToolShellExec:
		var p protocol.ShellExecParams
		if json.Unmarshal(req.Params, &p) == nil {
			shell := p.Shell
			if shell == "" {
				shell = "powershell"
			}
			detail := "[" + shell + "] " + p.Command
			if p.Description != "" {
				detail = p.Description + " | " + detail
			}
			return detail
		}
	case protocol.ToolFileRead:
		var p protocol.FileReadParams
		if json.Unmarshal(req.Params, &p) == nil {
			return p.Path
		}
	case protocol.ToolFileWrite:
		var p protocol.FileWriteParams
		if json.Unmarshal(req.Params, &p) == nil {
			return p.Path
		}
	case protocol.ToolFileEdit:
		var p protocol.FileEditParams
		if json.Unmarshal(req.Params, &p) == nil {
			return p.Path
		}
	case protocol.ToolFileList:
		var p protocol.FileListParams
		if json.Unmarshal(req.Params, &p) == nil {
			return p.Path
		}
	case protocol.ToolFileDelete:
		var p protocol.FileDeleteParams
		if json.Unmarshal(req.Params, &p) == nil {
			return p.Path
		}
	case protocol.ToolGlob:
		var p protocol.GlobParams
		if json.Unmarshal(req.Params, &p) == nil {
			return p.Pattern + " in " + p.Path
		}
	case protocol.ToolGrep:
		var p protocol.GrepParams
		if json.Unmarshal(req.Params, &p) == nil {
			return "'" + p.Pattern + "' in " + p.Path
		}
	case protocol.ToolFileUpload:
		var p protocol.FileUploadParams
		if json.Unmarshal(req.Params, &p) == nil {
			return "upload -> " + p.Path
		}
	case protocol.ToolFileDownload:
		var p protocol.FileDownloadParams
		if json.Unmarshal(req.Params, &p) == nil {
			return "download <- " + p.Path
		}
	case protocol.ToolFileUploadChunk:
		var p protocol.FileUploadChunkParams
		if json.Unmarshal(req.Params, &p) == nil {
			return fmt.Sprintf("upload chunk %d/%d -> %s", p.ChunkIndex+1, p.TotalChunks, p.Path)
		}
	case protocol.ToolFileDownloadChunk:
		var p protocol.FileDownloadChunkParams
		if json.Unmarshal(req.Params, &p) == nil {
			return fmt.Sprintf("download chunk offset=%d <- %s", p.Offset, p.Path)
		}
	case protocol.ToolRegistry:
		var p protocol.RegistryReadParams
		if json.Unmarshal(req.Params, &p) == nil {
			return p.Key
		}
	case protocol.ToolEventLog:
		var p protocol.EventLogParams
		if json.Unmarshal(req.Params, &p) == nil {
			return p.LogName
		}
	}
	return req.Tool
}

func handleShellExec(ctx context.Context, params json.RawMessage) protocol.Response {
	var p protocol.ShellExecParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	result := executor.Run(ctx, p.Command, p.Shell, p.Timeout)
	return protocol.Response{
		Success: true,
		Data: protocol.ShellExecResult{
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		},
	}
}

func handleFileRead(params json.RawMessage) protocol.Response {
	var p protocol.FileReadParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	result, err := fileops.ReadFile(p.Path, p.Offset, p.Limit)
	if err != nil {
		return protocol.Response{Success: false, Error: err.Error()}
	}
	return protocol.Response{Success: true, Data: result}
}

func handleFileWrite(params json.RawMessage) protocol.Response {
	var p protocol.FileWriteParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	if err := fileops.WriteFile(p.Path, p.Content); err != nil {
		return protocol.Response{Success: false, Error: err.Error()}
	}
	return protocol.Response{Success: true, Data: "file written successfully"}
}

func handleFileEdit(params json.RawMessage) protocol.Response {
	var p protocol.FileEditParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	if err := fileops.EditFile(p.Path, p.OldString, p.NewString, p.ReplaceAll); err != nil {
		return protocol.Response{Success: false, Error: err.Error()}
	}
	return protocol.Response{Success: true, Data: "file edited successfully"}
}

func handleFileList(params json.RawMessage) protocol.Response {
	var p protocol.FileListParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	entries, err := fileops.ListDir(p.Path)
	if err != nil {
		return protocol.Response{Success: false, Error: err.Error()}
	}
	return protocol.Response{Success: true, Data: entries}
}

func handleFileDelete(params json.RawMessage) protocol.Response {
	var p protocol.FileDeleteParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	if err := fileops.DeleteFile(p.Path); err != nil {
		return protocol.Response{Success: false, Error: err.Error()}
	}
	return protocol.Response{Success: true, Data: "file deleted"}
}

func handleGlob(params json.RawMessage) protocol.Response {
	var p protocol.GlobParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	matches, err := fileops.Glob(p.Pattern, p.Path)
	if err != nil {
		return protocol.Response{Success: false, Error: err.Error()}
	}
	return protocol.Response{Success: true, Data: matches}
}

func handleGrep(params json.RawMessage) protocol.Response {
	var p protocol.GrepParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	// Resolve context: -C sets both before/after, specific -A/-B override
	ctxBefore := p.Context
	ctxAfter := p.Context
	if p.ContextBefore > 0 {
		ctxBefore = p.ContextBefore
	}
	if p.ContextAfter > 0 {
		ctxAfter = p.ContextAfter
	}

	opts := fileops.GrepOptions{
		Pattern:         p.Pattern,
		Path:            p.Path,
		GlobFilter:      p.Glob,
		TypeFilter:      p.Type,
		OutputMode:      p.OutputMode,
		CaseInsensitive: p.CaseInsensitive,
		ContextBefore:   ctxBefore,
		ContextAfter:    ctxAfter,
		HeadLimit:       p.HeadLimit,
		Multiline:       p.Multiline,
	}

	result, err := fileops.GrepFile(opts)
	if err != nil {
		return protocol.Response{Success: false, Error: err.Error()}
	}
	return protocol.Response{Success: true, Data: result}
}

func handleSysInfo(ctx context.Context) protocol.Response {
	info := sysinfo.GetSysInfo(ctx)
	return protocol.Response{Success: true, Data: info}
}

func handleProcesses(ctx context.Context) protocol.Response {
	procs := sysinfo.GetProcesses(ctx)
	return protocol.Response{Success: true, Data: procs}
}

func handleServices(ctx context.Context) protocol.Response {
	svcs := sysinfo.GetServices(ctx)
	return protocol.Response{Success: true, Data: svcs}
}

func handleRegistry(ctx context.Context, params json.RawMessage) protocol.Response {
	var p protocol.RegistryReadParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	result, err := sysinfo.ReadRegistry(ctx, p.Key, p.Value)
	if err != nil {
		return protocol.Response{Success: false, Error: err.Error()}
	}
	return protocol.Response{Success: true, Data: result}
}

func handleNetInfo() protocol.Response {
	info := sysinfo.GetNetInfo()
	return protocol.Response{Success: true, Data: info}
}

func handleEnvVars() protocol.Response {
	vars := sysinfo.GetEnvVars()
	return protocol.Response{Success: true, Data: vars}
}

func handleInstalledSoftware(ctx context.Context) protocol.Response {
	sw := sysinfo.GetInstalledSoftware(ctx)
	return protocol.Response{Success: true, Data: sw}
}

func handleEventLog(ctx context.Context, params json.RawMessage) protocol.Response {
	var p protocol.EventLogParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	entries := sysinfo.GetEventLog(ctx, p.LogName, p.MaxItems, p.Level)
	return protocol.Response{Success: true, Data: entries}
}

func handleFileUpload(params json.RawMessage) protocol.Response {
	var p protocol.FileUploadParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	if err := fileops.UploadFile(p.Path, p.ContentBase64, p.Overwrite); err != nil {
		return protocol.Response{Success: false, Error: err.Error()}
	}
	return protocol.Response{Success: true, Data: "file uploaded successfully to " + p.Path}
}

func handleFileDownload(params json.RawMessage) protocol.Response {
	var p protocol.FileDownloadParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	result, err := fileops.DownloadFile(p.Path)
	if err != nil {
		return protocol.Response{Success: false, Error: err.Error()}
	}
	return protocol.Response{Success: true, Data: result}
}

func handleFileUploadChunk(params json.RawMessage) protocol.Response {
	var p protocol.FileUploadChunkParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	if err := fileops.UploadChunk(p.Path, p.ContentBase64, p.ChunkIndex, p.TotalChunks, p.Overwrite); err != nil {
		return protocol.Response{Success: false, Error: err.Error()}
	}

	pct := (p.ChunkIndex + 1) * 100 / p.TotalChunks
	status := fmt.Sprintf("chunk %d/%d (%d%%)", p.ChunkIndex+1, p.TotalChunks, pct)
	if p.ChunkIndex+1 == p.TotalChunks {
		status = fmt.Sprintf("upload complete: %s (%d bytes)", p.Path, p.TotalSize)
	}
	return protocol.Response{Success: true, Data: status}
}

func handleFileDownloadChunk(params json.RawMessage) protocol.Response {
	var p protocol.FileDownloadChunkParams
	if err := json.Unmarshal(params, &p); err != nil {
		return protocol.Response{Success: false, Error: "invalid params: " + err.Error()}
	}

	result, err := fileops.DownloadChunk(p.Path, p.Offset, p.ChunkSize)
	if err != nil {
		return protocol.Response{Success: false, Error: err.Error()}
	}
	return protocol.Response{Success: true, Data: result}
}
