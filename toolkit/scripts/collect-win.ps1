<#
.SYNOPSIS
    Raccoglie dati diagnostici da un sistema Windows.
    Salva tutto in un file di testo strutturato sulla chiavetta USB.

.PARAMETER OutputDir
    Directory dove salvare i risultati.
#>

param(
    [string]$OutputDir = ".\toolkit\logs"
)

$ErrorActionPreference = "Continue"
$timestamp = Get-Date -Format "yyyy-MM-dd_HH-mm-ss"
$hostname = $env:COMPUTERNAME
$outputFile = Join-Path $OutputDir "${hostname}_${timestamp}.txt"

if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

function Write-Section {
    param([string]$Title, [scriptblock]$Command)
    $separator = "=" * 60
    $result = "`n$separator`n### $Title`n$separator`n"
    try {
        $output = & $Command 2>&1 | Out-String
        $result += $output
    } catch {
        $result += "[ERRORE] $($_.Exception.Message)`n"
    }
    Add-Content -Path $outputFile -Value $result
    Write-Host "  [OK] $Title" -ForegroundColor Green
}

Write-Host "[*] Raccolta dati diagnostici per $hostname" -ForegroundColor Cyan
Write-Host "    Output: $outputFile" -ForegroundColor Gray
Write-Host ""

# Header
$header = @"
DIAGNOSTICA SISTEMA WINDOWS
============================
Data: $(Get-Date -Format "yyyy-MM-dd HH:mm:ss")
Hostname: $hostname
Utente: $env:USERNAME
"@
Set-Content -Path $outputFile -Value $header

Write-Section "INFORMAZIONI OS" {
    try {
        Get-CimInstance Win32_OperatingSystem | Format-List Caption, Version, BuildNumber, OSArchitecture, LastBootUpTime, InstallDate
    } catch {
        Get-WmiObject Win32_OperatingSystem | Format-List Caption, Version, BuildNumber, OSArchitecture, LastBootUpTime, InstallDate
    }
}

Write-Section "HARDWARE" {
    Write-Output "--- CPU ---"
    try {
        Get-CimInstance Win32_Processor | Format-List Name, NumberOfCores, NumberOfLogicalProcessors, MaxClockSpeed
    } catch {
        Get-WmiObject Win32_Processor | Format-List Name, NumberOfCores, NumberOfLogicalProcessors, MaxClockSpeed
    }
    Write-Output "--- RAM ---"
    try {
        $os = Get-CimInstance Win32_OperatingSystem
    } catch {
        $os = Get-WmiObject Win32_OperatingSystem
    }
    Write-Output "RAM Totale: $([math]::Round($os.TotalVisibleMemorySize / 1MB, 2)) GB"
    Write-Output "RAM Libera: $([math]::Round($os.FreePhysicalMemory / 1MB, 2)) GB"
    Write-Output "RAM Usata:  $([math]::Round(($os.TotalVisibleMemorySize - $os.FreePhysicalMemory) / 1MB, 2)) GB"
}

Write-Section "SPAZIO DISCO" {
    try {
        $disks = Get-CimInstance Win32_LogicalDisk -Filter "DriveType=3"
    } catch {
        $disks = Get-WmiObject Win32_LogicalDisk -Filter "DriveType=3"
    }
    $disks | Format-Table DeviceID,
        @{N="Dimensione GB";E={[math]::Round($_.Size/1GB,2)}},
        @{N="Libero GB";E={[math]::Round($_.FreeSpace/1GB,2)}},
        @{N="Uso %";E={[math]::Round(($_.Size - $_.FreeSpace) / $_.Size * 100, 1)}}
}

Write-Section "SERVIZI - STATI ANOMALI" {
    Get-Service | Where-Object { $_.StartType -eq 'Automatic' -and $_.Status -ne 'Running' } |
        Format-Table Name, DisplayName, Status, StartType -AutoSize
}

Write-Section "SERVIZI CRITICI" {
    $criticalServices = @("wuauserv", "W32Time", "Dhcp", "Dnscache", "EventLog", "LanmanServer", "LanmanWorkstation", "Spooler", "WinRM")
    foreach ($svc in $criticalServices) {
        $s = Get-Service -Name $svc -ErrorAction SilentlyContinue
        if ($s) {
            Write-Output "$($s.Name): $($s.Status) (StartType: $($s.StartType))"
        } else {
            Write-Output "${svc}: NON TROVATO"
        }
    }
}

# NOTE: event log queries may take 30+ seconds on systems with large logs
Write-Section "EVENT LOG - ERRORI CRITICI (ultime 24h)" {
    $since = (Get-Date).AddHours(-24)
    Get-EventLog -LogName System -EntryType Error -After $since -Newest 30 -ErrorAction SilentlyContinue |
        Format-Table TimeGenerated, Source, EventID, Message -Wrap -AutoSize
}

Write-Section "EVENT LOG - APPLICATION ERRORS (ultime 24h)" {
    $since = (Get-Date).AddHours(-24)
    Get-EventLog -LogName Application -EntryType Error -After $since -Newest 20 -ErrorAction SilentlyContinue |
        Format-Table TimeGenerated, Source, EventID, Message -Wrap -AutoSize
}

Write-Section "CONFIGURAZIONE RETE" {
    Get-NetIPConfiguration -ErrorAction SilentlyContinue | Format-List InterfaceAlias, IPv4Address, IPv4DefaultGateway, DNSServer
}

# NOTE: may take a few seconds on systems with many connections
Write-Section "CONNESSIONI ATTIVE (porte in ascolto)" {
    try {
        Get-NetTCPConnection -State Listen -ErrorAction Stop |
            Sort-Object LocalPort |
            Select-Object LocalAddress, LocalPort, OwningProcess,
                @{N="Process";E={(Get-Process -Id $_.OwningProcess -ErrorAction SilentlyContinue).ProcessName}} |
            Format-Table -AutoSize
    } catch {
        # Fallback for Windows 7 / Server 2008 where Get-NetTCPConnection is unavailable
        netstat -an | Select-String "LISTENING"
    }
}

# NOTE: hotfix enumeration may take a while on heavily patched systems
Write-Section "AGGIORNAMENTI WINDOWS RECENTI" {
    Get-HotFix | Sort-Object InstalledOn -Descending | Select-Object -First 15 |
        Format-Table HotFixID, Description, InstalledOn -AutoSize
}

Write-Section "UPTIME E BOOT" {
    try {
        $os = Get-CimInstance Win32_OperatingSystem
    } catch {
        $os = Get-WmiObject Win32_OperatingSystem
    }
    $lastBoot = $os.LastBootUpTime
    # WmiObject returns a string date, CimInstance returns DateTime - normalize
    if ($lastBoot -is [string]) {
        $lastBoot = [System.Management.ManagementDateTimeConverter]::ToDateTime($lastBoot)
    }
    $uptime = (Get-Date) - $lastBoot
    Write-Output "Ultimo boot: $lastBoot"
    Write-Output "Uptime: $($uptime.Days) giorni, $($uptime.Hours) ore, $($uptime.Minutes) minuti"
}

Write-Section "PROCESSI - TOP 15 PER MEMORIA" {
    Get-Process | Sort-Object WorkingSet64 -Descending | Select-Object -First 15 |
        Format-Table Name, Id,
            @{N="RAM MB";E={[math]::Round($_.WorkingSet64/1MB,1)}},
            @{N="CPU sec";E={[math]::Round($_.CPU,1)}} -AutoSize
}

# NOTE: scheduled task enumeration may take time on systems with many tasks
Write-Section "TASK SCHEDULATI FALLITI" {
    Get-ScheduledTask -ErrorAction SilentlyContinue |
        Where-Object { $_.LastTaskResult -ne 0 -and $_.LastTaskResult -ne 267011 -and $_.State -ne "Disabled" } |
        Select-Object -First 15 |
        Format-Table TaskName, LastRunTime, LastTaskResult, State -AutoSize
}

Write-Section "FIREWALL PROFILI" {
    try {
        Get-NetFirewallProfile -ErrorAction Stop | Format-Table Name, Enabled, DefaultInboundAction, DefaultOutboundAction
    } catch {
        # Fallback for Windows 7 / Server 2008 where Get-NetFirewallProfile is unavailable
        netsh advfirewall show allprofiles
    }
}

Write-Section "UTENTI LOCALI" {
    Get-LocalUser -ErrorAction SilentlyContinue | Format-Table Name, Enabled, LastLogon, PasswordLastSet
}

Write-Host ""
Write-Host "[COMPLETATO] Report salvato: $outputFile" -ForegroundColor Cyan
Write-Host "  Dimensione: $([math]::Round((Get-Item $outputFile).Length / 1KB, 1)) KB" -ForegroundColor Gray
