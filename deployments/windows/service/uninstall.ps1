# deployments/windows/service/uninstall.ps1
# Removes the ghost-silicon Windows Service.
# Must be run as Administrator.

param(
    [string]$ServiceName = "GhostSilicon"
)

$ErrorActionPreference = "Stop"

if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Error "This script must be run as Administrator."
    exit 1
}

$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if (-not $existing) {
    Write-Host "Service '$ServiceName' is not installed." -ForegroundColor Yellow
    exit 0
}

Write-Host "Stopping service '$ServiceName'..." -ForegroundColor Cyan
Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

sc.exe delete $ServiceName | Out-Null

Write-Host "Service '$ServiceName' removed." -ForegroundColor Green