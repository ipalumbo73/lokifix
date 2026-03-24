#!/usr/bin/env bash
#
# LokiFix - macOS Data Collection Script
# Collects diagnostic data for offline analysis.
#

set -euo pipefail

OUTPUT_DIR="${1:-.}"
mkdir -p "$OUTPUT_DIR"
HOSTNAME_VAL=$(hostname -s)
TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
OUTPUT_FILE="$OUTPUT_DIR/lokifix_${HOSTNAME_VAL}_${TIMESTAMP}.txt"

section() {
    echo "" >> "$OUTPUT_FILE"
    echo "========================================" >> "$OUTPUT_FILE"
    echo "  $1" >> "$OUTPUT_FILE"
    echo "========================================" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
}

echo "LokiFix - macOS Data Collection" > "$OUTPUT_FILE"
echo "Date: $(date)" >> "$OUTPUT_FILE"
echo "Host: $HOSTNAME_VAL" >> "$OUTPUT_FILE"

# SYSTEM INFO
section "SYSTEM INFO"
sw_vers >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
uname -a >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
system_profiler SPHardwareDataType >> "$OUTPUT_FILE" 2>/dev/null || true

# CPU
section "CPU"
sysctl -n machdep.cpu.brand_string >> "$OUTPUT_FILE" 2>/dev/null || true
echo "Cores: $(sysctl -n hw.ncpu 2>/dev/null || echo N/A)" >> "$OUTPUT_FILE"
echo "Load: $(sysctl -n vm.loadavg 2>/dev/null || echo N/A)" >> "$OUTPUT_FILE"

# MEMORY
section "MEMORY"
echo "Total: $(sysctl -n hw.memsize 2>/dev/null | awk '{printf "%.1f GB", $1/1073741824}' || echo N/A)" >> "$OUTPUT_FILE"
vm_stat >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
memory_pressure >> "$OUTPUT_FILE" 2>/dev/null || true

# DISK SPACE
section "DISK SPACE"
df -h >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
diskutil list >> "$OUTPUT_FILE" 2>/dev/null || true

# DISK SMART STATUS
section "DISK SMART STATUS"
diskutil info / | grep -E "SMART|Name|Type|Protocol" >> "$OUTPUT_FILE" 2>/dev/null || true

# NETWORK
section "NETWORK"
ifconfig >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
echo "--- DNS ---" >> "$OUTPUT_FILE"
scutil --dns | head -50 >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
echo "--- Routing ---" >> "$OUTPUT_FILE"
netstat -rn | head -30 >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
echo "--- Listening Ports ---" >> "$OUTPUT_FILE"
lsof -i -P -n | grep LISTEN >> "$OUTPUT_FILE" 2>/dev/null || true

# FIREWALL
section "FIREWALL"
/usr/libexec/ApplicationFirewall/socketfilterfw --getglobalstate >> "$OUTPUT_FILE" 2>/dev/null || true
/usr/libexec/ApplicationFirewall/socketfilterfw --listapps >> "$OUTPUT_FILE" 2>/dev/null || true

# SECURITY
section "SECURITY"
echo "--- SIP Status ---" >> "$OUTPUT_FILE"
csrutil status >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
echo "--- Gatekeeper ---" >> "$OUTPUT_FILE"
spctl --status >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
echo "--- FileVault ---" >> "$OUTPUT_FILE"
fdesetup status >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
echo "--- XProtect ---" >> "$OUTPUT_FILE"
system_profiler SPInstallHistoryDataType | grep -A 2 "XProtect" | tail -3 >> "$OUTPUT_FILE" 2>/dev/null || true

# USERS
section "USERS"
dscl . list /Users | grep -v "^_" >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
echo "--- Admin Users ---" >> "$OUTPUT_FILE"
dscl . -read /Groups/admin GroupMembership >> "$OUTPUT_FILE" 2>/dev/null || true

# SERVICES
section "SERVICES (LaunchDaemons/LaunchAgents)"
echo "--- System LaunchDaemons ---" >> "$OUTPUT_FILE"
ls /Library/LaunchDaemons/ >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
echo "--- User LaunchAgents ---" >> "$OUTPUT_FILE"
ls ~/Library/LaunchAgents/ >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
echo "--- Running Services ---" >> "$OUTPUT_FILE"
launchctl list | head -50 >> "$OUTPUT_FILE" 2>/dev/null || true

# SOFTWARE UPDATES
section "SOFTWARE UPDATES"
softwareupdate -l >> "$OUTPUT_FILE" 2>&1 || true

# RECENT LOGS (last 24h errors)
section "RECENT ERRORS (last 24h)"
log show --predicate 'messageType == error' --last 24h --style compact 2>/dev/null | tail -100 >> "$OUTPUT_FILE" || true

# TIME MACHINE
section "TIME MACHINE"
tmutil status >> "$OUTPUT_FILE" 2>/dev/null || true
echo "" >> "$OUTPUT_FILE"
tmutil latestbackup >> "$OUTPUT_FILE" 2>/dev/null || echo "No backup configured" >> "$OUTPUT_FILE"

# STARTUP ITEMS
section "STARTUP ITEMS"
echo "--- Login Items ---" >> "$OUTPUT_FILE"
osascript -e 'tell application "System Events" to get the name of every login item' >> "$OUTPUT_FILE" 2>/dev/null || true

# UPTIME
section "UPTIME"
uptime >> "$OUTPUT_FILE" 2>/dev/null || true

echo ""
echo "Data collection complete: $OUTPUT_FILE"
