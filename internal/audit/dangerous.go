package audit

import (
	"strings"
)

// DangerousPatterns are shell command patterns that require user confirmation.
var DangerousPatterns = []string{
	"rm ", "rm\t", "rmdir", "del ", "del\t",
	"format ", "format\t",
	"Remove-Item", "Remove-", "Clear-Content",
	"Stop-Service", "Stop-Process",
	"Restart-Service", "Restart-Computer",
	"Set-ItemProperty",  // registry write
	"New-ItemProperty",  // registry create
	"Remove-ItemProperty", // registry delete
	"Disable-NetAdapter",
	"diskpart",
	"cipher /w",
	"reg delete", "reg add",
	"net stop", "net user",
	"sc delete", "sc stop",
	"shutdown", "taskkill",
	"bcdedit",
	"sfc ", "dism ",
	"wmic os", "wmic process call terminate",
}

// DangerousFileOps are file paths that require confirmation for write/edit/delete.
var DangerousFilePaths = []string{
	`C:\Windows\`,
	`C:\Program Files\`,
	`C:\Program Files (x86)\`,
	`C:\ProgramData\`,
	`\System32\`,
	`\SysWOW64\`,
	`\drivers\`,
	`.exe`,
	`.dll`,
	`.sys`,
	`.bat`,
	`.cmd`,
	`.ps1`,
	`.reg`,
	`hosts`,
}

// IsDangerousCommand checks if a shell command matches a dangerous pattern.
func IsDangerousCommand(command string) (bool, string) {
	lower := strings.ToLower(command)
	for _, pattern := range DangerousPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true, pattern
		}
	}
	return false, ""
}

// IsDangerousFilePath checks if a file path is in a protected area.
func IsDangerousFilePath(path string) (bool, string) {
	lower := strings.ToLower(path)
	for _, pattern := range DangerousFilePaths {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true, pattern
		}
	}
	return false, ""
}
