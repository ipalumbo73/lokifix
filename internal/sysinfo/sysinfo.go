package sysinfo

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/ivanpalumbo/lokifix/internal/executor"
	"github.com/ivanpalumbo/lokifix/internal/protocol"
	"context"
)

// GetSysInfo collects system information via PowerShell.
func GetSysInfo(ctx context.Context) protocol.SysInfoResult {
	hostname, _ := os.Hostname()

	result := protocol.SysInfoResult{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUs:     runtime.NumCPU(),
	}

	// Get OS version and memory info via PowerShell
	psCmd := `
$os = Get-CimInstance Win32_OperatingSystem
$uptime = (Get-Date) - $os.LastBootUpTime
@{
    Version = $os.Caption + ' ' + $os.Version
    MemTotal = [math]::Round($os.TotalVisibleMemorySize / 1024)
    MemFree = [math]::Round($os.FreePhysicalMemory / 1024)
    UptimeHours = [math]::Round($uptime.TotalHours, 2)
} | ConvertTo-Json`

	exec := executor.Run(ctx, psCmd, "powershell", 10)
	if exec.ExitCode == 0 {
		// Parse JSON output
		lines := strings.Split(strings.TrimSpace(exec.Stdout), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "\"Version\"") {
				result.Version = extractJSONString(line)
			} else if strings.Contains(line, "\"MemTotal\"") {
				result.MemoryTotalMB = extractJSONInt(line)
			} else if strings.Contains(line, "\"MemFree\"") {
				result.MemoryFreeMB = extractJSONInt(line)
			} else if strings.Contains(line, "\"UptimeHours\"") {
				result.UptimeHours = extractJSONFloat(line)
			}
		}
	}

	return result
}

// GetProcesses returns a list of running processes.
func GetProcesses(ctx context.Context) []protocol.ProcessEntry {
	psCmd := `Get-Process | Sort-Object CPU -Descending | Select-Object -First 50 Id, ProcessName, @{N='CPU';E={[math]::Round($_.CPU,1)}}, @{N='MemMB';E={[math]::Round($_.WorkingSet64/1MB,1)}} | ConvertTo-Csv -NoTypeInformation`

	exec := executor.Run(ctx, psCmd, "powershell", 15)
	if exec.ExitCode != 0 {
		return nil
	}

	var entries []protocol.ProcessEntry
	lines := strings.Split(strings.TrimSpace(exec.Stdout), "\n")
	for i, line := range lines {
		if i == 0 { // skip header
			continue
		}
		fields := parseCSVLine(line)
		if len(fields) >= 4 {
			pid, _ := strconv.Atoi(fields[0])
			entries = append(entries, protocol.ProcessEntry{
				PID:   pid,
				Name:  fields[1],
				CPU:   fields[2],
				MemMB: fields[3],
			})
		}
	}
	return entries
}

// GetServices returns a list of Windows services.
func GetServices(ctx context.Context) []protocol.ServiceEntry {
	psCmd := `Get-Service | Select-Object Name, Status, StartType | ConvertTo-Csv -NoTypeInformation`

	exec := executor.Run(ctx, psCmd, "powershell", 15)
	if exec.ExitCode != 0 {
		return nil
	}

	var entries []protocol.ServiceEntry
	lines := strings.Split(strings.TrimSpace(exec.Stdout), "\n")
	for i, line := range lines {
		if i == 0 {
			continue
		}
		fields := parseCSVLine(line)
		if len(fields) >= 3 {
			entries = append(entries, protocol.ServiceEntry{
				Name:   fields[0],
				Status: fields[1],
				Start:  fields[2],
			})
		}
	}
	return entries
}

// ReadRegistry reads a Windows registry key/value.
func ReadRegistry(ctx context.Context, key, valueName string) (string, error) {
	if err := validateRegistryKey(key); err != nil {
		return "", err
	}
	if valueName != "" {
		if err := validateSafeString(valueName); err != nil {
			return "", fmt.Errorf("invalid value name: %w", err)
		}
	}

	// Use PowerShell variables to prevent injection
	psCmd := fmt.Sprintf(`$k = '%s'; `, escapePSString(key))
	if valueName == "" {
		psCmd += `Get-ItemProperty -Path $k | ConvertTo-Json -Depth 2`
	} else {
		psCmd += fmt.Sprintf(`$v = '%s'; (Get-ItemProperty -Path $k).$v`, escapePSString(valueName))
	}

	exec := executor.Run(ctx, psCmd, "powershell", 10)
	if exec.ExitCode != 0 {
		return "", fmt.Errorf("registry read failed: %s", exec.Stderr)
	}
	return strings.TrimSpace(exec.Stdout), nil
}

var validRegistryRoots = []string{
	"HKLM:", "HKCU:", "HKCR:", "HKU:", "HKCC:",
	"HKEY_LOCAL_MACHINE:", "HKEY_CURRENT_USER:",
	"HKEY_CLASSES_ROOT:", "HKEY_USERS:", "HKEY_CURRENT_CONFIG:",
}

func validateRegistryKey(key string) error {
	upper := strings.ToUpper(strings.ReplaceAll(key, "/", "\\"))
	for _, root := range validRegistryRoots {
		if strings.HasPrefix(upper, root) {
			return nil
		}
	}
	return fmt.Errorf("invalid registry key: must start with a valid hive (HKLM:, HKCU:, etc.)")
}

func validateSafeString(s string) error {
	for _, c := range s {
		if c == '\'' || c == '"' || c == ';' || c == '`' || c == '$' || c == '|' || c == '&' || c == '\n' || c == '\r' {
			return fmt.Errorf("contains forbidden character: %c", c)
		}
	}
	return nil
}

func escapePSString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// GetNetInfo collects network interface information.
func GetNetInfo() protocol.NetInfoResult {
	ifaces, err := net.Interfaces()
	if err != nil {
		return protocol.NetInfoResult{}
	}

	var result protocol.NetInfoResult
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		var addrStrs []string
		for _, addr := range addrs {
			addrStrs = append(addrStrs, addr.String())
		}

		status := "down"
		if iface.Flags&net.FlagUp != 0 {
			status = "up"
		}

		result.Interfaces = append(result.Interfaces, protocol.NetInterface{
			Name:   iface.Name,
			Addrs:  addrStrs,
			Status: status,
		})
	}

	return result
}

// GetEnvVars returns all environment variables.
func GetEnvVars() map[string]string {
	result := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// GetInstalledSoftware returns installed software from Windows registry.
func GetInstalledSoftware(ctx context.Context) []protocol.InstalledSoftwareEntry {
	psCmd := `
$paths = @(
    'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
    'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'
)
Get-ItemProperty $paths -ErrorAction SilentlyContinue |
    Where-Object { $_.DisplayName } |
    Sort-Object DisplayName |
    Select-Object DisplayName, DisplayVersion, Publisher |
    ConvertTo-Csv -NoTypeInformation`

	exec := executor.Run(ctx, psCmd, "powershell", 15)
	if exec.ExitCode != 0 {
		return nil
	}

	var entries []protocol.InstalledSoftwareEntry
	lines := strings.Split(strings.TrimSpace(exec.Stdout), "\n")
	for i, line := range lines {
		if i == 0 {
			continue
		}
		fields := parseCSVLine(line)
		if len(fields) >= 3 {
			entries = append(entries, protocol.InstalledSoftwareEntry{
				Name:    fields[0],
				Version: fields[1],
				Vendor:  fields[2],
			})
		}
	}
	return entries
}

// GetEventLog returns recent event log entries.
func GetEventLog(ctx context.Context, logName string, maxItems int, level string) []protocol.EventLogEntry {
	// Strict allowlist for log names to prevent injection
	allowedLogs := map[string]bool{
		"System": true, "Application": true, "Security": true,
	}
	if logName == "" {
		logName = "System"
	}
	if !allowedLogs[logName] {
		logName = "System"
	}
	if maxItems <= 0 || maxItems > 500 {
		maxItems = 50
	}

	// Strict allowlist for levels
	levelMap := map[string]int{
		"Error": 2, "Warning": 3, "Information": 4,
	}

	filter := fmt.Sprintf("-LogName '%s' -MaxEvents %d", logName, maxItems)
	if level != "" {
		if lvl, ok := levelMap[level]; ok {
			filter = fmt.Sprintf("-FilterHashtable @{LogName='%s'; Level=%d} -MaxEvents %d", logName, lvl, maxItems)
		}
	}

	psCmd := fmt.Sprintf(`Get-WinEvent %s -ErrorAction SilentlyContinue | Select-Object TimeCreated, LevelDisplayName, ProviderName, @{N='Msg';E={$_.Message -replace '[\r\n]+',' ' | Select-Object -First 1}} | ConvertTo-Csv -NoTypeInformation`, filter)

	exec := executor.Run(ctx, psCmd, "powershell", 20)
	if exec.ExitCode != 0 {
		return nil
	}

	var entries []protocol.EventLogEntry
	lines := strings.Split(strings.TrimSpace(exec.Stdout), "\n")
	for i, line := range lines {
		if i == 0 {
			continue
		}
		fields := parseCSVLine(line)
		if len(fields) >= 4 {
			entries = append(entries, protocol.EventLogEntry{
				TimeCreated: fields[0],
				Level:       fields[1],
				Source:      fields[2],
				Message:     fields[3],
			})
		}
	}
	return entries
}

func extractJSONString(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return ""
	}
	val := strings.TrimSpace(parts[1])
	val = strings.Trim(val, `",`)
	return val
}

func extractJSONInt(line string) int64 {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return 0
	}
	val := strings.TrimSpace(parts[1])
	val = strings.Trim(val, `",`)
	n, _ := strconv.ParseInt(val, 10, 64)
	return n
}

func extractJSONFloat(line string) float64 {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return 0
	}
	val := strings.TrimSpace(parts[1])
	val = strings.Trim(val, `",`)
	f, _ := strconv.ParseFloat(val, 64)
	return f
}

func parseCSVLine(line string) []string {
	line = strings.TrimSpace(line)
	var fields []string
	var current strings.Builder
	inQuotes := false

	for _, r := range line {
		switch {
		case r == '"':
			inQuotes = !inQuotes
		case r == ',' && !inQuotes:
			fields = append(fields, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	fields = append(fields, strings.TrimSpace(current.String()))
	return fields
}
