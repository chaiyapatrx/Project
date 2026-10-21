# Shared helper for the server installer. Never print secrets.
function Protect-LabAccessSecretPath {
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
