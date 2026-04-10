package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Action types for the audit log.
const (
	ActionShellExec       = "SHELL_EXEC"
	ActionFileRead        = "FILE_READ"
	ActionFileWrite       = "FILE_WRITE"
	ActionFileEdit        = "FILE_EDIT"
	ActionFileList        = "FILE_LIST"
	ActionFileDelete      = "FILE_DELETE"
	ActionGlob            = "GLOB"
	ActionGrep            = "GREP"
	ActionSysInfo         = "SYSINFO"
	ActionProcesses       = "PROCESSES"
	ActionServices        = "SERVICES"
	ActionRegistry        = "REGISTRY"
	ActionNetInfo         = "NETINFO"
	ActionEnvVars         = "ENV_VARS"
	ActionInstalledSW     = "INSTALLED_SW"
	ActionEventLog        = "EVENT_LOG"
	ActionConnect         = "SESSION_START"
	ActionDisconnect      = "SESSION_END"
	ActionDenied          = "DENIED"
	ActionApproved        = "APPROVED"
)

// Entry represents a single audit log entry.
type Entry struct {
	Timestamp   time.Time
	SessionID   string
	Action      string
	Detail      string
	Result      string // "OK", "ERROR", "DENIED"
	PrevHash    string
	Hash        string
}

// Logger writes tamper-evident audit logs with hash chain integrity.
type Logger struct {
	mu        sync.Mutex
	file      *os.File
	filePath  string
	sessionID string
	side      string // "OPERATOR" or "REMOTE"
	prevHash  string
	entries   []Entry
	startTime time.Time

	// OnEntry is called for each new entry (used for console display on remote side)
	OnEntry func(entry Entry)
}

// NewLogger creates a new audit logger.
// side is "OPERATOR" or "REMOTE".
// logDir is the directory to write logs to.
func NewLogger(side, logDir, sessionID string) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	fileName := fmt.Sprintf("lokifix-%s-%s-%s.log", strings.ToLower(side), sessionID, timestamp)
	filePath := filepath.Join(logDir, fileName)

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	l := &Logger{
		file:      f,
		filePath:  filePath,
		sessionID: sessionID,
		side:      side,
		prevHash:  "GENESIS",
		startTime: time.Now(),
	}

	// Write header
	header := fmt.Sprintf("# LokiFix Audit Log\n# Side: %s\n# Session: %s\n# Started: %s\n# Format: TIMESTAMP | SESSION | ACTION | DETAIL | RESULT | PREV_HASH | HASH\n#\n",
		side, sessionID, time.Now().Format(time.RFC3339))
	f.WriteString(header)

	return l, nil
}

// Log writes an audit entry to the log file.
func (l *Logger) Log(action, detail, result string) Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	// Truncate detail for log readability (keep full in hash)
	displayDetail := detail
	if len(displayDetail) > 200 {
		displayDetail = displayDetail[:200] + "..."
	}

	// Compute hash chain: SHA256(prev_hash + timestamp + action + detail + result)
	hashInput := fmt.Sprintf("%s|%s|%s|%s|%s",
		l.prevHash,
		now.Format(time.RFC3339Nano),
		action,
		detail,
		result,
	)
	hash := sha256.Sum256([]byte(hashInput))
	hashHex := hex.EncodeToString(hash[:])
	shortHash := hashHex[:16]

	entry := Entry{
		Timestamp: now,
		SessionID: l.sessionID,
		Action:    action,
		Detail:    displayDetail,
		Result:    result,
		PrevHash:  l.prevHash[:min(16, len(l.prevHash))],
		Hash:      shortHash,
	}

	// Write to file
	line := fmt.Sprintf("%s | %s | %-15s | %-200s | %-7s | %s | %s\n",
		now.Format("2006-01-02 15:04:05"),
		l.sessionID,
		action,
		displayDetail,
		result,
		entry.PrevHash,
		shortHash,
	)
	l.file.WriteString(line)

	l.prevHash = shortHash
	l.entries = append(l.entries, entry)

	// Notify listener
	if l.OnEntry != nil {
		l.OnEntry(entry)
	}

	return entry
}

// SessionSummary generates a session report.
func (l *Logger) SessionSummary() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	duration := time.Since(l.startTime)

	// Count actions by type
	counts := make(map[string]int)
	errors := 0
	denied := 0
	for _, e := range l.entries {
		counts[e.Action]++
		if e.Result == "ERROR" {
			errors++
		}
		if e.Result == "DENIED" {
			denied++
		}
	}

	var sb strings.Builder
	sb.WriteString("╔══════════════════════════════════════════════════╗\n")
	sb.WriteString("║          REPORT SESSIONE LOKIFIX                ║\n")
	sb.WriteString("╠══════════════════════════════════════════════════╣\n")
	sb.WriteString(fmt.Sprintf("║  Sessione:  %s\n", l.sessionID))
	sb.WriteString(fmt.Sprintf("║  Lato:      %s\n", l.side))
	sb.WriteString(fmt.Sprintf("║  Inizio:    %s\n", l.startTime.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("║  Fine:      %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("║  Durata:    %s\n", formatDuration(duration)))
	sb.WriteString(fmt.Sprintf("║  Azioni:    %d totali, %d errori, %d negate\n", len(l.entries), errors, denied))
	sb.WriteString("║\n")
	sb.WriteString("║  Riepilogo azioni:\n")

	for action, count := range counts {
		if action == ActionConnect || action == ActionDisconnect {
			continue
		}
		sb.WriteString(fmt.Sprintf("║    %-20s %d\n", action, count))
	}

	sb.WriteString("║\n")
	sb.WriteString(fmt.Sprintf("║  Hash integrità finale: %s\n", l.prevHash))
	sb.WriteString(fmt.Sprintf("║  Log file: %s\n", l.filePath))
	sb.WriteString("╚══════════════════════════════════════════════════╝\n")

	return sb.String()
}

// WriteReport writes the session summary to a separate report file.
func (l *Logger) WriteReport() (string, error) {
	summary := l.SessionSummary()

	reportPath := strings.TrimSuffix(l.filePath, ".log") + "-report.txt"
	if err := os.WriteFile(reportPath, []byte(summary), 0600); err != nil {
		return "", fmt.Errorf("write report: %w", err)
	}

	// Also append summary to the log file
	l.file.WriteString("\n# === SESSION SUMMARY ===\n")
	l.file.WriteString(summary)

	return reportPath, nil
}

// FilePath returns the path to the log file.
func (l *Logger) FilePath() string {
	return l.filePath
}

// Close flushes and closes the log file.
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Close()
	}
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
