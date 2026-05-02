# scripts/package.ps1
# Builds and packages ghost-silicon into a zip archive for distribution.
# Usage: .\scripts\package.ps1 [-Version "0.1.0"] [-OutDir "dist"]

param(
    [string]$Version = "0.1.0",
    [string]$OutDir  = "dist"
)

$ErrorActionPreference = "Stop"

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

# Build first
.\scripts\build.ps1 -OutDir "bin" -Version $Version

$ZipName = "ghost-silicon-v$Version-windows-amd64.zip"
$ZipPath = Join-Path $OutDir $ZipName

Write-Host "Packaging $ZipName..." -ForegroundColor Cyan

Compress-Archive -Force -Path @(
    "bin\ghost-silicon.exe",
    "configs\",
    "profiles\",
    "README.md",
    "LICENSE"
) -DestinationPath $ZipPath

Write-Host "Package ready: $ZipPath" -ForegroundColor Green