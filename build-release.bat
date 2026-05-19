@echo off
setlocal

cd /d "%~dp0"

echo ========================================
echo xk AI Building
echo ========================================
echo Project: %CD%
echo.

echo [1/4] Checking required tools...
where go >nul 2>nul
if errorlevel 1 (
    echo ERROR: Go was not found in PATH.
    goto failed
)

where node >nul 2>nul
if errorlevel 1 (
    echo ERROR: Node.js was not found in PATH.
    goto failed
)

where npm >nul 2>nul
if errorlevel 1 (
    echo ERROR: npm was not found in PATH.
    goto failed
)

echo [2/4] Building backend for Linux amd64...
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
set GOCACHE=%~dp0.gocache
set GOMODCACHE=%~dp0.gomodcache
go build -o "ai-getaway-linux-amd64" .
if errorlevel 1 goto failed

echo [3/4] Preparing frontend dependencies...
cd /d "%~dp0frontend"
if not exist "node_modules" (
    echo node_modules not found, running npm install...
    call npm install
    if errorlevel 1 goto failed
) else (
    echo node_modules exists, skipping npm install.
)

echo [4/4] Building frontend...
call npm run build
if errorlevel 1 goto failed

cd /d "%~dp0"
echo.
echo ========================================
echo Build completed successfully.
echo ========================================
echo Backend binary:
echo   %CD%\ai-getaway-linux-amd64
echo Frontend dist:
echo   %CD%\frontend\dist
echo.
echo Upload targets from deployment doc:
echo   ai-getaway-linux-amd64  -^> /opt/ai-getaway/ai-getaway
echo   frontend\dist           -^> /opt/ai-getaway/frontend/dist
echo.
pause
exit /b 0

:failed
cd /d "%~dp0" >nul 2>nul
echo.
echo ========================================
echo Build failed. Check the error above.
echo ========================================
pause
exit /b 1
