@echo off
setlocal
cd /d "%~dp0"
set "GOOS=windows"
set "GOARCH=amd64"
set "CGO_ENABLED=0"
echo ===================================================
echo Building Standalone Windows Client Agent (.exe)
echo ===================================================
set /p APP_VERSION=<VERSION
go build -ldflags="-s -w -H=windowsgui -X main.agentVersion=%APP_VERSION%" -o AUCCAgent.exe .
if errorlevel 1 goto :error
go build -ldflags="-s -w -H=windowsgui" -o AUCCUpdater.exe ./updater
if errorlevel 1 goto :error
echo.
if exist AUCCAgent.exe (
    echo [SUCCESS] AUCCAgent.exe and AUCCUpdater.exe built successfully!
    dir AUCCAgent.exe | findstr AUCCAgent.exe
) else (
    goto :error
)
pause
exit /b 0

:error
echo [ERROR] Build failed.
pause
exit /b 1
