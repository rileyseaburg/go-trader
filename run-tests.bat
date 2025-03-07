@echo off
REM Batch script to run E2E tests for the Algorithmic Trading System on Windows

echo Starting Go backend...
start /b cmd /c "go run main.go"
echo Backend started.

echo Starting Next.js frontend...
cd web-ui
start /b cmd /c "npm run dev"
echo Frontend started.

echo Waiting for services to be ready...
ping -n 11 127.0.0.1 > nul

echo Running API integration tests...
npm run test:api
set API_EXIT=%ERRORLEVEL%

if %API_EXIT% EQU 0 (
    echo [92m✅ API tests PASSED[0m
) else (
    echo [91m❌ API tests FAILED with exit code %API_EXIT%[0m
)

echo Running UI flow tests...
npm run test:ui
set UI_EXIT=%ERRORLEVEL%

if %UI_EXIT% EQU 0 (
    echo [92m✅ UI tests PASSED[0m
) else (
    echo [91m❌ UI tests FAILED with exit code %UI_EXIT%[0m
)

echo Tests completed.

echo -------------------------
echo TEST EXECUTION SUMMARY
echo -------------------------
if %API_EXIT% EQU 0 (
    echo API Tests: [92m✅ PASSED[0m
) else (
    echo API Tests: [91m❌ FAILED[0m
)

if %UI_EXIT% EQU 0 (
    echo UI Tests:  [92m✅ PASSED[0m
) else (
    echo UI Tests:  [91m❌ FAILED[0m
)
echo -------------------------

echo Cleaning up processes...
REM Find and terminate the node and go processes
for /f "tokens=2" %%a in ('tasklist ^| findstr "node.exe"') do (
    taskkill /F /PID %%a >nul 2>&1
)
for /f "tokens=2" %%a in ('tasklist ^| findstr "go.exe"') do (
    taskkill /F /PID %%a >nul 2>&1
)

if %API_EXIT% EQU 0 if %UI_EXIT% EQU 0 (
    echo All tests PASSED!
    exit /b 0
) else (
    echo Some tests FAILED. Check logs for details.
    exit /b 1
)
