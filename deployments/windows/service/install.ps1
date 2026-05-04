# deployments/windows/service/install.ps1
# Installs ghost-silicon as a Windows Service.
# Must be run as Administrator.

param(
    [string]$BinaryPath = "$PSScriptRoot\..\..\..\..\bin\ghost-silicon-svc.exe",
    [string]$ConfigPath = "$PSScriptRoot\..\..\..\..\configs\ghost-silicon.yaml",
    [string]$ServiceName = "GhostSilicon",
    [string]$DisplayName = "Ghost-Silicon Browser Supervisor",
    [string]$Description = "Manages isolated browser sessions with profile-based identity control."
)

$ErrorActionPreference = "Stop"

if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Error "This script must be run as Administrator."
    exit 1
}

$BinaryPath = Resolve-Path $BinaryPath -ErrorAction Stop
$ConfigPath  = Resolve-Path $ConfigPath  -ErrorAction Stop

$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existing) {
    Write-Host "Stopping existing service..." -ForegroundColor Yellow
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    sc.exe delete $ServiceName | Out-Null
}

$binWithArgs = "`"$BinaryPath`" -config `"$ConfigPath`""

New-Service `
    -Name        $ServiceName `
    -BinaryPathName $binWithArgs `
    -DisplayName $DisplayName `
    -Description $Description `
    -StartupType Automatic `
    | Out-Null

Write-Host "Service installed: $ServiceName" -ForegroundColor Green
Write-Host "Start with: Start-Service $ServiceName"