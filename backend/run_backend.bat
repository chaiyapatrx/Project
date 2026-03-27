@echo off
echo ===================================================
echo Starting FastAPI Backend (Accessible on LAN)
echo Please ensure your Windows Firewall allows port 8000.
echo ===================================================
call venv\Scripts\activate
uvicorn main:app --host 0.0.0.0 --port 8000 --reload
pause
