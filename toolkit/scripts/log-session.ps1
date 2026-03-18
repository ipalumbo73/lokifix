function Start-WolfixLog {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$SessionType,

        [Parameter(Mandatory = $false)]
        [string]$OutputDir = (Join-Path $PSScriptRoot '..\logs')
    )

    $OutputDir = [System.IO.Path]::GetFullPath($OutputDir)

    if (-not (Test-Path -Path $OutputDir)) {
        New-Item -Path $OutputDir -ItemType Directory -Force | Out-Null
    }

    $hostname = $env:COMPUTERNAME
    $timestamp = Get-Date -Format 'yyyy-MM-dd_HH-mm-ss'
    $filename = "wolfix_${hostname}_${timestamp}.log"
    $logPath = Join-Path $OutputDir $filename

    Start-Transcript -Path $logPath -Append | Out-Null

    Write-Host "WolfixLog started: $logPath"
    Write-Host "Session type: $SessionType"

    return $logPath
}

function Stop-WolfixLog {
    [CmdletBinding()]
    param()

    try {
        Stop-Transcript | Out-Null
        Write-Host 'WolfixLog stopped.'
    }
    catch {
        Write-Warning "No active transcript to stop: $_"
    }
}
