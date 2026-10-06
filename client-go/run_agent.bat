@echo off
setlocal
cd /d "%~dp0"
set "GOOS=windows"
set "GOARCH=amd64"
set "CGO_ENABLED=0"
set /p APP_VERSION=<VERSION
go build -ldflags="-s -w -H=windowsgui -X main.agentVersion=%APP_VERSION%" -o AUCCAgent.exe .
if errorlevel 1 (
    pause
    exit /b 1
)
go build -ldflags="-s -w -H=windowsgui" -o AUCCUpdater.exe ./updater
if errorlevel 1 (
    pause
    exit /b 1
)
start "" "%~dp0AUCCAgent.exe"
