# deployments/windows/firewall/rules.ps1
# Creates Windows Firewall rules for ghost-silicon.
# These rules allow the supervisor to communicate over the named pipe
# and optionally restrict renderer outbound traffic.
# Usage: .\rules.ps1 [-Remove]

param(
    [switch]$Remove,
    [string]$BinaryPath = "$env:ProgramFiles\ghost-silicon\ghost-silicon.exe"
)

$ErrorActionPreference = "Stop"
$RuleGroup = "Ghost-Silicon"

function Remove-GSRules {
    Write-Host "Removing Ghost-Silicon firewall rules..." -ForegroundColor Yellow
    Get-NetFirewallRule -Group $RuleGroup -ErrorAction SilentlyContinue |
        Remove-NetFirewallRule
    Write-Host "Rules removed." -ForegroundColor Green
}

function Add-GSRules {
    # Remove any stale rules first.
    Get-NetFirewallRule -Group $RuleGroup -ErrorAction SilentlyContinue |
        Remove-NetFirewallRule

    # Allow supervisor outbound (HTTP/HTTPS for any proxy or network calls).
    New-NetFirewallRule `
        -Name          "GS-Supervisor-Out" `
        -DisplayName   "Ghost-Silicon Supervisor Outbound" `
        -Group         $RuleGroup `
        -Direction     Outbound `
        -Action        Allow `
        -Program       $BinaryPath `
        -Protocol      TCP `
        -RemotePort    80,443 `
        -Enabled       True `
        | Out-Null

    # Block any inbound connections to the supervisor binary
    # (the named pipe is already restricted by DACL; this is defence-in-depth).
    New-NetFirewallRule `
        -Name          "GS-Supervisor-In-Block" `
        -DisplayName   "Ghost-Silicon Supervisor Inbound Block" `
        -Group         $RuleGroup `
        -Direction     Inbound `
        -Action        Block `
        -Program       $BinaryPath `
        -Enabled       True `
        | Out-Null

    Write-Host "Ghost-Silicon firewall rules applied." -ForegroundColor Green
}

if ($Remove) {
    Remove-GSRules
} else {
    Add-GSRules
}