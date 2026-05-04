# deployments/windows/scheduled-task/register.ps1
# Registers ghost-silicon as a Windows Scheduled Task that starts on logon.
# Usage: .\register.ps1

param(
    [string]$TaskName  = "GhostSilicon",
    [string]$TaskXML   = "$PSScriptRoot\task.xml"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $TaskXML)) {
    Write-Error "Task XML not found: $TaskXML"
    exit 1
}

# Remove existing task if present.
$existing = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($existing) {
    Write-Host "Removing existing task '$TaskName'..." -ForegroundColor Yellow
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
}

# Register from XML.
$xml = Get-Content -Path $TaskXML -Raw
Register-ScheduledTask -TaskName $TaskName -Xml $xml -Force | Out-Null

Write-Host "Scheduled task '$TaskName' registered." -ForegroundColor Green
Write-Host "It will start automatically on next logon."
Write-Host "Start now with: Start-ScheduledTask -TaskName '$TaskName'"