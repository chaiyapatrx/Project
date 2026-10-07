#define AppName "AUCC Server"
#ifndef AppVersion
#define AppVersion "1.0.0"
#endif
[Setup]
AppId={{AE304159-94AB-45EA-A68F-59E3E8A3A110}
AppName={#AppName}
AppVersion={#AppVersion}
DefaultDirName={commonappdata}\AUCC Server
DisableDirPage=yes
DefaultGroupName=AUCC Server
DisableProgramGroupPage=yes
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
WizardStyle=modern
OutputDir=dist
OutputBaseFilename=AUCCServerSetup-{#AppVersion}
Compression=lzma2/fast
SolidCompression=yes
CloseApplications=yes
RestartApplications=no
UninstallDisplayIcon={app}\backend-go\AUCCServer.exe

[Files]
Source: "dist\payload\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\Open AUCC"; Filename: "{app}\AUCC.url"
Name: "{group}\Admin login"; Filename: "{app}\admin-login.txt"
Name: "{group}\Client installer"; Filename: "{win}\explorer.exe"; Parameters: "/select,""{app}\AUCCClientSetup.exe"""
Name: "{commondesktop}\AUCC"; Filename: "{app}\AUCC.url"
Name: "{commondesktop}\AUCC Client installer"; Filename: "{win}\explorer.exe"; Parameters: "/select,""{app}\AUCCClientSetup.exe"""

[Run]
Filename: "{app}\admin-login.txt"; Description: "Show the initial admin login"; Flags: shellexec postinstall skipifsilent
Filename: "{app}\AUCC.url"; Description: "Open AUCC"; Flags: shellexec postinstall skipifsilent

[Code]
function RunOperation(Operation: String): String;
var
  Code: Integer;
begin
  Result := '';
  if not Exec(ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -ExecutionPolicy Bypass -File "' + ExpandConstant('{app}\scripts\install-server.ps1') +
    '" -Mode ' + Operation + ' -Root "' + ExpandConstant('{app}') + '"',
    '', SW_HIDE, ewWaitUntilTerminated, Code) or (Code <> 0) then
    Result := 'AUCC could not complete ' + Operation + '. Read ' + ExpandConstant('{app}\setup.log') + ' and run Setup again. Your database is preserved.';
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  Result := '';
  if FileExists(ExpandConstant('{app}\scripts\install-server.ps1')) then
    Result := RunOperation('Stop');
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  Error: String;
begin
  if CurStep = ssPostInstall then
  begin
    WizardForm.StatusLabel.Caption := 'Preparing database, HTTPS, client installer and automatic startup...';
    Error := RunOperation('Install');
    if Error <> '' then RaiseException(Error);
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Error: String;
begin
  if CurUninstallStep = usUninstall then
  begin
    Error := RunOperation('Remove');
    if Error <> '' then RaiseException(Error);
  end;
end;
