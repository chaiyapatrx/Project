@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0backup-db.ps1" %*
pause
