# scripts/lint.ps1
# Runs staticcheck and go vet on the entire codebase.
# Usage: .\scripts\lint.ps1

$ErrorActionPreference = "Stop"

$env:GOOS   = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

Write-Host "Running go vet..." -ForegroundColor Cyan
go vet ./...
if ($LASTEXITCODE -ne 0) { Write-Host "go vet failed." -ForegroundColor Red; exit 1 }

Write-Host "Checking staticcheck..." -ForegroundColor Cyan
$sc = Get-Command staticcheck -ErrorAction SilentlyContinue
if ($sc) {
    staticcheck ./...
    if ($LASTEXITCODE -ne 0) { Write-Host "staticcheck failed." -ForegroundColor Red; exit 1 }
} else {
    Write-Host "staticcheck not found — skipping (install: go install honnef.co/go/tools/cmd/staticcheck@latest)" -ForegroundColor Yellow
}

Write-Host "Lint passed." -ForegroundColor Green