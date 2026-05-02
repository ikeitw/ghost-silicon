# scripts/test.ps1
# Runs the full test suite on Windows.
# Usage: .\scripts\test.ps1 [-Race] [-Cover]

param(
    [switch]$Race,
    [switch]$Cover
)

$ErrorActionPreference = "Stop"

$env:GOOS   = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

$Args = @("test", "./...")

if ($Race)  { $Args += "-race" }
if ($Cover) { $Args += "-coverprofile=coverage.out" }

$Args += "-timeout=120s"
$Args += "-v"

Write-Host "Running tests..." -ForegroundColor Cyan
& go @Args

if ($LASTEXITCODE -ne 0) {
    Write-Host "Tests failed." -ForegroundColor Red
    exit 1
}

if ($Cover) {
    go tool cover -html=coverage.out -o coverage.html
    Write-Host "Coverage report: coverage.html" -ForegroundColor Green
}

Write-Host "All tests passed." -ForegroundColor Green