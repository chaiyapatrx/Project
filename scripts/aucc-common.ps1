# Shared configuration for Windows operational scripts. Never print secrets.
function Protect-AuccSecretPath {
    param([string]$Path, [System.Security.Principal.SecurityIdentifier]$Account = [System.Security.Principal.WindowsIdentity]::GetCurrent().User)
    $resolved = (Resolve-Path -LiteralPath $Path -ErrorAction Stop).Path
    $item = Get-Item -LiteralPath $resolved -Force
    if ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) { throw 'Refusing to change a linked secret path.' }
    $directory = $item.PSIsContainer
    if ($directory) { $acl = [System.IO.Directory]::GetAccessControl($resolved) }
    else { $acl = [System.IO.File]::GetAccessControl($resolved) }
    $acl.SetAccessRuleProtection($true, $false)
    foreach ($rule in @($acl.Access)) { [void]$acl.RemoveAccessRuleSpecific($rule) }
    $inheritance = [System.Security.AccessControl.InheritanceFlags]::None
    if ($directory) { $inheritance = [System.Security.AccessControl.InheritanceFlags]'ContainerInherit,ObjectInherit' }
    foreach ($sid in @($Account.Value, 'S-1-5-18', 'S-1-5-32-544') | Select-Object -Unique) {
        $identity = [System.Security.Principal.SecurityIdentifier]::new($sid)
        $rule = [System.Security.AccessControl.FileSystemAccessRule]::new($identity, 'FullControl', $inheritance, 'None', 'Allow')
        $acl.AddAccessRule($rule)
    }
    if ($directory) { [System.IO.Directory]::SetAccessControl($resolved, $acl) }
    else { [System.IO.File]::SetAccessControl($resolved, $acl) }
}

function Convert-AuccEnvValue {
    param([string]$Value, [hashtable]$Values)
    if ($Value.StartsWith("'")) {
        $quoted = [regex]::Match($Value, "^'([^']*)'\s*(?:#.*)?$")
        if (-not $quoted.Success) { throw 'Invalid single-quoted .env value.' }
        return $quoted.Groups[1].Value
    }
    if ($Value.StartsWith('"')) {
        $quoted = [regex]::Match($Value, '^"((?:\\.|[^"\\])*)"\s*(?:#.*)?$')
        if (-not $quoted.Success) { throw 'Invalid double-quoted .env value.' }
        $Value = [regex]::Replace($quoted.Groups[1].Value, '\\.', {
            param($match)
            switch ($match.Value) {
                '\n' { return "`n" }
                '\r' { return "`r" }
                default { return $match.Value }
            }
        })
        $Value = [regex]::Replace($Value, '\\([^$])', '$1')
    } else {
        $Value = ($Value -replace '\s+#.*$', '').Trim()
    }
    # Match the backend's godotenv expansion of earlier file entries.
    return [regex]::Replace($Value, '(\\)?(\$)(\()?\{?([A-Z0-9_]+)?\}?', {
        param($match)
        if ($match.Groups[1].Value -eq '\') { return $match.Value.Substring(1) }
        if ($match.Groups[4].Value) { return [string]$Values[$match.Groups[4].Value] }
        return $match.Value
    })
}

function Read-AuccDatabaseConfig {
    param([string]$ProjectRoot)
    $values = @{ DB_HOST = '127.0.0.1'; DB_PORT = '3306' }
    $envPath = Join-Path $ProjectRoot '.env'
    if (Test-Path -LiteralPath $envPath) {
        foreach ($line in Get-Content -LiteralPath $envPath -Encoding UTF8) {
            if ($line.Trim() -match '^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$') {
                $key = $Matches[1]
                $values[$key] = Convert-AuccEnvValue $Matches[2].Trim() $values
            }
        }
    }
    foreach ($key in @('DB_HOST','DB_PORT','DB_USER','DB_PASS','DB_NAME','DB_TLS_CA_FILE')) {
        $override = [Environment]::GetEnvironmentVariable($key, 'Process')
        if ($null -ne $override) { $values[$key] = $override }
    }
    foreach ($key in @('DB_HOST','DB_PORT','DB_USER','DB_PASS','DB_NAME')) {
        if (-not $values[$key]) { throw "Missing required setting: $key" }
    }
    $portNumber = 0
    if (-not [int]::TryParse($values.DB_PORT, [ref]$portNumber) -or $portNumber -lt 1 -or $portNumber -gt 65535) {
        throw 'DB_PORT must be between 1 and 65535.'
    }
    if ($values.DB_NAME -notmatch '^[A-Za-z0-9_]+$') { throw 'DB_NAME must contain only letters, digits and underscores for these scripts.' }
    return $values
}

function Find-AuccMySQLTool {
    param([string]$Name, [string]$MySQLBin)
    $candidate = Join-Path $MySQLBin ($Name + '.exe')
    if (Test-Path -LiteralPath $candidate) { return $candidate }
    $found = Get-Command ($Name + '.exe') -ErrorAction SilentlyContinue
    if ($found) { return $found.Source }
    throw "$Name.exe not found. Supply -MySQLBin or add MySQL tools to PATH."
}

function Get-AuccMySQLArguments {
    param([hashtable]$Config, [string]$Tool)
    $nativeArgs = @("--host=$($Config.DB_HOST)", "--port=$($Config.DB_PORT)", "--user=$($Config.DB_USER)", '--protocol=TCP')
    if ($Config.DB_HOST -notin @('localhost','127.0.0.1','::1','[::1]')) {
        $version = (& $Tool --version 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) { throw 'Cannot determine MySQL tool version.' }
        if ($version -match 'MariaDB') {
            $nativeArgs += @('--ssl', '--ssl-verify-server-cert')
        } else {
            $nativeArgs += '--ssl-mode=VERIFY_IDENTITY'
        }
        if ($Config.DB_TLS_CA_FILE) {
            if (-not (Test-Path -LiteralPath $Config.DB_TLS_CA_FILE)) { throw 'DB_TLS_CA_FILE not found.' }
            $nativeArgs += "--ssl-ca=$($Config.DB_TLS_CA_FILE)"
        }
    }
    return $nativeArgs
}
