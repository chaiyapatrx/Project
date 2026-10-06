[CmdletBinding()]
param([switch]$StopMySQL, [string]$MySQLBin = 'C:\xampp\mysql\bin')
$ErrorActionPreference = 'Stop'
$serverExe = Join-Path $PSScriptRoot 'backend-go\AUCCServer.exe'
# Windows Stop-Process terminates immediately. Use Ctrl+C for a foreground server.
Get-Process AUCCServer -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $serverExe } | Stop-Process
Write-Host 'AUCC backend stopped. Database remains running unless -StopMySQL is supplied.'
if ($StopMySQL) {
    . (Join-Path $PSScriptRoot 'scripts\aucc-common.ps1')
    $config = Read-AuccDatabaseConfig $PSScriptRoot
    if ($config.DB_HOST -notin @('localhost','127.0.0.1','::1','[::1]')) { throw 'Refusing to shut down a remote database.' }
    $mysqlAdmin = Find-AuccMySQLTool 'mysqladmin' $MySQLBin
    $nativeArgs = @(Get-AuccMySQLArguments $config $mysqlAdmin) + @('shutdown')
    $previousPassword = $env:MYSQL_PWD
    try {
        $env:MYSQL_PWD = $config.DB_PASS
        & $mysqlAdmin @nativeArgs
        if ($LASTEXITCODE -ne 0) { throw 'MySQL shutdown failed. Database was not forcibly terminated; use its service manager.' }
    } finally { $env:MYSQL_PWD = $previousPassword }
}
