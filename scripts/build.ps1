# scripts/build.ps1
# Builds ghost-silicon for Windows 11 (amd64).
# Usage: .\scripts\build.ps1 [-OutDir "bin"] [-Version "0.1.0"]

param(
    [string]$OutDir = "bin",
    [string]$Version = "0.1.0",
    [string]$CommitHash = ""
)

$ErrorActionPreference = "Stop"

if ($CommitHash -eq "") {
    try { $CommitHash = (git rev-parse --short HEAD 2>$null) } catch { $CommitHash = "unknown" }
}

$BuildTime = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")
$LdFlags = "-X ghost-silicon/pkg/version.CommitHash=$CommitHash " +
           "-X ghost-silicon/pkg/version.BuildTime=$BuildTime " +
           "-s -w"

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

Write-Host "Building ghost-silicon v$Version ($CommitHash) for windows/amd64..." -ForegroundColor Cyan

$env:GOOS   = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"

go build -ldflags $LdFlags -o "$OutDir\ghost-silicon.exe" .\cmd\ghost-silicon

if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed." -ForegroundColor Red
    exit 1
}

Write-Host "Output: $OutDir\ghost-silicon.exe" -ForegroundColor Green