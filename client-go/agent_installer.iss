#define AppName "ระบบเข้าใช้งานคอมพิวเตอร์ (เครื่องลูก)"
#ifndef AppVersion
#define AppVersion "1.0.0"
#endif
#ifndef AgentServerURL
#define AgentServerURL ""
#endif

[Setup]
AppId={{8DC60F6B-F9F0-48A6-BF01-D0831DDA5F85}
AppName={#AppName}
AppVersion={#AppVersion}
DefaultDirName={userpf}\LabAccess Agent
DefaultGroupName=ระบบเข้าใช้งานคอมพิวเตอร์
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
MinVersion=10.0
WizardStyle=modern
OutputDir=dist
OutputBaseFilename=LabAccessAgentSetup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
CloseApplications=yes
RestartApplications=no
UninstallDisplayIcon={app}\LabAccessAgent.exe

[Files]
Source: "dist\LabAccessAgent.exe"; DestDir: "{app}"; Flags: ignoreversion; AfterInstall: WriteStationConfig
Source: "dist\LabAccessUpdater.exe"; DestDir: "{app}"; Flags: ignoreversion
#ifdef DeploymentPackage
#ifdef EmbeddedDeployment
Source: "server-ca.pem"; DestDir: "{app}"; Flags: ignoreversion
Source: "deployment.ini"; Flags: dontcopy
#else
Source: "{src}\server-ca.pem"; DestDir: "{app}"; Flags: external ignoreversion
#endif
#else
#ifexist "dist\server-ca.pem"
Source: "dist\server-ca.pem"; DestDir: "{app}"; Flags: ignoreversion
#endif
#endif

[Icons]
Name: "{group}\ระบบเข้าใช้งานคอมพิวเตอร์"; Filename: "{app}\LabAccessAgent.exe"; WorkingDir: "{app}"

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "LabAccess Agent"; ValueData: """{app}\LabAccessAgent.exe"""; Flags: uninsdeletevalue

[Run]
Filename: "{app}\LabAccessAgent.exe"; Parameters: "--enroll"; WorkingDir: "{app}"; Flags: runhidden
Filename: "{app}\LabAccessAgent.exe"; Description: "เริ่มใช้งานทันที (หน้าจอนี้จะถูกล็อก)"; WorkingDir: "{app}"; Flags: postinstall nowait skipifsilent

[UninstallDelete]
Type: files; Name: "{localappdata}\LabAccess Agent\config.json"
Type: dirifempty; Name: "{localappdata}\LabAccess Agent"

[Code]
var
  SettingsPage: TInputQueryWizardPage;
  EnrollmentToken: String;
  DeploymentIniFile: String;

#ifdef DeploymentPackage
function InitializeSetup: Boolean;
var
  URL: String;
  CAFile: String;
begin
#ifdef EmbeddedDeployment
  ExtractTemporaryFile('deployment.ini');
  ExtractTemporaryFile('server-ca.pem');
  DeploymentIniFile := ExpandConstant('{tmp}\deployment.ini');
  CAFile := ExpandConstant('{tmp}\server-ca.pem');
#else
  DeploymentIniFile := ExpandConstant('{src}\deployment.ini');
  CAFile := ExpandConstant('{src}\server-ca.pem');
#endif
  URL := GetIniString('LabAccess', 'ServerURL', '', DeploymentIniFile);
  EnrollmentToken := GetIniString('LabAccess', 'EnrollmentToken', '', DeploymentIniFile);
  Result := (Pos('wss://', LowerCase(URL)) = 1) and
    (Copy(URL, Length(URL) - 12, 13) = '/api/ws/agent') and
    (Length(EnrollmentToken) >= 32) and (Length(EnrollmentToken) <= 256) and
    FileExists(CAFile);
  if not Result then
    MsgBox('ไฟล์ติดตั้งไม่ครบ ให้คัดลอก LabAccessClientSetup.exe จากเครื่องแม่มาใหม่', mbError, MB_OK);
end;
#endif

function ConfigFilePath: String;
begin
  Result := ExpandConstant('{localappdata}\LabAccess Agent\config.json');
end;

procedure InitializeWizard;
begin
  SettingsPage := CreateInputQueryPage(wpSelectDir, 'Station settings',
    'Connect this computer to LabAccess',
    'The agent will use this Windows computer name and request approval in Admin automatically. For a remote server, use a trusted wss:// URL.');
  SettingsPage.Add('Server URL:', False);
#ifdef DeploymentPackage
  SettingsPage.Values[0] := GetIniString('LabAccess', 'ServerURL', '', DeploymentIniFile);
#else
  if not FileExists(ConfigFilePath) then
  begin
    SettingsPage.Values[0] := '{#AgentServerURL}';
    if SettingsPage.Values[0] = '' then
      SettingsPage.Values[0] := 'ws://localhost:8000/api/ws/agent';
  end;
#endif
end;

function ServerURLValid(ServerURL: String): Boolean;
var
  LowerURL: String;
begin
  LowerURL := LowerCase(Trim(ServerURL));
  Result := (Length(LowerURL) >= 13) and
            ((Pos('wss://', LowerURL) = 1) or
             (Pos('ws://localhost:', LowerURL) = 1) or
             (Pos('ws://127.0.0.1:', LowerURL) = 1) or
             (Pos('ws://[::1]:', LowerURL) = 1)) and
            (Copy(LowerURL, Length(LowerURL) - 12, 13) = '/api/ws/agent');
end;

function ShouldSkipPage(PageID: Integer): Boolean;
begin
#ifdef DeploymentPackage
  Result := PageID = SettingsPage.ID;
#else
  Result := (PageID = SettingsPage.ID) and
    (FileExists(ConfigFilePath) or
     (('{#AgentServerURL}' <> '') and ServerURLValid(SettingsPage.Values[0])));
#endif
end;

function NextButtonClick(CurPageID: Integer): Boolean;
var
  ServerURL: String;
begin
  Result := True;
  if CurPageID <> SettingsPage.ID then Exit;

  ServerURL := Trim(SettingsPage.Values[0]);

  if not ServerURLValid(ServerURL) then
  begin
    MsgBox('Enter the server URL ending in /api/ws/agent. Use wss:// for a remote server.', mbError, MB_OK);
    Result := False;
  end;
end;

function JsonEscape(const Value: String): String;
var
  I: Integer;
  C: String;
begin
  Result := '';
  for I := 1 to Length(Value) do
  begin
    C := Copy(Value, I, 1);
    if C = '\' then Result := Result + '\\'
    else if C = '"' then Result := Result + '\"'
    else if C = #10 then Result := Result + '\n'
    else if C = #13 then Result := Result + '\r'
    else if C = #9 then Result := Result + '\t'
    else if Ord(Value[I]) < 32 then RaiseException('Invalid control character in station settings')
    else Result := Result + C;
  end;
end;

procedure WriteStationConfig;
var
  ConfigDir: String;
  CAFile: String;
  Lines: TArrayOfString;
begin
  if FileExists(ConfigFilePath) then Exit;

  ConfigDir := ExpandConstant('{localappdata}\LabAccess Agent');
  if not ForceDirectories(ConfigDir) then
    RaiseException('Could not create the private station settings folder');

#ifdef DeploymentPackage
  CAFile := ExpandConstant('{app}\server-ca.pem');
#else
#ifexist "dist\server-ca.pem"
  CAFile := ExpandConstant('{app}\server-ca.pem');
#else
  CAFile := '';
#endif
#endif
  SetArrayLength(Lines, 7);
  Lines[0] := '{';
  Lines[1] := '  "server_url": "' + JsonEscape(Trim(SettingsPage.Values[0])) + '",';
  Lines[2] := '  "server_ca_file": "' + JsonEscape(CAFile) + '",';
  Lines[3] := '  "ping_interval_seconds": 15,';
  Lines[4] := '  "auto_enroll": true,';
  Lines[5] := '  "enrollment_token": "' + JsonEscape(EnrollmentToken) + '"';
  Lines[6] := '}';
  if not SaveStringsToUTF8FileWithoutBOM(ConfigFilePath, Lines, False) then
    RaiseException('Could not save the private station settings');
end;

#ifdef DeploymentPackage
// Server name first, then the server's LAN IPs: the first one this computer reaches wins,
// so nobody has to type an address on networks where the server name does not resolve.
function PickServerURL(Candidates: String): String;
var
  OutFile: String;
  Picked: AnsiString;
  Code: Integer;
begin
  Result := GetIniString('LabAccess', 'ServerURL', '', DeploymentIniFile);
  OutFile := ExpandConstant('{tmp}\server-url.txt');
  if Exec(ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -ExecutionPolicy Bypass -Command "foreach ($x in (''' + Candidates + ''' -split '' '')) { ' +
    '$c = New-Object Net.Sockets.TcpClient; try { $u = [uri]$x; ' +
    'if ($c.ConnectAsync($u.Host, $u.Port).Wait(3000) -and $c.Connected) { Set-Content -LiteralPath ''' + OutFile + ''' -Value $x; break } } ' +
    'catch {} finally { $c.Dispose() } }"',
    '', SW_HIDE, ewWaitUntilTerminated, Code) and LoadStringFromFile(OutFile, Picked) and
    (Pos(' ' + Trim(String(Picked)) + ' ', ' ' + Candidates + ' ') > 0) and ServerURLValid(Trim(String(Picked))) then
    Result := Trim(String(Picked));
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  Existing: AnsiString;
  Candidates: String;
  ExistingURL: String;
  Start: Integer;
begin
  Result := '';
  Candidates := Trim(GetIniString('LabAccess', 'ServerURL', '', DeploymentIniFile) + ' ' +
    GetIniString('LabAccess', 'AlternateURLs', '', DeploymentIniFile));
  if FileExists(ConfigFilePath) then
  begin
    ExistingURL := '';
    if LoadStringFromFile(ConfigFilePath, Existing) then
    begin
      Start := Pos('"server_url": "', String(Existing));
      if Start > 0 then
      begin
        ExistingURL := Copy(String(Existing), Start + 15, Length(String(Existing)));
        ExistingURL := Copy(ExistingURL, 1, Pos('"', ExistingURL) - 1);
      end;
    end;
    if (ExistingURL = '') or (Pos(' ' + ExistingURL + ' ', ' ' + Candidates + ' ') = 0) then
      Result := 'เครื่องนี้ต่อกับเครื่องแม่เครื่องอื่นอยู่ ให้ถอนการติดตั้งของเดิมใน Settings > Apps ก่อน แล้วติดตั้งใหม่';
  end
  else
    SettingsPage.Values[0] := PickServerURL(Candidates);
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  Code: Integer;
begin
  if CurStep = ssPostInstall then
    if not Exec(ExpandConstant('{sys}\certutil.exe'),
      '-user -addstore Root "' + ExpandConstant('{app}\server-ca.pem') + '"',
      '', SW_HIDE, ewWaitUntilTerminated, Code) or (Code <> 0) then
      RaiseException('ติดตั้งใบรับรองความปลอดภัยไม่สำเร็จ ลองติดตั้งใหม่อีกครั้ง');
end;
#endif
