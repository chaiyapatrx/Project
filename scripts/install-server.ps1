[CmdletBinding()]
param(
    [ValidateSet('Install','Run','Stop','Remove','Export')][string]$Mode = 'Install',
    [string]$Root = (Split-Path $PSScriptRoot),
    [string]$ServerName = $env:COMPUTERNAME
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'aucc-common.ps1')
$Root = [IO.Path]::GetFullPath($Root)
$dbExe = Join-Path $Root 'mariadb\bin\mariadbd.exe'
$serverExe = Join-Path $Root 'backend-go\AUCCServer.exe'
$setupExe = Join-Path $Root 'backend-go\AUCCProvision.exe'
$taskName = 'AUCC Server'
$stopFile = Join-Path $Root 'stop-request'

function Invoke-Setup([string]$Operation) {
    & $setupExe $Operation $Root $ServerName
    if ($LASTEXITCODE -ne 0) { throw "Server preparation failed ($Operation)." }
}

function Clear-RuntimeOverrides {
    foreach ($line in Get-Content -LiteralPath (Join-Path $Root '.env') -Encoding UTF8) {
        if ($line -match '^([A-Z_]+)=') { [Environment]::SetEnvironmentVariable($Matches[1], $null, 'Process') }
    }
}

function Wait-HTTPS {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri 'https://localhost:8000/health/ready' -TimeoutSec 2
            if ($response.StatusCode -eq 200) { return }
        } catch { }
        Start-Sleep -Seconds 1
    }
    throw 'HTTPS readiness failed; inspect backend-go/server.err.log.'
}

function Wait-Database {
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        $socket = [Net.Sockets.TcpClient]::new()
        try { if ($socket.ConnectAsync('127.0.0.1', 3308).Wait(500) -and $socket.Connected) { return } }
        catch { } finally { $socket.Dispose() }
        Start-Sleep -Seconds 1
    }
    throw 'The private database did not start; inspect database.err.log.'
}

function Start-Database {
    $existing = Get-Process -Name mariadbd -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $dbExe }
    if ($existing) { return $existing }
    $arguments = @('--defaults-file="' + (Join-Path $Root 'my.ini') + '"')
    $initFile = Join-Path $Root 'database-init.sql'
    if (Test-Path -LiteralPath $initFile) { $arguments += '--init-file="' + $initFile + '"' }
    Start-Process -FilePath $dbExe -ArgumentList $arguments -WorkingDirectory $Root -WindowStyle Hidden -PassThru `
        -RedirectStandardOutput (Join-Path $Root 'database.out.log') -RedirectStandardError (Join-Path $Root 'database.err.log')
}

function Start-Backend {
    Start-Process -FilePath $serverExe -WorkingDirectory (Join-Path $Root 'backend-go') -WindowStyle Hidden -PassThru `
        -RedirectStandardOutput (Join-Path $Root 'backend-go\server.out.log') -RedirectStandardError (Join-Path $Root 'backend-go\server.err.log')
}

function Stop-OwnedProcesses {
    $task = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
    if ($task) {
        if ($task.Actions.Arguments -notlike ('*"' + $Root + '"*')) { throw 'A different AUCC installation owns the startup task.' }
    }
    [IO.File]::WriteAllText($stopFile, 'stop')
    try {
        Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $serverExe } | Stop-Process -Force
        Stop-Database
        if ($task) { Stop-ScheduledTask -TaskName $taskName }
    } finally {
        if ($task -and (Get-ScheduledTask -TaskName $taskName).State -eq 'Running') { Stop-ScheduledTask -TaskName $taskName }
        if (Test-Path -LiteralPath $stopFile) { Remove-Item -LiteralPath $stopFile -Force }
    }
}

function Stop-Database {
    $database = Get-Process -Name mariadbd -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $dbExe }
    if (-not $database) { return }
    $rootPassword = (Get-Content -LiteralPath (Join-Path $Root 'database-admin.txt') -Raw).Trim().Substring('MariaDB root (localhost only): '.Length)
    $options = Join-Path $Root 'database-shutdown.cnf'
    try {
        [IO.File]::WriteAllText($options, "[client]`nuser=root`npassword=$rootPassword`nhost=127.0.0.1`nport=3308`nprotocol=tcp`n", [Text.Encoding]::ASCII)
        Protect-AuccSecretPath $options
        $admin = Join-Path $Root 'mariadb\bin\mariadb-admin.exe'
        $shutdown = Start-Process -FilePath $admin -ArgumentList ('--defaults-file="' + $options + '"'),'--connect-timeout=5','shutdown' `
            -WindowStyle Hidden -PassThru -RedirectStandardError (Join-Path $Root 'database-shutdown.log')
        $null = $shutdown.Handle
        if (-not $shutdown.WaitForExit(15000) -or $shutdown.ExitCode -ne 0) { throw 'Safe database shutdown failed; the database was not force-killed.' }
        if (-not $database.WaitForExit(15000)) { throw 'Database shutdown timed out; do not replace its files yet.' }
    } finally {
        if (Test-Path -LiteralPath $options) { Remove-Item -LiteralPath $options -Force }
    }
}

function Export-Client {
    $package = Join-Path $Root 'client-package'
    Copy-Item -Path (Join-Path $Root 'client-template\*') -Destination $package -Recurse -Force
    $version = (Get-Content -LiteralPath (Join-Path $package 'VERSION') -Raw).Trim()
    $builder = Start-Process -FilePath (Join-Path $Root 'compiler\ISCC.exe') -ArgumentList '/Qp',('/DAppVersion=' + $version),
        '/DDeploymentPackage','/DEmbeddedDeployment',('/O"' + $Root + '"'),'/FAUCCClientSetup',('"' + (Join-Path $package 'agent_installer.iss') + '"') `
        -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $package 'build.log') -RedirectStandardError (Join-Path $package 'build.err.log')
    $null = $builder.Handle
    if (-not $builder.WaitForExit(180000)) { $builder.Kill(); throw 'Client packaging timed out.' }
    if ($builder.ExitCode -ne 0 -or -not (Test-Path -LiteralPath (Join-Path $Root 'AUCCClientSetup.exe'))) { throw "Client packaging failed (compiler exit $($builder.ExitCode)); inspect client-package/build.err.log." }
}

try {
    if ($Mode -eq 'Run') {
        Clear-RuntimeOverrides
        # ponytail: one server per Windows host; use services if multiple instances are needed.
        while (-not (Test-Path -LiteralPath $stopFile)) {
            $database = Start-Database
            Wait-Database
            $backend = Start-Backend
            while (-not $backend.HasExited -and -not $database.HasExited -and -not (Test-Path -LiteralPath $stopFile)) { Start-Sleep -Seconds 2 }
            if (-not $backend.HasExited) { $backend.Kill(); $backend.WaitForExit() }
            Start-Sleep -Seconds 2
        }
    } elseif ($Mode -in @('Stop','Remove')) {
        Stop-OwnedProcesses
        if ($Mode -eq 'Remove') {
            Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
            Get-NetFirewallRule -DisplayName 'AUCC HTTPS 8000 LAN' -ErrorAction SilentlyContinue | Remove-NetFirewallRule
            # Private data, credentials and certificates are deliberately preserved for recovery.
        }
    } elseif ($Mode -eq 'Export') {
        Export-Client
    } else {
        $identity = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
        if (-not $identity.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Run Server Setup as administrator.' }
        Protect-AuccSecretPath $Root
        Stop-OwnedProcesses
        foreach ($port in @(8000,3308)) {
            if (Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue) { throw "Port $port is already used by another program." }
        }
        Invoke-Setup 'prepare'
        Clear-RuntimeOverrides
        $ca = Join-Path $Root 'tls\ca.pem'
        Import-Certificate -FilePath $ca -CertStoreLocation Cert:\LocalMachine\Root | Out-Null
        if (-not (Test-Path -LiteralPath (Join-Path $Root 'data\mysql'))) {
            if (-not (Test-Path -LiteralPath (Join-Path $Root 'database-init.sql'))) { throw 'Database data is missing. Restore the database backup instead of initializing an empty replacement.' }
            $initializer = Join-Path $Root 'mariadb\bin\mariadb-install-db.exe'
            & $initializer ('--datadir=' + (Join-Path $Root 'data')) '--port=3308' | Out-Null
            if ($LASTEXITCODE -ne 0) { throw 'Database initialization failed; rerun Setup after checking setup.log.' }
        }
        $database = Start-Database
        Wait-Database
        # Use the shipped migration runner unchanged, with its own Python runtime.
        & (Join-Path $Root 'python\python.exe') (Join-Path $Root 'migrations\apply_migrations.py')
        if ($LASTEXITCODE -ne 0) { throw 'Database migration failed.' }
        $backend = Start-Backend
        Wait-HTTPS
        Invoke-Setup 'finish'
        Export-Client
        Stop-OwnedProcesses
        $action = New-ScheduledTaskAction -Execute "$env:WINDIR\System32\WindowsPowerShell\v1.0\powershell.exe" `
            -Argument ('-NoProfile -ExecutionPolicy Bypass -File "' + $PSCommandPath + '" -Mode Run -Root "' + $Root + '"') -WorkingDirectory $Root
        $settings = New-ScheduledTaskSettingsSet -StartWhenAvailable -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) `
            -ExecutionTimeLimit ([TimeSpan]::Zero) -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
        Register-ScheduledTask -TaskName $taskName -Action $action -Trigger (New-ScheduledTaskTrigger -AtStartup) `
            -User SYSTEM -RunLevel Highest -Settings $settings -Force | Out-Null
        & (Join-Path $Root 'enable-aucc-lan-firewall.ps1') -RemoteAddress LocalSubnet -Port 8000 | Out-Null
        # Only this backend and its local subnet, including Windows' default Public LAN profile.
        Get-NetFirewallRule -DisplayName 'AUCC HTTPS 8000 LAN' | Set-NetFirewallRule -Profile Any
        Get-NetFirewallRule -DisplayName 'AUCC HTTPS 8000 LAN' | Get-NetFirewallApplicationFilter | Set-NetFirewallApplicationFilter -Program $serverExe
        Start-ScheduledTask -TaskName $taskName
        Wait-HTTPS
        Write-Output 'AUCC is ready. Open AUCC.url; copy AUCCClientSetup.exe to the client computers.'
    }
} catch {
    $failure = $_
    if ($Mode -eq 'Install') {
        try { Stop-OwnedProcesses } catch { $_ | Out-String | Add-Content -LiteralPath (Join-Path $Root 'setup.log') -Encoding UTF8 }
    }
    $failure | Out-String | Add-Content -LiteralPath (Join-Path $Root 'setup.log') -Encoding UTF8
    Write-Error $failure -ErrorAction Continue
    exit 1
}
