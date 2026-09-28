#define AppName "AUCC Agent"
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
DefaultDirName={userpf}\AUCC Agent
DefaultGroupName=AUCC Agent
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
MinVersion=10.0
WizardStyle=modern
OutputDir=dist
OutputBaseFilename=AUCCAgentSetup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
CloseApplications=yes
RestartApplications=no
UninstallDisplayIcon={app}\AUCCAgent.exe

[Tasks]
Name: "autostart"; Description: "Start AUCC Agent when I sign in to Windows"

[Files]
Source: "dist\AUCCAgent.exe"; DestDir: "{app}"; Flags: ignoreversion; AfterInstall: WriteStationConfig
Source: "dist\AUCCUpdater.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\AUCC Agent"; Filename: "{app}\AUCCAgent.exe"; WorkingDir: "{app}"
Name: "{userstartup}\AUCC Agent"; Filename: "{app}\AUCCAgent.exe"; WorkingDir: "{app}"; Tasks: autostart; Flags: runminimized

[Run]
Filename: "{app}\AUCCAgent.exe"; Parameters: "--enroll"; WorkingDir: "{app}"; Flags: runhidden
Filename: "{app}\AUCCAgent.exe"; Description: "Start AUCC Agent now (locks this Windows session)"; WorkingDir: "{app}"; Flags: postinstall nowait skipifsilent unchecked

[UninstallDelete]
Type: files; Name: "{localappdata}\AUCC Agent\config.json"
Type: dirifempty; Name: "{localappdata}\AUCC Agent"

[Code]
var
  SettingsPage: TInputQueryWizardPage;

function ConfigFilePath: String;
begin
  Result := ExpandConstant('{localappdata}\AUCC Agent\config.json');
end;

procedure InitializeWizard;
begin
  SettingsPage := CreateInputQueryPage(wpSelectDir, 'Station settings',
    'Connect this computer to AUCC',
    'The agent will use this Windows computer name and request approval in Admin automatically. For a remote server, use a trusted wss:// URL.');
  SettingsPage.Add('Server URL:', False);
  if not FileExists(ConfigFilePath) then
  begin
    SettingsPage.Values[0] := '{#AgentServerURL}';
    if SettingsPage.Values[0] = '' then
      SettingsPage.Values[0] := 'ws://localhost:8000/api/ws/agent';
  end;
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
  Result := (PageID = SettingsPage.ID) and
    (FileExists(ConfigFilePath) or
     (('{#AgentServerURL}' <> '') and ServerURLValid(SettingsPage.Values[0])));
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
  Lines: TArrayOfString;
begin
  if FileExists(ConfigFilePath) then Exit;

  ConfigDir := ExpandConstant('{localappdata}\AUCC Agent');
  if not ForceDirectories(ConfigDir) then
    RaiseException('Could not create the private station settings folder');

  SetArrayLength(Lines, 5);
  Lines[0] := '{';
  Lines[1] := '  "server_url": "' + JsonEscape(Trim(SettingsPage.Values[0])) + '",';
  Lines[2] := '  "ping_interval_seconds": 15,';
  Lines[3] := '  "auto_enroll": true';
  Lines[4] := '}';
  if not SaveStringsToUTF8FileWithoutBOM(ConfigFilePath, Lines, False) then
    RaiseException('Could not save the private station settings');
end;
