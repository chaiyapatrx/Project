#define AppName "ระบบเข้าใช้งานคอมพิวเตอร์ (เครื่องแม่)"
#ifndef AppVersion
#define AppVersion "1.0.0"
#endif
[Setup]
AppId={{AE304159-94AB-45EA-A68F-59E3E8A3A110}
AppName={#AppName}
AppVersion={#AppVersion}
DefaultDirName={commonappdata}\LabAccess Server
DisableDirPage=yes
DefaultGroupName=ระบบเข้าใช้งานคอมพิวเตอร์
DisableProgramGroupPage=yes
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
WizardStyle=modern
OutputDir=dist
OutputBaseFilename=LabAccessServerSetup-{#AppVersion}
Compression=lzma2/fast
SolidCompression=yes
CloseApplications=yes
RestartApplications=no
UninstallDisplayIcon={app}\backend-go\LabAccessServer.exe

[Files]
Source: "dist\payload\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\เปิดหน้าเว็บ"; Filename: "{app}\LabAccess.url"
Name: "{group}\Admin login (รหัสผ่านผู้ดูแลครั้งแรก)"; Filename: "{app}\admin-login.txt"
Name: "{group}\ตัวติดตั้งเครื่องลูก"; Filename: "{win}\explorer.exe"; Parameters: "/select,""{app}\LabAccessClientSetup.exe"""
Name: "{group}\สำรองข้อมูลตอนนี้"; Filename: "{sys}\WindowsPowerShell\v1.0\powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File ""{app}\scripts\install-server.ps1"" -Mode Backup -Root ""{app}"""
Name: "{group}\เปิดโฟลเดอร์ไฟล์สำรอง"; Filename: "{app}\backups"
Name: "{commondesktop}\ระบบเข้าใช้งานคอมพิวเตอร์"; Filename: "{app}\LabAccess.url"
Name: "{commondesktop}\ตัวติดตั้งเครื่องลูก"; Filename: "{win}\explorer.exe"; Parameters: "/select,""{app}\LabAccessClientSetup.exe"""

[Dirs]
Name: "{app}\backups"; Flags: uninsneveruninstall

[Run]
Filename: "{app}\admin-login.txt"; Description: "แสดงชื่อผู้ใช้และรหัสผ่านผู้ดูแล"; Flags: shellexec postinstall skipifsilent
Filename: "{app}\LabAccess.url"; Description: "เปิดหน้าเว็บ"; Flags: shellexec postinstall skipifsilent

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
    Result := 'ติดตั้งไม่สำเร็จ (' + Operation + ') ข้อมูลเดิมยังอยู่ครบ' + #13#10 +
      'ลองรีสตาร์ตเครื่องแล้วติดตั้งใหม่ ถ้ายังไม่ได้ ส่งไฟล์นี้ให้ผู้พัฒนา:' + #13#10 + ExpandConstant('{app}\setup.log');
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
    WizardForm.StatusLabel.Caption := 'กำลังเตรียมฐานข้อมูล เว็บ และตัวติดตั้งเครื่องลูก รอสักครู่...';
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
