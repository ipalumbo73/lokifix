#Requires -Version 5.1
<#
.SYNOPSIS
    Setup script per preparare la chiavetta USB con Claude Code Portable.
    Esegui questo script UNA VOLTA dal tuo PC principale per configurare la chiavetta.

.PARAMETER UsbDrive
    Lettera del drive USB (es. "E", "F")

.PARAMETER NodeVersion
    Versione di Node.js da scaricare (default: 20.11.1)

.EXAMPLE
    .\setup-usb.ps1 -UsbDrive E
#>

param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[A-Z]$')]
    [string]$UsbDrive,

    [string]$NodeVersion = "20.11.1"
)

$ErrorActionPreference = "Stop"
$UsbRoot = "${UsbDrive}:\"

# --- Validazione ---
if (-not (Test-Path $UsbRoot)) {
    Write-Error "Drive ${UsbDrive}: non trovato. Inserisci la chiavetta USB."
    exit 1
}

$freeSpace = (Get-PSDrive $UsbDrive).Free
$requiredSpace = 500MB
if ($freeSpace -lt $requiredSpace) {
    Write-Error "Spazio insufficiente. Servono almeno 500MB liberi. Disponibili: $([math]::Round($freeSpace / 1MB))MB"
    exit 1
}

Write-Host "=== CLAUDE CODE PORTABLE - SETUP ===" -ForegroundColor Cyan
Write-Host "Drive USB: ${UsbDrive}:" -ForegroundColor Yellow
Write-Host "Node.js version: $NodeVersion" -ForegroundColor Yellow
Write-Host ""

# --- Creazione struttura directory ---
Write-Host "[1/6] Creazione struttura directory..." -ForegroundColor Green

$directories = @(
    "runtime\node-win-x64",
    "runtime\node-linux-x64",
    "claude-code",
    "config",
    "config\rules",
    "toolkit\prompts",
    "toolkit\scripts",
    "toolkit\logs"
)

foreach ($dir in $directories) {
    $fullPath = Join-Path $UsbRoot $dir
    if (-not (Test-Path $fullPath)) {
        New-Item -ItemType Directory -Path $fullPath -Force | Out-Null
    }
}

# --- Download Node.js Windows ---
Write-Host "[2/6] Download Node.js $NodeVersion per Windows x64..." -ForegroundColor Green

$nodeWinZip = Join-Path $env:TEMP "node-win-x64.zip"
$nodeWinUrl = "https://nodejs.org/dist/v${NodeVersion}/node-v${NodeVersion}-win-x64.zip"
$nodeWinDest = Join-Path $UsbRoot "runtime\node-win-x64"

if (-not (Test-Path (Join-Path $nodeWinDest "node.exe"))) {
    Write-Host "  Scaricamento da $nodeWinUrl ..."
    $maxRetries = 2
    for ($i = 1; $i -le $maxRetries; $i++) {
        try {
            Invoke-WebRequest -Uri $nodeWinUrl -OutFile $nodeWinZip -UseBasicParsing -TimeoutSec 120
            break
        } catch {
            if ($i -eq $maxRetries) { throw }
            Write-Host "[RETRY] Tentativo $i fallito, riprovo..." -ForegroundColor Yellow
        }
    }
    Write-Host "  Estrazione..."
    Expand-Archive -Path $nodeWinZip -DestinationPath (Join-Path $env:TEMP "node-win-extract") -Force
    $extractedDir = Get-ChildItem (Join-Path $env:TEMP "node-win-extract") | Select-Object -First 1
    Copy-Item -Path "$($extractedDir.FullName)\*" -Destination $nodeWinDest -Recurse -Force
    Remove-Item $nodeWinZip -Force -ErrorAction SilentlyContinue
    Remove-Item (Join-Path $env:TEMP "node-win-extract") -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host "  OK" -ForegroundColor Green
} else {
    Write-Host "  Gia' presente, skip." -ForegroundColor Yellow
}

# --- Download Node.js Linux ---
Write-Host "[3/6] Download Node.js $NodeVersion per Linux x64..." -ForegroundColor Green

$nodeLinuxTar = Join-Path $env:TEMP "node-linux-x64.tar.xz"
$nodeLinuxUrl = "https://nodejs.org/dist/v${NodeVersion}/node-v${NodeVersion}-linux-x64.tar.xz"
$nodeLinuxDest = Join-Path $UsbRoot "runtime\node-linux-x64"

if (-not (Test-Path (Join-Path $nodeLinuxDest "bin"))) {
    Write-Host "  Scaricamento da $nodeLinuxUrl ..."
    $maxRetries = 2
    for ($i = 1; $i -le $maxRetries; $i++) {
        try {
            Invoke-WebRequest -Uri $nodeLinuxUrl -OutFile $nodeLinuxTar -UseBasicParsing -TimeoutSec 120
            break
        } catch {
            if ($i -eq $maxRetries) { throw }
            Write-Host "[RETRY] Tentativo $i fallito, riprovo..." -ForegroundColor Yellow
        }
    }
    Write-Host "  NOTA: Estrai manualmente il .tar.xz su Linux con:"
    Write-Host "    tar -xf node-linux-x64.tar.xz -C /path/to/usb/runtime/node-linux-x64 --strip-components=1" -ForegroundColor Yellow
    Copy-Item $nodeLinuxTar -Destination $nodeLinuxDest -Force
    Remove-Item $nodeLinuxTar -Force -ErrorAction SilentlyContinue
    Write-Host "  File scaricato in $nodeLinuxDest" -ForegroundColor Green
} else {
    Write-Host "  Gia' presente, skip." -ForegroundColor Yellow
}

# --- Installazione Claude Code ---
Write-Host "[4/6] Installazione Claude Code..." -ForegroundColor Green

$nodePath = Join-Path $UsbRoot "runtime\node-win-x64\node.exe"
$npmPath = Join-Path $UsbRoot "runtime\node-win-x64\npm.cmd"
$claudeCodeDir = Join-Path $UsbRoot "claude-code"

$env:PATH = "$(Join-Path $UsbRoot 'runtime\node-win-x64');$env:PATH"
$env:NPM_CONFIG_PREFIX = $claudeCodeDir

Write-Host "  Installazione @anthropic-ai/claude-code..."
& $npmPath install -g @anthropic-ai/claude-code --prefix $claudeCodeDir 2>&1 | ForEach-Object {
    if ($_ -match "added|updated|claude") { Write-Host "  $_" -ForegroundColor Gray }
}
Write-Host "  OK" -ForegroundColor Green

# --- Login Claude ---
Write-Host "[5/6] Configurazione autenticazione..." -ForegroundColor Green

$env:CLAUDE_CONFIG_DIR = Join-Path $UsbRoot "config"
$claudeBin = Join-Path $claudeCodeDir "bin\claude.cmd"

if (Test-Path $claudeBin) {
    Write-Host "  Avvio login... Segui le istruzioni nel browser." -ForegroundColor Yellow
    & $claudeBin login
} else {
    Write-Host "  ATTENZIONE: claude.cmd non trovato in $claudeBin" -ForegroundColor Red
    Write-Host "  Esegui il login manualmente dopo il setup." -ForegroundColor Yellow
}

# --- Copia launcher e toolkit ---
Write-Host "[6/6] Copia launcher e toolkit sulla chiavetta..." -ForegroundColor Green

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$filesToCopy = @(
    "launch.bat",
    "launch.ps1",
    "launch.sh",
    "toolkit\prompts\windows-health.md",
    "toolkit\prompts\linux-health.md",
    "toolkit\prompts\esxi-health.md",
    "toolkit\prompts\vmware-health.md",
    "toolkit\prompts\server-2008-2012.md",
    "toolkit\scripts\collect-win.ps1",
    "toolkit\scripts\collect-linux.sh",
    "toolkit\scripts\collect-esxi.sh"
)

foreach ($file in $filesToCopy) {
    $source = Join-Path $scriptDir $file
    $dest = Join-Path $UsbRoot $file
    if (Test-Path $source) {
        $destDir = Split-Path $dest -Parent
        if (-not (Test-Path $destDir)) { New-Item -ItemType Directory -Path $destDir -Force | Out-Null }
        Copy-Item $source $dest -Force
        Write-Host "  Copiato: $file" -ForegroundColor Gray
    }
}

# --- Riepilogo ---
Write-Host ""
Write-Host "=== SETUP COMPLETATO ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "Struttura chiavetta ${UsbDrive}:\" -ForegroundColor Yellow
Write-Host "  runtime\         - Node.js portable (Win + Linux)"
Write-Host "  claude-code\     - Claude Code CLI"
Write-Host "  config\          - Configurazione e credenziali"
Write-Host "  toolkit\         - Prompt diagnostici e script"
Write-Host ""
Write-Host "Per usare la chiavetta:" -ForegroundColor Yellow
Write-Host "  Windows:  Doppio click su launch.bat (o launch.ps1)"
Write-Host "  Linux:    bash launch.sh"
Write-Host ""
Write-Host "IMPORTANTE: La chiavetta contiene le tue credenziali." -ForegroundColor Red
Write-Host "Considera di cifrarla con BitLocker o VeraCrypt." -ForegroundColor Red
