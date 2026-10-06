[CmdletBinding()]
param(
    [ValidateRange(1,36500)][int]$RetentionDays = 14,
    [string]$MySQLBin = 'C:\xampp\mysql\bin'
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'scripts\aucc-common.ps1')
$config = Read-AuccDatabaseConfig $PSScriptRoot
$dump = Find-AuccMySQLTool 'mysqldump' $MySQLBin
$nativeArgs = @(Get-AuccMySQLArguments $config $dump)
$backupDir = Join-Path $PSScriptRoot 'backups'
[void](New-Item -ItemType Directory -Path $backupDir -Force)
Protect-AuccSecretPath $backupDir
$stamp = Get-Date -Format 'yyyy-MM-dd_HH-mm-ss-fff'
$backupFile = Join-Path $backupDir ($config.DB_NAME + '_backup_' + $stamp + '.sql')
$partialFile = $backupFile + '.partial'
$nativeArgs += @('--single-transaction','--quick','--routines','--triggers','--default-character-set=utf8mb4',"--result-file=$partialFile",'--databases',$config.DB_NAME)
$previousPassword = $env:MYSQL_PWD
try {
    # Child inherits password; no credential in command-line arguments.
    $env:MYSQL_PWD = $config.DB_PASS
    & $dump @nativeArgs
    if ($LASTEXITCODE -ne 0) { throw "mysqldump failed (exit $LASTEXITCODE). No backups deleted." }
    if (-not (Test-Path -LiteralPath $partialFile) -or (Get-Item -LiteralPath $partialFile).Length -eq 0) {
        throw 'mysqldump produced an empty backup. No backups deleted.'
    }
    Move-Item -LiteralPath $partialFile -Destination $backupFile
} finally {
    $env:MYSQL_PWD = $previousPassword
    if (Test-Path -LiteralPath $partialFile) { Remove-Item -LiteralPath $partialFile -Force }
}
Write-Host "Backup complete: $backupFile"
$resolvedBackupDir = (Resolve-Path -LiteralPath $backupDir).Path
foreach ($oldBackup in Get-ChildItem -LiteralPath $resolvedBackupDir -File -Filter ($config.DB_NAME + '_backup_*.sql')) {
    if ($oldBackup.DirectoryName -ne $resolvedBackupDir) { throw 'Backup retention path escaped the backup directory.' }
    if ($oldBackup.LastWriteTime -lt (Get-Date).AddDays(-$RetentionDays)) {
        Remove-Item -LiteralPath $oldBackup.FullName -Force
    }
}
