[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$project = Split-Path $PSScriptRoot
$stage = Join-Path $PSScriptRoot 'dist\payload'
$cache = Join-Path $PSScriptRoot 'cache'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

function Get-VerifiedArchive([string]$URL, [string]$Name, [string]$SHA256) {
    $path = Join-Path $cache $Name
    if (-not (Test-Path -LiteralPath $path)) { Invoke-WebRequest -UseBasicParsing -Uri $URL -OutFile $path }
    $stream = [IO.File]::OpenRead($path)
    $hasher = [Security.Cryptography.SHA256]::Create()
    try { $actual = [BitConverter]::ToString($hasher.ComputeHash($stream)).Replace('-','').ToLowerInvariant() }
    finally { $stream.Dispose(); $hasher.Dispose() }
    if ($actual -ne $SHA256) { throw "Archive checksum mismatch: $Name. Remove this cached archive and rerun the build." }
    return $path
}

function Invoke-Build([string]$Tool, [string[]]$Arguments) {
    & $Tool @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Tool failed (exit $LASTEXITCODE)." }
}

if (Test-Path -LiteralPath $stage) {
    if ((Get-Item -LiteralPath $stage).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Refusing to remove a linked build directory.' }
    $resolvedStage = (Resolve-Path -LiteralPath $stage).Path
    if ($resolvedStage -ne ([IO.Path]::GetFullPath((Join-Path $PSScriptRoot 'dist\payload')))) { throw 'Unsafe build staging path.' }
    Remove-Item -LiteralPath $resolvedStage -Recurse -Force
}
New-Item -ItemType Directory -Path $stage,$cache -Force | Out-Null
$compiler = @(
    "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe", "$env:LOCALAPPDATA\Programs\Inno Setup 7\ISCC.exe",
    "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe", "$env:ProgramFiles\Inno Setup 7\ISCC.exe"
) | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
if (-not $compiler) { throw 'Install Inno Setup 6/7 on the build computer.' }
$mariaVersion = '11.4.11'
$mariaZip = Get-VerifiedArchive "https://archive.mariadb.org/mariadb-$mariaVersion/winx64-packages/mariadb-$mariaVersion-winx64.zip" `
    "mariadb-$mariaVersion.zip" 'dc8b121a2c0c34a12bd8f4aec37592e00734468aee578030a7f5c971adf68255'
$pythonZip = Get-VerifiedArchive 'https://www.python.org/ftp/python/3.13.16/python-3.13.16-embed-amd64.zip' `
    'python-3.13.16.zip' '97dae5274cc54867065e8d5a3226e48c35017ed332a0fdb0e27d5b5821961297'
if (-not (Test-Path -LiteralPath (Join-Path $cache "mariadb-$mariaVersion-winx64"))) { Expand-Archive -LiteralPath $mariaZip -DestinationPath $cache }
New-Item -ItemType Directory -Path (Join-Path $stage 'mariadb') -Force | Out-Null
Copy-Item -Path (Join-Path $cache "mariadb-$mariaVersion-winx64\*") -Destination (Join-Path $stage 'mariadb') -Recurse -Force
Expand-Archive -LiteralPath $pythonZip -DestinationPath (Join-Path $stage 'python') -Force
Invoke-Build 'python' @('-m','pip','install','--disable-pip-version-check','--no-compile','--no-deps','--upgrade',
    '--target',(Join-Path $stage 'python\Lib\site-packages'),'-r',(Join-Path $project 'migrations\requirements.txt'))
[IO.File]::WriteAllText((Join-Path $stage 'python\python313._pth'), "python313.zip`n.`nLib/site-packages`n../migrations`n", [Text.Encoding]::ASCII)
foreach ($dir in @('backend-go','frontend\dist','migrations','scripts','client-template\dist','compiler')) { New-Item -ItemType Directory -Path (Join-Path $stage $dir) -Force | Out-Null }
Get-ChildItem -LiteralPath (Split-Path $compiler) -File | Where-Object { $_.Name -notlike 'unins*' } | Copy-Item -Destination (Join-Path $stage 'compiler') -Force
Copy-Item -LiteralPath (Join-Path $project 'client-go\agent_installer.iss'),(Join-Path $project 'client-go\VERSION') -Destination (Join-Path $stage 'client-template') -Force
Copy-Item -Path (Join-Path $project 'migrations\*.up.sql'),(Join-Path $project 'migrations\apply_migrations.py') -Destination (Join-Path $stage 'migrations') -Force
Copy-Item -Path (Join-Path $project 'scripts\aucc-common.ps1'),(Join-Path $project 'scripts\install-server.ps1') -Destination (Join-Path $stage 'scripts') -Force
Copy-Item -LiteralPath (Join-Path $project 'enable-aucc-lan-firewall.ps1') -Destination $stage -Force
$version = (Get-Content -LiteralPath (Join-Path $project 'client-go\VERSION') -Raw).Trim()
$savedGo = @($env:GOOS,$env:GOARCH,$env:CGO_ENABLED)
Push-Location $project
try {
    $env:GOOS = 'windows'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
    Push-Location 'frontend'
    try {
        Invoke-Build 'npm.cmd' @('ci')
        # Windows PowerShell removes empty env values; set the empty override inside Node instead.
        Invoke-Build 'node' @('-e', "require('node:child_process').execFileSync(process.execPath, ['node_modules/vite/bin/vite.js', 'build'], {stdio: 'inherit', env: {...process.env, VITE_API_BASE_URL: ''}})")
        Copy-Item -Path 'dist\*' -Destination (Join-Path $stage 'frontend\dist') -Recurse -Force
    } finally { Pop-Location }
    Push-Location 'backend-go'
    try {
        Invoke-Build 'go' @('build','-trimpath','-o',(Join-Path $stage 'backend-go\AUCCServer.exe'),'./cmd/server')
        Invoke-Build 'go' @('build','-trimpath','-o',(Join-Path $stage 'backend-go\AUCCProvision.exe'),'./cmd/setup')
    } finally { Pop-Location }
    Push-Location 'client-go'
    try {
        New-Item -ItemType Directory -Path 'dist' -Force | Out-Null
        Invoke-Build 'go' @('build','-trimpath',('-ldflags=-s -w -H=windowsgui -X main.agentVersion=' + $version),'-o','dist\AUCCAgent.exe','.')
        Invoke-Build 'go' @('build','-trimpath','-ldflags=-s -w -H=windowsgui','-o','dist\AUCCUpdater.exe','./updater')
        Invoke-Build $compiler @('/Qp',('/DAppVersion=' + $version),'/DDeploymentPackage','agent_installer.iss')
        Copy-Item -LiteralPath 'dist\AUCCAgent.exe','dist\AUCCUpdater.exe' -Destination (Join-Path $stage 'client-template\dist') -Force
    } finally { Pop-Location }
    Invoke-Build $compiler @('/Qp',('/DAppVersion=' + $version),(Join-Path $PSScriptRoot 'server_installer.iss'))
} finally {
    Pop-Location
    $env:GOOS = $savedGo[0]; $env:GOARCH = $savedGo[1]; $env:CGO_ENABLED = $savedGo[2]
}
Write-Output "Ready: $PSScriptRoot\dist\AUCCServerSetup-$version.exe"
