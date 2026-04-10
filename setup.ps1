<#
.SYNOPSIS
    Setup LokiFix Remote Agent per Claude Code.
    Configura l'MCP server in Claude Code e compila i binari.

.DESCRIPTION
    Questo script:
    1. Verifica che Go sia installato
    2. Compila lokifix-agent.exe e lokifix-mcp.exe
    3. Configura Claude Code per usare lokifix-mcp come MCP server
    4. Scarica cloudflared per il tunnel

.EXAMPLE
    .\setup.ps1
#>

$ErrorActionPreference = "Stop"

Write-Host ""
Write-Host "  ╦  ╔═╗╦╔═╦╔═╗╦═╗ ╦" -ForegroundColor Cyan
Write-Host "  ║  ║ ║╠╩╗║╠╣ ║╔╩╦╝" -ForegroundColor Cyan
Write-Host "  ╩═╝╚═╝╩ ╩╩╚  ╩╩ ╚═" -ForegroundColor Cyan
Write-Host "  Setup v1.0.0" -ForegroundColor DarkGray
Write-Host ""

$projectDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$buildDir = Join-Path $projectDir "build"

# 1. Check Go
Write-Host "  [1/4] Verifica Go..." -ForegroundColor Yellow
try {
    $goVersion = & go version 2>&1
    Write-Host "  ✓ $goVersion" -ForegroundColor Green
} catch {
    Write-Host "  ✗ Go non trovato. Installa Go: winget install GoLang.Go" -ForegroundColor Red
    exit 1
}

# 2. Build binaries
Write-Host "  [2/4] Compilazione binari..." -ForegroundColor Yellow
Push-Location $projectDir

if (!(Test-Path $buildDir)) {
    New-Item -ItemType Directory -Path $buildDir -Force | Out-Null
}

Write-Host "    Compilando lokifix-mcp.exe..."
& go build -ldflags="-s -w" -o (Join-Path $buildDir "lokifix-mcp.exe") ./cmd/lokifix-mcp/
if ($LASTEXITCODE -ne 0) {
    Write-Host "  ✗ Compilazione MCP server fallita" -ForegroundColor Red
    Pop-Location
    exit 1
}

Write-Host "    Compilando lokifix-agent.exe..."
& go build -ldflags="-s -w" -o (Join-Path $buildDir "lokifix-agent.exe") ./cmd/lokifix-agent/
if ($LASTEXITCODE -ne 0) {
    Write-Host "  ✗ Compilazione agent fallita" -ForegroundColor Red
    Pop-Location
    exit 1
}

Pop-Location

$mcpExe = Join-Path $buildDir "lokifix-mcp.exe"
$agentExe = Join-Path $buildDir "lokifix-agent.exe"
$mcpSize = [math]::Round((Get-Item $mcpExe).Length / 1MB, 1)
$agentSize = [math]::Round((Get-Item $agentExe).Length / 1MB, 1)

Write-Host "  ✓ lokifix-mcp.exe   ($mcpSize MB)" -ForegroundColor Green
Write-Host "  ✓ lokifix-agent.exe ($agentSize MB)" -ForegroundColor Green

# 3. Configure Claude Code MCP
Write-Host "  [3/4] Configurazione Claude Code..." -ForegroundColor Yellow

$claudeConfigDir = Join-Path $env:USERPROFILE ".claude"
$claudeConfigFile = Join-Path $claudeConfigDir "claude_desktop_config.json"

# Also configure for claude CLI
$mcpExeEscaped = $mcpExe -replace '\\', '\\\\'

# Check if .claude directory exists
if (!(Test-Path $claudeConfigDir)) {
    New-Item -ItemType Directory -Path $claudeConfigDir -Force | Out-Null
}

# Read existing config or create new
$config = @{}
if (Test-Path $claudeConfigFile) {
    $config = Get-Content $claudeConfigFile -Raw | ConvertFrom-Json -AsHashtable
}

if (-not $config.ContainsKey("mcpServers")) {
    $config["mcpServers"] = @{}
}

$config["mcpServers"]["lokifix-remote"] = @{
    "command" = $mcpExe
    "args" = @()
}

$config | ConvertTo-Json -Depth 10 | Set-Content $claudeConfigFile -Encoding UTF8
Write-Host "  ✓ MCP server configurato in $claudeConfigFile" -ForegroundColor Green

# Also configure project-level .claude/settings.local.json
$projectClaudeDir = Join-Path $projectDir ".claude"
$projectSettings = Join-Path $projectClaudeDir "settings.local.json"

if (Test-Path $projectSettings) {
    $projConfig = Get-Content $projectSettings -Raw | ConvertFrom-Json -AsHashtable
} else {
    $projConfig = @{}
}

# Ensure permissions include the MCP tools
if (-not $projConfig.ContainsKey("permissions")) {
    $projConfig["permissions"] = @{}
}
if (-not $projConfig["permissions"].ContainsKey("allow")) {
    $projConfig["permissions"]["allow"] = @()
}

# Add MCP tool permissions
$mcpTools = @(
    "mcp__lokifix-remote__remote_shell",
    "mcp__lokifix-remote__remote_file_read",
    "mcp__lokifix-remote__remote_file_write",
    "mcp__lokifix-remote__remote_file_edit",
    "mcp__lokifix-remote__remote_file_list",
    "mcp__lokifix-remote__remote_file_delete",
    "mcp__lokifix-remote__remote_glob",
    "mcp__lokifix-remote__remote_grep",
    "mcp__lokifix-remote__remote_sysinfo",
    "mcp__lokifix-remote__remote_processes",
    "mcp__lokifix-remote__remote_services",
    "mcp__lokifix-remote__remote_registry",
    "mcp__lokifix-remote__remote_netinfo",
    "mcp__lokifix-remote__remote_env_vars",
    "mcp__lokifix-remote__remote_installed_software",
    "mcp__lokifix-remote__remote_event_log"
)

foreach ($tool in $mcpTools) {
    if ($projConfig["permissions"]["allow"] -notcontains $tool) {
        $projConfig["permissions"]["allow"] += $tool
    }
}

$projConfig | ConvertTo-Json -Depth 10 | Set-Content $projectSettings -Encoding UTF8
Write-Host "  ✓ Permessi MCP configurati" -ForegroundColor Green

# 4. Download cloudflared
Write-Host "  [4/4] Verifica cloudflared..." -ForegroundColor Yellow
$cloudflaredPath = Join-Path $buildDir "cloudflared.exe"

if (Test-Path $cloudflaredPath) {
    Write-Host "  ✓ cloudflared presente" -ForegroundColor Green
} else {
    $cfInPath = Get-Command cloudflared -ErrorAction SilentlyContinue
    if ($cfInPath) {
        Write-Host "  ✓ cloudflared trovato in PATH: $($cfInPath.Source)" -ForegroundColor Green
    } else {
        Write-Host "    Scaricando cloudflared..."
        $arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
        $cfUrl = "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-$arch.exe"
        try {
            Invoke-WebRequest -Uri $cfUrl -OutFile $cloudflaredPath -UseBasicParsing
            Write-Host "  ✓ cloudflared scaricato" -ForegroundColor Green
        } catch {
            Write-Host "  ⚠ Download cloudflared fallito. Il tunnel non sara' disponibile." -ForegroundColor Yellow
            Write-Host "    Scarica manualmente da: https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/installation/" -ForegroundColor DarkGray
        }
    }
}

Write-Host ""
Write-Host "  ════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  Setup completato!" -ForegroundColor Green
Write-Host ""
Write-Host "  COME USARE:" -ForegroundColor Yellow
Write-Host "  1. Riavvia Claude Code per caricare l'MCP server"
Write-Host "  2. Il codice di connessione apparira' nel file:"
Write-Host "     $env:USERPROFILE\lokifix-connection.txt"
Write-Host "  3. Copia lokifix-agent.exe sul PC remoto:"
Write-Host "     $agentExe"
Write-Host "  4. Esegui l'agent sul PC remoto e incolla il codice"
Write-Host ""
Write-Host "  MODALITA' STANDALONE (per test):" -ForegroundColor Yellow
Write-Host "  $mcpExe --standalone"
Write-Host "  ════════════════════════════════════════" -ForegroundColor Cyan
Write-Host ""
