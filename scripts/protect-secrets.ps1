[CmdletBinding()]
param(
    [string[]]$Path,
    [string]$ServiceAccount
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'aucc-common.ps1')
$account = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
if ($ServiceAccount) {
    $account = [System.Security.Principal.NTAccount]::new($ServiceAccount).Translate([System.Security.Principal.SecurityIdentifier])
}
if (-not $Path) {
    $projectRoot = Split-Path $PSScriptRoot
    $Path = @((Join-Path $projectRoot '.env'), (Join-Path $projectRoot '.env.before-xampp'), (Join-Path $projectRoot 'backups'))
}
foreach ($target in $Path) {
    if (Test-Path -LiteralPath $target) {
        Protect-AuccSecretPath $target $account
        if ([System.IO.Directory]::Exists((Resolve-Path -LiteralPath $target).Path)) {
            foreach ($file in Get-ChildItem -LiteralPath $target -File -Force) { Protect-AuccSecretPath $file.FullName $account }
        }
        Write-Output "Restricted secret access: $target"
    }
}
