@echo off
cd /d "%~dp0"
echo ===================================================
echo Building Standalone Windows Client Agent (.exe)
echo ===================================================
go mod tidy
if errorlevel 1 goto :error
go build -ldflags="-s -w" -o agent.exe .
if errorlevel 1 goto :error
echo.
if exist agent.exe (
    echo [SUCCESS] agent.exe built successfully!
    dir agent.exe | findstr agent.exe
) else (
    goto :error
)
pause
exit /b 0

:error
echo [ERROR] Build failed.
pause
exit /b 1
