@echo off
setlocal
cd /d "%~dp0"

set "GOOS=windows"
set "GOARCH=amd64"
set "CGO_ENABLED=0"
set /p APP_VERSION=<VERSION
if not exist "dist" mkdir "dist"
go build -trimpath -ldflags="-s -w -X main.agentVersion=%APP_VERSION%" -o "dist\AUCCAgent.exe" .
if errorlevel 1 exit /b 1
go build -trimpath -ldflags="-s -w -H=windowsgui" -o "dist\AUCCUpdater.exe" ./updater
if errorlevel 1 exit /b 1

set "ISCC=%LOCALAPPDATA%\Programs\Inno Setup 6\ISCC.exe"
if not exist "%ISCC%" set "ISCC=%LOCALAPPDATA%\Programs\Inno Setup 7\ISCC.exe"
if not exist "%ISCC%" set "ISCC=%ProgramFiles(x86)%\Inno Setup 6\ISCC.exe"
if not exist "%ISCC%" set "ISCC=%ProgramFiles%\Inno Setup 7\ISCC.exe"
if not exist "%ISCC%" (
    echo Inno Setup compiler not found. Install Inno Setup and run this script again.
    exit /b 1
)

if defined AUCC_SERVER_URL (
    "%ISCC%" /Qp /DAppVersion=%APP_VERSION% "/DAgentServerURL=%AUCC_SERVER_URL%" "agent_installer.iss"
) else (
    "%ISCC%" /Qp /DAppVersion=%APP_VERSION% "agent_installer.iss"
)
if errorlevel 1 exit /b 1
echo Installer ready: dist\AUCCAgentSetup-%APP_VERSION%.exe
