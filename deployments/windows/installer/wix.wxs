# deployments/windows/installer/installer.ps1
# Simple PowerShell installer for ghost-silicon on Windows 11.
# Copies the binary, configs, and profiles to the install directory,
# then registers the application in the Windows registry.
# Usage: .\installer.ps1 [-InstallDir "C:\Program Files\ghost-silicon"]

param(
    [string]$InstallDir  = "$env:ProgramFiles\ghost-silicon",
    [string]$BinaryPath  = "$PSScriptRoot\..\..\..\bin\ghost-silicon.exe",
    [string]$ConfigsDir  = "$PSScriptRoot\..\..\..\configs",
    [string]$ProfilesDir = "$PSScriptRoot\..\..\..\profiles"
)

$ErrorActionPreference = "Stop"

if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Error "Run this script as Administrator."
    exit 1
}

Write-Host "Installing ghost-silicon to $InstallDir" -ForegroundColor Cyan

# Create directories
New-Item -ItemType Directory -Force -Path $InstallDir              | Out-Null
New-Item -ItemType Directory -Force -Path "$InstallDir\configs"    | Out-Null
New-Item -ItemType Directory -Force -Path "$InstallDir\profiles"   | Out-Null

# Copy files
Copy-Item -Force $BinaryPath  "$InstallDir\ghost-silicon.exe"
Copy-Item -Force -Recurse "$ConfigsDir\*"  "$InstallDir\configs\"
Copy-Item -Force -Recurse "$ProfilesDir\*" "$InstallDir\profiles\"

# Register uninstall info in Add/Remove Programs
$regPath = "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\GhostSilicon"
New-Item -Path $regPath -Force | Out-Null
Set-ItemProperty -Path $regPath -Name "DisplayName"          -Value "Ghost-Silicon"
Set-ItemProperty -Path $regPath -Name "UninstallString"      -Value "$InstallDir\uninstall.ps1"
Set-ItemProperty -Path $regPath -Name "InstallLocation"      -Value $InstallDir
Set-ItemProperty -Path $regPath -Name "Publisher"            -Value "Ghost-Silicon Project"
Set-ItemProperty -Path $regPath -Name "NoModify"             -Value 1 -Type DWord
Set-ItemProperty -Path $regPath -Name "NoRepair"             -Value 1 -Type DWord

Write-Host "Installation complete." -ForegroundColor Green
Write-Host "Binary:  $InstallDir\ghost-silicon.exe"
Write-Host "Configs: $InstallDir\configs\"