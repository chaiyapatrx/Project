# Run in an elevated PowerShell. Supply the actual client subnet.
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string[]]$RemoteAddress,
    [ValidateRange(1,65535)][int]$Port = 8000
)
$ErrorActionPreference = 'Stop'
$ruleName = "AUCC HTTPS $Port LAN"
$rules = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
if ($rules) {
    $rules | Set-NetFirewallRule -Enabled True -Direction Inbound -Action Allow -Profile Private
    $rules | Get-NetFirewallAddressFilter | Set-NetFirewallAddressFilter -RemoteAddress $RemoteAddress
    $rules | Get-NetFirewallPortFilter | Set-NetFirewallPortFilter -Protocol TCP -LocalPort $Port
} else {
    New-NetFirewallRule -DisplayName $ruleName -Direction Inbound -Action Allow -Protocol TCP -LocalPort $Port -Profile Private -RemoteAddress $RemoteAddress | Out-Null
}
Get-NetFirewallRule -DisplayName $ruleName | Select-Object DisplayName,Enabled,Profile
