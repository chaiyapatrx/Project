$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'aucc-common.ps1')
function Assert-True($Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
}
$projectRoot = Split-Path $PSScriptRoot
$temporaryRoot = Join-Path $PSScriptRoot ('.test-' + [guid]::NewGuid().ToString('N'))
$saved = @{}
foreach ($key in @('DB_HOST','DB_PORT','DB_USER','DB_PASS','DB_NAME','DB_TLS_CA_FILE')) {
    $saved[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
    [Environment]::SetEnvironmentVariable($key, $null, 'Process')
}
try {
    [void](New-Item -ItemType Directory -Path $temporaryRoot)
    @'
DB_HOST=127.0.0.1
DB_PORT=3307 # comment
DB_USER=aucc
DB_PASS="test # password=with spaces" # comment after quotes
DB_NAME=aucc_test
'@ | Set-Content -LiteralPath (Join-Path $temporaryRoot '.env') -Encoding UTF8
    $config = Read-AuccDatabaseConfig $temporaryRoot
    Protect-AuccSecretPath (Join-Path $temporaryRoot '.env')
    $secretAcl = [System.IO.File]::GetAccessControl((Join-Path $temporaryRoot '.env'))
    Assert-True $secretAcl.AreAccessRulesProtected 'Secret ACL still inherits broad rights.'
    $allowedSids = @([System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value, 'S-1-5-18', 'S-1-5-32-544')
    foreach ($rule in $secretAcl.Access) {
        Assert-True ($rule.IdentityReference.Translate([System.Security.Principal.SecurityIdentifier]).Value -in $allowedSids) 'Unexpected principal can read the secret.'
    }
    Assert-True ($config.DB_PORT -eq '3307') 'Port or inline-comment parsing failed.'
    Assert-True ($config.DB_PASS -eq 'test # password=with spaces') 'Quoted password was corrupted.'
    Assert-True ((Convert-AuccEnvValue '"quoted\"pass\\word" # comment' @{}) -eq 'quoted"pass\word') 'Escaped quoted value was corrupted.'
    Assert-True ((Convert-AuccEnvValue '${FIRST}_suffix' @{ FIRST = 'prefix' }) -eq 'prefix_suffix') 'Variable expansion failed.'
    Assert-True ((Convert-AuccEnvValue "'literal`$FIRST' # comment" @{ FIRST = 'prefix' }) -eq 'literal$FIRST') 'Single quotes expanded a literal variable.'
    Assert-True ((Convert-AuccEnvValue '\$FIRST' @{ FIRST = 'prefix' }) -eq '$FIRST') 'Escaped dollar was expanded.'
    $env:DB_PORT = '3308'
    Assert-True ((Read-AuccDatabaseConfig $temporaryRoot).DB_PORT -eq '3308') 'Environment override ignored.'
    $env:DB_PORT = '0'
    $rejected = $false
    try { Read-AuccDatabaseConfig $temporaryRoot | Out-Null } catch { $rejected = $true }
    Assert-True $rejected 'Invalid port was accepted.'

    function MockMySQL { $global:LASTEXITCODE = 0; return 'mysql Ver 8.4' }
    function MockMariaDB { $global:LASTEXITCODE = 0; return 'mysqldump Ver 10.4 MariaDB' }
    $config.DB_HOST = 'db.example.invalid'
    $mysqlArgs = @(Get-AuccMySQLArguments $config 'MockMySQL')
    $mariaArgs = @(Get-AuccMySQLArguments $config 'MockMariaDB')
    Assert-True ($mysqlArgs -contains '--ssl-mode=VERIFY_IDENTITY') 'Remote MySQL lacks verified TLS.'
    Assert-True ($mariaArgs -contains '--ssl-verify-server-cert') 'Remote MariaDB lacks verified TLS.'
    Assert-True (-not ($mysqlArgs -match 'password')) 'Credential leaked into native arguments.'

    $files = @(Get-ChildItem -LiteralPath $projectRoot -Filter *.ps1) + @(Get-ChildItem -LiteralPath $PSScriptRoot -Filter *.ps1)
    foreach ($file in $files) {
        $tokens = $null
        $parseErrors = $null
        [void][System.Management.Automation.Language.Parser]::ParseFile($file.FullName, [ref]$tokens, [ref]$parseErrors)
        Assert-True ($parseErrors.Count -eq 0) ($file.Name + ' has syntax errors.')
    }
    Write-Output 'Operational script checks passed; no database, process or firewall was changed.'
} finally {
    foreach ($key in $saved.Keys) { [Environment]::SetEnvironmentVariable($key, $saved[$key], 'Process') }
    if (Test-Path -LiteralPath $temporaryRoot) {
        $resolved = (Resolve-Path -LiteralPath $temporaryRoot).Path
        if ((Split-Path $resolved) -ne $PSScriptRoot -or (Split-Path $resolved -Leaf) -notlike '.test-*') { throw 'Unsafe test cleanup path.' }
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
