# scripts/dev.ps1
# Runs ghost-silicon in development mode with verbose logging and the mock adapter.
# Usage: .\scripts\dev.ps1 [-Config "configs\ghost-silicon.yaml"]

param(
    [string]$Config = "configs\ghost-silicon.yaml"
)

$ErrorActionPreference = "Stop"

$env:GOOS   = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
$env:GS_LOG_LEVEL = "debug"
$env:GS_LOG_FORMAT = "text"

Write-Host "Starting ghost-silicon (dev mode)..." -ForegroundColor Cyan

go run .\cmd\ghost-silicon -config $Config