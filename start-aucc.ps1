[CmdletBinding()]
param([string]$MySQLBin = 'C:\xampp\mysql\bin')
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'scripts\aucc-common.ps1')
$config = Read-AuccDatabaseConfig $PSScriptRoot
$backendDir = Join-Path $PSScriptRoot 'backend-go'
$serverExe = Join-Path $backendDir 'AUCCServer.exe'
if (-not (Test-Path -LiteralPath $serverExe)) { throw 'Build backend-go\AUCCServer.exe first.' }

function Test-DatabasePort {
    $socket = [System.Net.Sockets.TcpClient]::new()
    try {
        $task = $socket.ConnectAsync($config.DB_HOST.Trim('[',']'), [int]$config.DB_PORT)
        return ($task.Wait(1000) -and $socket.Connected)
    } catch { return $false } finally { $socket.Dispose() }
}
if (-not (Test-DatabasePort)) {
    if ($config.DB_HOST -notin @('localhost','127.0.0.1','::1','[::1]')) { throw 'Remote database is unreachable; no local database started.' }
    $mysqlExe = Find-AuccMySQLTool 'mysqld' $MySQLBin
    $mysqlIni = Join-Path $MySQLBin 'my.ini'
    if (-not (Test-Path -LiteralPath $mysqlIni)) { throw 'XAMPP my.ini is missing. Start your database service separately.' }
    Start-Process -FilePath $mysqlExe -ArgumentList ('--defaults-file="' + $mysqlIni + '"'),'--standalone',("--port=" + $config.DB_PORT) -WorkingDirectory $MySQLBin -WindowStyle Hidden
    $ready = $false
    for ($i = 0; $i -lt 30; $i++) {
        if (Test-DatabasePort) { $ready = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) { throw 'Database did not open the configured port.' }
}
$existing = Get-Process AUCCServer -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $serverExe }
if (-not $existing) {
    Start-Process -FilePath $serverExe -WorkingDirectory $backendDir -RedirectStandardOutput (Join-Path $backendDir 'server.out.log') -RedirectStandardError (Join-Path $backendDir 'server.err.log') -WindowStyle Hidden
}
Write-Host 'Backend started; check /health/ready and backend-go\server.err.log.'
