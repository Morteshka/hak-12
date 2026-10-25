@echo off
chcp 65001 >nul
cd /d "%~dp0"
if not exist .env copy .env.example .env
docker info >nul 2>&1 || (echo Docker Desktop ne zapushen. Zapustite ego i povtorite. & pause & exit /b 1)
docker compose up -d --build > build.log 2>&1
if errorlevel 1 (type build.log & echo. & echo === Oshibka sborki. Tekst sohranen v build.log - otpravte ego Claude === & pause & exit /b 1)
docker compose ps
echo.
echo Gotovo: http://localhost:3000  (admin / change-me-admin, user / user123)
start http://localhost:3000
pause
